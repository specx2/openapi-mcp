package executor

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		Header:     http.Header{"Content-Type": []string{"application/json"}},
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
	if result.Result.Meta == nil {
		t.Fatalf("expected meta to be populated")
	}
	status, ok := result.Result.Meta.AdditionalFields["status"].(int)
	if !ok || status != http.StatusInternalServerError {
		t.Fatalf("expected meta status 500, got %v", result.Result.Meta.AdditionalFields["status"])
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

func TestResponseProcessorProcessSuccessMeta(t *testing.T) {
	processor := NewResponseProcessor(nil, false, NewErrorHandler("info"))

	reqURL, _ := url.Parse("https://api.example.com/widgets")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"value":"ok"}`)),
		Request:    &http.Request{Method: "GET", URL: reqURL},
	}

	result, err := processor.Process(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result")
	}
	structured, ok := result.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured content map")
	}
	if structured["value"] != "ok" {
		t.Fatalf("unexpected structured content %v", structured)
	}
	if result.Result.Meta == nil {
		t.Fatalf("expected meta to be populated")
	}
	if status, ok := result.Result.Meta.AdditionalFields["status"].(int); !ok || status != http.StatusOK {
		t.Fatalf("expected status 200 in meta, got %v", result.Result.Meta.AdditionalFields["status"])
	}
	if method, ok := result.Result.Meta.AdditionalFields["requestMethod"].(string); !ok || method != "GET" {
		t.Fatalf("expected request method in meta, got %v", result.Result.Meta.AdditionalFields["requestMethod"])
	}
}

func TestResponseProcessorProcessNoContent(t *testing.T) {
	processor := NewResponseProcessor(nil, true, NewErrorHandler("info"))
	resp := &http.Response{
		StatusCode: http.StatusNoContent,
		Status:     "204 No Content",
		Body:       io.NopCloser(strings.NewReader("")),
	}

	result, err := processor.Process(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result")
	}
	structured, ok := result.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured content map")
	}
	if _, exists := structured["result"]; !exists {
		t.Fatalf("expected wrap result map to include 'result' key")
	}
	if result.Result.Meta == nil {
		t.Fatalf("expected meta to be populated")
	}
	if status, ok := result.Result.Meta.AdditionalFields["status"].(int); !ok || status != http.StatusNoContent {
		t.Fatalf("expected status 204 in meta, got %v", result.Result.Meta.AdditionalFields["status"])
	}
}
