package factory

import (
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/parser"
)

func (cf *ComponentFactory) combineSchemas(route ir.HTTPRoute, paramMap map[string]ir.ParamMapping) (ir.Schema, error) {
	schema := ir.Schema{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	var required []string

	// 处理参数（使用参数映射）
	for mcpName, mapping := range paramMap {
		if mapping.Location != "body" {
			schema["properties"].(map[string]interface{})[mcpName] = cf.getParameterSchema(route, mapping.OpenAPIName)

			if cf.isParameterRequired(route, mapping.OpenAPIName) {
				required = append(required, mcpName)
			}
		}
	}

	// 处理请求体属性
	if route.RequestBody != nil {
		contentType := parser.GetContentType(route.RequestBody.ContentSchemas)
		if contentType != "" {
			bodySchema := route.RequestBody.ContentSchemas[contentType]

			for propName, propSchema := range bodySchema.Properties() {
				schema["properties"].(map[string]interface{})[propName] = propSchema

				if route.RequestBody.Required && cf.isPropertyRequired(bodySchema, propName) {
					required = append(required, propName)
				}
			}
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	if len(route.SchemaDefs.Definitions()) > 0 {
		schema["$defs"] = route.SchemaDefs.Definitions()
	}

	return schema, nil
}

// getParameterSchema 获取参数的 schema
func (cf *ComponentFactory) getParameterSchema(route ir.HTTPRoute, paramName string) ir.Schema {
	for _, param := range route.Parameters {
		if param.Name == paramName {
			return param.Schema
		}
	}
	return nil
}

// isParameterRequired 检查参数是否必需
func (cf *ComponentFactory) isParameterRequired(route ir.HTTPRoute, paramName string) bool {
	for _, param := range route.Parameters {
		if param.Name == paramName {
			return param.Required
		}
	}
	return false
}

// isPropertyRequired 检查属性是否必需
func (cf *ComponentFactory) isPropertyRequired(schema ir.Schema, propName string) bool {
	if required, ok := schema["required"].([]string); ok {
		for _, req := range required {
			if req == propName {
				return true
			}
		}
	}
	return false
}

func (cf *ComponentFactory) extractOutputSchema(route ir.HTTPRoute) (ir.Schema, bool) {
	successStatuses := []string{"200", "201", "202", "204"}

	var responseInfo *ir.ResponseInfo
	for _, status := range successStatuses {
		if resp, ok := route.Responses[status]; ok {
			responseInfo = &resp
			break
		}
	}

	if responseInfo == nil || len(responseInfo.ContentSchemas) == 0 {
		return nil, false
	}

	contentType := parser.GetContentType(responseInfo.ContentSchemas)
	if contentType == "" {
		return nil, false
	}

	schema := responseInfo.ContentSchemas[contentType]

	wrappedSchema, wrapResult := parser.WrapNonObjectSchema(schema)

	if len(route.SchemaDefs.Definitions()) > 0 {
		wrappedSchema = parser.MergeSchemaDefinitions(wrappedSchema, route.SchemaDefs)
	}

	optimizedSchema := parser.OptimizeSchema(wrappedSchema)

	return optimizedSchema, wrapResult
}
