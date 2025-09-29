package forgebird

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/specx2/mcp-forgebird/core/interfaces"
	executorpkg "github.com/specx2/openapi-mcp/core/executor"
	openapifactory "github.com/specx2/openapi-mcp/core/factory"
)

type executorFactory struct {
	baseURL string
	timeout time.Duration
}

// NewExecutorFactory constructs an executor factory wired to the plugin configuration.
func NewExecutorFactory(config interfaces.ConversionConfig) interfaces.ExecutorFactory {
	timeout := time.Duration(config.Timeout) * time.Second
	if config.Timeout <= 0 {
		timeout = 0
	}
	return &executorFactory{
		baseURL: config.BaseURL,
		timeout: timeout,
	}
}

func (f *executorFactory) CreateExecutor(operation interfaces.Operation, config interfaces.ConversionConfig) (interfaces.OperationExecutor, error) {
	op, ok := operation.(*openapiOperation)
	if !ok {
		return nil, fmt.Errorf("unsupported operation type %T", operation)
	}

	httpClient := executorpkg.NewDefaultHTTPClient()
	if f.timeout > 0 {
		httpClient.WithTimeout(f.timeout)
	}

	factory := openapifactory.NewComponentFactory(httpClient, f.baseURL)
	if len(config.Mapping.CustomNames) > 0 {
		factory = factory.WithCustomNames(config.Mapping.CustomNames)
	}

	tool, err := factory.CreateTool(op.route, op.tags, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor for %s: %w", op.id, err)
	}

	exec := &openapiOperationExecutor{
		tool:   tool,
		client: httpClient,
	}
	op.SetExecutor(exec)

	return exec, nil
}

func (f *executorFactory) GetClientType() string {
	return "http"
}

func (f *executorFactory) CreateClient(config interfaces.ConversionConfig) (interface{}, error) {
	client := executorpkg.NewDefaultHTTPClient()
	if f.timeout > 0 {
		client.WithTimeout(f.timeout)
	}
	return client, nil
}

func (f *executorFactory) ValidateConfig(config interfaces.ConversionConfig) error {
	return nil
}

type openapiOperationExecutor struct {
	tool   *executorpkg.OpenAPITool
	client executorpkg.HTTPClient
}

func (e *openapiOperationExecutor) Execute(ctx context.Context, args map[string]interface{}) (*interfaces.ExecutionResult, error) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      e.tool.Tool().Name,
			Arguments: args,
		},
	}

	result, err := e.tool.Run(ctx, req)
	if err != nil {
		return nil, err
	}

	execResult := &interfaces.ExecutionResult{
		Data:     result.StructuredContent,
		Metadata: convertMeta(result.Result.Meta),
	}

	if len(result.Content) > 0 {
		execResult.Metadata["content"] = result.Content
	}

	if result.IsError {
		execResult.Error = &interfaces.ExecutionError{
			Code:    "tool_error",
			Message: "OpenAPI tool execution returned an error",
			Details: result.StructuredContent,
		}
	}

	return execResult, nil
}

func (e *openapiOperationExecutor) Validate(args map[string]interface{}) error {
	return nil
}

func (e *openapiOperationExecutor) GetClient() interface{} {
	return e.client
}

func convertMeta(meta *mcp.Meta) map[string]interface{} {
	if meta == nil {
		return make(map[string]interface{})
	}

	result := make(map[string]interface{}, len(meta.AdditionalFields)+1)
	for k, v := range meta.AdditionalFields {
		result[k] = cloneGenericValue(v)
	}
	if meta.ProgressToken != nil {
		result["progressToken"] = meta.ProgressToken
	}
	return result
}
