package executor

import (
	"testing"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

func TestOpenAPIToolValidateArgs(t *testing.T) {
	route := ir.HTTPRoute{
		Path:   "/check",
		Method: "POST",
	}

	inputSchema := ir.Schema{
		"type": "object",
		"properties": map[string]interface{}{
			"name":  map[string]interface{}{"type": "string"},
			"count": map[string]interface{}{"type": "integer"},
		},
		"required": []interface{}{"name"},
	}

	tool := NewOpenAPITool(
		"check",
		"Check inputs",
		inputSchema,
		nil,
		false,
		route,
		nil,
		"https://api.example.com",
		nil,
		nil,
		nil,
	)

	if tool.validator == nil {
		t.Fatalf("expected validator to be compiled")
	}

	if err := tool.validateArgs(map[string]interface{}{"name": "alice", "count": float64(3)}); err != nil {
		t.Fatalf("expected validation to pass, got %v", err)
	}

	if err := tool.validateArgs(map[string]interface{}{"count": float64(1)}); err == nil {
		t.Fatalf("expected validation error for missing required field")
	}

	if tags := tool.Tags(); len(tags) != 0 {
		t.Fatalf("expected no tags, got %v", tags)
	}
}
