package forgebird

import (
	"context"
	"fmt"
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
			// 应用外部传入规则；若为空则保持底层 RouteMapper 的默认（Tool-only），与 v1.0.0 一致
			if len(config.Mapping.Rules) > 0 {
				for _, rule := range config.Mapping.Rules {
					mapper.AddRule(rule)
				}
			}
			if len(config.Mapping.GlobalTags) > 0 {
				mapper.WithGlobalTags(config.Mapping.GlobalTags...)
			}
			return mapper, nil
		},
		ToolAnnotationStrategy:      factory.NewHTTPToolAnnotationStrategy(),
		ComponentDescriptorStrategy: factory.NewHTTPComponentDescriptorStrategy(),
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
	params := extractParametersFromURI(req.Params.URI, tpl.URITemplate.Template.Raw())
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

		default:
			return fmt.Errorf("unsupported component type: %s", component.GetType())
		}
	}
	return nil
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

func extractParametersFromURI(uri string, template string) map[string]string {
	if strings.TrimSpace(template) == "" {
		return map[string]string{}
	}
	uri = strings.TrimPrefix(uri, "resource://")
	template = strings.TrimPrefix(template, "resource://")
	uriSeg := strings.Split(uri, "/")
	tplSeg := strings.Split(template, "/")
	params := make(map[string]string)
	if len(uriSeg) != len(tplSeg) {
		return params
	}
	for i, seg := range tplSeg {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			key := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			params[key] = uriSeg[i]
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
