package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strings"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

type RequestBuilder struct {
	route           ir.HTTPRoute
	paramMap        map[string]ir.ParamMapping
	baseURL         string
	bodyContentType string
	bodyEncoding    map[string]ir.EncodingInfo
}

func NewRequestBuilder(route ir.HTTPRoute, paramMap map[string]ir.ParamMapping, baseURL string) *RequestBuilder {
	bodyContentType := ""
	if route.RequestBody != nil && len(route.RequestBody.ContentSchemas) > 0 {
		for ct := range route.RequestBody.ContentSchemas {
			// 选择首选的 JSON 类型，其次任意类型
			if bodyContentType == "" || strings.Contains(ct, "json") {
				bodyContentType = ct
				if strings.Contains(ct, "json") {
					break
				}
			}
		}
	}

	if len(paramMap) == 0 && len(route.ParameterMap) > 0 {
		paramMap = route.ParameterMap
	}

	rb := &RequestBuilder{
		route:    route,
		paramMap: paramMap,
		baseURL:  baseURL,
	}
	rb.setContentType(bodyContentType)
	return rb
}

func (rb *RequestBuilder) Build(ctx context.Context, args map[string]interface{}) (*http.Request, error) {
	pathParams := make(map[string]string)
	queryParams := url.Values{}
	headerParams := make(map[string]string)
	cookieParams := make(map[string]string)
	bodyParams := make(map[string]interface{})
	var rawBody interface{}
	var overrideContentType string

	for argName, argValue := range args {
		if argName == "_contentType" {
			if s, ok := argValue.(string); ok {
				overrideContentType = s
			}
			continue
		}

		if argName == "_rawBody" {
			rawBody = argValue
			continue
		}

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

	if overrideContentType != "" {
		rb.setContentType(overrideContentType)
	}

	if missing := rb.missingPathParameters(pathParams); len(missing) > 0 {
		return nil, fmt.Errorf("missing required path parameter(s): %s", strings.Join(missing, ", "))
	}

	reqURL, err := rb.buildURL(pathParams, queryParams)
	if err != nil {
		return nil, err
	}

	bodyReader, contentType, err := rb.buildBody(bodyParams, rawBody)
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

func (rb *RequestBuilder) missingPathParameters(pathParams map[string]string) []string {
	var missing []string
	for _, param := range rb.route.Parameters {
		if param.In != ir.ParameterInPath || !param.Required {
			continue
		}
		if _, ok := pathParams[param.Name]; ok {
			continue
		}
		missing = append(missing, param.Name)
	}

	if len(missing) > 1 {
		sort.Strings(missing)
	}

	return missing
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

func (rb *RequestBuilder) buildBody(bodyParams map[string]interface{}, rawBody interface{}) (io.Reader, string, error) {
	contentType := rb.bodyContentType
	schema := rb.lookupBodySchema(contentType)

	if rawBody != nil {
		return rb.encodeRawBody(rawBody, schema)
	}

	if len(bodyParams) == 0 {
		return nil, "", nil
	}

	if contentType == "" {
		if len(bodyParams) == 1 {
			return rb.encodeRawBodyFromParams(bodyParams, schema)
		}
		contentType = "application/json"
	}

	if len(bodyParams) == 1 &&
		!strings.Contains(contentType, "multipart/form-data") &&
		!strings.Contains(contentType, "application/x-www-form-urlencoded") {
		for name, value := range bodyParams {
			propSchema := rb.lookupPropertySchema(schema, name)
			if rb.shouldUseRawBody(schema, propSchema) {
				return rb.encodeRawBody(value, propSchema)
			}
		}
	}

	switch {
	case strings.Contains(contentType, "application/json"):
		return rb.encodeJSONBody(bodyParams, contentType)
	case strings.Contains(contentType, "application/x-www-form-urlencoded"):
		return rb.encodeFormBody(bodyParams)
	case strings.Contains(contentType, "multipart/form-data"):
		return rb.encodeMultipartBody(bodyParams)
	case strings.HasPrefix(contentType, "text/"):
		return rb.encodeTextBody(bodyParams, contentType)
	default:
		return rb.encodeGenericBody(bodyParams, contentType)
	}
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

func (rb *RequestBuilder) shouldUseRawBody(parent ir.Schema, property ir.Schema) bool {
	if property == nil {
		return true
	}

	if t := property.Type(); t != "" && t != "object" {
		return true
	}

	if len(property.Properties()) == 0 {
		return true
	}

	return false
}

func (rb *RequestBuilder) encodeRawBodyFromParams(bodyParams map[string]interface{}, schema ir.Schema) (io.Reader, string, error) {
	for name, value := range bodyParams {
		propSchema := rb.lookupPropertySchema(schema, name)
		return rb.encodeRawBody(value, propSchema)
	}
	return nil, "", nil
}

func (rb *RequestBuilder) encodeRawBody(value interface{}, schema ir.Schema) (io.Reader, string, error) {
	contentType := rb.bodyContentType
	if contentType == "" {
		if schema != nil {
			if t := schema.Type(); t == "string" {
				contentType = "text/plain; charset=utf-8"
			} else if strings.HasPrefix(t, "binary") {
				contentType = "application/octet-stream"
			} else {
				contentType = "application/json"
			}
		} else {
			contentType = "application/json"
		}
	}

	switch v := value.(type) {
	case []byte:
		return bytes.NewReader(v), contentType, nil
	case string:
		return strings.NewReader(v), contentType, nil
	case fmt.Stringer:
		return strings.NewReader(v.String()), contentType, nil
	case json.RawMessage:
		return bytes.NewReader(v), contentType, nil
	default:
		if strings.HasPrefix(contentType, "text/") {
			return strings.NewReader(fmt.Sprintf("%v", v)), contentType, nil
		}
		data, err := json.Marshal(v)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal request body: %w", err)
		}
		return bytes.NewReader(data), contentType, nil
	}
}

func (rb *RequestBuilder) encodeJSONBody(body map[string]interface{}, contentType string) (io.Reader, string, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request body: %w", err)
	}
	return bytes.NewReader(data), contentType, nil
}

func (rb *RequestBuilder) encodeFormBody(body map[string]interface{}) (io.Reader, string, error) {
	values := url.Values{}
	for name, val := range body {
		encoding := rb.lookupEncoding(name)
		style := encoding.Style
		if style == "" {
			style = "form"
		}
		param := ParamInfo{
			Name:    name,
			In:      "form",
			Style:   style,
			Explode: encoding.Explode,
		}
		serialized, err := SerializeParameter(param, val)
		if err != nil {
			return nil, "", err
		}
		rb.addSerializedFormValue(values, name, serialized, style, encoding)
	}
	return strings.NewReader(values.Encode()), "application/x-www-form-urlencoded", nil
}

func (rb *RequestBuilder) encodeMultipartBody(body map[string]interface{}) (io.Reader, string, error) {
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)

	for name, val := range body {
		encoding := rb.lookupEncoding(name)
		headers := make(textproto.MIMEHeader)
		headers.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, name))
		if encoding.ContentType != "" {
			headers.Set("Content-Type", encoding.ContentType)
		}

		part, err := writer.CreatePart(headers)
		if err != nil {
			return nil, "", err
		}

		switch v := val.(type) {
		case []byte:
			if _, err := part.Write(v); err != nil {
				return nil, "", err
			}
		case string:
			if _, err := io.WriteString(part, v); err != nil {
				return nil, "", err
			}
		case fmt.Stringer:
			if _, err := io.WriteString(part, v.String()); err != nil {
				return nil, "", err
			}
		default:
			data, err := json.Marshal(v)
			if err != nil {
				return nil, "", err
			}
			if _, err := part.Write(data); err != nil {
				return nil, "", err
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return buf, writer.FormDataContentType(), nil
}

func (rb *RequestBuilder) encodeTextBody(body map[string]interface{}, contentType string) (io.Reader, string, error) {
	if len(body) == 1 {
		for _, value := range body {
			return rb.encodeRawBody(value, nil)
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request body: %w", err)
	}
	return bytes.NewReader(data), contentType, nil
}

func (rb *RequestBuilder) encodeGenericBody(body map[string]interface{}, contentType string) (io.Reader, string, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request body: %w", err)
	}
	return bytes.NewReader(data), contentType, nil
}

func (rb *RequestBuilder) addSerializedFormValue(values url.Values, name string, data interface{}, style string, encoding ir.EncodingInfo) {
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			values.Add(name, fmt.Sprintf("%v", item))
		}
	case map[string]interface{}:
		if style == "deepObject" {
			for key, val := range v {
				if strings.HasPrefix(key, "[") {
					values.Add(fmt.Sprintf("%s%s", name, key), fmt.Sprintf("%v", val))
				} else {
					values.Add(fmt.Sprintf("%s[%s]", name, key), fmt.Sprintf("%v", val))
				}
			}
			return
		}
		explode := true
		if encoding.Explode != nil {
			explode = *encoding.Explode
		}
		if explode {
			for key, val := range v {
				values.Add(key, fmt.Sprintf("%v", val))
			}
		} else {
			var parts []string
			for key, val := range v {
				parts = append(parts, fmt.Sprintf("%s,%v", key, val))
			}
			values.Add(name, strings.Join(parts, ","))
		}
	default:
		values.Add(name, fmt.Sprintf("%v", v))
	}
}

func (rb *RequestBuilder) lookupEncoding(name string) ir.EncodingInfo {
	if rb.bodyEncoding != nil {
		if enc, ok := rb.bodyEncoding[name]; ok {
			return enc
		}
	}
	return ir.EncodingInfo{}
}

func (rb *RequestBuilder) lookupBodySchema(contentType string) ir.Schema {
	if rb.route.RequestBody == nil {
		return nil
	}
	if rb.route.RequestBody.ContentSchemas == nil {
		return nil
	}
	if schema, ok := rb.route.RequestBody.ContentSchemas[contentType]; ok {
		return schema
	}
	return nil
}

func (rb *RequestBuilder) lookupPropertySchema(parent ir.Schema, name string) ir.Schema {
	if parent == nil {
		return nil
	}
	properties := parent.Properties()
	if prop, ok := properties[name]; ok {
		return prop
	}
	return nil
}

func (rb *RequestBuilder) setContentType(contentType string) {
	if contentType != "" {
		rb.bodyContentType = contentType
	}
	if rb.route.RequestBody != nil && rb.route.RequestBody.Encodings != nil {
		rb.bodyEncoding = rb.route.RequestBody.Encodings[rb.bodyContentType]
	} else {
		rb.bodyEncoding = nil
	}
}
