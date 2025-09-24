package parser

import (
	"encoding/json"
	"fmt"
	"strings"
)

func NewParser(spec []byte) (OpenAPIParser, error) {
	var rawSpec map[string]interface{}
	if err := json.Unmarshal(spec, &rawSpec); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	version, ok := rawSpec["openapi"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'openapi' field")
	}

	if strings.HasPrefix(version, "3.0") {
		parser := NewOpenAPI30Parser()
		if _, err := parser.ParseSpec(spec); err != nil {
			return nil, err
		}
		return parser, nil
	} else if strings.HasPrefix(version, "3.1") {
		parser := NewOpenAPI31Parser()
		if _, err := parser.ParseSpec(spec); err != nil {
			return nil, err
		}
		return parser, nil
	}

	return nil, fmt.Errorf("unsupported OpenAPI version: %s", version)
}

func DetectOpenAPIVersion(spec []byte) (string, error) {
	var rawSpec map[string]interface{}
	if err := json.Unmarshal(spec, &rawSpec); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	version, ok := rawSpec["openapi"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'openapi' field")
	}

	return version, nil
}
