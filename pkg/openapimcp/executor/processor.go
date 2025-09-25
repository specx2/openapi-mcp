package executor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

type ResponseProcessor struct {
	outputSchema ir.Schema
	wrapResult   bool
	errorHandler *ErrorHandler
	validator    *jsonschema.Schema
}

func NewResponseProcessor(outputSchema ir.Schema, wrapResult bool, errorHandler *ErrorHandler) *ResponseProcessor {
	var validator *jsonschema.Schema
	if outputSchema != nil {
		validator = compileIRSchema(outputSchema)
	}
	return &ResponseProcessor{
		outputSchema: outputSchema,
		wrapResult:   wrapResult,
		errorHandler: errorHandler,
		validator:    validator,
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
	structured := rp.prepareStructuredResult(result)
	if rp.validator != nil {
		if err := rp.validator.Validate(structured); err != nil {
			if rp.errorHandler != nil {
				return rp.errorHandler.HandleResponseValidationError(err), nil
			}
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					mcp.NewTextContent("Response validation failed: " + err.Error()),
				},
			}, nil
		}
	}
	return &mcp.CallToolResult{
		StructuredContent: structured,
	}, nil
}

func (rp *ResponseProcessor) prepareStructuredResult(result interface{}) map[string]interface{} {
	if rp.wrapResult {
		return map[string]interface{}{ "result": result }
	}
	if resultMap, ok := result.(map[string]interface{}); ok {
		return resultMap
	}
	return map[string]interface{}{ "result": result }
}

func (rp *ResponseProcessor) processError(resp *http.Response) (*mcp.CallToolResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if rp.errorHandler != nil {
			return rp.errorHandler.HandleHTTPStatus(resp.StatusCode, resp.Status, nil), nil
		}
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)),
			},
		}, nil
	}

	if rp.errorHandler != nil {
		return rp.errorHandler.HandleHTTPResponse(resp, body), nil
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
