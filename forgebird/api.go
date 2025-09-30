package forgebird

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	srv "github.com/mark3labs/mcp-go/server"
	"github.com/specx2/mcp-forgebird/core/factory"
	"github.com/specx2/mcp-forgebird/core/interfaces"
	executorpkg "github.com/specx2/openapi-mcp/core/executor"
)

// NewPipeline 暴露基于 openapi-mcp 的 Forgebird Pipeline 构造器
func NewPipeline() interfaces.Pipeline {
	return interfaces.Pipeline{
		Name:        SpecTypeOpenAPI,
		Version:     "0.1.0",
		Description: "Converts OpenAPI specifications into MCP tools and resources",
		ParserBuilder: func(config interfaces.ConversionConfig) (interfaces.SpecParser, error) {
			return NewParser(config), nil
		},
		ExecutorFactoryBuilder: func(config interfaces.ConversionConfig) (interfaces.ExecutorFactory, error) {
			return NewExecutorFactory(config), nil
		},
		RouteMapperBuilder: func(config interfaces.ConversionConfig) (interfaces.RouteMapper, error) {
			mapper := NewOpenAPIRouteMapper()

			// 如果没有外部传入规则，使用智能映射规则（所有 GET -> ResourceTemplate，其他 -> Tool）
			if len(config.Mapping.Rules) == 0 {
				// 添加智能映射规则：所有 GET 请求都生成 ResourceTemplate（因为可能有查询参数）
				smartRules := []interfaces.MappingRule{
					{
						Methods:     []string{"*"},
						PathPattern: regexp.MustCompile(".*"),
						MCPType:     interfaces.MCPTypeTool,
					},
				}
				for _, rule := range smartRules {
					mapper = mapper.AddRule(rule)
				}
			} else {
				// 应用外部传入规则
				for _, rule := range config.Mapping.Rules {
					mapper = mapper.AddRule(rule)
				}
			}

			if len(config.Mapping.GlobalTags) > 0 {
				mapper = mapper.WithGlobalTags(config.Mapping.GlobalTags...)
			}

			// 启用一对多映射：GET 请求同时生成 Tool 和 ResourceTemplate
			mapper = mapper.WithMapFunc(func(operation interfaces.Operation) (*interfaces.MappingDecision, error) {
				// 明确的一对多映射逻辑：为 GET 请求额外生成 ResourceTemplate
				if metadata := operation.GetMetadata(); metadata != nil && metadata.Method == "GET" {
					return &interfaces.MappingDecision{
						MCPType: interfaces.MCPTypeResourceTemplate,
						Tags:    operation.GetTags(),
					}, nil
				}
				return nil, nil
			})

			return mapper, nil
		},
		ToolAnnotationStrategy:      factory.NewHTTPToolAnnotationStrategy(),
		ComponentDescriptorStrategy: NewOpenAPIMCPDescriptorStrategy(), // 使用 openapi-mcp 自己的策略，支持 RFC 6570 查询参数
	}
}

// handlerConfig 仅承载“handler 级”可选信息
type handlerConfig struct {
	HTTPClient *executorpkg.DefaultHTTPClient
	BaseURL    string
	Extra      map[string]any
}

// HandlerOption：外部可扩展 handler 行为的可选项
type HandlerOption interface{ applyHandler(*handlerConfig) }

type httpClientOpt struct {
	c *executorpkg.DefaultHTTPClient
}

func (o httpClientOpt) applyHandler(cfg *handlerConfig)             { cfg.HTTPClient = o.c }
func WithHTTPClient(c *executorpkg.DefaultHTTPClient) HandlerOption { return httpClientOpt{c: c} }

type baseURLOpt struct{ u string }

func (o baseURLOpt) applyHandler(cfg *handlerConfig) { cfg.BaseURL = o.u }
func WithBaseURL(u string) HandlerOption             { return baseURLOpt{u: u} }

// WithValue: 将任意键值注入 handlerConfig.Extra，便于外部无侵入扩展
type valueOpt struct {
	k string
	v any
}

func (o valueOpt) applyHandler(cfg *handlerConfig) {
	if cfg.Extra == nil {
		cfg.Extra = make(map[string]any)
	}
	cfg.Extra[o.k] = o.v
}
func WithValue(key string, v any) HandlerOption { return valueOpt{k: key, v: v} }

// registryConfig 仅承载“注册阶段”可选信息（选择使用哪个处理器）
type registryConfig struct {
	Tool     ToolHandler
	Resource ResourceHandler
	Template TemplateHandler
}

// RegistryOption：外部可替换默认处理器
type RegistryOption interface{ applyRegistry(*registryConfig) }

type toolHandlerOpt struct{ h ToolHandler }

func (o toolHandlerOpt) applyRegistry(cfg *registryConfig) { cfg.Tool = o.h }
func WithToolHandler(h ToolHandler) RegistryOption         { return toolHandlerOpt{h: h} }

type resourceHandlerOpt struct{ h ResourceHandler }

func (o resourceHandlerOpt) applyRegistry(cfg *registryConfig) { cfg.Resource = o.h }
func WithResourceHandler(h ResourceHandler) RegistryOption     { return resourceHandlerOpt{h: h} }

type templateHandlerOpt struct{ h TemplateHandler }

func (o templateHandlerOpt) applyRegistry(cfg *registryConfig) { cfg.Template = o.h }
func WithTemplateHandler(h TemplateHandler) RegistryOption     { return templateHandlerOpt{h: h} }

// 统一的 Handler 签名（ctx, req, component, opts...）
type ToolHandler func(ctx context.Context, req mcp.CallToolRequest, component interfaces.MCPComponent, opts ...HandlerOption) (*mcp.CallToolResult, error)
type ResourceHandler func(ctx context.Context, req mcp.ReadResourceRequest, component interfaces.MCPComponent, opts ...HandlerOption) ([]mcp.ResourceContents, error)
type TemplateHandler func(ctx context.Context, req mcp.ReadResourceRequest, component interfaces.MCPComponent, opts ...HandlerOption) ([]mcp.ResourceContents, error)

// DefaultToolHandler 默认工具处理
func DefaultToolHandler(ctx context.Context, req mcp.CallToolRequest, component interfaces.MCPComponent, opts ...HandlerOption) (*mcp.CallToolResult, error) {
	// 解析 handler 配置选项
	cfg := &handlerConfig{}
	for _, opt := range opts {
		opt.applyHandler(cfg)
	}

	// 如果有自定义 HTTP 客户端，通过 context 传递
	if cfg.HTTPClient != nil {
		ctx = context.WithValue(ctx, "custom_http_client", cfg.HTTPClient)
	}

	op := component.GetOperation()
	if oa, ok := AsOpenAPIOperation(op); ok {
		if t := oa.OpenAPITool(); t != nil {
			return t.Run(ctx, req)
		}
	}
	exec := op.GetExecutor()
	if exec == nil {
		return nil, fmt.Errorf("no executor injected")
	}
	res, err := exec.Execute(ctx, req.GetArguments())
	if err != nil {
		return nil, err
	}
	return executionResultToCallToolResult(res), nil
}

// DefaultResourceHandler 默认资源读取
func DefaultResourceHandler(ctx context.Context, req mcp.ReadResourceRequest, component interfaces.MCPComponent, opts ...HandlerOption) ([]mcp.ResourceContents, error) {
	cfg := &handlerConfig{}
	for _, opt := range opts {
		opt.applyHandler(cfg)
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = executorpkg.NewDefaultHTTPClient()
	}

	res := component.GetMCPResource()
	if res == nil {
		return nil, fmt.Errorf("resource missing")
	}
	op := component.GetOperation()
	oa, ok := AsOpenAPIOperation(op)
	if !ok {
		return nil, fmt.Errorf("not an openapi operation")
	}
	reader := executorpkg.NewOpenAPIResource(res.Name, res.Description, oa.Route(), cfg.HTTPClient, cfg.BaseURL)
	text, err := reader.Read(ctx)
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{mcp.TextResourceContents{URI: res.URI, MIMEType: valueOrDefault(res.MIMEType, "application/json"), Text: text}}, nil
}

// DefaultTemplateHandler 默认资源模板读取
func DefaultTemplateHandler(ctx context.Context, req mcp.ReadResourceRequest, component interfaces.MCPComponent, opts ...HandlerOption) ([]mcp.ResourceContents, error) {
	cfg := &handlerConfig{}
	for _, opt := range opts {
		opt.applyHandler(cfg)
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = executorpkg.NewDefaultHTTPClient()
	}

	tpl := component.GetMCPResourceTemplate()
	if tpl == nil {
		return nil, fmt.Errorf("template missing")
	}
	if tpl.URITemplate == nil {
		return nil, fmt.Errorf("missing URI template")
	}
	op := component.GetOperation()
	oa, ok := AsOpenAPIOperation(op)
	if !ok {
		return nil, fmt.Errorf("not an openapi operation")
	}
	params := ExtractParametersFromURI(req.Params.URI, tpl.URITemplate.Template.Raw())
	reader := executorpkg.NewOpenAPIParameterizedResource(tpl.Name, tpl.Description, oa.Route(), cfg.HTTPClient, cfg.BaseURL, params)
	text, err := reader.Read(ctx)
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{mcp.TextResourceContents{URI: req.Params.URI, MIMEType: valueOrDefault(tpl.MIMEType, "application/json"), Text: text}}, nil
}

// RegisterComponents 将组件注册到 mcp-go；opts 同时支持 RegistryOption 与 HandlerOption
func RegisterComponents(server *srv.MCPServer, components []interfaces.MCPComponent, opts ...interface{}) error {
	rc := &registryConfig{}
	// 收集 HandlerOptions（供默认 handler 使用）
	hOpts := make([]HandlerOption, 0, len(opts))
	for _, opt := range opts {
		switch o := opt.(type) {
		case RegistryOption:
			o.applyRegistry(rc)
		case HandlerOption:
			hOpts = append(hOpts, o)
		}
	}

	for _, component := range components {
		switch component.GetType() {
		case interfaces.MCPTypeTool:
			tool := component.GetMCPTool()
			if tool == nil {
				continue
			}

			toolHandler := rc.Tool
			if toolHandler == nil {
				toolHandler = DefaultToolHandler
			}
			handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return toolHandler(ctx, req, component, hOpts...)
			}
			server.AddTool(*tool, handler)

		case interfaces.MCPTypeResource:
			res := component.GetMCPResource()
			if res == nil {
				continue
			}
			resHandler := rc.Resource
			if resHandler == nil {
				resHandler = DefaultResourceHandler
			}
			h := func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return resHandler(ctx, req, component, hOpts...)
			}
			server.AddResource(*res, h)

		case interfaces.MCPTypeResourceTemplate:
			tpl := component.GetMCPResourceTemplate()
			if tpl == nil {
				continue
			}
			tplHandler := rc.Template
			if tplHandler == nil {
				tplHandler = DefaultTemplateHandler
			}
			h := func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return tplHandler(ctx, req, component, hOpts...)
			}
			server.AddResourceTemplate(*tpl, h)

			// 同时将 ResourceTemplate 注册为 Resource，以便在 resources/list 中显示
			resourceURI := buildResourceURIFromTemplate(tpl)
			resource := mcp.Resource{
				URI:         resourceURI,
				Name:        tpl.Name,
				Description: tpl.Description,
				MIMEType:    tpl.MIMEType,
				Meta:        tpl.Meta,
			}
			// 复制 Annotations（嵌入的 Annotated 字段）
			resource.Annotated = tpl.Annotated

			// Resource handler: 与 Template handler 相同
			resourceHandler := func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return tplHandler(ctx, req, component, hOpts...)
			}
			server.AddResource(resource, resourceHandler)

		default:
			return fmt.Errorf("unsupported component type: %s", component.GetType())
		}
	}

	// 打印注册统计信息
	printRegistrationSummary(server)

	return nil
}

// printRegistrationSummary 打印注册到 server 的所有组件
func printRegistrationSummary(server *srv.MCPServer) {
	ctx := context.Background()

	log.Println("========================================")
	log.Println("=== MCP Server Registration Summary ===")
	log.Println("========================================")

	// 1. 打印 tools/list - 完整的 tool 对象
	registeredTools := server.ListTools()
	log.Printf("[Tools/List] Total: %d", len(registeredTools))
	for name, serverTool := range registeredTools {
		toolJSON, _ := json.MarshalIndent(serverTool.Tool, "", "  ")
		log.Printf("Tool [%s]:\n%s", name, string(toolJSON))
	}

	// 2. 打印 resources/list - 完整的响应 JSON
	listResourcesMessage := `{"jsonrpc": "2.0", "id": 1, "method": "resources/list"}`
	resourceResponse := server.HandleMessage(ctx, []byte(listResourcesMessage))
	if resourceResponse != nil {
		resourceJSON, _ := json.MarshalIndent(resourceResponse, "", "  ")
		log.Printf("\n[Resources/List] Response:\n%s", string(resourceJSON))
	}

	// 3. 打印 resources/templates/list - 完整的响应 JSON
	listTemplatesMessage := `{"jsonrpc": "2.0", "id": 2, "method": "resources/templates/list"}`
	templateResponse := server.HandleMessage(ctx, []byte(listTemplatesMessage))
	if templateResponse != nil {
		templateJSON, _ := json.MarshalIndent(templateResponse, "", "  ")
		log.Printf("\n[Resources/Templates/List] Response:\n%s", string(templateJSON))
	}

	log.Println("========================================")
	log.Println("=== Registration Complete ===")
	log.Println("========================================")
}

func executionResultToCallToolResult(result *interfaces.ExecutionResult) *mcp.CallToolResult {
	if result == nil {
		return &mcp.CallToolResult{}
	}
	structured := ensureStructured(result.Data)
	meta := mapToMeta(result.Metadata)
	call := &mcp.CallToolResult{StructuredContent: structured, Result: mcp.Result{Meta: meta}}
	if len(result.Headers) > 0 {
		if call.Result.Meta == nil {
			call.Result.Meta = mcp.NewMetaFromMap(map[string]interface{}{"headers": result.Headers})
		} else {
			call.Result.Meta.AdditionalFields["headers"] = result.Headers
		}
	}
	if result.Error != nil {
		call.IsError = true
		reason := result.Error.Message
		if strings.TrimSpace(reason) == "" {
			reason = "operation execution failed"
		}
		call.Content = []mcp.Content{mcp.NewTextContent(reason)}
		if result.Error.Details != nil {
			if structured == nil {
				structured = map[string]interface{}{"error": result.Error.Details}
			} else {
				structured["error"] = result.Error.Details
			}
			call.StructuredContent = structured
		}
	}
	return call
}

func ensureStructured(data interface{}) map[string]interface{} {
	if data == nil {
		return map[string]interface{}{}
	}
	if m, ok := data.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{"result": data}
}

func mapToMeta(fields map[string]interface{}) *mcp.Meta {
	if len(fields) == 0 {
		return nil
	}
	return mcp.NewMetaFromMap(fields)
}

func ExtractParametersFromURI(uri string, template string) map[string]string {
	if strings.TrimSpace(template) == "" {
		return map[string]string{}
	}

	uri = strings.TrimPrefix(uri, "resource://")
	template = strings.TrimPrefix(template, "resource://")

	params := make(map[string]string)

	// 分离路径和查询字符串部分
	var uriPath, uriQuery string
	if queryIndex := strings.Index(uri, "?"); queryIndex != -1 {
		uriPath = uri[:queryIndex]
		uriQuery = uri[queryIndex+1:]
	} else {
		uriPath = uri
	}

	// 处理模板：可能是 RFC 6570 格式 (path{?param1,param2}) 或旧格式 (path?param1={param1})
	var tplPath string
	var expectedParams []string

	// 检查是否是 RFC 6570 格式：{?param1,param2}
	if idx := strings.Index(template, "{?"); idx != -1 {
		tplPath = template[:idx]
		// 提取参数名列表
		endIdx := strings.Index(template[idx:], "}")
		if endIdx != -1 {
			paramList := template[idx+2 : idx+endIdx]
			expectedParams = strings.Split(paramList, ",")
		}
	} else {
		// 旧格式或没有查询参数
		if queryIndex := strings.Index(template, "?"); queryIndex != -1 {
			tplPath = template[:queryIndex]
		} else {
			tplPath = template
		}
	}

	// 处理路径参数（路径中的 {id} 这种）
	uriSeg := strings.Split(uriPath, "/")
	tplSeg := strings.Split(tplPath, "/")
	if len(uriSeg) == len(tplSeg) {
		for i, seg := range tplSeg {
			if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
				key := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
				params[key] = uriSeg[i]
			}
		}
	}

	// 处理查询参数
	if uriQuery != "" {
		// 解析实际 URI 的查询参数
		queryParams := make(map[string]string)
		for _, param := range strings.Split(uriQuery, "&") {
			if equalsIndex := strings.Index(param, "="); equalsIndex != -1 {
				name := param[:equalsIndex]
				value := param[equalsIndex+1:]
				queryParams[name] = value
			}
		}

		// 如果有期望的参数列表（RFC 6570 格式），则匹配
		if len(expectedParams) > 0 {
			for _, expectedParam := range expectedParams {
				expectedParam = strings.TrimSpace(expectedParam)
				if value, exists := queryParams[expectedParam]; exists {
					// URL 解码参数值
					decodedValue, err := url.QueryUnescape(value)
					if err != nil {
						// 如果解码失败，使用原始值
						decodedValue = value
					}

					// 检查是否是 Header 参数
					if strings.HasPrefix(expectedParam, "__header__") {
						actualParamName := strings.TrimPrefix(expectedParam, "__header__")
						params[actualParamName] = decodedValue
					} else {
						params[expectedParam] = decodedValue
					}
				}
			}
		} else {
			// 旧格式或直接使用所有查询参数
			for name, value := range queryParams {
				// URL 解码参数值
				decodedValue, err := url.QueryUnescape(value)
				if err != nil {
					// 如果解码失败，使用原始值
					decodedValue = value
				}

				if strings.HasPrefix(name, "__header__") {
					actualParamName := strings.TrimPrefix(name, "__header__")
					params[actualParamName] = decodedValue
				} else {
					params[name] = decodedValue
				}
			}
		}
	}

	return params
}

func valueOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// buildResourceURIFromTemplate 从 ResourceTemplate 构建固定的 Resource URI
func buildResourceURIFromTemplate(tpl *mcp.ResourceTemplate) string {
	if tpl == nil || tpl.URITemplate == nil {
		log.Printf("[buildResourceURIFromTemplate] tpl or URITemplate is nil")
		return "resource://unknown"
	}

	// 使用模板的原始字符串作为 URI，保留查询参数部分
	// 例如: "users{?page,limit}" -> "resource://users{?page,limit}"
	//      "users/{id}" -> "resource://users/{id}"
	templateStr := tpl.URITemplate.Template.Raw()
	log.Printf("[buildResourceURIFromTemplate] Template.Raw() returned: %s", templateStr)

	if templateStr == "" {
		log.Printf("[buildResourceURIFromTemplate] Template is empty, using name: %s", tpl.Name)
		templateStr = tpl.Name
	}

	// 如果已经有 resource:// 前缀，直接使用
	if strings.HasPrefix(templateStr, "resource://") {
		log.Printf("[buildResourceURIFromTemplate] Already has resource:// prefix, returning: %s", templateStr)
		return templateStr
	}

	// 否则添加 resource:// 前缀，保留完整的模板字符串
	result := fmt.Sprintf("resource://%s", strings.TrimPrefix(templateStr, "/"))
	log.Printf("[buildResourceURIFromTemplate] Final URI: %s", result)

	// 检查是否保留了查询参数
	if strings.Contains(templateStr, "{?") {
		log.Printf("[buildResourceURIFromTemplate] ✓ Query parameters preserved in URI")
	} else {
		log.Printf("[buildResourceURIFromTemplate] ⚠️ No query parameters in template (may be expected)")
	}

	return result
}
