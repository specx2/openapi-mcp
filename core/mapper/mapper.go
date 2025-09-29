package mapper

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/specx2/openapi-mcp/core/ir"
)

type RouteMapFunc func(route ir.HTTPRoute, decision RouteDecision) *RouteDecision

type RouteMapper struct {
	routeMaps []RouteMap
	mapFunc   RouteMapFunc
	globTags  []string
}

func NewRouteMapper(routeMaps []RouteMap) *RouteMapper {
	clone := make([]RouteMap, len(routeMaps))
	copy(clone, routeMaps)
	clone = append(clone, DefaultRouteMappings()...)
	return &RouteMapper{
		routeMaps: clone,
	}
}

func (rm *RouteMapper) WithMapFunc(mapFunc RouteMapFunc) *RouteMapper {
	rm.mapFunc = mapFunc
	return rm
}

func (rm *RouteMapper) WithGlobalTags(tags ...string) *RouteMapper {
	rm.globTags = uniqueStrings(tags)
	return rm
}

func (rm *RouteMapper) matches(route ir.HTTPRoute, mapping RouteMap) bool {
	if !rm.matchesMethods(route.Method, mapping.Methods) {
		return false
	}

	if !mapping.PathPattern.MatchString(route.Path) {
		return false
	}

	if len(mapping.Tags) > 0 {
		if !rm.matchesTags(route.Tags, mapping.Tags) {
			return false
		}
	}

	return true
}

func (rm *RouteMapper) matchesMethods(method string, allowedMethods []string) bool {
	for _, allowed := range allowedMethods {
		if allowed == "*" || allowed == method {
			return true
		}
	}
	return false
}

func (rm *RouteMapper) matchesTags(routeTags []string, requiredTags []string) bool {
	routeTagSet := make(map[string]bool)
	for _, tag := range routeTags {
		routeTagSet[tag] = true
	}

	for _, required := range requiredTags {
		if !routeTagSet[required] {
			return false
		}
	}

	return true
}

type MappedRoute struct {
	Route       ir.HTTPRoute
	MCPType     MCPType
	Tags        []string
	Annotations *mcp.ToolAnnotation
}

func (rm *RouteMapper) MapRoutes(routes []ir.HTTPRoute) []MappedRoute {
	var mappedRoutes []MappedRoute

	for _, route := range routes {
		decision := rm.MapRouteDecision(route)

		if decision.MCPType == MCPTypeExclude {
			continue
		}

		mappedRoutes = append(mappedRoutes, MappedRoute{
			Route:       route,
			MCPType:     decision.MCPType,
			Tags:        decision.Tags,
			Annotations: decision.Annotations,
		})
	}

	return mappedRoutes
}

type RouteDecision struct {
	MCPType     MCPType
	Tags        []string
	Annotations *mcp.ToolAnnotation
}

func (rm *RouteMapper) MapRouteDecision(route ir.HTTPRoute) RouteDecision {
	decision := RouteDecision{
		MCPType: MCPTypeTool,
		Tags:    rm.combineTags(route, nil),
	}

	for idx := range rm.routeMaps {
		mapping := rm.routeMaps[idx]
		if !rm.matches(route, mapping) {
			continue
		}

		decision.MCPType = mapping.MCPType
		decision.Tags = rm.combineTags(route, &mapping)
		if mapping.Annotations != nil {
			clone := *mapping.Annotations
			decision.Annotations = &clone
		}
		break
	}

	if rm.mapFunc != nil {
		if override := rm.mapFunc(route, decision); override != nil {
			decision = *override
			decision.Tags = uniqueStrings(decision.Tags)
		}
	}

	decision.Tags = uniqueStrings(decision.Tags)
	return decision
}

func (rm *RouteMapper) MapRoute(route ir.HTTPRoute) MCPType {
	return rm.MapRouteDecision(route).MCPType
}

func (rm *RouteMapper) combineTags(route ir.HTTPRoute, mapping *RouteMap) []string {
	var combined []string
	combined = append(combined, route.Tags...)
	if mapping != nil {
		combined = append(combined, mapping.MCPTags...)
	}
	combined = append(combined, rm.globTags...)
	return uniqueStrings(combined)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		lower := strings.TrimSpace(v)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		result = append(result, lower)
	}
	return result
}
