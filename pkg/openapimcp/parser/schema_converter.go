package parser

import (
	"fmt"

	"github.com/specx2/openapi-mcp/pkg/openapimcp/ir"
)

type schemaConverter struct {
	resolver    *schemaResolver
	isOpenAPI30 bool
	defs        map[string]ir.Schema
	visitedRefs map[string]struct{}
}

func newSchemaConverter(resolver *schemaResolver, isOpenAPI30 bool) *schemaConverter {
	return &schemaConverter{
		resolver:    resolver,
		isOpenAPI30: isOpenAPI30,
		defs:        make(map[string]ir.Schema),
		visitedRefs: make(map[string]struct{}),
	}
}

func (c *schemaConverter) definitions() map[string]ir.Schema {
	return c.defs
}

func (c *schemaConverter) convert(value interface{}) ir.Schema {
	if value == nil {
		return nil
	}

	schemaMap, err := convertToGenericMap(value)
	if err != nil {
		return nil
	}

	return c.convertMap(schemaMap)
}

func (c *schemaConverter) convertMap(input map[string]interface{}) ir.Schema {
	if input == nil {
		return nil
	}

	result := make(ir.Schema)

	nullable := false
	if c.isOpenAPI30 {
		if nv, ok := input["nullable"].(bool); ok && nv {
			nullable = true
		}
	}

	for key, raw := range input {
		switch key {
		case "nullable":
			// handled after loop
			continue
		case "$ref":
			refStr := fmt.Sprintf("%v", raw)
			refName, err := c.convertReference(refStr)
			if err != nil {
				result[key] = raw
				continue
			}
			result[key] = "#/$defs/" + refName
		case "properties", "patternProperties":
			if propMap, ok := raw.(map[string]interface{}); ok {
				converted := make(map[string]interface{}, len(propMap))
				for propName, propSchema := range propMap {
					converted[propName] = c.convertValue(propSchema)
				}
				result[key] = converted
			} else {
				result[key] = raw
			}
		case "additionalProperties", "unevaluatedProperties", "propertyNames":
			result[key] = c.convertValue(raw)
		case "items", "contains":
			result[key] = c.convertValue(raw)
		case "allOf", "anyOf", "oneOf", "prefixItems":
			if arr, ok := raw.([]interface{}); ok {
				converted := make([]interface{}, len(arr))
				for i, item := range arr {
					converted[i] = c.convertValue(item)
				}
				result[key] = converted
			} else {
				result[key] = raw
			}
		case "not", "if", "then", "else":
			result[key] = c.convertValue(raw)
		default:
			result[key] = cloneGenericValue(raw)
		}
	}

	if nullable {
		c.applyNullable(result)
	}

	return result
}

func (c *schemaConverter) convertValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return c.convertMap(v)
	case ir.Schema:
		return c.convertMap(v)
	case []interface{}:
		converted := make([]interface{}, len(v))
		for i, item := range v {
			converted[i] = c.convertValue(item)
		}
		return converted
	default:
		return cloneGenericValue(value)
	}
}

func (c *schemaConverter) convertReference(ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("empty reference")
	}

	if name, ok := c.resolver.getDefinitionName(ref); ok {
		return name, nil
	}

	name, schemaMap, err := c.resolver.resolveRef(ref)
	if err != nil {
		return "", err
	}

	if _, exists := c.defs[name]; exists {
		return name, nil
	}

	c.defs[name] = c.convertMap(schemaMap)
	return name, nil
}

func (c *schemaConverter) applyNullable(schema ir.Schema) {
	if schema == nil {
		return
	}

	if anyOf, ok := schema["anyOf"].([]interface{}); ok {
		schema["anyOf"] = append(anyOf, map[string]interface{}{"type": "null"})
		return
	}
	if oneOf, ok := schema["oneOf"].([]interface{}); ok {
		schema["oneOf"] = append(oneOf, map[string]interface{}{"type": "null"})
		return
	}
	if t, ok := schema["type"]; ok {
		schema["anyOf"] = []interface{}{
			map[string]interface{}{"type": t},
			map[string]interface{}{"type": "null"},
		}
		delete(schema, "type")
		return
	}
	schema["type"] = []interface{}{"null"}
}

func (c *schemaConverter) registerComponent(ref string, schema ir.Schema) {
	name, ok := c.resolver.getDefinitionName(ref)
	if !ok || name == "" {
		name = deriveRefBase(ref)
	}
	c.defs[name] = schema
	c.resolver.register(ref, name, nil)
}
