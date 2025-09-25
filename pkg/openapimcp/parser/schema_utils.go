package parser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

func sanitizeDefinitionName(name string) string {
	if name == "" {
		return "schema"
	}
	replacer := strings.NewReplacer("-", "_", ".", "_", " ", "_")
	sanitized := replacer.Replace(name)
	sanitized = strings.Map(func(r rune) rune {
		if r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return r
		}
		return '_'
	}, sanitized)
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		return "schema"
	}
	return sanitized
}

func cloneGenericMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(src))
	for k, v := range src {
		cloned[k] = cloneGenericValue(v)
	}
	return cloned
}

func cloneGenericValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return cloneGenericMap(v)
	case []interface{}:
		arr := make([]interface{}, len(v))
		for i, val := range v {
			arr[i] = cloneGenericValue(val)
		}
		return arr
	default:
		return value
	}
}

func cloneIRSchema(schema ir.Schema) ir.Schema {
	if schema == nil {
		return nil
	}
	cloned := make(ir.Schema, len(schema))
	for key, value := range schema {
		cloned[key] = cloneGenericValue(value)
	}
	return cloned
}

func convertToGenericMap(value interface{}) (map[string]interface{}, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		return cloneGenericMap(v), nil
	case nil:
		return nil, nil
	default:
		bytes, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var result map[string]interface{}
		if err := json.Unmarshal(bytes, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
}

func parseArrayIndex(token string, length int) (int, error) {
	idx, err := strconv.Atoi(token)
	if err != nil {
		return 0, fmt.Errorf("invalid array index %q", token)
	}
	if idx < 0 || idx >= length {
		return 0, fmt.Errorf("array index %d out of bounds", idx)
	}
	return idx, nil
}

func deriveRefBase(ref string) string {
	if ref == "" {
		return "schema"
	}
	if strings.HasPrefix(ref, "#/") {
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return sanitizeDefinitionName(parts[len(parts)-1])
		}
		return "schema"
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return "schema"
	}
	fragment := parsed.Fragment
	parsed.Fragment = ""
	base := path.Base(parsed.Path)
	if idx := strings.Index(base, "."); idx >= 0 {
		base = base[:idx]
	}
	if fragment != "" {
		segments := strings.Split(fragment, "/")
		fragBase := segments[len(segments)-1]
		if fragBase != "" {
			base = fragBase
		}
	}
	if base == "" {
		base = "schema"
	}
	return sanitizeDefinitionName(base)
}
