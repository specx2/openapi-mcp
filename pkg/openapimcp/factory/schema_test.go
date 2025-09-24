package factory

import (
	"testing"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

func TestCombineSchemasOptionalParameterNullable(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	route := ir.HTTPRoute{
		Parameters: []ir.ParameterInfo{
			{
				Name:     "filter",
				In:       ir.ParameterInQuery,
				Required: false,
				Schema: ir.Schema{
					"type": "string",
				},
			},
		},
	}

	schema, _, err := cf.combineSchemas(route)
	if err != nil {
		t.Fatalf("combineSchemas returned error: %v", err)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %T", schema["properties"])
	}

	var filterSchema map[string]interface{}
	switch v := props["filter"].(type) {
	case map[string]interface{}:
		filterSchema = v
	case ir.Schema:
		filterSchema = map[string]interface{}(v)
	default:
		t.Fatalf("expected filter schema map, got %T", props["filter"])
	}

	anyOf, ok := filterSchema["anyOf"].([]interface{})
	if !ok {
		t.Fatalf("expected anyOf slice for optional parameter, got %T", filterSchema["anyOf"])
	}
	if len(anyOf) != 2 {
		t.Fatalf("expected anyOf to have 2 entries, got %d", len(anyOf))
	}
}

func TestCombineSchemasPrunesDefinitions(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	route := ir.HTTPRoute{
		Parameters: []ir.ParameterInfo{
			{
				Name:     "user",
				In:       ir.ParameterInQuery,
				Required: false,
				Schema: ir.Schema{
					"$ref": "#/$defs/User",
				},
			},
		},
		SchemaDefs: ir.Schema{
			"$defs": map[string]interface{}{
				"User": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"address": map[string]interface{}{
							"$ref": "#/$defs/Address",
						},
					},
				},
				"Address": map[string]interface{}{
					"type": "string",
				},
				"Unused": map[string]interface{}{
					"type": "integer",
				},
			},
		},
	}

	schema, _, err := cf.combineSchemas(route)
	if err != nil {
		t.Fatalf("combineSchemas returned error: %v", err)
	}

	var defs map[string]interface{}
	switch v := schema["$defs"].(type) {
	case map[string]interface{}:
		defs = v
	case ir.Schema:
		defs = map[string]interface{}(v)
	case nil:
		t.Fatalf("expected $defs map, got <nil>")
	default:
		t.Fatalf("expected $defs map, got %T", schema["$defs"])
	}

	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d (schema: %#v)", len(defs), schema)
	}

	if _, ok := defs["User"]; !ok {
		t.Fatalf("expected User definition to be present")
	}
	if _, ok := defs["Address"]; !ok {
		t.Fatalf("expected Address definition to be present")
	}
	if _, ok := defs["Unused"]; ok {
		t.Fatalf("did not expect Unused definition to be present")
	}
}

func TestCombineSchemasMergesAllOf(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	route := ir.HTTPRoute{
		RequestBody: &ir.RequestBodyInfo{
			ContentSchemas: map[string]ir.Schema{
				"application/json": {
					"allOf": []interface{}{
						map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"first": map[string]interface{}{"type": "string"},
							},
							"required": []interface{}{"first"},
						},
						map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"second": map[string]interface{}{"type": "integer"},
							},
						},
					},
				},
			},
			Required: true,
		},
	}

	schema, _, err := cf.combineSchemas(route)
	if err != nil {
		t.Fatalf("combineSchemas returned error: %v", err)
	}

	props := extractProperties(t, schema["properties"])
	if _, ok := props["first"]; !ok {
		t.Fatalf("expected first property")
	}
	if _, ok := props["second"]; !ok {
		t.Fatalf("expected second property")
	}

	req := extractRequired(t, schema["required"])
	if len(req) != 1 || req[0] != "first" {
		t.Fatalf("expected required to contain first, got %v", req)
	}
}

func TestDetermineBodyPropertyNameFromTitle(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	route := ir.HTTPRoute{
		RequestBody: &ir.RequestBodyInfo{
			ContentSchemas: map[string]ir.Schema{
				"application/json": {
					"title": "Payload Data",
					"type":  "string",
				},
			},
		},
	}

	schema, paramMap, err := cf.combineSchemas(route)
	if err != nil {
		t.Fatalf("combineSchemas returned error: %v", err)
	}

	props := extractProperties(t, schema["properties"])
	if _, ok := props["payload_data"]; !ok {
		t.Fatalf("expected payload_data property, got keys %v", props)
	}

	if _, ok := paramMap["payload_data"]; !ok {
		t.Fatalf("expected payload_data mapping")
	}
}

func extractProperties(t *testing.T, value interface{}) map[string]interface{} {
	t.Helper()
	switch v := value.(type) {
	case map[string]interface{}:
		return v
	case ir.Schema:
		return map[string]interface{}(v)
	default:
		t.Fatalf("unexpected properties type %T", value)
		return nil
	}
}

func extractRequired(t *testing.T, value interface{}) []string {
	t.Helper()
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		t.Fatalf("unexpected required type %T", value)
		return nil
	}
}
