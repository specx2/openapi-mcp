package factory

import (
	"testing"

	"github.com/specx2/openapi-mcp/core/executor"
	"github.com/specx2/openapi-mcp/core/ir"
)

func TestGenerateNameUsesOperationIDPrefix(t *testing.T) {
	cf := NewComponentFactory(executor.NewDefaultHTTPClient(), "")

	route := ir.HTTPRoute{
		Method:      "GET",
		Path:        "/users/{id}",
		OperationID: "getUser__v2",
	}

	name := cf.generateName(route, "tool")

	if name != "getUser" {
		t.Fatalf("expected name to be 'getUser', got %q", name)
	}
}

func TestGenerateNameWithCustomNames(t *testing.T) {
	cf := NewComponentFactory(executor.NewDefaultHTTPClient(), "")
	cf = cf.WithCustomNames(map[string]string{
		"getUser":       "fetch_user",
		"GET /orders":   "list_orders",
		"post:/widgets": "create_widget",
	})

	routeWithOperation := ir.HTTPRoute{
		Method:      "GET",
		Path:        "/users/{id}",
		OperationID: "getUser",
	}

	if got := cf.generateName(routeWithOperation, "tool"); got != "fetch_user" {
		t.Fatalf("expected custom name 'fetch_user', got %q", got)
	}

	routeWithMethodPath := ir.HTTPRoute{
		Method: "GET",
		Path:   "/orders",
	}

	if got := cf.generateName(routeWithMethodPath, "resource"); got != "list_orders" {
		t.Fatalf("expected custom name 'list_orders', got %q", got)
	}

	routeWithLowerMethod := ir.HTTPRoute{
		Method: "POST",
		Path:   "/widgets",
	}

	if got := cf.generateName(routeWithLowerMethod, "resource_template"); got != "create_widget" {
		t.Fatalf("expected custom name 'create_widget', got %q", got)
	}
}

func TestGenerateNameHandlesCollisions(t *testing.T) {
	cf := NewComponentFactory(executor.NewDefaultHTTPClient(), "")

	routeOne := ir.HTTPRoute{
		Method:  "GET",
		Path:    "/orders",
		Summary: "List Orders",
	}

	routeTwo := ir.HTTPRoute{
		Method:  "GET",
		Path:    "/orders/legacy",
		Summary: "List Orders",
	}

	first := cf.generateName(routeOne, "tool")
	second := cf.generateName(routeTwo, "tool")

	if first != "List_Orders" {
		t.Fatalf("expected first name 'List_Orders', got %q", first)
	}

	if second != "List_Orders_2" {
		t.Fatalf("expected second name 'List_Orders_2', got %q", second)
	}
}

func TestGenerateNameFallsBackWhenEmpty(t *testing.T) {
	cf := NewComponentFactory(executor.NewDefaultHTTPClient(), "")

	route := ir.HTTPRoute{
		Method: "POST",
		Path:   "/",
	}

	name := cf.generateName(route, "tool")

	if name != "post_root" {
		t.Fatalf("expected fallback name 'post_root', got %q", name)
	}
}
