package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

	meta := buildResponseMeta(resp)

	if resp.StatusCode >= 400 {
		return rp.processError(resp, meta)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		structured := rp.prepareStructuredResult(nil)
		return &mcp.CallToolResult{
			StructuredContent: structured,
			Result:            mcp.Result{Meta: cloneMeta(meta)},
		}, nil
	}

	var result interface{}
	if err := json.Unmarshal(trimmed, &result); err == nil {
		toolResult, err := rp.processJSON(result)
		if err != nil {
			return nil, err
		}
		toolResult.Result.Meta = mergeMeta(toolResult.Result.Meta, meta)
		return toolResult, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(trimmed)),
		},
		Result: mcp.Result{Meta: cloneMeta(meta)},
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
		return map[string]interface{}{"result": result}
	}
	if resultMap, ok := result.(map[string]interface{}); ok {
		return resultMap
	}
	return map[string]interface{}{"result": result}
}

func buildResponseMeta(resp *http.Response) *mcp.Meta {
	if resp == nil {
		return nil
	}

	fields := make(map[string]any)
	fields["status"] = resp.StatusCode

	statusText := strings.TrimSpace(resp.Status)
	if statusText == "" {
		statusText = http.StatusText(resp.StatusCode)
	}
	if statusText != "" {
		fields["statusText"] = statusText
	}

	if resp.Request != nil {
		if resp.Request.Method != "" {
			fields["requestMethod"] = resp.Request.Method
		}
		if resp.Request.URL != nil {
			fields["requestURL"] = resp.Request.URL.String()
		}
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		fields["contentType"] = ct
	}

	if len(resp.Header) > 0 {
		headers := make(map[string][]string, len(resp.Header))
		for k, values := range resp.Header {
			copied := make([]string, len(values))
			copy(copied, values)
			headers[k] = copied
		}
		fields["headers"] = headers
	}

	if len(fields) == 0 {
		return nil
	}

	return mcp.NewMetaFromMap(fields)
}

func mergeMeta(existing, additional *mcp.Meta) *mcp.Meta {
	if additional == nil {
		return existing
	}
	if existing == nil {
		return cloneMeta(additional)
	}

	if existing.ProgressToken == nil && additional.ProgressToken != nil {
		existing.ProgressToken = additional.ProgressToken
	}

	if len(additional.AdditionalFields) > 0 {
		if existing.AdditionalFields == nil {
			existing.AdditionalFields = make(map[string]any, len(additional.AdditionalFields))
		}
		for k, v := range additional.AdditionalFields {
			if _, exists := existing.AdditionalFields[k]; !exists {
				existing.AdditionalFields[k] = v
			}
		}
	}

	return existing
}

func cloneMeta(meta *mcp.Meta) *mcp.Meta {
	if meta == nil {
		return nil
	}

	fields := make(map[string]any, len(meta.AdditionalFields)+1)
	for k, v := range meta.AdditionalFields {
		fields[k] = v
	}
	if meta.ProgressToken != nil {
		fields["progressToken"] = meta.ProgressToken
	}

	return mcp.NewMetaFromMap(fields)
}

func (rp *ResponseProcessor) processError(resp *http.Response, meta *mcp.Meta) (*mcp.CallToolResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if rp.errorHandler != nil {
			result := rp.errorHandler.HandleHTTPStatus(resp.StatusCode, resp.Status, nil)
			result.Result.Meta = mergeMeta(result.Result.Meta, meta)
			return result, nil
		}
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)),
			},
			Result: mcp.Result{Meta: cloneMeta(meta)},
		}, nil
	}

	if rp.errorHandler != nil {
		result := rp.errorHandler.HandleHTTPResponse(resp, body)
		result.Result.Meta = mergeMeta(result.Result.Meta, meta)
		return result, nil
	}

	var errorData map[string]interface{}
	if json.Unmarshal(body, &errorData) == nil {
		errorJSON, _ := json.Marshal(errorData)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(errorJSON))),
			},
			Result: mcp.Result{Meta: cloneMeta(meta)},
		}, nil
	}

	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))),
		},
		Result: mcp.Result{Meta: cloneMeta(meta)},
	}, nil
}
