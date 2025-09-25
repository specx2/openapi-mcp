package factory

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/executor"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/mapper"
)

type ComponentFunc func(route ir.HTTPRoute, component interface{})

type ComponentFactory struct {
	client      executor.HTTPClient
	baseURL     string
	nameCounter map[string]map[string]int
	customNames map[string]string
	componentFn ComponentFunc
}

func NewComponentFactory(client executor.HTTPClient, baseURL string) *ComponentFactory {
	return &ComponentFactory{
		client:      client,
		baseURL:     baseURL,
		nameCounter: make(map[string]map[string]int),
		customNames: make(map[string]string),
	}
}

func (cf *ComponentFactory) WithCustomNames(names map[string]string) *ComponentFactory {
	cf.customNames = normalizeCustomNames(names)
	return cf
}

func (cf *ComponentFactory) WithComponentFunc(fn ComponentFunc) *ComponentFactory {
	cf.componentFn = fn
	return cf
}

func (cf *ComponentFactory) CreateComponents(mappedRoutes []mapper.MappedRoute) ([]interface{}, error) {
	var components []interface{}

	for _, mapped := range mappedRoutes {
		switch mapped.MCPType {
		case mapper.MCPTypeTool:
			tool, err := cf.CreateTool(mapped.Route, mapped.Tags, mapped.Annotations)
			if err != nil {
				return nil, err
			}
			components = append(components, tool)

		case mapper.MCPTypeResource:
			resource, err := cf.CreateResource(mapped.Route, mapped.Tags)
			if err != nil {
				return nil, err
			}
			components = append(components, resource)

		case mapper.MCPTypeResourceTemplate:
			template, err := cf.CreateResourceTemplate(mapped.Route, mapped.Tags)
			if err != nil {
				return nil, err
			}
			components = append(components, template)
		}
	}

	return components, nil
}

func (cf *ComponentFactory) CreateTool(route ir.HTTPRoute, tags []string, annotations *mcp.ToolAnnotation) (*executor.OpenAPITool, error) {
	inputSchema, paramMap, err := cf.combineSchemas(route)
	if err != nil {
		return nil, err
	}

	outputSchema, wrapResult := cf.extractOutputSchema(route)

	name := cf.generateName(route, "tool")

	description := cf.formatDescription(route)

	// Persist parameter map on the route for downstream consumers (parity with fastmcp)
	route.ParameterMap = paramMap

	tool := executor.NewOpenAPITool(
		name,
		description,
		inputSchema,
		outputSchema,
		wrapResult,
		route,
		cf.client,
		cf.baseURL,
		paramMap,
		tags,
		annotations,
	)

	if cf.componentFn != nil {
		cf.componentFn(route, tool)
	}

	return tool, nil
}

func (cf *ComponentFactory) CreateResource(route ir.HTTPRoute, tags []string) (*executor.OpenAPIResource, error) {
	name := cf.generateName(route, "resource")
	description := cf.formatDescription(route)

	resource := executor.NewOpenAPIResource(
		name,
		description,
		route,
		cf.client,
		cf.baseURL,
	)

	if cf.componentFn != nil {
		cf.componentFn(route, resource)
	}

	return resource, nil
}

func (cf *ComponentFactory) CreateResourceTemplate(route ir.HTTPRoute, tags []string) (*executor.OpenAPIResourceTemplate, error) {
	name := cf.generateName(route, "resource_template")
	description := cf.formatDescription(route)

	template := executor.NewOpenAPIResourceTemplate(
		name,
		description,
		route,
		cf.client,
		cf.baseURL,
	)

	if cf.componentFn != nil {
		cf.componentFn(route, template)
	}

	return template, nil
}
