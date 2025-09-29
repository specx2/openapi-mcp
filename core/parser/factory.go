package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

type ParserConfig struct {
	SpecURL string
}

type ParserOption func(*ParserConfig)

func WithSpecURL(url string) ParserOption {
	return func(cfg *ParserConfig) {
		cfg.SpecURL = url
	}
}

type configurableParser interface {
	setConfig(ParserConfig)
}

func NewParser(spec []byte, opts ...ParserOption) (OpenAPIParser, error) {
	config := ParserConfig{}
	for _, opt := range opts {
		opt(&config)
	}

	_, version, err := detectVersionAndNormalize(spec)
	if err != nil {
		return nil, err
	}

	var parser OpenAPIParser
	if strings.HasPrefix(version, "3.0") {
		parser = NewOpenAPI30Parser()
	} else if strings.HasPrefix(version, "3.1") {
		parser = NewOpenAPI31Parser()
	} else {
		return nil, fmt.Errorf("unsupported OpenAPI version: %s", version)
	}

	if configurable, ok := parser.(configurableParser); ok {
		configurable.setConfig(config)
	}

	return parser, nil
}

func DetectOpenAPIVersion(spec []byte) (string, error) {
	_, version, err := detectVersionAndNormalize(spec)
	return version, err
}

func detectVersionAndNormalize(spec []byte) ([]byte, string, error) {
	var rawSpec map[string]interface{}
	if err := json.Unmarshal(spec, &rawSpec); err != nil {
		converted, convErr := yaml.YAMLToJSON(spec)
		if convErr != nil {
			return nil, "", fmt.Errorf("failed to parse spec: %w", err)
		}
		if err := json.Unmarshal(converted, &rawSpec); err != nil {
			return nil, "", fmt.Errorf("failed to parse normalised spec: %w", err)
		}
		spec = converted
	}

	version, ok := rawSpec["openapi"].(string)
	if !ok {
		return nil, "", fmt.Errorf("missing or invalid 'openapi' field")
	}

	return spec, version, nil
}
