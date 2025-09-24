package test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/executor"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/factory"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

// MockHTTPClient provides a minimal implementation for tool execution tests.
type MockHTTPClient struct{}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       http.NoBody,
	}, nil
}

func TestCreateToolParameterCollisions(t *testing.T) {
	route := ir.HTTPRoute{
		Path:        "/users/{id}",
		Method:      "PUT",
		OperationID: "updateUser",
		Parameters: []ir.ParameterInfo{
			{
				Name:     "id",
				In:       ir.ParameterInPath,
				Required: true,
				Schema:   ir.Schema{"type": "integer"},
			},
			{
				Name:     "id",
				In:       ir.ParameterInQuery,
				Required: false,
				Schema:   ir.Schema{"type": "integer"},
			},
		},
		RequestBody: &ir.RequestBodyInfo{
			Required: true,
			ContentSchemas: map[string]ir.Schema{
				"application/json": {
					"type": "object",
					"properties": map[string]interface{}{
						"id":   ir.Schema{"type": "integer"},
						"name": ir.Schema{"type": "string"},
					},
					"required": []interface{}{"name"},
				},
			},
		},
	}

	cf := factory.NewComponentFactory(&MockHTTPClient{}, "https://api.example.com")
	tool, err := cf.CreateTool(route, nil)
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}

	mappings := tool.ParameterMappings()

	if mapping, ok := mappings["id__path"]; !ok || !mapping.IsSuffixed || mapping.Location != ir.ParameterInPath {
		t.Fatalf("expected suffixed path parameter mapping, got %#v", mapping)
	}

	if mapping, ok := mappings["id__query"]; !ok || !mapping.IsSuffixed || mapping.Location != ir.ParameterInQuery {
		t.Fatalf("expected suffixed query parameter mapping, got %#v", mapping)
	}

	if mapping, ok := mappings["name"]; !ok || mapping.Location != "body" {
		t.Fatalf("expected body mapping for name, got %#v", mapping)
	}

}

func TestCreateToolWithoutCollisions(t *testing.T) {
	route := ir.HTTPRoute{
		Path:        "/users/{userId}",
		Method:      "GET",
		OperationID: "getUser",
		Parameters: []ir.ParameterInfo{
			{
				Name:     "userId",
				In:       ir.ParameterInPath,
				Required: true,
				Schema:   ir.Schema{"type": "integer"},
			},
			{
				Name:     "tags",
				In:       ir.ParameterInQuery,
				Required: false,
				Schema:   ir.Schema{"type": "array", "items": map[string]interface{}{"type": "string"}},
				Style:    "form",
				Explode:  boolPtr(true),
			},
		},
	}

	cf := factory.NewComponentFactory(&MockHTTPClient{}, "https://api.example.com")
	tool, err := cf.CreateTool(route, nil)
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}

	mappings := tool.ParameterMappings()
	if mapping, ok := mappings["userId"]; !ok || mapping.IsSuffixed {
		t.Fatalf("unexpected suffix for userId mapping: %#v", mapping)
	}
	if _, ok := mappings["tags"]; !ok {
		t.Fatalf("expected mapping for tags query parameter")
	}
}

func TestRequestBuilderUsesParameterMappings(t *testing.T) {
	route := ir.HTTPRoute{
		Path:   "/users/{id}",
		Method: "POST",
		Parameters: []ir.ParameterInfo{
			{Name: "id", In: ir.ParameterInPath, Required: true, Schema: ir.Schema{"type": "integer"}},
			{Name: "filter", In: ir.ParameterInQuery, Schema: ir.Schema{"type": "object"}, Style: "deepObject", Explode: boolPtr(true)},
		},
		RequestBody: &ir.RequestBodyInfo{
			ContentSchemas: map[string]ir.Schema{
				"application/json": {
					"type": "object",
					"properties": map[string]interface{}{
						"name": ir.Schema{"type": "string"},
					},
					"required": []interface{}{"name"},
				},
			},
		},
	}

	cf := factory.NewComponentFactory(&MockHTTPClient{}, "https://api.example.com")
	tool, err := cf.CreateTool(route, nil)
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}

	args := map[string]interface{}{
		"id":     float64(42), // JSON numbers decode as float64
		"filter": map[string]interface{}{"status": "active"},
		"name":   "Jane",
	}

	reqBuilder := executor.NewRequestBuilder(route, tool.ParameterMappings(), "https://api.example.com")
	req, err := reqBuilder.Build(context.Background(), args)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if req.URL.Path != "/users/42" {
		t.Fatalf("expected path substitution, got %s", req.URL.Path)
	}

	values, _ := url.ParseQuery(req.URL.RawQuery)
	if values.Get("filter[status]") != "active" {
		t.Fatalf("expected deepObject serialization, got %s", req.URL.RawQuery)
	}

	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected JSON content type, got %s", req.Header.Get("Content-Type"))
	}
}

func TestRequestBuilderHandlesPrimitiveBody(t *testing.T) {
	route := ir.HTTPRoute{
		Path:   "/metrics",
		Method: "POST",
		RequestBody: &ir.RequestBodyInfo{
			Required: true,
			ContentSchemas: map[string]ir.Schema{
				"application/json": {
					"type": "number",
				},
			},
		},
	}

	cf := factory.NewComponentFactory(&MockHTTPClient{}, "https://api.example.com")
	tool, err := cf.CreateTool(route, nil)
	if err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}

	args := map[string]interface{}{
		"body": 3.14,
	}

	reqBuilder := executor.NewRequestBuilder(route, tool.ParameterMappings(), "https://api.example.com")
	req, err := reqBuilder.Build(context.Background(), args)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected JSON content type, got %s", req.Header.Get("Content-Type"))
	}

	if req.URL.Path != "/metrics" {
		t.Fatalf("unexpected request path %s", req.URL.Path)
	}

	if req.Body == nil {
		t.Fatal("expected non-nil body")
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(bodyBytes) != "3.14" {
		t.Fatalf("expected raw body '3.14', got %s", string(bodyBytes))
	}
}

func boolPtr(v bool) *bool {
	return &v
}
