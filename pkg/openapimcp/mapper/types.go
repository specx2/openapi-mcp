package mapper

import (
	"regexp"

	"github.com/mark3labs/mcp-go/mcp"
)

type MCPType string

const (
	MCPTypeTool             MCPType = "tool"
	MCPTypeResource         MCPType = "resource"
	MCPTypeResourceTemplate MCPType = "resource_template"
	MCPTypeExclude          MCPType = "exclude"
)

type RouteMap struct {
	Methods     []string
	PathPattern *regexp.Regexp
	Tags        []string
	MCPType     MCPType
	MCPTags     []string
	Annotations *mcp.ToolAnnotation
}

func NewRouteMap() *RouteMap {
	return &RouteMap{
		Methods:     []string{"*"},
		PathPattern: regexp.MustCompile(".*"),
		MCPType:     MCPTypeTool,
	}
}

func (rm *RouteMap) WithMethods(methods ...string) *RouteMap {
	rm.Methods = methods
	return rm
}

func (rm *RouteMap) WithPathPattern(pattern string) *RouteMap {
	rm.PathPattern = regexp.MustCompile(pattern)
	return rm
}

func (rm *RouteMap) WithTags(tags ...string) *RouteMap {
	rm.Tags = tags
	return rm
}

func (rm *RouteMap) WithMCPType(mcpType MCPType) *RouteMap {
	rm.MCPType = mcpType
	return rm
}

func (rm *RouteMap) WithMCPTags(tags ...string) *RouteMap {
	rm.MCPTags = tags
	return rm
}

func (rm *RouteMap) WithAnnotations(annotation *mcp.ToolAnnotation) *RouteMap {
	rm.Annotations = annotation
	return rm
}
