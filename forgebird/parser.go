package forgebird

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/specx2/mcp-forgebird/core/interfaces"
	"github.com/specx2/openapi-mcp/core/executor"
	openapifactory "github.com/specx2/openapi-mcp/core/factory"
	"github.com/specx2/openapi-mcp/core/ir"
	openapiparser "github.com/specx2/openapi-mcp/core/parser"
)

type parser struct {
	config  interfaces.ConversionConfig
	version string
	backend openapiparser.OpenAPIParser
}

// NewParser constructs a new OpenAPI parser bound to the provided plugin configuration.
func NewParser(config interfaces.ConversionConfig) interfaces.SpecParser {
	return &parser{config: config}
}

func (p *parser) ParseSpec(data []byte) ([]interfaces.Operation, error) {
	parserOpts := []openapiparser.ParserOption{}
	if specURL := strings.TrimSpace(p.config.Spec.SpecURL); specURL != "" {
		parserOpts = append(parserOpts, openapiparser.WithSpecURL(specURL))
	}

	backend, err := openapiparser.NewParser(data, parserOpts...)
	if err != nil {
		return nil, err
	}

	routes, err := backend.ParseSpec(data)
	if err != nil {
		return nil, err
	}

	p.backend = backend
	p.version = backend.GetVersion()

	operations := make([]interfaces.Operation, 0, len(routes))
	for _, route := range routes {
		op, err := buildOperationFromRoute(route)
		if err != nil {
			return nil, fmt.Errorf("failed to build operation for %s %s: %w", route.Method, route.Path, err)
		}
		operations = append(operations, op)
	}

	return operations, nil
}

func (p *parser) GetVersion() string {
	return p.version
}

func (p *parser) Validate() error {
	if p.backend == nil {
		return nil
	}
	return p.backend.Validate()
}

func (p *parser) GetInfo() *interfaces.SpecInfo {
	return &interfaces.SpecInfo{
		Version: p.version,
	}
}

func buildOperationFromRoute(route ir.HTTPRoute) (*openapiOperation, error) {
	dummyClient := executor.NewDefaultHTTPClient()
	cf := openapifactory.NewComponentFactory(dummyClient, "")

	tool, err := cf.CreateTool(route, route.Tags, nil)
	if err != nil {
		return nil, err
	}

	inputSchema, err := decodeSchema(tool.Tool().RawInputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to decode input schema: %w", err)
	}

	outputSchema, err := decodeSchema(tool.Tool().RawOutputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to decode output schema: %w", err)
	}

	paramMap := tool.ParameterMappings()

	return newOpenAPIOperation(route, inputSchema, outputSchema, interfaces.Schema(cloneGenericMap(route.SchemaDefs)), paramMap), nil
}

func decodeSchema(raw json.RawMessage) (interfaces.Schema, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	return interfaces.Schema(schema), nil
}

func cloneGenericMap(value interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}
	if original, ok := value.(map[string]interface{}); ok {
		clone := make(map[string]interface{}, len(original))
		for k, v := range original {
			clone[k] = cloneGenericValue(v)
		}
		return clone
	}
	return nil
}

func cloneGenericValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return cloneGenericMap(v)
	case []interface{}:
		cloned := make([]interface{}, len(v))
		for i, item := range v {
			cloned[i] = cloneGenericValue(item)
		}
		return cloned
	default:
		return v
	}
}
