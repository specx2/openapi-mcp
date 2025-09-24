package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

type contextKey string

const mcpHeadersKey contextKey = "mcp_headers"

func GetMCPHeaders(ctx context.Context) map[string]string {
	if headers, ok := ctx.Value(mcpHeadersKey).(map[string]string); ok {
		return headers
	}
	return nil
}

func SetMCPHeaders(ctx context.Context, headers map[string]string) context.Context {
	return context.WithValue(ctx, mcpHeadersKey, headers)
}

func ParseArguments(request mcp.CallToolRequest) (map[string]interface{}, error) {
	args := request.GetArguments()
	if args == nil {
		return make(map[string]interface{}), nil
	}
	return args, nil
}

func MarshalJSONSchema(schema interface{}) (string, error) {
	if schema == nil {
		return "", nil
	}

	bytes, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema: %w", err)
	}

	return string(bytes), nil
}