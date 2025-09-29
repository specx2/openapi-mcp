package executor

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/specx2/openapi-mcp/core/ir"
)

type EncodedParameter struct {
	Name          string
	Value         string
	AllowReserved bool
}

func encodeParameterValues(param ir.ParameterInfo, value interface{}) ([]EncodedParameter, error) {
	if value == nil {
		return nil, nil
	}

	style := param.Style
	if style == "" {
		style = defaultStyleForLocation(param.In)
	}

	explode := defaultExplodeFor(style, param.In)
	if param.Explode != nil {
		explode = *param.Explode
	}

	var (
		encoded []EncodedParameter
		err     error
	)

	switch style {
	case "form":
		encoded = encodeFormStyle(param.Name, value, explode)
	case "simple":
		encoded = encodeSimpleStyle(param.Name, value, explode)
	case "spaceDelimited":
		encoded, err = encodeDelimitedStyle(param.Name, value, " ")
	case "pipeDelimited":
		encoded, err = encodeDelimitedStyle(param.Name, value, "|")
	case "deepObject":
		encoded = encodeDeepObjectStyle(param.Name, value)
	default:
		encoded = encodeFormStyle(param.Name, value, explode)
	}

	if err != nil {
		return nil, err
	}

	if len(encoded) == 0 {
		return encoded, nil
	}

	if param.AllowReserved {
		for i := range encoded {
			encoded[i].AllowReserved = true
		}
	}

	return encoded, nil
}

func defaultStyleForLocation(location string) string {
	switch location {
	case ir.ParameterInPath, ir.ParameterInHeader:
		return "simple"
	case ir.ParameterInCookie:
		return "form"
	default:
		return "form"
	}
}

func defaultExplodeFor(style, location string) bool {
	switch location {
	case ir.ParameterInPath:
		return false
	case ir.ParameterInQuery:
		if style == "form" {
			return true
		}
	case ir.ParameterInHeader:
		return false
	case ir.ParameterInCookie:
		if style == "form" {
			return true
		}
	}

	if style == "form" {
		return true
	}

	return false
}

func encodeFormStyle(name string, value interface{}, explode bool) []EncodedParameter {
	if arr, ok := valueAsSlice(value); ok {
		if explode {
			return encodeSlice(name, arr)
		}
		return []EncodedParameter{{Name: name, Value: strings.Join(stringifySlice(arr), ",")}}
	}

	if obj, ok := valueAsMap(value); ok {
		keys := sortedKeys(obj)
		if explode {
			pairs := make([]EncodedParameter, 0, len(keys))
			for _, key := range keys {
				pairs = append(pairs, EncodedParameter{Name: key, Value: formatScalar(obj[key])})
			}
			return pairs
		}
		parts := make([]string, 0, len(keys)*2)
		for _, key := range keys {
			parts = append(parts, key, formatScalar(obj[key]))
		}
		return []EncodedParameter{{Name: name, Value: strings.Join(parts, ",")}}
	}

	return []EncodedParameter{{Name: name, Value: formatScalar(value)}}
}

func encodeSimpleStyle(name string, value interface{}, explode bool) []EncodedParameter {
	if arr, ok := valueAsSlice(value); ok {
		if explode {
			return encodeSlice(name, arr)
		}
		return []EncodedParameter{{Name: name, Value: strings.Join(stringifySlice(arr), ",")}}
	}

	if obj, ok := valueAsMap(value); ok {
		keys := sortedKeys(obj)
		if explode {
			pairs := make([]EncodedParameter, 0, len(keys))
			for _, key := range keys {
				pairs = append(pairs, EncodedParameter{Name: key, Value: formatScalar(obj[key])})
			}
			return pairs
		}
		parts := make([]string, 0, len(keys)*2)
		for _, key := range keys {
			parts = append(parts, key, formatScalar(obj[key]))
		}
		return []EncodedParameter{{Name: name, Value: strings.Join(parts, ",")}}
	}

	return []EncodedParameter{{Name: name, Value: formatScalar(value)}}
}

func encodeDelimitedStyle(name string, value interface{}, delimiter string) ([]EncodedParameter, error) {
	arr, ok := valueAsSlice(value)
	if !ok {
		return nil, fmt.Errorf("%s style requires array value", delimiter)
	}
	return []EncodedParameter{
		{
			Name:  name,
			Value: strings.Join(stringifySlice(arr), delimiter),
		},
	}, nil
}

func encodeDeepObjectStyle(name string, value interface{}) []EncodedParameter {
	obj, ok := valueAsMap(value)
	if !ok {
		return []EncodedParameter{}
	}
	keys := sortedKeys(obj)
	pairs := make([]EncodedParameter, 0, len(keys))
	for _, key := range keys {
		encodedName := fmt.Sprintf("%s[%s]", name, key)
		pairs = append(pairs, EncodedParameter{Name: encodedName, Value: formatScalar(obj[key])})
	}
	return pairs
}

func encodeSlice(name string, values []interface{}) []EncodedParameter {
	pairs := make([]EncodedParameter, len(values))
	for i, item := range values {
		pairs[i] = EncodedParameter{Name: name, Value: formatScalar(item)}
	}
	return pairs
}

func valueAsSlice(value interface{}) ([]interface{}, bool) {
	switch v := value.(type) {
	case []interface{}:
		return v, true
	case []string:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result, true
	case []int:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result, true
	case []float64:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result, true
	default:
		return nil, false
	}
}

func valueAsMap(value interface{}) (map[string]interface{}, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		return v, true
	case map[string]string:
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			result[key] = val
		}
		return result, true
	default:
		return nil, false
	}
}

func stringifySlice(values []interface{}) []string {
	result := make([]string, len(values))
	for i, item := range values {
		result[i] = formatScalar(item)
	}
	return result
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatScalar(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case *bool:
		if v == nil {
			return ""
		}
		if *v {
			return "true"
		}
		return "false"
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", value)
	}
}
