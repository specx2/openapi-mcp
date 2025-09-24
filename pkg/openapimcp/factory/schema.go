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
	bodyProps := cf.collectBodyProperties(route)

	for _, param := range route.Parameters {
		if param.Schema == nil {
			continue
		}

		argName := param.Name
		if bodyProps[param.Name] {
			argName = fmt.Sprintf("%s__%s", param.Name, param.In)
		}

		schemaProps := schema["properties"].(map[string]interface{})
		schemaProps[argName] = param.Schema

		if param.Required {
			required = append(required, argName)
		}

		paramMap[argName] = ir.ParamMapping{
			OpenAPIName:  param.Name,
			Location:     param.In,
			IsSuffixed:   argName != param.Name,
			OriginalName: param.Name,
		}
	}

	if route.RequestBody != nil {
		contentType := parser.GetContentType(route.RequestBody.ContentSchemas)
		if contentType != "" {
			bodySchema := route.RequestBody.ContentSchemas[contentType]
			if bodySchema != nil {
				for propName, propSchema := range bodySchema.Properties() {
					schema["properties"].(map[string]interface{})[propName] = propSchema
					paramMap[propName] = ir.ParamMapping{
						OpenAPIName:  propName,
						Location:     "body",
						IsSuffixed:   false,
						OriginalName: propName,
					}
				}

				if route.RequestBody.Required {
					for _, prop := range bodySchema.Required() {
						required = append(required, prop)
					}
				}
			}
		}
	}

	if len(required) > 0 {
		required = deduplicate(required)
		schema["required"] = required
	}

	if len(route.SchemaDefs.Definitions()) > 0 {
		schema["$defs"] = route.SchemaDefs.Definitions()
	}

	return schema, paramMap, nil
}

func (cf *ComponentFactory) collectBodyProperties(route ir.HTTPRoute) map[string]bool {
	bodyProps := make(map[string]bool)

	if route.RequestBody == nil {
		return bodyProps
	}

	contentType := parser.GetContentType(route.RequestBody.ContentSchemas)
	if contentType == "" {
		return bodyProps
	}

	bodySchema := route.RequestBody.ContentSchemas[contentType]
	if bodySchema == nil {
		return bodyProps
	}

	for propName := range bodySchema.Properties() {
		bodyProps[propName] = true
	}

	return bodyProps
}

func deduplicate(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
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
