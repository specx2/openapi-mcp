package executor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

type ResponseProcessor struct {
	outputSchema ir.Schema
	wrapResult   bool
}

func NewResponseProcessor(outputSchema ir.Schema, wrapResult bool) *ResponseProcessor {
	return &ResponseProcessor{
		outputSchema: outputSchema,
		wrapResult:   wrapResult,
	}
}

func (rp *ResponseProcessor) Process(resp *http.Response) (*mcp.CallToolResult, error) {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return rp.processError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		return rp.processJSON(result)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(body)),
		},
	}, nil
}

func (rp *ResponseProcessor) processJSON(result interface{}) (*mcp.CallToolResult, error) {
	if rp.wrapResult {
		return &mcp.CallToolResult{
			StructuredContent: map[string]interface{}{
				"result": result,
			},
		}, nil
	}

	if resultMap, ok := result.(map[string]interface{}); ok {
		return &mcp.CallToolResult{
			StructuredContent: resultMap,
		}, nil
	}

	return &mcp.CallToolResult{
		StructuredContent: map[string]interface{}{
			"result": result,
		},
	}, nil
}

func (rp *ResponseProcessor) processError(resp *http.Response) (*mcp.CallToolResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)),
			},
		}, nil
	}

	var errorData map[string]interface{}
	if json.Unmarshal(body, &errorData) == nil {
		errorJSON, _ := json.Marshal(errorData)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(errorJSON))),
			},
		}, nil
	}

	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))),
		},
	}, nil
}