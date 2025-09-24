package factory

import (
	"fmt"
	"strings"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

func (cf *ComponentFactory) formatDescription(route ir.HTTPRoute) string {
	var parts []string

	if route.Description != "" {
		parts = append(parts, route.Description)
	} else if route.Summary != "" {
		parts = append(parts, route.Summary)
	} else {
		parts = append(parts, fmt.Sprintf("%s %s", route.Method, route.Path))
	}

	if route.RequestBody != nil && route.RequestBody.Description != "" {
		parts = append(parts, fmt.Sprintf("**Request Body:** %s", route.RequestBody.Description))
	}

	if len(route.Responses) > 0 {
		var responseParts []string
		for status, response := range route.Responses {
			if response.Description != "" {
				responseParts = append(responseParts, fmt.Sprintf("- %s: %s", status, response.Description))
			}
		}
		if len(responseParts) > 0 {
			parts = append(parts, "**Responses:**")
			parts = append(parts, strings.Join(responseParts, "\n"))
		}
	}

	if len(route.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("**Tags:** %s", strings.Join(route.Tags, ", ")))
	}

	return strings.Join(parts, "\n\n")
}