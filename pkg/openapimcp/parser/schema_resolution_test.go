package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

func TestParserResolvesExternalAndAdvancedSchemas(t *testing.T) {
	tempDir := t.TempDir()

	meta := `{
  "Meta": {
    "type": "object",
    "properties": {
      "createdAt": {
        "type": "string",
        "format": "date-time"
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "meta.json"), []byte(meta), 0o600); err != nil {
		t.Fatalf("failed to write meta.json: %v", err)
	}

	common := `openapi: 3.0.0
components:
  schemas:
    Info:
      type: object
      properties:
        details:
          type: string
        meta:
          $ref: ./meta.json#/Meta
      not:
        type: object
        properties:
          forbidden:
            type: boolean
`
	if err := os.WriteFile(filepath.Join(tempDir, "common.yaml"), []byte(common), 0o600); err != nil {
		t.Fatalf("failed to write common.yaml: %v", err)
	}

	spec := `openapi: 3.0.3
info:
  title: Test
  version: 1.0.0
paths:
  /pets:
    get:
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Cat'
components:
  schemas:
    Pet:
      type: object
      properties:
        info:
          $ref: './common.yaml#/components/schemas/Info'
        status:
          type: string
      required:
        - info
    Cat:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - type: object
          properties:
            hunts:
              type: boolean
      discriminator:
        propertyName: petType
`

	specPath := filepath.Join(tempDir, "spec.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatalf("failed to write spec.yaml: %v", err)
	}

	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed to read spec: %v", err)
	}

	parser, err := NewParser(specBytes, WithSpecURL("file://"+specPath))
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}

	routes, err := parser.ParseSpec(specBytes)
	if err != nil {
		t.Fatalf("ParseSpec failed: %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	defsSchema := routes[0].SchemaDefs
	if defsSchema == nil {
		t.Fatalf("expected schema definitions to be populated")
	}

	defs, ok := defsSchema["$defs"].(map[string]ir.Schema)
	if !ok {
		t.Fatalf("expected $defs map, got %T", defsSchema["$defs"])
	}

	for _, required := range []string{"Pet", "Cat", "Info", "Meta"} {
		if _, exists := defs[required]; !exists {
			t.Fatalf("expected definition %q to be registered", required)
		}
	}

	catSchema := defs["Cat"]
	if _, ok := catSchema["discriminator"].(map[string]interface{}); !ok {
		t.Errorf("expected discriminator preserved in Cat schema")
	}

	allOf, ok := catSchema["allOf"].([]interface{})
	if !ok || len(allOf) == 0 {
		t.Fatalf("expected Cat schema to have allOf entries")
	}

	foundRef := false
	for _, item := range allOf {
		var entry map[string]interface{}
		switch v := item.(type) {
		case ir.Schema:
			entry = map[string]interface{}(v)
		case map[string]interface{}:
			entry = v
		}
		if entry == nil {
			continue
		}
		if ref, ok := entry["$ref"].(string); ok && ref == "#/$defs/Pet" {
			foundRef = true
		}
	}
	if !foundRef {
		t.Errorf("expected Cat schema allOf to reference Pet definition")
	}

	petSchema := defs["Pet"]
	propsVal := petSchema["properties"]
	var props map[string]interface{}
	switch v := propsVal.(type) {
	case map[string]interface{}:
		props = v
	case map[string]ir.Schema:
		props = make(map[string]interface{}, len(v))
		for k, val := range v {
			props[k] = map[string]interface{}(val)
		}
	}
	infoPropVal := props["info"]
	var infoProp map[string]interface{}
	switch v := infoPropVal.(type) {
	case map[string]interface{}:
		infoProp = v
	case ir.Schema:
		infoProp = map[string]interface{}(v)
	}
	if ref, ok := infoProp["$ref"].(string); !ok || ref != "#/$defs/Info" {
		t.Fatalf("expected pet info property to reference Info definition, got %v", infoProp["$ref"])
	}

	infoSchema := defs["Info"]
	switch infoSchema["not"].(type) {
	case map[string]interface{}, ir.Schema:
		// ok
	default:
		t.Errorf("expected Info schema to preserve not clause")
	}

	metaSchema := defs["Meta"]
	metaProps, _ := metaSchema["properties"].(map[string]interface{})
	if _, exists := metaProps["createdAt"]; !exists {
		t.Errorf("expected Meta schema to contain createdAt property")
	}

	resolved, err := parser.ResolveReference("#/components/schemas/Pet")
	if err != nil {
		t.Fatalf("ResolveReference failed: %v", err)
	}
	if resolved == nil {
		t.Fatalf("expected resolved schema, got nil")
	}
}
