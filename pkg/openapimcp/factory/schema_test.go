package factory

import (
	"strings"
	"testing"

	"github.com/specx2/openapi-mcp/pkg/openapimcp/ir"
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

func TestCombineSchemasAnnotatesParameterDescription(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	route := ir.HTTPRoute{
		Parameters: []ir.ParameterInfo{
			{
				Name:        "userId",
				In:          ir.ParameterInPath,
				Required:    true,
				Description: "User identifier",
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

	props := extractProperties(t, schema["properties"])
	paramSchema := extractSchemaMap(t, props["userId"])
	desc, ok := paramSchema["description"].(string)
	if !ok {
		t.Fatalf("expected description string, got %T", paramSchema["description"])
	}
	if !strings.Contains(desc, "User identifier") || !strings.Contains(desc, "Path parameter") {
		t.Fatalf("expected description to include user text and location, got %q", desc)
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

func TestCombineSchemasPropagatesParameterMetadata(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	route := ir.HTTPRoute{
		Parameters: []ir.ParameterInfo{
			{
				Name:            "status",
				In:              ir.ParameterInQuery,
				Schema:          ir.Schema{"type": "string"},
				Example:         "active",
				Examples:        map[string]interface{}{"sample": map[string]interface{}{"value": "active", "summary": "A"}},
				Deprecated:      true,
				AllowEmptyValue: true,
			},
		},
	}

	schema, _, err := cf.combineSchemas(route)
	if err != nil {
		t.Fatalf("combineSchemas returned error: %v", err)
	}

	props := extractProperties(t, schema["properties"])
	paramSchema := extractSchemaMap(t, props["status"])

	if paramSchema["example"] != "active" {
		t.Fatalf("expected example to be 'active', got %#v", paramSchema["example"])
	}

	examples, ok := paramSchema["examples"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected examples map, got %T", paramSchema["examples"])
	}
	if _, exists := examples["sample"]; !exists {
		t.Fatalf("expected examples to contain 'sample'")
	}

	if deprecated, _ := paramSchema["deprecated"].(bool); !deprecated {
		t.Fatalf("expected deprecated flag to be true")
	}

	if allowEmpty, _ := paramSchema["x-allowEmptyValue"].(bool); !allowEmpty {
		t.Fatalf("expected x-allowEmptyValue to be true")
	}
}

func TestCombineSchemasPropagatesBodyExamplesToProperties(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	requestBodySchema := ir.Schema{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "integer"},
		},
	}

	bodyExample := map[string]interface{}{
		"name": "Alice",
		"age":  float64(30),
	}

	bodyNamedExample := map[string]interface{}{
		"value": map[string]interface{}{
			"name": "Bob",
			"age":  float64(25),
		},
		"summary": "Example body",
	}

	route := ir.HTTPRoute{
		RequestBody: &ir.RequestBodyInfo{
			ContentSchemas: map[string]ir.Schema{
				"application/json": requestBodySchema,
			},
			MediaExamples: map[string]interface{}{
				"application/json": bodyExample,
			},
			MediaExampleSets: map[string]map[string]interface{}{
				"application/json": {
					"sample": bodyNamedExample,
				},
			},
		},
	}

	schema, _, err := cf.combineSchemas(route)
	if err != nil {
		t.Fatalf("combineSchemas returned error: %v", err)
	}

	props := extractProperties(t, schema["properties"])
	nameSchema := extractSchemaMap(t, props["name"])
	if nameSchema["example"] != "Alice" {
		t.Fatalf("expected example for name to be Alice, got %#v", nameSchema["example"])
	}
	nameExamples := extractExamplesMap(t, nameSchema["examples"])
	if len(nameExamples) != 1 {
		t.Fatalf("expected 1 named example for name property, got %d", len(nameExamples))
	}
	if value := extractExampleValueFromEntry(t, nameExamples["sample"]); value != "Bob" {
		t.Fatalf("expected named example value Bob, got %#v", value)
	}

	ageSchema := extractSchemaMap(t, props["age"])
	if value, ok := ageSchema["example"].(float64); !ok || value != 30 {
		t.Fatalf("expected example for age to be 30, got %#v", ageSchema["example"])
	}
	ageExamples := extractExamplesMap(t, ageSchema["examples"])
	if value := extractExampleValueFromEntry(t, ageExamples["sample"]); value != float64(25) {
		t.Fatalf("expected named example value 25, got %#v", value)
	}
}

func TestCombineSchemasBodyExampleOnPrimitivePayload(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	bodySchema := ir.Schema{
		"type": "string",
	}

	route := ir.HTTPRoute{
		RequestBody: &ir.RequestBodyInfo{
			ContentSchemas: map[string]ir.Schema{
				"text/plain": bodySchema,
			},
			MediaExamples: map[string]interface{}{
				"text/plain": "hello",
			},
			MediaExampleSets: map[string]map[string]interface{}{
				"text/plain": {
					"alt": map[string]interface{}{
						"value": "hi",
					},
				},
			},
		},
	}

	schema, paramMap, err := cf.combineSchemas(route)
	if err != nil {
		t.Fatalf("combineSchemas returned error: %v", err)
	}

	props := extractProperties(t, schema["properties"])
	var propName string
	for name := range props {
		propName = name
	}
	if propName == "" {
		t.Fatalf("expected body property to be present")
	}
	bodyProp := extractSchemaMap(t, props[propName])
	if bodyProp["example"] != "hello" {
		t.Fatalf("expected body example to be 'hello', got %#v", bodyProp["example"])
	}
	bodyExamples := extractExamplesMap(t, bodyProp["examples"])
	if value := extractExampleValueFromEntry(t, bodyExamples["alt"]); value != "hi" {
		t.Fatalf("expected named body example 'hi', got %#v", value)
	}

	if _, ok := paramMap[propName]; !ok {
		t.Fatalf("expected param map to contain body property")
	}
}

func TestExtractOutputSchemaPrunesDefinitions(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	route := ir.HTTPRoute{
		Responses: map[string]ir.ResponseInfo{
			"200": {
				ContentSchemas: map[string]ir.Schema{
					"application/json": {
						"$ref": "#/$defs/Result",
					},
				},
			},
		},
		SchemaDefs: ir.Schema{
			"$defs": map[string]interface{}{
				"Result": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"user": map[string]interface{}{
							"$ref": "#/$defs/User",
						},
					},
				},
				"User": map[string]interface{}{
					"type": "object",
				},
				"Unused": map[string]interface{}{
					"type": "integer",
				},
			},
		},
	}

	schema, wrap := cf.extractOutputSchema(route)
	if wrap {
		t.Fatalf("expected wrapResult=false")
	}

	defs := extractProperties(t, schema["$defs"])
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	if _, ok := defs["Result"]; !ok {
		t.Fatalf("expected Result definition")
	}
	if _, ok := defs["User"]; !ok {
		t.Fatalf("expected User definition")
	}
	if _, ok := defs["Unused"]; ok {
		t.Fatalf("did not expect Unused definition")
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

func extractSchemaMap(t *testing.T, value interface{}) map[string]interface{} {
	t.Helper()
	switch v := value.(type) {
	case map[string]interface{}:
		return v
	case ir.Schema:
		return map[string]interface{}(v)
	default:
		t.Fatalf("unexpected schema type %T", value)
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

func extractExamplesMap(t *testing.T, value interface{}) map[string]interface{} {
	t.Helper()
	switch v := value.(type) {
	case map[string]interface{}:
		return v
	case ir.Schema:
		return map[string]interface{}(v)
	default:
		t.Fatalf("unexpected examples type %T", value)
	}
	return nil
}

func extractExampleValueFromEntry(t *testing.T, entry interface{}) interface{} {
	t.Helper()
	entryMap, ok := entry.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected example entry type %T", entry)
	}
	value, ok := entryMap["value"]
	if !ok {
		t.Fatalf("example entry missing value field: %#v", entryMap)
	}
	return value
}
