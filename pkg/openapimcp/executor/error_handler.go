package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// HTTPError 表示 HTTP 请求错误
type HTTPError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// ErrorHandler 处理各种类型的错误并转换为 MCP 格式
type ErrorHandler struct {
	logLevel string
}

// NewErrorHandler 创建新的错误处理器
func NewErrorHandler(logLevel string) *ErrorHandler {
	return &ErrorHandler{
		logLevel: logLevel,
	}
}

// HandleHTTPError 处理 HTTP 错误
func (eh *ErrorHandler) HandleHTTPError(err error) *mcp.CallToolResult {
	if httpErr, ok := err.(*HTTPError); ok {
		return eh.HandleHTTPStatus(httpErr.StatusCode, httpErr.Message, []byte(httpErr.Body))
	}

	retryable := IsRetryableError(err)
	message := fmt.Sprintf("Request failed: %s", err.Error())
	structured := map[string]interface{}{
		"error":     err.Error(),
		"retryable": retryable,
	}

	return &mcp.CallToolResult{
		IsError:           true,
		Content:           []mcp.Content{mcp.NewTextContent(message)},
		StructuredContent: structured,
	}
}

// HandleValidationError 处理参数验证错误
func (eh *ErrorHandler) HandleValidationError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.NewTextContent("Parameter validation failed: " + err.Error()),
		},
	}
}

// HandleParseError 处理解析错误
func (eh *ErrorHandler) HandleParseError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.NewTextContent("Failed to parse arguments: " + err.Error()),
		},
	}
}

// HandleBuildError 处理请求构建错误
func (eh *ErrorHandler) HandleBuildError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.NewTextContent("Failed to build request: " + err.Error()),
		},
	}
}

// HandleResponseError 处理响应处理错误
func (eh *ErrorHandler) HandleResponseError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.NewTextContent("Failed to process response: " + err.Error()),
		},
	}
}

// HandleHTTPResponse 将 HTTP 响应转换为 MCP 错误结果
func (eh *ErrorHandler) HandleHTTPResponse(resp *http.Response, body []byte) *mcp.CallToolResult {
	statusText := strings.TrimSpace(resp.Status)
	if statusText == "" {
		statusText = http.StatusText(resp.StatusCode)
	}
	return eh.HandleHTTPStatus(resp.StatusCode, statusText, body)
}

// HandleHTTPStatus 处理带状态码与响应体的错误信息
func (eh *ErrorHandler) HandleHTTPStatus(statusCode int, statusText string, body []byte) *mcp.CallToolResult {
	if statusText == "" {
		statusText = getStatusMessage(statusCode)
	}

	message := fmt.Sprintf("HTTP %d: %s", statusCode, statusText)
	structured := map[string]interface{}{
		"status":    statusCode,
		"reason":    statusText,
		"retryable": statusCode >= 500 || statusCode == http.StatusTooManyRequests || statusCode == http.StatusRequestTimeout,
	}

	if len(body) > 0 {
		trimmed := strings.TrimSpace(string(body))
		if json.Valid(body) {
			var jsonBody interface{}
			if err := json.Unmarshal(body, &jsonBody); err == nil {
				structured["body"] = jsonBody
				if pretty, err := formatJSON(body); err == nil {
					message += fmt.Sprintf(" - %s", pretty)
				} else if trimmed != "" {
					message += fmt.Sprintf(" - %s", trimmed)
				}
			} else if trimmed != "" {
				structured["body"] = trimmed
				message += fmt.Sprintf(" - %s", trimmed)
			}
		} else if trimmed != "" {
			structured["body"] = trimmed
			message += fmt.Sprintf(" - %s", trimmed)
		}
	}

	return &mcp.CallToolResult{
		IsError:           true,
		Content:           []mcp.Content{mcp.NewTextContent(message)},
		StructuredContent: structured,
	}
}

// CreateHTTPError 从 HTTP 响应创建错误
func CreateHTTPError(statusCode int, body string) *HTTPError {
	message := http.StatusText(statusCode)
	if message == "" {
		message = getStatusMessage(statusCode)
	}

	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
		Body:       body,
	}
}

// getStatusMessage 获取 HTTP 状态码对应的消息
func getStatusMessage(statusCode int) string {
	switch {
	case statusCode >= 400 && statusCode < 500:
		return "Client Error"
	case statusCode >= 500 && statusCode < 600:
		return "Server Error"
	default:
		return http.StatusText(statusCode)
	}
}

func formatJSON(data []byte) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	if httpErr, ok := err.(*HTTPError); ok {
		// 5xx 错误和部分 4xx 错误可重试
		return httpErr.StatusCode >= 500 ||
			httpErr.StatusCode == 429 || // Too Many Requests
			httpErr.StatusCode == 408 // Request Timeout
	}

	// 网络错误通常可重试
	errorStr := strings.ToLower(err.Error())
	return strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "connection") ||
		strings.Contains(errorStr, "network")
}
