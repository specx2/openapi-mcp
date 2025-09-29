package parser

import (
	"fmt"
	"strings"

	"github.com/specx2/openapi-mcp/core/ir"
)

func ConvertToJSONSchema(openAPISchema map[string]interface{}, isOpenAPI30 bool) ir.Schema {
	result := make(ir.Schema)

	for key, value := range openAPISchema {
		switch key {
		case "$ref":
			result[key] = convertRef(value)
		case "nullable":
			if isOpenAPI30 && value == true {
				continue
			}
			result[key] = value
		case "type":
			if isOpenAPI30 && openAPISchema["nullable"] == true {
				result["anyOf"] = []interface{}{
					map[string]interface{}{"type": value},
					map[string]interface{}{"type": "null"},
				}
				continue
			}
			result[key] = value
		case "properties":
			if props, ok := value.(map[string]interface{}); ok {
				convertedProps := make(map[string]interface{})
				for propName, propSchema := range props {
					if propMap, ok := propSchema.(map[string]interface{}); ok {
						convertedProps[propName] = ConvertToJSONSchema(propMap, isOpenAPI30)
					} else {
						convertedProps[propName] = propSchema
					}
				}
				result[key] = convertedProps
			} else {
				result[key] = value
			}
		case "items":
			if itemsMap, ok := value.(map[string]interface{}); ok {
				result[key] = ConvertToJSONSchema(itemsMap, isOpenAPI30)
			} else {
				result[key] = value
			}
		case "additionalProperties":
			if addPropsMap, ok := value.(map[string]interface{}); ok {
				result[key] = ConvertToJSONSchema(addPropsMap, isOpenAPI30)
			} else {
				result[key] = value
			}
		case "allOf", "anyOf", "oneOf":
			if schemas, ok := value.([]interface{}); ok {
				convertedSchemas := make([]interface{}, len(schemas))
				for i, schema := range schemas {
					if schemaMap, ok := schema.(map[string]interface{}); ok {
						convertedSchemas[i] = ConvertToJSONSchema(schemaMap, isOpenAPI30)
					} else {
						convertedSchemas[i] = schema
					}
				}
				result[key] = convertedSchemas
			} else {
				result[key] = value
			}
		default:
			result[key] = value
		}
	}

	return result
}

func convertRef(ref interface{}) string {
	if refStr, ok := ref.(string); ok {
		if strings.HasPrefix(refStr, "#/components/schemas/") {
			return strings.Replace(refStr, "#/components/schemas/", "#/$defs/", 1)
		}
	}
	return fmt.Sprintf("%v", ref)
}

func MergeSchemaDefinitions(primary, secondary ir.Schema) ir.Schema {
	if primary == nil {
		primary = make(ir.Schema)
	}

	primaryDefs := primary.Definitions()
	secondaryDefs := secondary.Definitions()

	if len(primaryDefs) == 0 && len(secondaryDefs) == 0 {
		return primary
	}

	merged := make(map[string]interface{})
	for k, v := range primaryDefs {
		merged[k] = v
	}
	for k, v := range secondaryDefs {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}

	result := make(ir.Schema)
	for k, v := range primary {
		result[k] = v
	}
	result["$defs"] = merged

	return result
}

func GetContentType(contentSchemas map[string]ir.Schema) string {
	if len(contentSchemas) == 0 {
		return ""
	}

	preferredTypes := []string{
		"application/json",
		"application/vnd.api+json",
		"application/hal+json",
	}

	for _, contentType := range preferredTypes {
		if _, exists := contentSchemas[contentType]; exists {
			return contentType
		}
	}

	for contentType := range contentSchemas {
		if strings.Contains(contentType, "json") {
			return contentType
		}
	}

	for contentType := range contentSchemas {
		return contentType
	}

	return ""
}

func IsObjectType(schema ir.Schema) bool {
	if schema.Type() == "object" {
		return true
	}

	if rawType, ok := schema["type"]; ok {
		switch t := rawType.(type) {
		case []interface{}:
			for _, v := range t {
				if s, ok := v.(string); ok && s == "object" {
					return true
				}
			}
		case []string:
			for _, v := range t {
				if v == "object" {
					return true
				}
			}
		}
	}

	if props, ok := schema["properties"]; ok {
		switch props.(type) {
		case map[string]interface{}:
			return true
		case map[string]ir.Schema:
			return true
		case ir.Schema:
			return true
		}
	}

	return false
}

func WrapNonObjectSchema(schema ir.Schema) (ir.Schema, bool) {
	if IsObjectType(schema) {
		return schema, false
	}

	wrapped := ir.Schema{
		"type": "object",
		"properties": map[string]interface{}{
			"result": schema,
		},
		"required": []string{"result"},
	}

	if defs := schema.Definitions(); len(defs) > 0 {
		wrapped["$defs"] = defs
	}

	return wrapped, true
}

func OptimizeSchema(schema ir.Schema) ir.Schema {
	result := make(ir.Schema)
	for k, v := range schema {
		if k == "additionalProperties" && v == false {
			continue
		}
		result[k] = v
	}

	if defs := result.Definitions(); len(defs) > 0 {
		usedRefs := findReferences(result)
		optimizedDefs := make(map[string]interface{})
		for defName, defSchema := range defs {
			if usedRefs[defName] {
				optimizedDefs[defName] = defSchema
			}
		}
		if len(optimizedDefs) > 0 {
			result["$defs"] = optimizedDefs
		} else {
			delete(result, "$defs")
		}
	}

	return result
}

func findReferences(schema ir.Schema) map[string]bool {
	refs := make(map[string]bool)
	findRefsRecursive(schema, refs)
	return refs
}

func findRefsRecursive(value interface{}, refs map[string]bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if key == "$ref" {
				if refStr, ok := val.(string); ok {
					if strings.HasPrefix(refStr, "#/$defs/") {
						defName := strings.TrimPrefix(refStr, "#/$defs/")
						refs[defName] = true
					}
				}
			} else {
				findRefsRecursive(val, refs)
			}
		}
	case []interface{}:
		for _, item := range v {
			findRefsRecursive(item, refs)
		}
	}
}
