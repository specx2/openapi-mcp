package factory

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

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

		schemaCopy := normalizeSchema(param.Schema)
		if !param.Required {
			schemaCopy = makeOptionalNullable(schemaCopy)
		}

		argName := param.Name
		if bodyProps[param.Name] {
			argName = fmt.Sprintf("%s__%s", param.Name, param.In)
		}

		schemaProps := schema["properties"].(map[string]interface{})
		schemaProps[argName] = schemaCopy

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
				normalizedBody := normalizeSchema(bodySchema)
				properties := normalizedBody.Properties()
				if len(properties) == 0 {
					propName := determineBodyPropertyName(normalizedBody)
					schema["properties"].(map[string]interface{})[propName] = normalizedBody
					paramMap[propName] = ir.ParamMapping{
						OpenAPIName:  propName,
						Location:     "body",
						IsSuffixed:   false,
						OriginalName: propName,
					}
					if route.RequestBody.Required {
						required = append(required, propName)
					}
				} else {
					for propName, propSchema := range properties {
						schema["properties"].(map[string]interface{})[propName] = normalizeSchema(propSchema)
						paramMap[propName] = ir.ParamMapping{
							OpenAPIName:  propName,
							Location:     "body",
							IsSuffixed:   false,
							OriginalName: propName,
						}
					}

					if route.RequestBody.Required {
						for _, prop := range normalizedBody.Required() {
							required = append(required, prop)
						}
					}
				}
			}
		}
	}

	if len(required) > 0 {
		required = deduplicate(required)
		schema["required"] = required
	}

	if defs := pruneSchemaDefinitions(schema, route.SchemaDefs); len(defs) > 0 {
		schema["$defs"] = defs
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

	properties := normalizeSchema(bodySchema).Properties()
	if len(properties) == 0 {
		propName := determineBodyPropertyName(bodySchema)
		bodyProps[propName] = true
		return bodyProps
	}

	for propName := range properties {
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

func cloneSchema(schema ir.Schema) ir.Schema {
	if schema == nil {
		return nil
	}
	cloned := make(ir.Schema, len(schema))
	for key, value := range schema {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value interface{}) interface{} {
	switch v := value.(type) {
	case ir.Schema:
		return cloneSchema(v)
	case map[string]interface{}:
		cloned := make(map[string]interface{}, len(v))
		for k, val := range v {
			cloned[k] = cloneValue(val)
		}
		return cloned
	case []interface{}:
		cloned := make([]interface{}, len(v))
		for i, val := range v {
			cloned[i] = cloneValue(val)
		}
		return cloned
	default:
		return v
	}
}

func makeOptionalNullable(schema ir.Schema) ir.Schema {
	if schema == nil {
		return nil
	}

	if _, ok := schema["anyOf"]; ok {
		return schema
	}
	if _, ok := schema["oneOf"]; ok {
		return schema
	}
	if _, ok := schema["allOf"]; ok {
		return schema
	}

	if types, ok := schema["type"].([]interface{}); ok {
		for _, t := range types {
			if str, ok := t.(string); ok && str == "null" {
				return schema
			}
		}
	}
	if t, ok := schema["type"].(string); ok && t == "null" {
		return schema
	}

	original := cloneSchema(schema)
	wrapper := make(ir.Schema)
	for _, field := range []string{"default", "description", "title", "example"} {
		if val, ok := original[field]; ok {
			wrapper[field] = val
			delete(original, field)
		}
	}

	wrapper["anyOf"] = []interface{}{
		original,
		map[string]interface{}{"type": "null"},
	}

	return wrapper
}

func pruneSchemaDefinitions(schema ir.Schema, defs ir.Schema) map[string]interface{} {
	if defs == nil {
		return nil
	}
	definitions := defs.Definitions()
	if len(definitions) == 0 {
		return nil
	}

	used := make(map[string]struct{})
	collectRefsInto(schema, used)
	if len(used) == 0 {
		return nil
	}

	pruned := make(map[string]interface{})
	visited := make(map[string]struct{})
	queue := make([]string, 0, len(used))
	for name := range used {
		queue = append(queue, name)
	}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if _, seen := visited[name]; seen {
			continue
		}
		visited[name] = struct{}{}

		definition, ok := definitions[name]
		if !ok {
			continue
		}

		cloned := cloneSchema(definition)
		pruned[name] = cloned

		nested := make(map[string]struct{})
		collectRefsInto(cloned, nested)
		for ref := range nested {
			queue = append(queue, ref)
		}
	}

	if len(pruned) == 0 {
		return nil
	}

	return pruned
}

func collectRefsInto(value interface{}, refs map[string]struct{}) {
	switch v := value.(type) {
	case ir.Schema:
		collectRefsInto(map[string]interface{}(v), refs)
	case map[string]interface{}:
		for key, val := range v {
			if key == "$ref" {
				if refStr, ok := val.(string); ok {
					if strings.HasPrefix(refStr, "#/$defs/") {
						refs[strings.TrimPrefix(refStr, "#/$defs/")] = struct{}{}
					}
				}
				continue
			}
			collectRefsInto(val, refs)
		}
	case []interface{}:
		for _, item := range v {
			collectRefsInto(item, refs)
		}
	}
}

func normalizeSchema(schema ir.Schema) ir.Schema {
	cloned := cloneSchema(schema)
	cloned = mergeAllOf(cloned)
	return cloned
}

func mergeAllOf(schema ir.Schema) ir.Schema {
	allOf, ok := schema["allOf"].([]interface{})
	if !ok {
		return schema
	}

	combinedProps := make(map[string]interface{})
	var combinedRequired []string

	for _, item := range allOf {
		sub := toSchema(item)
		sub = mergeAllOf(sub)

		if props, ok := sub["properties"].(map[string]interface{}); ok {
			for k, v := range props {
				combinedProps[k] = v
			}
		}

		if req, ok := toStringSlice(sub["required"]); ok {
			combinedRequired = append(combinedRequired, req...)
		}

		for key, val := range sub {
			if key == "properties" || key == "required" || key == "allOf" {
				continue
			}
			if _, exists := schema[key]; !exists {
				schema[key] = val
			}
		}
	}

	if len(combinedProps) > 0 {
		if existing, ok := schema["properties"].(map[string]interface{}); ok {
			for k, v := range existing {
				combinedProps[k] = v
			}
		}
		schema["properties"] = combinedProps
	}

	if len(combinedRequired) > 0 {
		schema["required"] = deduplicate(combinedRequired)
	}

	delete(schema, "allOf")
	return schema
}

func toSchema(value interface{}) ir.Schema {
	switch v := value.(type) {
	case ir.Schema:
		return cloneSchema(v)
	case map[string]interface{}:
		return cloneSchema(v)
	default:
		return ir.Schema{}
	}
}

func toStringSlice(value interface{}) ([]string, bool) {
	if value == nil {
		return nil, false
	}
	switch v := value.(type) {
	case []string:
		return v, true
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, true
	default:
		return nil, false
	}
}

var nonWord = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func determineBodyPropertyName(schema ir.Schema) string {
	if title, ok := schema["title"].(string); ok && strings.TrimSpace(title) != "" {
		name := strings.ToLower(nonWord.ReplaceAllString(strings.TrimSpace(title), "_"))
		name = strings.Trim(name, "_")
		if name == "" {
			return "body"
		}
		if firstRune := []rune(name)[0]; unicode.IsDigit(firstRune) {
			return "body_" + name
		}
		return name
	}
	return "body"
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
