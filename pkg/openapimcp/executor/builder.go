package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

type RequestBuilder struct {
	route           ir.HTTPRoute
	paramMap        map[string]ir.ParamMapping
	baseURL         string
	bodyContentType string
	bodySchema      ir.Schema
}

func NewRequestBuilder(route ir.HTTPRoute, paramMap map[string]ir.ParamMapping, baseURL string) *RequestBuilder {
	bodyContentType := ""
	var bodySchema ir.Schema
	if route.RequestBody != nil && len(route.RequestBody.ContentSchemas) > 0 {
		for ct := range route.RequestBody.ContentSchemas {
			// 选择首选的 JSON 类型，其次任意类型
			if bodyContentType == "" || strings.Contains(ct, "json") {
				bodyContentType = ct
				bodySchema = route.RequestBody.ContentSchemas[ct]
				if strings.Contains(ct, "json") {
					break
				}
			}
		}
	}

	if len(paramMap) == 0 && len(route.ParameterMap) > 0 {
		paramMap = route.ParameterMap
	}

	return &RequestBuilder{
		route:           route,
		paramMap:        paramMap,
		baseURL:         baseURL,
		bodyContentType: bodyContentType,
		bodySchema:      bodySchema,
	}
}

func (rb *RequestBuilder) Build(ctx context.Context, args map[string]interface{}) (*http.Request, error) {
	pathParams := make(map[string]string)
	queryParams := url.Values{}
	headerParams := make(map[string]string)
	cookieParams := make(map[string]string)
	bodyParams := make(map[string]interface{})

	for argName, argValue := range args {
		mapping, ok := rb.paramMap[argName]
		if !ok {
			// 未显式映射的参数默认归入请求体，保持向后兼容
			if argValue != nil {
				bodyParams[argName] = argValue
			}
			continue
		}

		switch mapping.Location {
		case ir.ParameterInPath:
			if argValue != nil {
				pathParams[mapping.OpenAPIName] = fmt.Sprintf("%v", argValue)
			}
		case ir.ParameterInQuery:
			if argValue != nil {
				rb.addQueryParam(queryParams, mapping, argValue)
			}
		case ir.ParameterInHeader:
			if argValue != nil {
				headerParams[mapping.OpenAPIName] = fmt.Sprintf("%v", argValue)
			}
		case ir.ParameterInCookie:
			if argValue != nil {
				cookieParams[mapping.OpenAPIName] = fmt.Sprintf("%v", argValue)
			}
		case "body":
			if argValue != nil {
				bodyParams[mapping.OpenAPIName] = argValue
			}
		}
	}

	reqURL, err := rb.buildURL(pathParams, queryParams)
	if err != nil {
		return nil, err
	}

	bodyReader, contentType, err := rb.buildBody(bodyParams)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, rb.route.Method, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	for name, value := range headerParams {
		req.Header.Set(name, value)
	}

	for name, value := range cookieParams {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	return req, nil
}

func (rb *RequestBuilder) buildURL(pathParams map[string]string, queryParams url.Values) (string, error) {
	urlPath := rb.route.Path

	for paramName, paramValue := range pathParams {
		placeholder := fmt.Sprintf("{%s}", paramName)
		urlPath = strings.ReplaceAll(urlPath, placeholder, paramValue)
	}

	var fullURL string
	if rb.baseURL != "" {
		baseURL := strings.TrimSuffix(rb.baseURL, "/")
		urlPath = strings.TrimPrefix(urlPath, "/")
		fullURL = fmt.Sprintf("%s/%s", baseURL, urlPath)
	} else {
		fullURL = urlPath
	}

	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if len(queryParams) > 0 {
		parsedURL.RawQuery = queryParams.Encode()
	}

	return parsedURL.String(), nil
}

func (rb *RequestBuilder) buildBody(bodyParams map[string]interface{}) (io.Reader, string, error) {
	if len(bodyParams) == 0 {
		return nil, "", nil
	}

	contentType := rb.bodyContentType
	if contentType == "" {
		contentType = "application/json"
	}

	// 如果请求体映射只有 body 字段且 schema 不是对象，则直接发送原始值
	if len(bodyParams) == 1 {
		if raw, ok := bodyParams["body"]; ok {
			if rb.shouldUseRawBody() {
				return rb.encodeBody(raw, contentType)
			}
		}
	}

	return rb.encodeBody(bodyParams, contentType)
}

func (rb *RequestBuilder) addQueryParam(params url.Values, mapping ir.ParamMapping, value interface{}) {
	name := mapping.OpenAPIName
	paramInfo := rb.findParameterInfo(name, ir.ParameterInQuery)

	switch v := value.(type) {
	case []interface{}:
		rb.addArrayParam(params, name, v, paramInfo)
	case map[string]interface{}:
		rb.addObjectParam(params, name, v, paramInfo)
	default:
		params.Add(name, fmt.Sprintf("%v", value))
	}
}

func (rb *RequestBuilder) addArrayParam(params url.Values, name string, values []interface{}, info *ir.ParameterInfo) {
	if info != nil {
		switch info.Style {
		case "spaceDelimited":
			params.Add(name, strings.Join(rb.toStringSlice(values), " "))
			return
		case "pipeDelimited":
			params.Add(name, strings.Join(rb.toStringSlice(values), "|"))
			return
		}
	}

	explode := true
	if info != nil && info.Explode != nil {
		explode = *info.Explode
	}

	if explode {
		for _, v := range values {
			params.Add(name, fmt.Sprintf("%v", v))
		}
	} else {
		strValues := make([]string, len(values))
		for i, v := range values {
			strValues[i] = fmt.Sprintf("%v", v)
		}
		params.Add(name, strings.Join(strValues, ","))
	}
}

func (rb *RequestBuilder) addObjectParam(params url.Values, name string, obj map[string]interface{}, info *ir.ParameterInfo) {
	if info != nil && info.Style == "deepObject" {
		for key, value := range obj {
			params.Add(fmt.Sprintf("%s[%s]", name, key), fmt.Sprintf("%v", value))
		}
	} else {
		explode := true
		if info != nil && info.Explode != nil {
			explode = *info.Explode
		}

		if explode {
			for key, value := range obj {
				params.Add(key, fmt.Sprintf("%v", value))
			}
		} else {
			pairs := make([]string, 0, len(obj)*2)
			for key, value := range obj {
				pairs = append(pairs, fmt.Sprintf("%s,%v", key, value))
			}
			params.Add(name, strings.Join(pairs, ","))
		}
	}
}

func (rb *RequestBuilder) findParameterInfo(name string, location string) *ir.ParameterInfo {
	for i := range rb.route.Parameters {
		param := &rb.route.Parameters[i]
		if param.Name == name && param.In == location {
			return param
		}
	}
	return nil
}

func (rb *RequestBuilder) toStringSlice(values []interface{}) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = fmt.Sprintf("%v", v)
	}
	return result
}

func (rb *RequestBuilder) shouldUseRawBody() bool {
	if rb.bodySchema == nil {
		return true
	}

	if rb.bodySchema.Type() != "" && rb.bodySchema.Type() != "object" {
		return true
	}

	if len(rb.bodySchema.Properties()) == 0 {
		return true
	}

	return false
}

func (rb *RequestBuilder) encodeBody(body interface{}, contentType string) (io.Reader, string, error) {
	switch {
	case strings.Contains(contentType, "json"):
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal request body: %w", err)
		}
		return bytes.NewReader(bodyJSON), contentType, nil
	case strings.Contains(contentType, "x-www-form-urlencoded"):
		values := url.Values{}
		if formMap, ok := body.(map[string]interface{}); ok {
			for key, v := range formMap {
				values.Add(key, fmt.Sprintf("%v", v))
			}
		}
		return strings.NewReader(values.Encode()), contentType, nil
	default:
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal request body: %w", err)
		}
		return bytes.NewReader(bodyJSON), contentType, nil
	}
}
