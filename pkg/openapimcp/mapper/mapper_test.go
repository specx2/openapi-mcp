package mapper

import (
	"regexp"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/specx2/openapi-mcp/pkg/openapimcp/ir"
)

func TestRouteMapperAggregatesTagsAndAnnotations(t *testing.T) {
	route := ir.HTTPRoute{
		Method: "GET",
		Path:   "/items",
		Tags:   []string{"public"},
	}

	mapping := RouteMap{
		Methods:     []string{"GET"},
		PathPattern: regexp.MustCompile(`^/items$`),
		MCPType:     MCPTypeResource,
		MCPTags:     []string{"inventory"},
		Annotations: &mcp.ToolAnnotation{ReadOnlyHint: boolPtr(true)},
	}

	mapper := NewRouteMapper([]RouteMap{mapping})
	mapper = mapper.WithGlobalTags("api")

	decision := mapper.MapRouteDecision(route)
	if decision.MCPType != MCPTypeResource {
		t.Fatalf("expected MCPTypeResource, got %s", decision.MCPType)
	}

	if len(decision.Tags) != 3 {
		t.Fatalf("expected three tags, got %v", decision.Tags)
	}

	if decision.Annotations == nil || decision.Annotations.ReadOnlyHint == nil || !*decision.Annotations.ReadOnlyHint {
		t.Fatalf("expected annotation to be cloned from mapping")
	}
}

func TestRouteMapperOverrideFunc(t *testing.T) {
	route := ir.HTTPRoute{Method: "POST", Path: "/items"}

	mapping := RouteMap{
		Methods:     []string{"POST"},
		PathPattern: regexp.MustCompile(`^/items$`),
		MCPType:     MCPTypeTool,
	}

	mapper := NewRouteMapper([]RouteMap{mapping})
	mapper = mapper.WithMapFunc(func(route ir.HTTPRoute, decision RouteDecision) *RouteDecision {
		decision.MCPType = MCPTypeResourceTemplate
		decision.Tags = append(decision.Tags, "override")
		decision.Annotations = &mcp.ToolAnnotation{DestructiveHint: boolPtr(false)}
		return &decision
	})

	decision := mapper.MapRouteDecision(route)
	if decision.MCPType != MCPTypeResourceTemplate {
		t.Fatalf("expected override to set MCPTypeResourceTemplate, got %s", decision.MCPType)
	}
	found := false
	for _, tag := range decision.Tags {
		if tag == "override" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected override tag to be present, got %v", decision.Tags)
	}
	if decision.Annotations == nil || decision.Annotations.DestructiveHint == nil || *decision.Annotations.DestructiveHint {
		t.Fatalf("expected destructive hint to be overridden to false")
	}
}

func boolPtr(v bool) *bool {
	value := v
	return &value
}
