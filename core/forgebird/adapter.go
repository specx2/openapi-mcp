package forgebird

import (
	"context"
	"fmt"
	"time"

	fb "github.com/specx2/mcp-forgebird/core/interfaces"
	"github.com/specx2/mcp-forgebird/core/mapper"
	"github.com/specx2/openapi-mcp/core/executor"
	"github.com/specx2/openapi-mcp/core/ir"
	op "github.com/specx2/openapi-mcp/core/parser"
)

// NewPipeline 构建一个基于 openapi-mcp 的 Forgebird Pipeline，不改动原有实现。
// 外部仅需提供 fb.ConversionConfig 与 OpenAPI 规范字节，即可通过 Forgebird.ConvertSpec 生成组件。
func NewPipeline() fb.Pipeline {
	return fb.Pipeline{
		Name:    "openapi",
		Version: "1.0.0",
		ParserBuilder: func(cfg fb.ConversionConfig) (fb.SpecParser, error) {
			return &openapiSpecParser{cfg: cfg}, nil
		},
		ExecutorFactoryBuilder: func(cfg fb.ConversionConfig) (fb.ExecutorFactory, error) {
			return &openapiExecutorFactory{cfg: cfg}, nil
		},
		RouteMapperBuilder: func(cfg fb.ConversionConfig) (fb.RouteMapper, error) {
			return mapper.NewHTTPRouteMapper(cfg.Mapping.Rules), nil
		},
	}
}

// openapiSpecParser 适配 openapi-mcp 的解析器为 Forgebird 的 SpecParser。
type openapiSpecParser struct {
	cfg  fb.ConversionConfig
	info *fb.SpecInfo
}

func (p *openapiSpecParser) ParseSpec(spec []byte) ([]fb.Operation, error) {
	parser, err := op.NewParser(spec, op.WithSpecURL(p.cfg.Spec.SpecURL))
	if err != nil {
		return nil, err
	}
	routes, err := parser.ParseSpec(spec)
	if err != nil {
		return nil, err
	}
	// 记录基本信息
	p.info = &fb.SpecInfo{Version: parser.GetVersion(), BaseURL: p.cfg.BaseURL}

	ops := make([]fb.Operation, 0, len(routes))
	for i := range routes {
		ops = append(ops, &openapiOperation{route: routes[i]})
	}
	return ops, nil
}

func (p *openapiSpecParser) GetVersion() string {
	if p.info != nil {
		return p.info.Version
	}
	return ""
}
func (p *openapiSpecParser) Validate() error       { return nil }
func (p *openapiSpecParser) GetInfo() *fb.SpecInfo { return p.info }

// openapiOperation 将 ir.HTTPRoute 适配为 fb.Operation。
type openapiOperation struct {
	route ir.HTTPRoute
	exec  fb.OperationExecutor
}

func (o *openapiOperation) GetID() string {
	if o.route.OperationID != "" {
		return o.route.OperationID
	}
	return fmt.Sprintf("%s %s", o.route.Method, o.route.Path)
}
func (o *openapiOperation) GetName() string {
	if o.route.Summary != "" {
		return o.route.Summary
	}
	return o.GetID()
}
func (o *openapiOperation) GetDescription() string                { return o.route.Description }
func (o *openapiOperation) GetTags() []string                     { return o.route.Tags }
func (o *openapiOperation) GetExtensions() map[string]interface{} { return o.route.Extensions }
func (o *openapiOperation) GetInputSchema() fb.Schema {
	if o.route.RequestBody == nil || len(o.route.RequestBody.ContentSchemas) == 0 {
		return nil
	}
	// 优先选择 application/json
	if s, ok := o.route.RequestBody.ContentSchemas["application/json"]; ok {
		return fb.Schema(s)
	}
	for _, schema := range o.route.RequestBody.ContentSchemas {
		return fb.Schema(schema)
	}
	return nil
}
func (o *openapiOperation) GetOutputSchema() fb.Schema {
	// 找 2xx 或 default 的第一个 schema
	for _, code := range []string{"200", "201", "202", "203", "204", "default"} {
		if resp, ok := o.route.Responses[code]; ok {
			if ct, ok := resp.ContentSchemas["application/json"]; ok {
				return fb.Schema(ct)
			}
			for _, schema := range resp.ContentSchemas {
				return fb.Schema(schema)
			}
		}
	}
	for _, resp := range o.route.Responses {
		for _, schema := range resp.ContentSchemas {
			return fb.Schema(schema)
		}
	}
	return nil
}
func (o *openapiOperation) GetSchemaDefs() fb.Schema          { return fb.Schema(o.route.SchemaDefs) }
func (o *openapiOperation) GetExecutor() fb.OperationExecutor { return o.exec }
func (o *openapiOperation) GetMetadata() *fb.OperationMetadata {
	return &fb.OperationMetadata{Method: o.route.Method, Path: o.route.Path}
}

// SetExecutor 允许 Forgebird 注入执行器
func (o *openapiOperation) SetExecutor(exec fb.OperationExecutor) { o.exec = exec }

// openapiExecutorFactory 适配 openapi-mcp 执行路径为 Forgebird 的 ExecutorFactory。
type openapiExecutorFactory struct{ cfg fb.ConversionConfig }

func (f *openapiExecutorFactory) CreateExecutor(operation fb.Operation, cfg fb.ConversionConfig) (fb.OperationExecutor, error) {
	httpClient := executor.NewDefaultHTTPClient()
	if cfg.Timeout > 0 {
		httpClient.WithTimeout(time.Duration(cfg.Timeout) * time.Second)
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = ""
	}
	return &openapiExecutor{client: httpClient, baseURL: baseURL, op: operation}, nil
}
func (f *openapiExecutorFactory) GetClientType() string { return "http" }
func (f *openapiExecutorFactory) CreateClient(cfg fb.ConversionConfig) (interface{}, error) {
	return executor.NewDefaultHTTPClient(), nil
}
func (f *openapiExecutorFactory) ValidateConfig(cfg fb.ConversionConfig) error { return nil }

// openapiExecutor 使用 openapi-mcp 的 RequestBuilder 和 ResponseProcessor 执行请求。
type openapiExecutor struct {
	client  *executor.DefaultHTTPClient
	baseURL string
	op      fb.Operation
}

func (e *openapiExecutor) Execute(ctx context.Context, args map[string]interface{}) (*fb.ExecutionResult, error) {
	// 提取路由
	wrapped, ok := e.op.(*openapiOperation)
	if !ok {
		return nil, fmt.Errorf("unexpected operation type")
	}
	rb := executor.NewRequestBuilder(wrapped.route, wrapped.route.ParameterMap, e.baseURL)
	req, err := rb.Build(ctx, args)
	if err != nil {
		return nil, err
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}

	// 选择输出 schema
	output := wrapped.GetOutputSchema()
	proc := executor.NewResponseProcessor(ir.Schema(output), false, executor.NewErrorHandler("info"))
	result, err := proc.Process(resp)
	if err != nil {
		return nil, err
	}

	meta := map[string]interface{}{}
	if result.Result.Meta != nil {
		// 将 mcp.Meta 的 AdditionalFields 透传到脚手架的 Metadata 中
		for k, v := range result.Result.Meta.AdditionalFields {
			meta[k] = v
		}
	}

	return &fb.ExecutionResult{
		Data:     map[string]interface{}{"content": result.Content, "structured": result.StructuredContent},
		Metadata: meta,
		Status:   "ok",
	}, nil
}

func (e *openapiExecutor) Validate(args map[string]interface{}) error { return nil }
func (e *openapiExecutor) GetClient() interface{}                     { return e.client }
