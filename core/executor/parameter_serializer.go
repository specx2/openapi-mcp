package executor

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParameterSerializer 处理 OpenAPI 参数序列化
type ParameterSerializer struct {
	style    string
	explode  bool
	location string
}

// NewParameterSerializer 创建新的参数序列化器
func NewParameterSerializer(style string, explode bool, location string) *ParameterSerializer {
	return &ParameterSerializer{
		style:    style,
		explode:  explode,
		location: location,
	}
}

// Serialize 序列化参数值
func (ps *ParameterSerializer) Serialize(value interface{}) (interface{}, error) {
	switch ps.style {
	case "deepObject":
		return ps.serializeDeepObject(value)
	case "form":
		return ps.serializeForm(value)
	case "simple":
		return ps.serializeSimple(value)
	case "spaceDelimited":
		return ps.serializeSpaceDelimited(value)
	case "pipeDelimited":
		return ps.serializePipeDelimited(value)
	default:
		// 默认使用 form 样式
		return ps.serializeForm(value)
	}
}

// serializeDeepObject 处理 deepObject 样式
func (ps *ParameterSerializer) serializeDeepObject(value interface{}) (interface{}, error) {
	if !ps.explode {
		// deepObject 与 explode=false 的组合不标准，回退到 JSON
		return json.Marshal(value)
	}

	valueMap, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("deepObject style requires map value, got %T", value)
	}

	result := make(map[string]interface{})
	for key, val := range valueMap {
		bracketedKey := fmt.Sprintf("[%s]", key)
		result[bracketedKey] = val
	}

	return result, nil
}

// serializeForm 处理 form 样式（默认查询参数样式）
func (ps *ParameterSerializer) serializeForm(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []interface{}:
		if ps.explode {
			// explode=true: 传递数组本身，HTTP 客户端会处理为多个同名参数
			return v, nil
		} else {
			// explode=false: 逗号分隔的字符串
			return ps.serializeArrayToString(v, ","), nil
		}
	case map[string]interface{}:
		if ps.explode {
			// explode=true: 对象属性成为单独的参数
			return v, nil
		} else {
			// explode=false: 逗号分隔的键值对
			return ps.serializeObjectToString(v, ",", ","), nil
		}
	default:
		return v, nil
	}
}

// serializeSimple 处理 simple 样式（默认路径和头部参数样式）
func (ps *ParameterSerializer) serializeSimple(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []interface{}:
		// simple 样式：逗号分隔
		return ps.serializeArrayToString(v, ","), nil
	case map[string]interface{}:
		// simple 样式：逗号分隔的键值对
		return ps.serializeObjectToString(v, ",", ","), nil
	default:
		return v, nil
	}
}

// serializeSpaceDelimited 处理 spaceDelimited 样式
func (ps *ParameterSerializer) serializeSpaceDelimited(value interface{}) (interface{}, error) {
	if v, ok := value.([]interface{}); ok {
		return ps.serializeArrayToString(v, " "), nil
	}
	return nil, fmt.Errorf("spaceDelimited style requires array value")
}

// serializePipeDelimited 处理 pipeDelimited 样式
func (ps *ParameterSerializer) serializePipeDelimited(value interface{}) (interface{}, error) {
	if v, ok := value.([]interface{}); ok {
		return ps.serializeArrayToString(v, "|"), nil
	}
	return nil, fmt.Errorf("pipeDelimited style requires array value")
}

// serializeArrayToString 将数组序列化为分隔符分隔的字符串
func (ps *ParameterSerializer) serializeArrayToString(arr []interface{}, delimiter string) string {
	var parts []string
	for _, item := range arr {
		parts = append(parts, fmt.Sprintf("%v", item))
	}
	return strings.Join(parts, delimiter)
}

// serializeObjectToString 将对象序列化为分隔符分隔的键值对字符串
func (ps *ParameterSerializer) serializeObjectToString(obj map[string]interface{}, pairDelimiter, keyValueDelimiter string) string {
	var parts []string
	for key, value := range obj {
		parts = append(parts, fmt.Sprintf("%s%s%v", key, keyValueDelimiter, value))
	}
	return strings.Join(parts, pairDelimiter)
}

// SerializeParameter 根据参数信息序列化参数值
func SerializeParameter(param ParamInfo, value interface{}) (interface{}, error) {
	style := param.Style
	if style == "" {
		// 设置默认样式
		switch param.In {
		case "path", "header":
			style = "simple"
		case "query":
			style = "form"
		default:
			style = "form"
		}
	}

	explode := param.Explode
	if explode == nil {
		// 设置默认 explode 值
		var defaultExplode bool
		switch style {
		case "simple":
			defaultExplode = false
		case "form":
			defaultExplode = true
		default:
			defaultExplode = true
		}
		explode = &defaultExplode
	}

	serializer := NewParameterSerializer(style, *explode, param.In)
	return serializer.Serialize(value)
}

// ParamInfo 参数信息结构
type ParamInfo struct {
	Name    string
	In      string
	Style   string
	Explode *bool
}
