package forgebird

import (
	"fmt"
	"sort"
	"strings"

	"github.com/specx2/mcp-forgebird/core/interfaces"
	"github.com/specx2/openapi-mcp/core/executor"
	"github.com/specx2/openapi-mcp/core/ir"
)

// OpenAPIOperation exposes additional OpenAPI-specific helpers for operations produced by this plugin.
type OpenAPIOperation interface {
	interfaces.Operation
	Route() ir.HTTPRoute
	ParameterMap() map[string]ir.ParamMapping
	OpenAPITool() *executor.OpenAPITool
}

type openapiOperation struct {
	id           string
	name         string
	description  string
	tags         []string
	extensions   map[string]interface{}
	inputSchema  interfaces.Schema
	outputSchema interfaces.Schema
	schemaDefs   interfaces.Schema
	metadata     *interfaces.OperationMetadata
	route        ir.HTTPRoute
	paramMap     map[string]ir.ParamMapping
	executor     interfaces.OperationExecutor
}

func newOpenAPIOperation(route ir.HTTPRoute, input, output, defs interfaces.Schema, paramMap map[string]ir.ParamMapping) *openapiOperation {
	id := deriveOperationID(route)
	name := deriveOperationName(route)
	description := buildRouteDescription(route)
	metadata := buildOperationMetadata(route)
	return &openapiOperation{
		id:           id,
		name:         name,
		description:  description,
		tags:         append([]string(nil), route.Tags...),
		extensions:   cloneGenericMap(route.Extensions),
		inputSchema:  cloneSchema(input),
		outputSchema: cloneSchema(output),
		schemaDefs:   cloneSchema(defs),
		metadata:     metadata,
		route:        route,
		paramMap:     cloneParamMap(paramMap),
	}
}

func (o *openapiOperation) GetID() string {
	return o.id
}

func (o *openapiOperation) GetName() string {
	return o.name
}

func (o *openapiOperation) GetDescription() string {
	return o.description
}

func (o *openapiOperation) GetTags() []string {
	return append([]string(nil), o.tags...)
}

func (o *openapiOperation) GetExtensions() map[string]interface{} {
	return cloneGenericMap(o.extensions)
}

func (o *openapiOperation) GetInputSchema() interfaces.Schema {
	return cloneSchema(o.inputSchema)
}

func (o *openapiOperation) GetOutputSchema() interfaces.Schema {
	return cloneSchema(o.outputSchema)
}

func (o *openapiOperation) GetSchemaDefs() interfaces.Schema {
	return cloneSchema(o.schemaDefs)
}

func (o *openapiOperation) GetExecutor() interfaces.OperationExecutor {
	return o.executor
}

// SetExecutor stores the executor produced by the plugin's executor factory.
func (o *openapiOperation) SetExecutor(exec interfaces.OperationExecutor) {
	o.executor = exec
}

// OpenAPITool returns the underlying OpenAPITool if the executor supports it.
func (o *openapiOperation) OpenAPITool() *executor.OpenAPITool {
	if exec, ok := o.executor.(*openapiOperationExecutor); ok {
		return exec.tool
	}
	return nil
}

func (o *openapiOperation) setExecutor(exec interfaces.OperationExecutor) {
	o.executor = exec
}

func (o *openapiOperation) GetMetadata() *interfaces.OperationMetadata {
	if o.metadata == nil {
		return nil
	}
	copy := *o.metadata
	copy.Examples = append([]interfaces.Example(nil), copy.Examples...)
	return &copy
}

// GetHTTPRequestDescriptor 实现 interfaces.HTTPOperation 接口
func (o *openapiOperation) GetHTTPRequestDescriptor() *interfaces.HTTPRequestDescriptor {
	// 将 ir.HTTPRoute 转换为 interfaces.HTTPRequestDescriptor
	descriptor := &interfaces.HTTPRequestDescriptor{
		Path:        o.route.Path,
		Method:      o.route.Method,
		OperationID: o.route.OperationID,
		Summary:     o.route.Summary,
		Description: o.route.Description,
		Tags:        append([]string(nil), o.route.Tags...),
		Parameters:  make([]interfaces.ParameterInfo, len(o.route.Parameters)),
		SchemaDefs:  cloneSchema(o.schemaDefs),
		Extensions:  cloneGenericMap(o.route.Extensions),
	}

	// 转换参数
	for i, param := range o.route.Parameters {
		descriptor.Parameters[i] = interfaces.ParameterInfo{
			Name:        param.Name,
			In:          interfaces.ParameterLocation(param.In),
			Required:    param.Required,
			Schema:      convertIRSchemaToInterfacesSchema(param.Schema),
			Description: param.Description,
			Example:     param.Example,
		}
	}

	// 转换 RequestBody
	if o.route.RequestBody != nil {
		descriptor.RequestBody = &interfaces.RequestBodyInfo{
			Required:       o.route.RequestBody.Required,
			ContentSchemas: make(map[string]interfaces.Schema),
			Description:    o.route.RequestBody.Description,
		}
		for ct, schema := range o.route.RequestBody.ContentSchemas {
			descriptor.RequestBody.ContentSchemas[ct] = convertIRSchemaToInterfacesSchema(schema)
		}
	}

	// 转换 Responses
	descriptor.Responses = make(map[string]interfaces.ResponseInfo)
	for status, resp := range o.route.Responses {
		responseInfo := interfaces.ResponseInfo{
			Description:    resp.Description,
			ContentSchemas: make(map[string]interfaces.Schema),
		}
		for ct, schema := range resp.ContentSchemas {
			responseInfo.ContentSchemas[ct] = convertIRSchemaToInterfacesSchema(schema)
		}
		descriptor.Responses[status] = responseInfo
	}

	return descriptor
}

// GetParameterMappings 实现 interfaces.HTTPOperation 接口
func (o *openapiOperation) GetParameterMappings() map[string]interfaces.ParamMapping {
	if len(o.paramMap) == 0 {
		return nil
	}
	result := make(map[string]interfaces.ParamMapping, len(o.paramMap))
	for k, v := range o.paramMap {
		result[k] = interfaces.ParamMapping{
			OpenAPIName:  v.OpenAPIName,
			Location:     interfaces.ParameterLocation(v.Location),
			IsSuffixed:   v.IsSuffixed,
			OriginalName: v.OriginalName,
		}
	}
	return result
}

func deriveOperationID(route ir.HTTPRoute) string {
	if route.OperationID != "" {
		return route.OperationID
	}
	return strings.ToLower(route.Method) + " " + route.Path
}

func deriveOperationName(route ir.HTTPRoute) string {
	if route.Summary != "" {
		return route.Summary
	}
	if route.OperationID != "" {
		return route.OperationID
	}
	return strings.ToLower(route.Method) + " " + route.Path
}

func buildOperationMetadata(route ir.HTTPRoute) *interfaces.OperationMetadata {
	metadata := &interfaces.OperationMetadata{
		Method: route.Method,
		Path:   route.Path,
	}
	switch strings.ToUpper(route.Method) {
	case "GET", "HEAD", "OPTIONS":
		metadata.IsReadOnly = true
	}
	switch strings.ToUpper(route.Method) {
	case "GET", "HEAD", "PUT", "DELETE", "OPTIONS":
		metadata.IsIdempotent = true
	}
	if deprecated, ok := route.Extensions["deprecated"].(bool); ok {
		metadata.Deprecated = deprecated
	}
	return metadata
}

func buildRouteDescription(route ir.HTTPRoute) string {
	var parts []string

	if desc := strings.TrimSpace(route.Description); desc != "" {
		parts = append(parts, desc)
	} else if summary := strings.TrimSpace(route.Summary); summary != "" {
		parts = append(parts, summary)
	} else {
		parts = append(parts, fmt.Sprintf("%s %s", strings.ToUpper(route.Method), route.Path))
	}

	if section := formatParameterSection(route.Parameters); section != "" {
		parts = append(parts, section)
	}

	if rb := route.RequestBody; rb != nil {
		if rb.Description != "" {
			parts = append(parts, "**Request Body:** "+rb.Description)
		}
		if summary := summarizeRequestBody(*rb); summary != "" {
			parts = append(parts, "**Request Body Schema:** "+summary)
		}
	}

	if len(route.Responses) > 0 {
		if section := formatResponseSection(route); section != "" {
			parts = append(parts, section)
		}
	}

	if len(route.Tags) > 0 {
		parts = append(parts, "**Tags:** "+strings.Join(route.Tags, ", "))
	}

	return strings.Join(parts, "\n\n")
}

func formatParameterSection(params []ir.ParameterInfo) string {
	if len(params) == 0 {
		return ""
	}

	grouped := map[string][]string{}
	for _, param := range params {
		line := formatParameterLine(param)
		if line == "" {
			continue
		}
		grouped[param.In] = append(grouped[param.In], line)
	}

	var sections []string
	for _, key := range []string{ir.ParameterInPath, ir.ParameterInQuery, ir.ParameterInHeader, ir.ParameterInCookie} {
		if lines := grouped[key]; len(lines) > 0 {
			sections = append(sections, fmt.Sprintf("**%s Parameters:**\n%s", strings.Title(key), strings.Join(lines, "\n")))
		}
	}

	return strings.Join(sections, "\n\n")
}

func formatParameterLine(param ir.ParameterInfo) string {
	if param.Name == "" {
		return ""
	}

	label := fmt.Sprintf("- %s", param.Name)
	if param.Required {
		label += " (required)"
	}

	var details []string
	if desc := strings.TrimSpace(param.Description); desc != "" {
		details = append(details, desc)
	}

	if summary, schemaType := summarizeSchema(param.Schema); summary != "" {
		details = append(details, summary)
	} else if schemaType != "" {
		details = append(details, "type: "+schemaType)
	}

	if param.Schema != nil {
		if def, ok := param.Schema["default"]; ok {
			if formatted := formatExample(def); formatted != "" {
				details = append(details, "default: "+formatted)
			}
		}
	}

	if param.Example != nil {
		if formatted := formatExample(param.Example); formatted != "" {
			details = append(details, "example: "+formatted)
		}
	}

	if len(details) > 0 {
		label += ": " + strings.Join(details, "; ")
	}

	return label
}

func formatResponseSection(route ir.HTTPRoute) string {
	if len(route.Responses) == 0 {
		return ""
	}

	statuses := make([]string, 0, len(route.Responses))
	for status := range route.Responses {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)

	var lines []string
	for _, status := range statuses {
		resp := route.Responses[status]
		line := fmt.Sprintf("- %s", status)
		if desc := strings.TrimSpace(resp.Description); desc != "" {
			line += ": " + desc
		}
		if len(resp.ContentSchemas) > 0 {
			line += " (" + summarizeContentTypes(resp.ContentSchemas) + ")"
		}
		lines = append(lines, line)
	}

	return "**Responses:**\n" + strings.Join(lines, "\n")
}

func summarizeContentTypes(content map[string]ir.Schema) string {
	if len(content) == 0 {
		return ""
	}
	types := make([]string, 0, len(content))
	for media := range content {
		types = append(types, media)
	}
	sort.Strings(types)
	return strings.Join(types, ", ")
}

func summarizeRequestBody(body ir.RequestBodyInfo) string {
	if len(body.ContentSchemas) == 0 {
		return ""
	}
	types := make([]string, 0, len(body.ContentSchemas))
	for media := range body.ContentSchemas {
		types = append(types, media)
	}
	sort.Strings(types)
	return strings.Join(types, ", ")
}

func summarizeSchema(schema ir.Schema) (summary string, schemaType string) {
	if schema == nil {
		return "", ""
	}

	if t, ok := schema["type"].(string); ok {
		schemaType = t
	}

	if format, ok := schema["format"].(string); ok && format != "" {
		summary = fmt.Sprintf("type: %s (%s)", schemaType, format)
	} else if schemaType != "" {
		summary = "type: " + schemaType
	}

	return summary, schemaType
}

func formatExample(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

func cloneSchema(schema interfaces.Schema) interfaces.Schema {
	if schema == nil {
		return nil
	}
	clone := make(interfaces.Schema, len(schema))
	for k, v := range schema {
		clone[k] = cloneGenericValue(v)
	}
	return clone
}

// convertIRSchemaToInterfacesSchema 将 ir.Schema 转换为 interfaces.Schema
func convertIRSchemaToInterfacesSchema(irSchema ir.Schema) interfaces.Schema {
	if irSchema == nil {
		return nil
	}
	result := make(interfaces.Schema, len(irSchema))
	for k, v := range irSchema {
		result[k] = cloneGenericValue(v)
	}
	return result
}

func cloneParamMap(paramMap map[string]ir.ParamMapping) map[string]ir.ParamMapping {
	if len(paramMap) == 0 {
		return nil
	}
	clone := make(map[string]ir.ParamMapping, len(paramMap))
	for k, v := range paramMap {
		clone[k] = v
	}
	return clone
}

func (o *openapiOperation) Route() ir.HTTPRoute {
	return o.route
}

func (o *openapiOperation) ParameterMap() map[string]ir.ParamMapping {
	return cloneParamMap(o.paramMap)
}

// AsOpenAPIOperation tries to cast a generic operation to the OpenAPI-specific implementation.
func AsOpenAPIOperation(op interfaces.Operation) (OpenAPIOperation, bool) {
	if typed, ok := op.(*openapiOperation); ok {
		return typed, true
	}
	return nil, false
}
