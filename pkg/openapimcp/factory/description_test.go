package factory

import (
	"strings"
	"testing"

	"github.com/specx2/openapi-mcp/pkg/openapimcp/ir"
)

func TestFormatDescriptionIncludesVariantsAndExamples(t *testing.T) {
	cf := NewComponentFactory(nil, "")

	route := ir.HTTPRoute{
		Method:  "GET",
		Path:    "/users",
		Summary: "List users",
		Parameters: []ir.ParameterInfo{
			{
				Name:        "userId",
				In:          ir.ParameterInPath,
				Required:    true,
				Description: "User identifier",
				Schema:      ir.Schema{"type": "string"},
				Extensions: map[string]interface{}{
					"x-param-hint": "id",
				},
			},
		},
		Extensions: map[string]interface{}{
			"x-openapi-role": "admin-only",
		},
		RequestBody: &ir.RequestBodyInfo{
			Extensions: map[string]interface{}{
				"x-body-mode": "bulk",
			},
			ContentSchemas: map[string]ir.Schema{
				"application/json": {
					"oneOf": []interface{}{
						map[string]interface{}{
							"title": "FilterByRole",
							"type":  "object",
							"properties": map[string]interface{}{
								"role": map[string]interface{}{"type": "string"},
							},
						},
						map[string]interface{}{
							"title": "FilterByTeam",
							"type":  "object",
							"properties": map[string]interface{}{
								"team": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
			MediaExamples: map[string]interface{}{
				"application/json": map[string]interface{}{"role": "admin"},
			},
		},
		Responses: map[string]ir.ResponseInfo{
			"200": {
				Description: "Successful response",
				ContentSchemas: map[string]ir.Schema{
					"application/json": {
						"oneOf": []interface{}{
							map[string]interface{}{
								"title": "UserList",
								"type":  "object",
								"properties": map[string]interface{}{
									"users": map[string]interface{}{
										"type":  "array",
										"items": map[string]interface{}{"type": "object"},
									},
								},
							},
							map[string]interface{}{
								"title": "EmptyResult",
								"type":  "object",
								"properties": map[string]interface{}{
									"users": map[string]interface{}{
										"type": "array",
									},
								},
							},
						},
					},
				},
				MediaExamples: map[string]interface{}{
					"application/json": map[string]interface{}{
						"users": []interface{}{
							map[string]interface{}{"id": 1},
						},
					},
				},
				Extensions: map[string]interface{}{
					"x-response-mode": "paged",
				},
			},
			"404": {
				Description: "Not found",
			},
		},
		Callbacks: []ir.CallbackInfo{
			{
				Name:       "onData",
				Expression: "{$request.body#/callbackUrl}",
				Operations: []ir.CallbackOperation{
					{
						Method:      "POST",
						Summary:     "Receive data",
						Description: "Callback when new data is ready",
						Extensions: map[string]interface{}{
							"x-callback-extra": true,
						},
					},
				},
			},
		},
	}

	description := cf.formatDescription(route)

	if !strings.Contains(description, "**Responses:**") {
		t.Fatalf("expected responses section in description, got %q", description)
	}

	if !strings.Contains(description, "200") || !strings.Contains(description, "Successful response") {
		t.Fatalf("expected 200 response description, got %q", description)
	}

	if !strings.Contains(description, "variants: oneOf(UserList, EmptyResult)") {
		t.Fatalf("expected response variant summary, got %q", description)
	}

	if !strings.Contains(description, "Example: ") {
		t.Fatalf("expected example in responses section, got %q", description)
	}

	if !strings.Contains(description, "**Path Parameters:**") || !strings.Contains(description, "extensions: x-param-hint=id") {
		t.Fatalf("expected parameter section with extensions, got %q", description)
	}

	if !strings.Contains(description, "Request Body Schema") || !strings.Contains(description, "variants: oneOf(FilterByRole, FilterByTeam)") {
		t.Fatalf("expected request body variant summary, got %q", description)
	}

	if !strings.Contains(description, "Request Body Extensions") || !strings.Contains(description, "x-body-mode=bulk") {
		t.Fatalf("expected request body extensions to be summarized, got %q", description)
	}

	if !strings.Contains(description, "x-openapi-role=admin-only") {
		t.Fatalf("expected extensions summary, got %q", description)
	}

	if !strings.Contains(description, "**Callbacks:**") || !strings.Contains(description, "onData") || !strings.Contains(description, "{$request.body#/callbackUrl}") {
		t.Fatalf("expected callback summary, got %q", description)
	}

	if !strings.Contains(description, "[Extensions: x-response-mode=paged]") {
		t.Fatalf("expected response extensions to be summarized, got %q", description)
	}
}
