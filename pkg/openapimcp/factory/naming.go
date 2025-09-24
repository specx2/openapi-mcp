package factory

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

func (cf *ComponentFactory) generateName(route ir.HTTPRoute, componentType string) string {
	var baseName string

	if name, ok := cf.customNames[route.OperationID]; ok {
		baseName = name
	} else if route.OperationID != "" {
		parts := strings.Split(route.OperationID, "__")
		baseName = parts[0]
	} else if route.Summary != "" {
		baseName = slugify(route.Summary)
	} else {
		baseName = fmt.Sprintf("%s_%s",
			strings.ToLower(route.Method),
			slugify(route.Path))
	}

	if len(baseName) > 56 {
		baseName = baseName[:56]
	}

	baseName = strings.Trim(baseName, "_")

	if cf.nameCounter[componentType] == nil {
		cf.nameCounter[componentType] = make(map[string]int)
	}

	cf.nameCounter[componentType][baseName]++
	count := cf.nameCounter[componentType][baseName]

	if count == 1 {
		return baseName
	}
	return fmt.Sprintf("%s_%d", baseName, count)
}

func slugify(text string) string {
	slug := regexp.MustCompile(`[\s\-\.]+`).ReplaceAllString(text, "_")

	slug = regexp.MustCompile(`[^a-zA-Z0-9_]`).ReplaceAllString(slug, "")

	slug = regexp.MustCompile(`_+`).ReplaceAllString(slug, "_")

	return strings.Trim(slug, "_")
}