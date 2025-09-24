package mapper

import (
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

type RouteMapFunc func(route ir.HTTPRoute, mappedType MCPType) *MCPType

type RouteMapper struct {
	routeMaps []RouteMap
	mapFunc   RouteMapFunc
}

func NewRouteMapper(routeMaps []RouteMap) *RouteMapper {
	return &RouteMapper{
		routeMaps: routeMaps,
	}
}

func (rm *RouteMapper) WithMapFunc(mapFunc RouteMapFunc) *RouteMapper {
	rm.mapFunc = mapFunc
	return rm
}

func (rm *RouteMapper) MapRoute(route ir.HTTPRoute) MCPType {
	for _, mapping := range rm.routeMaps {
		if rm.matches(route, mapping) {
			mappedType := mapping.MCPType

			if rm.mapFunc != nil {
				if override := rm.mapFunc(route, mappedType); override != nil {
					mappedType = *override
				}
			}

			return mappedType
		}
	}

	return MCPTypeTool
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
	Route   ir.HTTPRoute
	MCPType MCPType
	MCPTags []string
}

func (rm *RouteMapper) MapRoutes(routes []ir.HTTPRoute) []MappedRoute {
	var mappedRoutes []MappedRoute

	for _, route := range routes {
		mcpType := rm.MapRoute(route)

		if mcpType == MCPTypeExclude {
			continue
		}

		var mcpTags []string
		for _, mapping := range rm.routeMaps {
			if rm.matches(route, mapping) {
				mcpTags = append(mcpTags, mapping.MCPTags...)
				break
			}
		}

		mappedRoutes = append(mappedRoutes, MappedRoute{
			Route:   route,
			MCPType: mcpType,
			MCPTags: mcpTags,
		})
	}

	return mappedRoutes
}