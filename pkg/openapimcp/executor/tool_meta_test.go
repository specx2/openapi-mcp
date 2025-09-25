package executor

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

func TestOpenAPIToolMetaIncludesEncodingHeaders(t *testing.T) {
	route := ir.HTTPRoute{
		Path:        "/upload",
		Method:      "POST",
		OperationID: "uploadFile",
		RequestBody: &ir.RequestBodyInfo{
			Required: true,
			ContentSchemas: map[string]ir.Schema{
				"multipart/form-data": {
					"type": "object",
					"properties": map[string]any{
						"file": ir.Schema{"type": "string", "format": "binary"},
					},
				},
			},
			MediaDefaults: map[string]any{
				"multipart/form-data": map[string]any{"file": nil},
			},
			MediaExtensions: map[string]map[string]any{
				"multipart/form-data": {
					"x-openapi-mcp-test": "enabled",
				},
			},
			Encodings: map[string]map[string]ir.EncodingInfo{
				"multipart/form-data": {
					"file": {
						ContentType: "application/octet-stream",
						Headers: map[string]ir.HeaderInfo{
							"X-Debug": {
								Name:   "X-Debug",
								Schema: ir.Schema{"type": "string", "default": "trace"},
							},
						},
					},
				},
			},
		},
	}

	annotation := &mcp.ToolAnnotation{
		ReadOnlyHint:    boolPtr(false),
		DestructiveHint: boolPtr(true),
		IdempotentHint:  boolPtr(false),
		OpenWorldHint:   boolPtr(true),
		Title:           "Upload",
	}

	tool := NewOpenAPITool(
		"upload",
		"Upload a file",
		ir.Schema{"type": "object"},
		nil,
		false,
		route,
		nil,
		"https://api.example.com",
		nil,
		[]string{"uploads", "multipart"},
		annotation,
	)

	meta := tool.Tool().Meta
	if meta == nil {
		t.Fatalf("expected tool meta to be set")
	}
	openapiMeta, ok := meta.AdditionalFields["openapi"].(map[string]any)
	if !ok {
		t.Fatalf("expected openapi metadata, got %#v", meta.AdditionalFields)
	}

	requestBody, ok := openapiMeta["requestBody"].(map[string]any)
	if !ok {
		t.Fatalf("expected requestBody metadata, got %#v", openapiMeta)
	}
	content, ok := requestBody["content"].(map[string]any)
	if !ok {
		t.Fatalf("expected content metadata, got %#v", requestBody)
	}
	media, ok := content["multipart/form-data"].(map[string]any)
	if !ok {
		t.Fatalf("expected multipart metadata, got %#v", content)
	}
	if media["extensions"].(map[string]any)["x-openapi-mcp-test"] != "enabled" {
		t.Fatalf("expected media extension to be propagated")
	}
	encodings, ok := media["encodings"].(map[string]any)
	if !ok {
		t.Fatalf("expected encodings metadata, got %#v", media)
	}
	fileMeta, ok := encodings["file"].(map[string]any)
	if !ok {
		t.Fatalf("expected file encoding metadata, got %#v", encodings)
	}
	headers, ok := fileMeta["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected encoding headers metadata, got %#v", fileMeta)
	}
	debugHeader, ok := headers["X-Debug"].(map[string]any)
	if !ok {
		t.Fatalf("expected X-Debug header metadata, got %#v", headers)
	}
	schemaVal := debugHeader["schema"]
	switch v := schemaVal.(type) {
	case map[string]any:
		if v["default"] != "trace" {
			t.Fatalf("expected header default to be propagated")
		}
	case ir.Schema:
		if v["default"] != "trace" {
			t.Fatalf("expected header default to be propagated")
		}
	default:
		t.Fatalf("unexpected schema type %#v", schemaVal)
	}

	tagsField, ok := meta.AdditionalFields["tags"].([]string)
	if !ok {
		t.Fatalf("expected meta tags array, got %#v", meta.AdditionalFields["tags"])
	}
	if len(tagsField) != 2 {
		t.Fatalf("expected two tags, got %v", tagsField)
	}

	anno := tool.Tool().Annotations
	if anno.ReadOnlyHint == nil || *anno.ReadOnlyHint != false {
		t.Fatalf("expected readOnlyHint to match override")
	}
	if anno.Title != "Upload" {
		t.Fatalf("expected annotation title to be propagated")
	}

	if len(tool.Tags()) != 2 {
		t.Fatalf("expected tool tags accessor to include provided tags")
	}
}
