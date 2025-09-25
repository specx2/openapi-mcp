package parser

import (
	"encoding/json"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	low "github.com/pb33f/libopenapi/datamodel/low"
	"github.com/pb33f/libopenapi/orderedmap"
	yaml "go.yaml.in/yaml/v4"
)

func extractExampleValue(node *yaml.Node) interface{} {
	if node == nil {
		return nil
	}
	var value interface{}
	if err := node.Decode(&value); err != nil {
		// fall back to the raw scalar to avoid dropping information
		if node.Kind == yaml.ScalarNode {
			return node.Value
		}
		return nil
	}
	return value
}

func convertExamplesMap(src *orderedmap.Map[string, *base.Example]) map[string]interface{} {
	if src == nil || src.Len() == 0 {
		return nil
	}
	result := make(map[string]interface{}, src.Len())
	for key, example := range src.FromOldest() {
		if example == nil {
			result[key] = nil
			continue
		}
		exampleMap := make(map[string]interface{})
		if example.Summary != "" {
			exampleMap["summary"] = example.Summary
		}
		if example.Description != "" {
			exampleMap["description"] = example.Description
		}
		if val := extractExampleValue(example.Value); val != nil {
			exampleMap["value"] = val
		}
		if example.ExternalValue != "" {
			exampleMap["externalValue"] = example.ExternalValue
		}
		if example.DataValue != nil {
			if val := extractExampleValue(example.DataValue); val != nil {
				exampleMap["dataValue"] = val
			}
		}
		if example.SerializedValue != "" {
			exampleMap["serializedValue"] = example.SerializedValue
		}
		if len(exampleMap) == 0 {
			result[key] = nil
			continue
		}
		result[key] = exampleMap
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneAny(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned interface{}
	if err := json.Unmarshal(data, &cloned); err != nil {
		return value
	}
	return cloned
}

func convertExtensionsMap(exts *orderedmap.Map[string, *yaml.Node]) map[string]interface{} {
	if exts == nil || exts.Len() == 0 {
		return nil
	}
	result := make(map[string]interface{}, exts.Len())
	for key, node := range exts.FromOldest() {
		if node == nil {
			continue
		}
		var value interface{}
		if err := node.Decode(&value); err != nil {
			if node.Kind == yaml.ScalarNode {
				result[key] = node.Value
			}
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func convertLowExtensionsMap(exts *orderedmap.Map[low.KeyReference[string], low.ValueReference[*yaml.Node]]) map[string]interface{} {
	if exts == nil || exts.Len() == 0 {
		return nil
	}
	result := make(map[string]interface{}, exts.Len())
	for keyRef, valueRef := range exts.FromOldest() {
		key := keyRef.Value
		node := valueRef.Value
		if node == nil {
			continue
		}
		var value interface{}
		if err := node.Decode(&value); err != nil {
			if node.Kind == yaml.ScalarNode {
				result[key] = node.Value
			}
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
