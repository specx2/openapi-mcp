package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strings"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/parser"
)

type RequestBuilder struct {
	route           ir.HTTPRoute
	paramMap        map[string]ir.ParamMapping
	baseURL         string
	bodyContentType string
	bodyEncoding    map[string]ir.EncodingInfo
}

func NewRequestBuilder(route ir.HTTPRoute, paramMap map[string]ir.ParamMapping, baseURL string) *RequestBuilder {
	bodyContentType := preferredInitialContentType(route.RequestBody)

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
	queryParams := make([]EncodedParameter, 0)
	var headerParams []EncodedParameter
	var cookieParams []EncodedParameter
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
				serialized, err := rb.serializePathParam(mapping.OpenAPIName, argValue)
				if err != nil {
					return nil, err
				}
				pathParams[mapping.OpenAPIName] = serialized
			}
		case ir.ParameterInQuery:
			if argValue != nil {
				paramInfo := rb.findParameterInfo(mapping.OpenAPIName, mapping.Location)
				info := ir.ParameterInfo{Name: mapping.OpenAPIName, In: mapping.Location}
				if paramInfo != nil {
					info = *paramInfo
				}
				encoded, err := encodeParameterValues(info, argValue)
				if err != nil {
					return nil, err
				}
				queryParams = append(queryParams, encoded...)
			}
		case ir.ParameterInHeader:
			if argValue != nil {
				paramInfo := rb.findParameterInfo(mapping.OpenAPIName, mapping.Location)
				info := ir.ParameterInfo{Name: mapping.OpenAPIName, In: mapping.Location}
				if paramInfo != nil {
					info = *paramInfo
				}
				encoded, err := encodeParameterValues(info, argValue)
				if err != nil {
					return nil, err
				}
				headerParams = append(headerParams, encoded...)
			}
		case ir.ParameterInCookie:
			if argValue != nil {
				paramInfo := rb.findParameterInfo(mapping.OpenAPIName, mapping.Location)
				info := ir.ParameterInfo{Name: mapping.OpenAPIName, In: mapping.Location}
				if paramInfo != nil {
					info = *paramInfo
				}
				encoded, err := encodeParameterValues(info, argValue)
				if err != nil {
					return nil, err
				}
				cookieParams = append(cookieParams, encoded...)
			}
		case "body":
			if argValue != nil {
				bodyParams[mapping.OpenAPIName] = argValue
			}
		}
	}

	if rawBody == nil {
		rb.applyBodyDefaults(bodyParams)
	}

	if overrideContentType != "" {
		rb.setContentType(overrideContentType)
	} else {
		selected := rb.chooseContentType(bodyParams, rawBody)
		rb.setContentType(selected)
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

	for _, pair := range headerParams {
		req.Header.Add(pair.Name, pair.Value)
	}

	for _, cookie := range cookieParams {
		req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}

	if req.Header.Get("Accept") == "" {
		if accept := preferredResponseContentType(rb.route); accept != "" {
			req.Header.Set("Accept", accept)
		}
	}

	return req, nil
}

func cloneAnyValue(value interface{}) interface{} {
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

func resolveEncodingHeaderValue(header ir.HeaderInfo) (string, bool) {
	if header.Schema != nil {
		if def, ok := header.Schema["default"]; ok {
			return formatScalar(def), true
		}
	}
	if header.Example != nil {
		return formatScalar(header.Example), true
	}
	for _, example := range header.Examples {
		switch v := example.(type) {
		case map[string]interface{}:
			if val, ok := v["value"]; ok {
				return formatScalar(val), true
			}
		default:
			return formatScalar(v), true
		}
	}
	return "", false
}

func (rb *RequestBuilder) applyBodyDefaults(bodyParams map[string]interface{}) {
	if rb.route.RequestBody == nil || len(rb.route.RequestBody.ContentSchemas) == 0 {
		return
	}

	contentType := rb.bodyContentType
	if contentType == "" {
		for ct := range rb.route.RequestBody.ContentSchemas {
			contentType = ct
			break
		}
	}

	schema := rb.lookupBodySchema(contentType)
	if schema == nil {
		return
	}

	properties := schema.Properties()
	if len(properties) > 0 {
		for name, propSchema := range properties {
			if _, exists := bodyParams[name]; exists {
				continue
			}
			if def, ok := propSchema["default"]; ok {
				bodyParams[name] = cloneAnyValue(def)
			}
		}
		return
	}

	if def, ok := schema["default"]; ok {
		propName := rb.primaryBodyPropertyName()
		if propName == "" {
			return
		}
		if _, exists := bodyParams[propName]; !exists {
			bodyParams[propName] = cloneAnyValue(def)
		}
	}
}

func (rb *RequestBuilder) primaryBodyPropertyName() string {
	for name, mapping := range rb.paramMap {
		if mapping.Location == "body" {
			return name
		}
	}
	return ""
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

func (rb *RequestBuilder) buildURL(pathParams map[string]string, queryParams []EncodedParameter) (string, error) {
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
		parsedURL.RawQuery = buildQueryString(queryParams)
	}

	return parsedURL.String(), nil
}

var reservedReplacer = strings.NewReplacer(
	"%3A", ":",
	"%2F", "/",
	"%3F", "?",
	"%23", "#",
	"%5B", "[",
	"%5D", "]",
	"%40", "@",
	"%21", "!",
	"%24", "$",
	"%26", "&",
	"%27", "'",
	"%28", "(",
	"%29", ")",
	"%2A", "*",
	"%2B", "+",
	"%2C", ",",
	"%3B", ";",
	"%3D", "=",
)

func buildQueryString(params []EncodedParameter) string {
	if len(params) == 0 {
		return ""
	}

	var builder strings.Builder
	for i, param := range params {
		if i > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(url.QueryEscape(param.Name))
		builder.WriteByte('=')
		builder.WriteString(encodeQueryComponent(param.Value, param.AllowReserved))
	}
	return builder.String()
}

func encodeQueryComponent(value string, allowReserved bool) string {
	escaped := url.QueryEscape(value)
	if !allowReserved {
		return escaped
	}
	return reservedReplacer.Replace(escaped)
}

func (rb *RequestBuilder) chooseContentType(bodyParams map[string]interface{}, rawBody interface{}) string {
	if rb.route.RequestBody == nil || len(rb.route.RequestBody.ContentSchemas) == 0 {
		return rb.bodyContentType
	}

	available := rb.availableContentTypes()
	if len(available) == 0 {
		return rb.bodyContentType
	}

	if rawBody != nil {
		if ct := rb.contentTypeForRawBody(rawBody, available); ct != "" {
			return ct
		}
	}

	if containsBinaryValues(bodyParams) {
		if ct := firstMatchingContentType(available, func(ct string) bool {
			return strings.EqualFold(ct, "multipart/form-data")
		}); ct != "" {
			return ct
		}
		if ct := firstMatchingContentType(available, func(ct string) bool {
			return strings.EqualFold(ct, "application/octet-stream")
		}); ct != "" {
			return ct
		}
	}

	if rb.bodyContentType != "" && rb.hasContentType(rb.bodyContentType) {
		return rb.bodyContentType
	}

	return rb.selectBestContentType(available, bodyParams)
}

func (rb *RequestBuilder) hasContentType(contentType string) bool {
	if rb.route.RequestBody == nil {
		return false
	}
	if _, ok := rb.route.RequestBody.ContentSchemas[contentType]; ok {
		return true
	}
	return false
}

func containsBinaryValues(bodyParams map[string]interface{}) bool {
	for _, value := range bodyParams {
		switch value.(type) {
		case []byte:
			return true
		}
	}
	return false
}

func preferredInitialContentType(body *ir.RequestBodyInfo) string {
	if body == nil || len(body.ContentSchemas) == 0 {
		return ""
	}

	candidates := body.ContentOrder
	if len(candidates) == 0 {
		for ct := range body.ContentSchemas {
			candidates = append(candidates, ct)
		}
		sort.Strings(candidates)
	}

	var first string
	for _, ct := range candidates {
		if _, ok := body.ContentSchemas[ct]; !ok {
			continue
		}
		if first == "" {
			first = ct
		}
		if strings.Contains(strings.ToLower(ct), "json") {
			return ct
		}
	}
	return first
}

func preferredResponseContentType(route ir.HTTPRoute) string {
	successStatuses := []string{"200", "201", "202", "203", "204"}

	for _, status := range successStatuses {
		if response, ok := route.Responses[status]; ok {
			if ct := parser.GetContentType(response.ContentSchemas); ct != "" {
				return ct
			}
		}
	}

	if response, ok := route.Responses["default"]; ok {
		if ct := parser.GetContentType(response.ContentSchemas); ct != "" {
			return ct
		}
	}

	for _, response := range route.Responses {
		if ct := parser.GetContentType(response.ContentSchemas); ct != "" {
			return ct
		}
	}

	return ""
}

func (rb *RequestBuilder) availableContentTypes() []string {
	if rb.route.RequestBody == nil {
		return nil
	}
	order := rb.route.RequestBody.ContentOrder
	if len(order) > 0 {
		result := make([]string, 0, len(order))
		seen := make(map[string]struct{}, len(order))
		for _, ct := range order {
			if _, ok := rb.route.RequestBody.ContentSchemas[ct]; !ok {
				continue
			}
			if _, exists := seen[ct]; exists {
				continue
			}
			seen[ct] = struct{}{}
			result = append(result, ct)
		}
		if len(result) > 0 {
			return result
		}
	}

	types := make([]string, 0, len(rb.route.RequestBody.ContentSchemas))
	for ct := range rb.route.RequestBody.ContentSchemas {
		types = append(types, ct)
	}
	sort.Strings(types)
	return types
}

func (rb *RequestBuilder) contentTypeForRawBody(rawBody interface{}, available []string) string {
	switch raw := rawBody.(type) {
	case []byte:
		if ct := firstMatchingContentType(available, func(ct string) bool {
			return strings.EqualFold(ct, "application/octet-stream")
		}); ct != "" {
			return ct
		}
	case string:
		trimmed := strings.TrimSpace(raw)
		if isLikelyJSON(trimmed) {
			if ct := firstMatchingContentType(available, func(ct string) bool {
				lower := strings.ToLower(ct)
				return lower == "application/json" || strings.Contains(lower, "+json")
			}); ct != "" {
				return ct
			}
		}
		if ct := firstMatchingContentType(available, func(ct string) bool {
			return strings.EqualFold(ct, "text/plain")
		}); ct != "" {
			return ct
		}
	}
	return ""
}

func (rb *RequestBuilder) selectBestContentType(available []string, bodyParams map[string]interface{}) string {
	best := ""
	bestScore := math.MinInt
	for idx, ct := range available {
		score := rb.scoreContentType(ct, idx, bodyParams)
		if score > bestScore {
			bestScore = score
			best = ct
		}
	}
	if best == "" {
		return rb.bodyContentType
	}
	return best
}

func (rb *RequestBuilder) scoreContentType(contentType string, index int, bodyParams map[string]interface{}) int {
	score := 0
	lower := strings.ToLower(contentType)

	// Prefer earlier entries in the OpenAPI document
	score -= index

	if rb.bodyContentType != "" && strings.EqualFold(contentType, rb.bodyContentType) {
		score += 40
	}

	switch {
	case lower == "application/json":
		score += 400
	case strings.HasSuffix(lower, "+json") || strings.Contains(lower, "+json"):
		score += 360
	case lower == "application/x-www-form-urlencoded":
		score += 300
	case lower == "multipart/form-data":
		score += 280
	case lower == "text/plain":
		score += 200
	case lower == "application/octet-stream":
		score += 180
	}

	schema := rb.lookupBodySchema(contentType)
	if schema != nil {
		switch schema.Type() {
		case "object":
			if len(bodyParams) > 0 {
				score += 60
			}
		case "array":
			if len(bodyParams) > 0 {
				score += 30
			}
		}
		if schemaIndicatesBinary(schema) {
			score += 45
		}
	}

	return score
}

func schemaIndicatesBinary(schema ir.Schema) bool {
	if schema == nil {
		return false
	}
	if format, ok := schema["format"].(string); ok {
		switch strings.ToLower(format) {
		case "binary", "byte":
			return true
		}
	}
	if enc, ok := schema["contentEncoding"].(string); ok && enc != "" {
		return true
	}
	if mediaType, ok := schema["contentMediaType"].(string); ok && mediaType != "" {
		return true
	}
	switch schema.Type() {
	case "array":
		if items, ok := schema["items"].(map[string]interface{}); ok {
			return schemaIndicatesBinary(items)
		}
	case "object":
		for _, prop := range schema.Properties() {
			if schemaIndicatesBinary(prop) {
				return true
			}
		}
	}
	return false
}

func firstMatchingContentType(types []string, predicate func(string) bool) string {
	for _, ct := range types {
		if predicate(ct) {
			return ct
		}
	}
	return ""
}

func isLikelyJSON(value string) bool {
	if value == "" {
		return false
	}
	trimmed := strings.TrimSpace(value)
	return (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))
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

func (rb *RequestBuilder) serializePathParam(name string, value interface{}) (string, error) {
	paramInfo := rb.findParameterInfo(name, ir.ParameterInPath)
	style := "simple"
	var explodePtr *bool
	if paramInfo != nil {
		if paramInfo.Style != "" {
			style = paramInfo.Style
		}
		explodePtr = paramInfo.Explode
	}

	explode := false
	if explodePtr != nil {
		explode = *explodePtr
	} else if style == "form" || style == "simple" {
		// defaults already false for path
	}

	switch style {
	case "label":
		return serializeLabelPath(value, explode), nil
	case "matrix":
		return serializeMatrixPath(name, value, explode), nil
	default:
		return serializeSimplePath(value, explode), nil
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

func serializeSimplePath(value interface{}, explode bool) string {
	if arr, ok := valueAsSlice(value); ok {
		return strings.Join(stringifySlice(arr), ",")
	}

	if obj, ok := valueAsMap(value); ok {
		keys := sortedKeys(obj)
		if explode {
			pairs := make([]string, 0, len(keys))
			for _, key := range keys {
				pairs = append(pairs, fmt.Sprintf("%s=%s", key, formatScalar(obj[key])))
			}
			return strings.Join(pairs, ",")
		}
		parts := make([]string, 0, len(keys)*2)
		for _, key := range keys {
			parts = append(parts, key, formatScalar(obj[key]))
		}
		return strings.Join(parts, ",")
	}

	return formatScalar(value)
}

func serializeLabelPath(value interface{}, explode bool) string {
	if arr, ok := valueAsSlice(value); ok {
		delimiter := ","
		if explode {
			delimiter = "."
		}
		return "." + strings.Join(stringifySlice(arr), delimiter)
	}

	if obj, ok := valueAsMap(value); ok {
		keys := sortedKeys(obj)
		if explode {
			pairs := make([]string, 0, len(keys))
			for _, key := range keys {
				pairs = append(pairs, fmt.Sprintf("%s=%s", key, formatScalar(obj[key])))
			}
			return "." + strings.Join(pairs, ".")
		}
		parts := make([]string, 0, len(keys)*2)
		for _, key := range keys {
			parts = append(parts, key, formatScalar(obj[key]))
		}
		return "." + strings.Join(parts, ",")
	}

	return "." + formatScalar(value)
}

func serializeMatrixPath(name string, value interface{}, explode bool) string {
	if arr, ok := valueAsSlice(value); ok {
		values := stringifySlice(arr)
		if explode {
			segments := make([]string, len(values))
			for i, v := range values {
				segments[i] = fmt.Sprintf(";%s=%s", name, v)
			}
			return strings.Join(segments, "")
		}
		return fmt.Sprintf(";%s=%s", name, strings.Join(values, ","))
	}

	if obj, ok := valueAsMap(value); ok {
		keys := sortedKeys(obj)
		if explode {
			segments := make([]string, 0, len(keys))
			for _, key := range keys {
				segments = append(segments, fmt.Sprintf(";%s=%s", key, formatScalar(obj[key])))
			}
			return strings.Join(segments, "")
		}
		parts := make([]string, 0, len(keys)*2)
		for _, key := range keys {
			parts = append(parts, key, formatScalar(obj[key]))
		}
		return fmt.Sprintf(";%s=%s", name, strings.Join(parts, ","))
	}

	return fmt.Sprintf(";%s=%s", name, formatScalar(value))
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
		if len(encoding.Headers) > 0 {
			for headerName, headerInfo := range encoding.Headers {
				if headerValue, ok := resolveEncodingHeaderValue(headerInfo); ok {
					headers.Set(headerName, headerValue)
				}
			}
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
