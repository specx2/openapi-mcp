package factory

import (
	"fmt"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/parser"
)

func (cf *ComponentFactory) combineSchemas(route ir.HTTPRoute) (ir.Schema, map[string]ir.ParamMapping, error) {
	schema := ir.Schema{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	var required []string
	paramMap := make(map[string]ir.ParamMapping)
	allNames := make(map[string]string)

	for _, param := range route.Parameters {
		allNames[param.Name] = param.In
	}

	bodyProps := make(map[string]bool)
	if route.RequestBody != nil {
		contentType := parser.GetContentType(route.RequestBody.ContentSchemas)
		if contentType != "" {
			bodySchema := route.RequestBody.ContentSchemas[contentType]
			for propName := range bodySchema.Properties() {
				bodyProps[propName] = true
			}
		}
	}

	for _, param := range route.Parameters {
		paramName := param.Name

		if bodyProps[param.Name] {
			paramName = fmt.Sprintf("%s__%s", param.Name, param.In)
		}

		schema["properties"].(map[string]interface{})[paramName] = param.Schema

		if param.Required {
			required = append(required, paramName)
		}

		paramMap[paramName] = ir.ParamMapping{
			OpenAPIName: param.Name,
			Location:    param.In,
			IsSuffixed:  paramName != param.Name,
		}
	}

	if route.RequestBody != nil {
		contentType := parser.GetContentType(route.RequestBody.ContentSchemas)
		if contentType != "" {
			bodySchema := route.RequestBody.ContentSchemas[contentType]

			for propName, propSchema := range bodySchema.Properties() {
				schema["properties"].(map[string]interface{})[propName] = propSchema

				paramMap[propName] = ir.ParamMapping{
					OpenAPIName: propName,
					Location:    "body",
					IsSuffixed:  false,
				}
			}

			if route.RequestBody.Required {
				bodyRequired := bodySchema.Required()
				required = append(required, bodyRequired...)
			}
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	if len(route.SchemaDefs.Definitions()) > 0 {
		schema["$defs"] = route.SchemaDefs.Definitions()
	}

	return schema, paramMap, nil
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