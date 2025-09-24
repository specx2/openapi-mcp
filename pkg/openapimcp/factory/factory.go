package factory

import (
	"fmt"

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
	cf.customNames = names
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
			tool, err := cf.CreateTool(mapped.Route, mapped.MCPTags)
			if err != nil {
				return nil, err
			}
			components = append(components, tool)

		case mapper.MCPTypeResource:
			resource, err := cf.CreateResource(mapped.Route, mapped.MCPTags)
			if err != nil {
				return nil, err
			}
			components = append(components, resource)

		case mapper.MCPTypeResourceTemplate:
			template, err := cf.CreateResourceTemplate(mapped.Route, mapped.MCPTags)
			if err != nil {
				return nil, err
			}
			components = append(components, template)
		}
	}

	return components, nil
}

func (cf *ComponentFactory) CreateTool(route ir.HTTPRoute, tags []string) (*executor.OpenAPITool, error) {
	// 检测参数冲突并生成映射
	paramMap := cf.DetectParameterCollisions(route)

	inputSchema, err := cf.combineSchemas(route, paramMap)
	if err != nil {
		return nil, err
	}

	outputSchema, wrapResult := cf.extractOutputSchema(route)

	name := cf.generateName(route, "tool")

	description := cf.formatDescription(route)

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

// DetectParameterCollisions 检测参数名冲突并生成参数映射
func (cf *ComponentFactory) DetectParameterCollisions(route ir.HTTPRoute) map[string]ir.ParamMapping {
	paramMap := make(map[string]ir.ParamMapping)

	// 提取请求体属性名
	bodyProps := cf.extractBodyProperties(route)

	// 处理路径、查询、头部参数
	for _, param := range route.Parameters {
		if bodyProps[param.Name] {
			// 参数冲突，添加位置后缀
			suffixedName := fmt.Sprintf("%s__%s", param.Name, param.In)
			paramMap[suffixedName] = ir.ParamMapping{
				OpenAPIName:  param.Name,
				Location:     param.In,
				IsSuffixed:   true,
				OriginalName: param.Name,
			}
		} else {
			// 无冲突，使用原始名称
			paramMap[param.Name] = ir.ParamMapping{
				OpenAPIName:  param.Name,
				Location:     param.In,
				IsSuffixed:   false,
				OriginalName: param.Name,
			}
		}
	}

	return paramMap
}

// extractBodyProperties 提取请求体属性名集合
func (cf *ComponentFactory) extractBodyProperties(route ir.HTTPRoute) map[string]bool {
	bodyProps := make(map[string]bool)

	if route.RequestBody != nil && route.RequestBody.ContentSchemas != nil {
		// 获取第一个内容类型的 schema
		for _, schema := range route.RequestBody.ContentSchemas {
			if schema != nil && schema["properties"] != nil {
				if props, ok := schema["properties"].(map[string]interface{}); ok {
					for propName := range props {
						bodyProps[propName] = true
					}
				}
			}
			break // 只处理第一个内容类型
		}
	}

	return bodyProps
}
