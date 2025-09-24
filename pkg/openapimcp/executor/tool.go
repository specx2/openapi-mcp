package executor

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/internal"
)

type OpenAPITool struct {
	tool         mcp.Tool
	route        ir.HTTPRoute
	client       HTTPClient
	baseURL      string
	paramMap     map[string]ir.ParamMapping
	outputSchema ir.Schema
	wrapResult   bool
}

func NewOpenAPITool(
	name string,
	description string,
	inputSchema ir.Schema,
	outputSchema ir.Schema,
	wrapResult bool,
	route ir.HTTPRoute,
	client HTTPClient,
	baseURL string,
	paramMap map[string]ir.ParamMapping,
) *OpenAPITool {
	inputSchemaJSON, _ := json.Marshal(inputSchema)
	var outputSchemaJSON json.RawMessage
	if outputSchema != nil {
		outputSchemaJSON, _ = json.Marshal(outputSchema)
	}

	tool := mcp.NewTool(name,
		mcp.WithDescription(description),
		mcp.WithRawInputSchema(inputSchemaJSON),
	)

	if outputSchema != nil {
		tool = mcp.NewTool(name,
			mcp.WithDescription(description),
			mcp.WithRawInputSchema(inputSchemaJSON),
			mcp.WithRawOutputSchema(outputSchemaJSON),
		)
	}

	return &OpenAPITool{
		tool:         tool,
		route:        route,
		client:       client,
		baseURL:      baseURL,
		paramMap:     paramMap,
		outputSchema: outputSchema,
		wrapResult:   wrapResult,
	}
}

func (t *OpenAPITool) Tool() mcp.Tool {
	return t.tool
}

func (t *OpenAPITool) SetTool(tool mcp.Tool) {
	t.tool = tool
}

func (t *OpenAPITool) Run(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, err := internal.ParseArguments(request)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent("Failed to parse arguments: " + err.Error()),
			},
		}, nil
	}

	builder := NewRequestBuilder(t.route, t.paramMap, t.baseURL)
	httpReq, err := builder.Build(ctx, args)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent("Failed to build request: " + err.Error()),
			},
		}, nil
	}

	if mcpHeaders := internal.GetMCPHeaders(ctx); mcpHeaders != nil {
		for k, v := range mcpHeaders {
			httpReq.Header.Set(k, v)
		}
	}

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent("Request failed: " + err.Error()),
			},
		}, nil
	}

	processor := NewResponseProcessor(t.outputSchema, t.wrapResult)
	return processor.Process(resp)
}