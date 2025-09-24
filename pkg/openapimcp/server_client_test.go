package openapimcp

import (
	"net/http"
	"testing"
	"time"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/executor"
)

func TestPrepareHTTPClientDefaultConfig(t *testing.T) {
	opts := defaultServerOptions()
	opts.HTTPConfig = nil

	client, cfg := prepareHTTPClient(opts)

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Headers == nil {
		t.Fatalf("expected headers map to be initialized")
	}
}

func TestPrepareHTTPClientAppliesConfig(t *testing.T) {
	opts := defaultServerOptions()
	opts.HTTPConfig = &HTTPClientConfig{
		BaseURL: "https://api.example.com",
		Timeout: 5 * time.Second,
		Headers: http.Header{"X-Test": []string{"value"}},
	}

	client, cfg := prepareHTTPClient(opts)

	defaultClient, ok := client.(*executor.DefaultHTTPClient)
	if !ok {
		t.Fatalf("expected default HTTP client, got %T", client)
	}

	if defaultClient.Client().Timeout != 5*time.Second {
		t.Fatalf("expected timeout to be set, got %v", defaultClient.Client().Timeout)
	}

	if defaultClient.Headers().Get("X-Test") != "value" {
		t.Fatalf("expected header to be set")
	}

	if cfg.BaseURL != "https://api.example.com" {
		t.Fatalf("expected config BaseURL to be preserved")
	}
}

func TestNewServerUsesClientConfigBaseURL(t *testing.T) {
	spec := []byte(`{
        "openapi": "3.1.0",
        "info": {"title": "Test", "version": "1.0.0"},
        "paths": {}
    }`)

	srv, err := NewServer(spec,
		WithHTTPClientConfig(&HTTPClientConfig{
			BaseURL: "https://api.example.com",
			Headers: http.Header{"X-Test": []string{"value"}},
		}),
	)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	if srv.options.BaseURL != "https://api.example.com" {
		t.Fatalf("expected options BaseURL to be set from config, got %s", srv.options.BaseURL)
	}

	defaultClient, ok := srv.options.HTTPClient.(*executor.DefaultHTTPClient)
	if !ok {
		t.Fatalf("expected default HTTP client, got %T", srv.options.HTTPClient)
	}

	if defaultClient.Headers().Get("X-Test") != "value" {
		t.Fatalf("expected header to persist on server client")
	}
}

func TestExtractParametersFromURI(t *testing.T) {
	params := extractParametersFromURI(
		"resource://orders/123/items/456",
		"resource://orders/{orderId}/items/{itemId}",
	)

	expected := map[string]string{"orderId": "123", "itemId": "456"}
	if len(params) != len(expected) {
		t.Fatalf("expected %d parameters, got %#v", len(expected), params)
	}
	for key, value := range expected {
		if params[key] != value {
			t.Fatalf("expected %s=%s, got %s", key, value, params[key])
		}
	}
}

func TestExtractParametersFromURIHandlesEncoding(t *testing.T) {
	params := extractParametersFromURI(
		"resource://files/report%202024/version/1",
		"resource://files/{name}/version/{version}",
	)

	if params["name"] != "report 2024" {
		t.Fatalf("expected decoded value, got %q", params["name"])
	}
	if params["version"] != "1" {
		t.Fatalf("expected version to be 1, got %q", params["version"])
	}
}

func TestExtractParametersFromURIMismatchSegments(t *testing.T) {
	params := extractParametersFromURI(
		"resource://users/123",
		"resource://users/{id}/profile",
	)

	if len(params) != 0 {
		t.Fatalf("expected no parameters for mismatched segments, got %#v", params)
	}
}
