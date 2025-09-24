package executor

import (
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
		errorMessage := fmt.Sprintf("HTTP %d: %s", httpErr.StatusCode, httpErr.Message)

		// 添加响应体信息（如果存在）
		if httpErr.Body != "" {
			errorMessage += fmt.Sprintf(" - %s", httpErr.Body)
		}

		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent(errorMessage),
			},
		}
	}

	// 处理其他类型的错误
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.NewTextContent("Request failed: " + err.Error()),
		},
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

// CreateHTTPError 从 HTTP 响应创建错误
func CreateHTTPError(statusCode int, body string) *HTTPError {
	message := getStatusMessage(statusCode)

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
