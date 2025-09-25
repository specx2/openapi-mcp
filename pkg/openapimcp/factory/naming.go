package factory

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

const maxComponentNameLength = 56

func (cf *ComponentFactory) generateName(route ir.HTTPRoute, componentType string) string {
	baseName := cf.resolveBaseName(route)

	slug := slugify(baseName)
	if slug == "" {
		slug = slugify(fallbackNameFromRoute(route))
	}

	if slug == "" {
		slug = "operation"
	}

	if len(slug) > maxComponentNameLength {
		slug = slug[:maxComponentNameLength]
	}

	if cf.nameCounter[componentType] == nil {
		cf.nameCounter[componentType] = make(map[string]int)
	}

	cf.nameCounter[componentType][slug]++
	count := cf.nameCounter[componentType][slug]

	if count == 1 {
		return slug
	}

	return fmt.Sprintf("%s_%d", slug, count)
}

func (cf *ComponentFactory) resolveBaseName(route ir.HTTPRoute) string {
	if name, ok := cf.lookupCustomName(route); ok {
		return name
	}

	if route.OperationID != "" {
		operationID := route.OperationID
		if idx := strings.Index(operationID, "__"); idx > 0 {
			operationID = operationID[:idx]
		}
		return operationID
	}

	if route.Summary != "" {
		return route.Summary
	}

	if route.Description != "" {
		return route.Description
	}

	return fallbackNameFromRoute(route)
}

func (cf *ComponentFactory) lookupCustomName(route ir.HTTPRoute) (string, bool) {
	if len(cf.customNames) == 0 {
		return "", false
	}

	var candidates []string

	if route.OperationID != "" {
		opID := strings.TrimSpace(route.OperationID)
		if opID != "" {
			candidates = append(candidates, opID)
			if idx := strings.Index(opID, "__"); idx > 0 {
				candidates = append(candidates, strings.TrimSpace(opID[:idx]))
			}
		}
	}

	if route.Method != "" && route.Path != "" {
		methodUpper := strings.ToUpper(route.Method)
		methodLower := strings.ToLower(route.Method)
		trimmedPath := strings.TrimSpace(route.Path)
		if trimmedPath != "" {
			candidates = append(candidates,
				fmt.Sprintf("%s %s", methodUpper, trimmedPath),
				fmt.Sprintf("%s %s", methodLower, trimmedPath),
				fmt.Sprintf("%s:%s", methodUpper, trimmedPath),
				fmt.Sprintf("%s:%s", methodLower, trimmedPath),
			)
		}
	}

	if route.Path != "" {
		trimmedPath := strings.TrimSpace(route.Path)
		if trimmedPath != "" {
			candidates = append(candidates, trimmedPath)
		}
	}

	for _, key := range candidates {
		if key == "" {
			continue
		}
		if name, ok := cf.customNames[key]; ok {
			return name, true
		}
	}

	return "", false
}

func fallbackNameFromRoute(route ir.HTTPRoute) string {
	method := strings.ToLower(strings.TrimSpace(route.Method))
	if method == "" {
		method = "operation"
	}

	path := strings.TrimSpace(route.Path)
	if path == "" {
		path = "endpoint"
	}

	path = strings.Trim(path, "/")
	if path == "" {
		path = "root"
	}
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")

	return fmt.Sprintf("%s_%s", method, path)
}

func normalizeCustomNames(names map[string]string) map[string]string {
	if len(names) == 0 {
		return nil
	}

	normalized := make(map[string]string, len(names))
	for key, value := range names {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		normalized[trimmedKey] = trimmedValue
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func slugify(text string) string {
	slug := regexp.MustCompile(`[\s\-\.]+`).ReplaceAllString(text, "_")

	slug = regexp.MustCompile(`[^a-zA-Z0-9_]`).ReplaceAllString(slug, "")

	slug = regexp.MustCompile(`_+`).ReplaceAllString(slug, "_")

	return strings.Trim(slug, "_")
}
