package mapper

import "regexp"

func DefaultRouteMappings() []RouteMap {
	return []RouteMap{
		{
			Methods:     []string{"*"},
			PathPattern: regexp.MustCompile(".*"),
			MCPType:     MCPTypeTool,
		},
	}
}

func SmartRouteMappings() []RouteMap {
	return []RouteMap{
		{
			Methods:     []string{"GET"},
			PathPattern: regexp.MustCompile(`.*\{.*\}.*`),
			MCPType:     MCPTypeResourceTemplate,
		},
		{
			Methods:     []string{"GET"},
			PathPattern: regexp.MustCompile(".*"),
			MCPType:     MCPTypeResource,
		},
		{
			Methods:     []string{"*"},
			PathPattern: regexp.MustCompile(".*"),
			MCPType:     MCPTypeTool,
		},
	}
}

func ResourceOnlyMappings() []RouteMap {
	return []RouteMap{
		{
			Methods:     []string{"GET"},
			PathPattern: regexp.MustCompile(`.*\{.*\}.*`),
			MCPType:     MCPTypeResourceTemplate,
		},
		{
			Methods:     []string{"GET"},
			PathPattern: regexp.MustCompile(".*"),
			MCPType:     MCPTypeResource,
		},
		{
			Methods:     []string{"*"},
			PathPattern: regexp.MustCompile(".*"),
			MCPType:     MCPTypeExclude,
		},
	}
}

func ToolOnlyMappings() []RouteMap {
	return []RouteMap{
		{
			Methods:     []string{"*"},
			PathPattern: regexp.MustCompile(".*"),
			MCPType:     MCPTypeTool,
		},
	}
}

func AdminExcludeMappings() []RouteMap {
	return []RouteMap{
		{
			Methods:     []string{"*"},
			PathPattern: regexp.MustCompile(`^/admin/.*`),
			MCPType:     MCPTypeExclude,
		},
		{
			Methods:     []string{"GET"},
			PathPattern: regexp.MustCompile(`.*\{.*\}.*`),
			MCPType:     MCPTypeResourceTemplate,
		},
		{
			Methods:     []string{"GET"},
			PathPattern: regexp.MustCompile(".*"),
			MCPType:     MCPTypeResource,
		},
		{
			Methods:     []string{"*"},
			PathPattern: regexp.MustCompile(".*"),
			MCPType:     MCPTypeTool,
		},
	}
}