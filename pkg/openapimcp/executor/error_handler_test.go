package executor

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

func TestHandleHTTPResponseJSON(t *testing.T) {
	handler := NewErrorHandler("info")

	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Status:     "404 Not Found",
	}

	bodyBytes := []byte(`{"error":"not found","detail":"missing"}`)
	result := handler.HandleHTTPResponse(resp, bodyBytes)

	if !result.IsError {
		t.Fatalf("expected IsError to be true")
	}

	if len(result.Content) == 0 {
		t.Fatalf("expected content message")
	}

	content, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}

	msg := content.Text
	if !strings.Contains(msg, "HTTP 404") {
		t.Fatalf("expected message to contain status, got %s", msg)
	}
	if !strings.Contains(msg, "not found") {
		t.Fatalf("expected message to contain body, got %s", msg)
	}

	structured, ok := result.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured content")
	}

	if status, ok := structured["status"].(int); !ok || status != http.StatusNotFound {
		t.Fatalf("expected structured status 404, got %v", structured["status"])
	}

	body, ok := structured["body"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured body map, got %T", structured["body"])
	}
	if body["error"] != "not found" {
		t.Fatalf("expected body.error to be 'not found', got %v", body["error"])
	}
}

func TestHandleHTTPErrorNetwork(t *testing.T) {
	handler := NewErrorHandler("info")

	result := handler.HandleHTTPError(fmt.Errorf("timeout while connecting"))

	if !result.IsError {
		t.Fatalf("expected error result")
	}
	if len(result.Content) == 0 {
		t.Fatalf("expected content message")
	}
	content, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if !strings.Contains(content.Text, "Request failed") {
		t.Fatalf("expected request failed message, got %s", content.Text)
	}
	structured, ok := result.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured content map")
	}
	if retryable, ok := structured["retryable"].(bool); !ok || !retryable {
		t.Fatalf("expected retryable=true, got %v", structured["retryable"])
	}
}

func TestResponseProcessorProcessErrorUsesHandler(t *testing.T) {
	handler := NewErrorHandler("info")
	processor := NewResponseProcessor(nil, false, handler)

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       io.NopCloser(strings.NewReader(`{"error":"boom"}`)),
	}

	result, err := processor.Process(resp)
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true")
	}
	if len(result.Content) == 0 {
		t.Fatalf("expected content message")
	}
	content, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if !strings.Contains(content.Text, "HTTP 500") {
		t.Fatalf("unexpected content: %s", content.Text)
	}
}

func TestResponseProcessorValidationFailure(t *testing.T) {
	schema := ir.Schema{
		"type": "object",
		"properties": map[string]interface{}{
			"value": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"value"},
	}

	processor := NewResponseProcessor(schema, false, NewErrorHandler("info"))
	result, err := processor.processJSON(map[string]interface{}{"other": "x"})
	if err != nil {
		t.Fatalf("processJSON returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected validation failure to produce error result")
	}
	if len(result.Content) == 0 {
		t.Fatalf("expected error message content")
	}
	message := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(message, "Response validation failed") {
		t.Fatalf("expected validation message, got %s", message)
	}
}

func TestResponseProcessorValidationSuccess(t *testing.T) {
	schema := ir.Schema{
		"type": "object",
		"properties": map[string]interface{}{
			"value": map[string]interface{}{"type": "string"},
		},
	}
	processor := NewResponseProcessor(schema, false, NewErrorHandler("info"))
	result, err := processor.processJSON(map[string]interface{}{"value": "hello"})
	if err != nil {
		t.Fatalf("processJSON returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result")
	}
	structured, ok := result.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured content map")
	}
	if structured["value"] != "hello" {
		t.Fatalf("unexpected structured content %v", structured)
	}
}
