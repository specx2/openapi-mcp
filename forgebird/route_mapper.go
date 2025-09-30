package forgebird

import (
	"fmt"
	"strings"

	"github.com/specx2/mcp-forgebird/core/interfaces"
	"github.com/specx2/openapi-mcp/core/ir"
	openapimapper "github.com/specx2/openapi-mcp/core/mapper"
)

type openapiRouteMapper struct {
	rules         []openapimapper.RouteMap
	globalTags    []string
	customMapFunc interfaces.MappingFunc
}

// NewOpenAPIRouteMapper constructs a mapper translating OpenAPI routes to MCP components.
func NewOpenAPIRouteMapper() interfaces.RouteMapper {
	return &openapiRouteMapper{}
}

func (m *openapiRouteMapper) MapOperations(operations []interfaces.Operation) ([]interfaces.MappedOperation, error) {
	if len(operations) == 0 {
		return nil, nil
	}

	internal := openapimapper.NewRouteMapper(m.rules)
	if len(m.globalTags) > 0 {
		internal = internal.WithGlobalTags(m.globalTags...)
	}

	lookup := make(map[string]*openapiOperation)

	var irRoutes []ir.HTTPRoute
	for _, op := range operations {
		realOp, ok := op.(*openapiOperation)
		if !ok {
			continue
		}
		key := routeKey(realOp.route)
		lookup[key] = realOp
		irRoutes = append(irRoutes, realOp.route)
	}

	mapped := internal.MapRoutes(irRoutes)

	// 如果有一对多映射函数，扩展结果
	if m.customMapFunc != nil {
		var expandedMapped []openapimapper.MappedRoute
		for _, entry := range mapped {
			// 首先添加原始映射
			expandedMapped = append(expandedMapped, entry)

			// 调用自定义映射函数生成额外的映射
			op := lookup[routeKey(entry.Route)]
			if op != nil {
				if additional, err := m.customMapFunc(op); err == nil && additional != nil {
					// 将额外的映射决策转换为 MappedRoute
					additionalRoute := openapimapper.MappedRoute{
						Route:       entry.Route,
						MCPType:     reverseMCPType(additional.MCPType),
						Tags:        additional.Tags,
						Annotations: additional.Annotations,
					}
					expandedMapped = append(expandedMapped, additionalRoute)
				}
			}
		}
		mapped = expandedMapped
	}

	results := make([]interfaces.MappedOperation, 0, len(mapped))
	for _, entry := range mapped {
		op := lookup[routeKey(entry.Route)]
		if op == nil {
			continue
		}
		name := op.GetName()
		if name == "" {
			name = op.GetID()
		}
		results = append(results, interfaces.MappedOperation{
			Operation:   op,
			MCPType:     convertMCPType(entry.MCPType),
			Name:        name,
			Tags:        mergeTags(entry.Tags, m.globalTags, op.GetTags()),
			Annotations: entry.Annotations,
		})
	}

	return results, nil
}

func (m *openapiRouteMapper) AddRule(rule interfaces.MappingRule) interfaces.RouteMapper {
	clone := openapimapper.NewRouteMap()
	if len(rule.Methods) > 0 {
		clone.WithMethods(rule.Methods...)
	}
	if rule.PathPattern != nil {
		clone.WithPathPattern(rule.PathPattern.String())
	}
	if len(rule.Tags) > 0 {
		clone.WithTags(rule.Tags...)
	}
	clone.WithMCPType(reverseMCPType(rule.MCPType))
	if len(rule.MCPTags) > 0 {
		clone.WithMCPTags(rule.MCPTags...)
	}
	if rule.Annotations != nil {
		clone.WithAnnotations(rule.Annotations)
	}
	m.rules = append(m.rules, *clone)
	return m
}

func (m *openapiRouteMapper) WithMapFunc(fn interfaces.MappingFunc) interfaces.RouteMapper {
	m.customMapFunc = fn
	return m
}

func (m *openapiRouteMapper) WithGlobalTags(tags ...string) interfaces.RouteMapper {
	m.globalTags = append(m.globalTags, tags...)
	return m
}

func convertMCPType(t openapimapper.MCPType) interfaces.MCPType {
	switch t {
	case openapimapper.MCPTypeResource:
		return interfaces.MCPTypeResource
	case openapimapper.MCPTypeResourceTemplate:
		return interfaces.MCPTypeResourceTemplate
	case openapimapper.MCPTypeExclude:
		return interfaces.MCPTypeExclude
	default:
		return interfaces.MCPTypeTool
	}
}

func reverseMCPType(t interfaces.MCPType) openapimapper.MCPType {
	switch t {
	case interfaces.MCPTypeResource:
		return openapimapper.MCPTypeResource
	case interfaces.MCPTypeResourceTemplate:
		return openapimapper.MCPTypeResourceTemplate
	case interfaces.MCPTypeExclude:
		return openapimapper.MCPTypeExclude
	default:
		return openapimapper.MCPTypeTool
	}
}

func mergeTags(tagLists ...[]string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, list := range tagLists {
		for _, tag := range list {
			if tag == "" {
				continue
			}
			lower := strings.TrimSpace(tag)
			if lower == "" {
				continue
			}
			if _, ok := seen[lower]; ok {
				continue
			}
			seen[lower] = struct{}{}
			result = append(result, lower)
		}
	}
	return result
}

func routeKey(route ir.HTTPRoute) string {
	return fmt.Sprintf("%s %s", strings.ToUpper(route.Method), route.Path)
}
