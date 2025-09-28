package executor_test

import (
	"context"
	"testing"

	"github.com/specx2/openapi-mcp/pkg/openapimcp/executor"
	"github.com/specx2/openapi-mcp/pkg/openapimcp/ir"
)

func TestRequestBuilderPrefersJSONVariants(t *testing.T) {
	route := ir.HTTPRoute{
		Path:   "/widgets",
		Method: "PATCH",
		RequestBody: &ir.RequestBodyInfo{
			ContentSchemas: map[string]ir.Schema{
				"application/merge-patch+json": {
					"type": "object",
					"properties": map[string]interface{}{
						"name": ir.Schema{"type": "string"},
					},
				},
				"application/xml": {
					"type": "object",
				},
			},
			ContentOrder: []string{"application/xml", "application/merge-patch+json"},
		},
	}

	paramMap := map[string]ir.ParamMapping{
		"name": {
			OpenAPIName: "name",
			Location:    "body",
		},
	}

	builder := executor.NewRequestBuilder(route, paramMap, "")
	req, err := builder.Build(context.Background(), map[string]interface{}{"name": "example"})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if got := req.Header.Get("Content-Type"); got != "application/merge-patch+json" {
		t.Fatalf("expected content type application/merge-patch+json, got %q", got)
	}
}

func TestRequestBuilderRawBodyChoosesJsonWhenAvailable(t *testing.T) {
	route := ir.HTTPRoute{
		Path:   "/notes",
		Method: "PUT",
		RequestBody: &ir.RequestBodyInfo{
			ContentSchemas: map[string]ir.Schema{
				"application/problem+json": {
					"type": "object",
				},
				"text/plain": {
					"type": "string",
				},
			},
			ContentOrder: []string{"text/plain", "application/problem+json"},
		},
	}

	builder := executor.NewRequestBuilder(route, nil, "")
	req, err := builder.Build(context.Background(), map[string]interface{}{"_rawBody": `{"message":"ok"}`})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if got := req.Header.Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}
}

func TestRequestBuilderRawBodyPrefersTextForPlainString(t *testing.T) {
	route := ir.HTTPRoute{
		Path:   "/logs",
		Method: "POST",
		RequestBody: &ir.RequestBodyInfo{
			ContentSchemas: map[string]ir.Schema{
				"text/plain": {
					"type": "string",
				},
				"application/problem+json": {
					"type": "object",
				},
			},
			ContentOrder: []string{"application/problem+json", "text/plain"},
		},
	}

	builder := executor.NewRequestBuilder(route, nil, "")
	req, err := builder.Build(context.Background(), map[string]interface{}{"_rawBody": "plain text"})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if got := req.Header.Get("Content-Type"); got != "text/plain" {
		t.Fatalf("expected text/plain content type, got %q", got)
	}
}

func TestRequestBuilderSetsAcceptHeaderFromResponses(t *testing.T) {
	route := ir.HTTPRoute{
		Path:   "/widgets",
		Method: "GET",
		Responses: map[string]ir.ResponseInfo{
			"200": {
				ContentSchemas: map[string]ir.Schema{
					"application/json": {"type": "object"},
				},
			},
		},
	}

	builder := executor.NewRequestBuilder(route, nil, "")
	req, err := builder.Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("expected Accept header application/json, got %q", got)
	}
}

func TestRequestBuilderDoesNotOverrideAcceptHeader(t *testing.T) {
	paramMap := map[string]ir.ParamMapping{
		"Accept": {
			OpenAPIName: "Accept",
			Location:    ir.ParameterInHeader,
		},
	}

	route := ir.HTTPRoute{
		Path:   "/widgets",
		Method: "GET",
		Parameters: []ir.ParameterInfo{
			{
				Name: "Accept",
				In:   ir.ParameterInHeader,
			},
		},
		Responses: map[string]ir.ResponseInfo{
			"200": {
				ContentSchemas: map[string]ir.Schema{
					"application/json": {"type": "object"},
				},
			},
		},
	}

	builder := executor.NewRequestBuilder(route, paramMap, "")
	req, err := builder.Build(context.Background(), map[string]interface{}{"Accept": "application/xml"})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if got := req.Header.Get("Accept"); got != "application/xml" {
		t.Fatalf("expected Accept header application/xml, got %q", got)
	}
}
