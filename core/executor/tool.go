package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/specx2/openapi-mcp/core/internal"
	"github.com/specx2/openapi-mcp/core/ir"
)

type OpenAPITool struct {
	tool         mcp.Tool
	route        ir.HTTPRoute
	client       HTTPClient
	baseURL      string
	paramMap     map[string]ir.ParamMapping
	outputSchema ir.Schema
	wrapResult   bool
	validator    *jsonschema.Schema
	tags         []string
}

func NewOpenAPITool(
	name string,
	description string,
	inputSchema ir.Schema,
	outputSchema ir.Schema,
	wrapResult bool,
	route ir.HTTPRoute,
	client HTTPClient,
	baseURL string,
	paramMap map[string]ir.ParamMapping,
	tags []string,
	annotations *mcp.ToolAnnotation,
) *OpenAPITool {
	inputSchemaJSON, _ := json.Marshal(inputSchema)
	var outputSchemaJSON json.RawMessage
	if outputSchema != nil {
		outputSchemaJSON, _ = json.Marshal(outputSchema)
	}

	options := []mcp.ToolOption{
		mcp.WithDescription(description),
		// Custom option to set raw schema and clear structured schema
		func(t *mcp.Tool) {
			t.InputSchema.Type = ""
			t.RawInputSchema = inputSchemaJSON
		},
	}
	if outputSchema != nil {
		options = append(options, mcp.WithRawOutputSchema(outputSchemaJSON))
	}
	if derived := deriveToolAnnotations(route.Method, route.Summary, annotations); derived != nil {
		options = append(options, mcp.WithToolAnnotation(*derived))
	}

	tool := mcp.NewTool(name, options...)

	if meta := buildToolMeta(route, tags); len(meta) > 0 {
		tool.Meta = mcp.NewMetaFromMap(meta)
	}

	validator := compileJSONSchema(inputSchemaJSON)

	return &OpenAPITool{
		tool:         tool,
		route:        route,
		client:       client,
		baseURL:      baseURL,
		paramMap:     paramMap,
		outputSchema: outputSchema,
		wrapResult:   wrapResult,
		validator:    validator,
		tags:         uniqueStrings(tags),
	}
}

func (t *OpenAPITool) Tool() mcp.Tool {
	return t.tool
}

func (t *OpenAPITool) SetTool(tool mcp.Tool) {
	t.tool = tool
}

// ParameterMappings 返回公开的参数名称到 OpenAPI 参数的映射，主要用于测试和调试。
func (t *OpenAPITool) ParameterMappings() map[string]ir.ParamMapping {
	result := make(map[string]ir.ParamMapping, len(t.paramMap))
	for k, v := range t.paramMap {
		result[k] = v
	}
	return result
}

func (t *OpenAPITool) Tags() []string {
	if len(t.tags) == 0 {
		return nil
	}
	copyTags := make([]string, len(t.tags))
	copy(copyTags, t.tags)
	return copyTags
}

func (t *OpenAPITool) Run(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	errorHandler := NewErrorHandler("info")

	args, err := internal.ParseArguments(request)
	if err != nil {
		return errorHandler.HandleParseError(err), nil
	}

	log.Printf("tool %s received arguments: %v", t.tool.Name, request.Params.Arguments)

	t.normalizeArguments(args)

	if err := t.validateArgs(args); err != nil {
		return errorHandler.HandleBuildError(err), nil
	}

	builder := NewRequestBuilder(t.route, t.paramMap, t.baseURL)
	httpReq, err := builder.Build(ctx, args)
	if err != nil {
		return errorHandler.HandleBuildError(err), nil
	}

	if mcpHeaders := internal.GetMCPHeaders(ctx); mcpHeaders != nil {
		for k, v := range mcpHeaders {
			httpReq.Header.Set(k, v)
		}
	}

	// 检查是否有自定义 HTTP 客户端通过 context 传递
	var client HTTPClient = t.client
	if customClient, ok := ctx.Value("custom_http_client").(*DefaultHTTPClient); ok {
		client = customClient
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return errorHandler.HandleHTTPError(err), nil
	}

	processor := NewResponseProcessor(t.outputSchema, t.wrapResult, errorHandler)
	callResult, err := processor.Process(resp)
	if err != nil {
		log.Printf("tool %s failed to process response: %v", t.tool.Name, err)
		return nil, err
	}

	log.Printf("tool %s returning result: isError=%v structured=%v content=%v meta=%v", t.tool.Name, callResult.IsError, callResult.StructuredContent, callResult.Content, callResult.Result.Meta)
	return callResult, nil
}

func (t *OpenAPITool) validateArgs(args map[string]interface{}) error {
	if t.validator == nil {
		return nil
	}
	if err := t.validator.Validate(args); err != nil {
		return fmt.Errorf("argument validation failed: %w", err)
	}
	return nil
}

func (t *OpenAPITool) normalizeArguments(args map[string]interface{}) {
	for name, value := range args {
		if value == nil {
			continue
		}

		mapping, ok := t.paramMap[name]
		if !ok {
			continue
		}

		param := t.findRouteParameter(mapping)
		if param == nil || param.Schema == nil {
			continue
		}

		if coerced, changed := coerceValueForSchema(value, param.Schema); changed {
			args[name] = coerced
		}
	}
}

func (t *OpenAPITool) findRouteParameter(mapping ir.ParamMapping) *ir.ParameterInfo {
	for i := range t.route.Parameters {
		param := &t.route.Parameters[i]
		if param.Name == mapping.OpenAPIName && param.In == mapping.Location {
			return param
		}
	}
	return nil
}

func coerceValueForSchema(value interface{}, schema ir.Schema) (interface{}, bool) {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return value, false
		}
		if schemaAllowsType(schema, "integer") {
			if parsed, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
				return float64(parsed), true
			}
		}
		if schemaAllowsType(schema, "number") {
			if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
				return parsed, true
			}
		}
		if schemaAllowsType(schema, "boolean") {
			if parsed, err := strconv.ParseBool(trimmed); err == nil {
				return parsed, true
			}
		}
	case json.Number:
		if schemaAllowsType(schema, "integer") {
			if parsed, err := v.Int64(); err == nil {
				return float64(parsed), true
			}
		}
		if schemaAllowsType(schema, "number") {
			if parsed, err := v.Float64(); err == nil {
				return parsed, true
			}
		}
	}

	return value, false
}

func schemaAllowsType(schema ir.Schema, typ string) bool {
	if schema == nil || typ == "" {
		return false
	}

	if schema.Type() == typ {
		return true
	}

	if rawTypes, ok := schema["type"].([]interface{}); ok {
		for _, item := range rawTypes {
			if s, ok := item.(string); ok && s == typ {
				return true
			}
		}
	}

	if anyOf, ok := schema["anyOf"].([]interface{}); ok {
		for _, candidate := range anyOf {
			if m, ok := candidate.(map[string]interface{}); ok {
				if schemaAllowsType(ir.Schema(m), typ) {
					return true
				}
			}
		}
	}

	if oneOf, ok := schema["oneOf"].([]interface{}); ok {
		for _, candidate := range oneOf {
			if m, ok := candidate.(map[string]interface{}); ok {
				if schemaAllowsType(ir.Schema(m), typ) {
					return true
				}
			}
		}
	}

	if allOf, ok := schema["allOf"].([]interface{}); ok {
		for _, candidate := range allOf {
			if m, ok := candidate.(map[string]interface{}); ok {
				if schemaAllowsType(ir.Schema(m), typ) {
					return true
				}
			}
		}
	}

	return false
}

func buildToolMeta(route ir.HTTPRoute, tags []string) map[string]any {
	meta := make(map[string]any)
	openapiMeta := make(map[string]any)

	if route.OperationID != "" {
		openapiMeta["operationId"] = route.OperationID
	}
	if route.Method != "" {
		openapiMeta["method"] = route.Method
	}
	if route.Path != "" {
		openapiMeta["path"] = route.Path
	}
	if len(route.Tags) > 0 {
		openapiMeta["tags"] = uniqueStrings(route.Tags)
	}
	if len(route.Extensions) > 0 {
		openapiMeta["extensions"] = route.Extensions
	}

	if route.RequestBody != nil {
		openapiMeta["requestBody"] = summarizeRequestBody(*route.RequestBody)
	}

	if len(route.Callbacks) > 0 {
		openapiMeta["callbacks"] = summarizeCallbacks(route.Callbacks)
	}

	if len(openapiMeta) == 0 {
		return addMetaTags(meta, tags)
	}

	meta["openapi"] = openapiMeta
	return addMetaTags(meta, tags)
}

func addMetaTags(meta map[string]any, tags []string) map[string]any {
	combined := uniqueStrings(tags)
	if len(combined) == 0 {
		return meta
	}
	if meta == nil {
		meta = make(map[string]any)
	}
	if existing, ok := meta["tags"].([]string); ok {
		meta["tags"] = uniqueStrings(append(existing, combined...))
	} else {
		meta["tags"] = combined
	}
	return meta
}

func deriveToolAnnotations(method, summary string, override *mcp.ToolAnnotation) *mcp.ToolAnnotation {
	if override != nil {
		clone := *override
		return &clone
	}
	switch strings.ToUpper(method) {
	case "GET", "HEAD":
		return annotationFor(boolPtr(true), boolPtr(false), boolPtr(true), boolPtr(true), summary)
	case "PUT":
		return annotationFor(boolPtr(false), boolPtr(true), boolPtr(true), boolPtr(true), summary)
	case "DELETE":
		return annotationFor(boolPtr(false), boolPtr(true), boolPtr(true), boolPtr(true), summary)
	default:
		return nil
	}
}

func annotationFor(readOnly, destructive, idempotent, openWorld *bool, summary string) *mcp.ToolAnnotation {
	annotation := mcp.ToolAnnotation{
		ReadOnlyHint:    readOnly,
		DestructiveHint: destructive,
		IdempotentHint:  idempotent,
		OpenWorldHint:   openWorld,
	}
	if summary != "" {
		annotation.Title = summary
	}
	return &annotation
}

func boolPtr(v bool) *bool {
	value := v
	return &value
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func summarizeRequestBody(body ir.RequestBodyInfo) map[string]any {
	result := map[string]any{
		"required": body.Required,
	}
	if len(body.Extensions) > 0 {
		result["extensions"] = body.Extensions
	}
	content := make(map[string]any)
	for mediaType := range body.ContentSchemas {
		mediaMeta := make(map[string]any)
		if body.MediaDefaults != nil {
			if def, ok := body.MediaDefaults[mediaType]; ok {
				mediaMeta["default"] = def
			}
		}
		if body.MediaExamples != nil {
			if example, ok := body.MediaExamples[mediaType]; ok {
				mediaMeta["example"] = example
			}
		}
		if body.MediaExampleSets != nil {
			if examples, ok := body.MediaExampleSets[mediaType]; ok {
				mediaMeta["examples"] = examples
			}
		}
		if body.MediaExtensions != nil {
			if exts, ok := body.MediaExtensions[mediaType]; ok {
				mediaMeta["extensions"] = exts
			}
		}
		if encodings := body.Encodings[mediaType]; len(encodings) > 0 {
			mediaMeta["encodings"] = summarizeEncodings(encodings)
		}
		if len(mediaMeta) > 0 {
			content[mediaType] = mediaMeta
		}
	}
	if len(content) > 0 {
		result["content"] = content
	}
	return result
}

func summarizeCallbacks(callbacks []ir.CallbackInfo) []map[string]any {
	if len(callbacks) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(callbacks))
	for _, cb := range callbacks {
		entry := make(map[string]any)
		if cb.Name != "" {
			entry["name"] = cb.Name
		}
		if cb.Expression != "" {
			entry["expression"] = cb.Expression
		}
		if len(cb.Extensions) > 0 {
			entry["extensions"] = cb.Extensions
		}
		if len(cb.Operations) > 0 {
			ops := make([]map[string]any, 0, len(cb.Operations))
			for _, op := range cb.Operations {
				opEntry := map[string]any{"method": op.Method}
				if op.Summary != "" {
					opEntry["summary"] = op.Summary
				}
				if op.Description != "" {
					opEntry["description"] = op.Description
				}
				if op.RequestBody != nil {
					opEntry["requestBody"] = summarizeRequestBody(*op.RequestBody)
				}
				if len(op.Responses) > 0 {
					opEntry["responses"] = op.Responses
				}
				if len(op.Extensions) > 0 {
					opEntry["extensions"] = op.Extensions
				}
				ops = append(ops, opEntry)
			}
			entry["operations"] = ops
		}
		result = append(result, entry)
	}
	return result
}

func summarizeEncodings(encodings map[string]ir.EncodingInfo) map[string]any {
	result := make(map[string]any)
	for name, encoding := range encodings {
		encodingMeta := make(map[string]any)
		if encoding.ContentType != "" {
			encodingMeta["contentType"] = encoding.ContentType
		}
		if encoding.Style != "" {
			encodingMeta["style"] = encoding.Style
		}
		if encoding.Explode != nil {
			encodingMeta["explode"] = encoding.Explode
		}
		if encoding.AllowReserved {
			encodingMeta["allowReserved"] = true
		}
		if len(encoding.Headers) > 0 {
			encodingMeta["headers"] = summarizeEncodingHeaders(encoding.Headers)
		}
		if len(encoding.Extensions) > 0 {
			encodingMeta["extensions"] = encoding.Extensions
		}
		if len(encodingMeta) > 0 {
			result[name] = encodingMeta
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func summarizeEncodingHeaders(headers map[string]ir.HeaderInfo) map[string]any {
	result := make(map[string]any)
	for name, header := range headers {
		headerMeta := make(map[string]any)
		if header.Description != "" {
			headerMeta["description"] = header.Description
		}
		if header.Required {
			headerMeta["required"] = true
		}
		if header.Deprecated {
			headerMeta["deprecated"] = true
		}
		if header.AllowEmptyValue {
			headerMeta["allowEmptyValue"] = true
		}
		if header.Style != "" {
			headerMeta["style"] = header.Style
		}
		if header.Explode {
			headerMeta["explode"] = true
		}
		if header.AllowReserved {
			headerMeta["allowReserved"] = true
		}
		if header.Schema != nil {
			headerMeta["schema"] = header.Schema
		}
		if header.Example != nil {
			headerMeta["example"] = header.Example
		}
		if len(header.Examples) > 0 {
			headerMeta["examples"] = header.Examples
		}
		if len(header.Extensions) > 0 {
			headerMeta["extensions"] = header.Extensions
		}
		if len(headerMeta) > 0 {
			result[name] = headerMeta
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
