package parser

import (
	"testing"
)

func TestOpenAPI30ParserResolvesCallbackReferences(t *testing.T) {
	spec := `{
        "openapi": "3.0.1",
        "info": {"title": "Callbacks", "version": "1.0"},
        "paths": {
            "/subscribe": {
                "post": {
                    "operationId": "subscribe",
                    "responses": {"200": {"description": "ok"}},
                    "callbacks": {
                        "onEvent": {
                            "$ref": "#/components/callbacks/EventCallback",
                            "x-callback-ref": "extra"
                        }
                    }
                }
            }
        },
        "components": {
            "callbacks": {
                "EventCallback": {
                    "x-callback-info": "component",
                    "{$request.body#/callbackUrl}": {
                        "$ref": "#/components/pathItems/EventCallbackPath"
                    }
                }
            },
            "pathItems": {
                "EventCallbackPath": {
                    "post": {
                        "summary": "Deliver event",
                        "x-operation-flag": true,
                        "responses": {
                            "200": {"description": "done"}
                        }
                    }
                }
            }
        }
    }`

	parser := NewOpenAPI30Parser()
	routes, err := parser.ParseSpec([]byte(spec))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	callbacks := routes[0].Callbacks
	if len(callbacks) != 1 {
		t.Fatalf("expected 1 callback, got %d", len(callbacks))
	}
	cb := callbacks[0]
	if cb.Name != "onEvent" {
		t.Fatalf("expected callback name onEvent, got %q", cb.Name)
	}
	if cb.Expression == "" || cb.Expression != "{$request.body#/callbackUrl}" {
		t.Fatalf("expected expression to include runtime URL, got %q", cb.Expression)
	}
	if cb.Extensions == nil || cb.Extensions["x-callback-info"] != "component" {
		t.Fatalf("expected component extensions to be merged, got %#v", cb.Extensions)
	}
	if len(cb.Operations) != 1 {
		t.Fatalf("expected 1 callback operation, got %d", len(cb.Operations))
	}
	op := cb.Operations[0]
	if op.Method != "POST" {
		t.Fatalf("expected callback method POST, got %s", op.Method)
	}
	if op.Extensions == nil || op.Extensions["x-operation-flag"] != true {
		t.Fatalf("expected operation extensions to be preserved, got %#v", op.Extensions)
	}
}

func TestOpenAPI31ParserResolvesCallbackReferences(t *testing.T) {
	spec := `{
        "openapi": "3.1.0",
        "info": {"title": "Callbacks", "version": "1.0"},
        "paths": {
            "/subscribe": {
                "post": {
                    "operationId": "subscribe",
                    "responses": {"200": {"description": "ok"}},
                    "callbacks": {
                        "onEvent": {
                            "$ref": "#/components/callbacks/EventCallback"
                        }
                    }
                }
            }
        },
        "components": {
            "callbacks": {
                "EventCallback": {
                    "{$request.body#/callbackUrl}": {
                        "$ref": "#/components/pathItems/EventCallbackPath"
                    }
                }
            },
            "pathItems": {
                "EventCallbackPath": {
                    "post": {
                        "summary": "Deliver (31)",
                        "responses": {
                            "200": {"description": "done"}
                        }
                    }
                }
            }
        }
    }`

	parser := NewOpenAPI31Parser()
	routes, err := parser.ParseSpec([]byte(spec))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	callbacks := routes[0].Callbacks
	if len(callbacks) != 1 {
		t.Fatalf("expected 1 callback, got %d", len(callbacks))
	}
	cb := callbacks[0]
	if cb.Expression != "{$request.body#/callbackUrl}" {
		t.Fatalf("expected runtime expression, got %q", cb.Expression)
	}
	if len(cb.Operations) != 1 || cb.Operations[0].Method != "POST" {
		t.Fatalf("expected POST operation from referenced path item, got %#v", cb.Operations)
	}
}
