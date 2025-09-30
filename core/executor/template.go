package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/specx2/openapi-mcp/core/internal"
	"github.com/specx2/openapi-mcp/core/ir"
)

type OpenAPIResourceTemplate struct {
	template mcp.ResourceTemplate
	route    ir.HTTPRoute
	client   HTTPClient
	baseURL  string
}

func NewOpenAPIResourceTemplate(
	name string,
	description string,
	route ir.HTTPRoute,
	client HTTPClient,
	baseURL string,
) *OpenAPIResourceTemplate {
	// 构建 URI 模板字符串（包含查询参数和 Header 参数）
	uriTemplate := buildURITemplate(name, route)

	// 创建 MCP ResourceTemplate
	template := mcp.NewResourceTemplate(
		uriTemplate,
		name,
		mcp.WithTemplateDescription(description),
		mcp.WithTemplateMIMEType("application/json"),
	)

	return &OpenAPIResourceTemplate{
		template: template,
		route:    route,
		client:   client,
		baseURL:  baseURL,
	}
}

func (rt *OpenAPIResourceTemplate) Template() mcp.ResourceTemplate {
	return rt.template
}

func (rt *OpenAPIResourceTemplate) SetTemplate(template mcp.ResourceTemplate) {
	rt.template = template
}

func (rt *OpenAPIResourceTemplate) GetRoute() ir.HTTPRoute {
	return rt.route
}

func (rt *OpenAPIResourceTemplate) GetClient() HTTPClient {
	return rt.client
}

func (rt *OpenAPIResourceTemplate) GetBaseURL() string {
	return rt.baseURL
}

func (rt *OpenAPIResourceTemplate) CreateResource(ctx context.Context, uri string, params map[string]string) (mcp.Resource, error) {
	name := fmt.Sprintf("%s_%s", rt.template.Name, generateResourceSuffix(params))

	resource := NewOpenAPIParameterizedResource(
		name,
		rt.template.Description,
		rt.route,
		rt.client,
		rt.baseURL,
		params,
	)

	return resource.Resource(), nil
}

func extractPathParameters(route ir.HTTPRoute) []string {
	var pathParams []string
	for _, param := range route.Parameters {
		if param.In == ir.ParameterInPath {
			pathParams = append(pathParams, param.Name)
		}
	}
	return pathParams
}

func buildURITemplate(name string, route ir.HTTPRoute) string {
	// 构建完整的路径模板，包括路径参数（不加 resource:// 前缀，遵循 MCP 对模板的约定）
	pathTemplate := route.Path
	if pathTemplate == "" {
		pathTemplate = "/" + name
	}

	// 模板中不需要以斜杠开头
	base := strings.TrimPrefix(pathTemplate, "/")

	// Collect query parameters and Header parameters
	var queryParamNames []string
	var headerParamNames []string
	for _, param := range route.Parameters {
		if param.In == ir.ParameterInQuery {
			queryParamNames = append(queryParamNames, param.Name)
		} else if param.In == ir.ParameterInHeader {
			// Header parameters use special prefix
			headerParamNames = append(headerParamNames, "__header__"+param.Name)
		}
	}

	// Use RFC 6570 URI Template syntax to add query parameters: users{?page,limit,__header__Authorization}
	if len(queryParamNames) > 0 || len(headerParamNames) > 0 {
		var allParams []string
		allParams = append(allParams, queryParamNames...)
		allParams = append(allParams, headerParamNames...)
		base += "{?" + strings.Join(allParams, ",") + "}"
	}

	return base
}

func buildParametersSchema(route ir.HTTPRoute) ir.Schema {
	schema := ir.Schema{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	var required []string

	// 处理所有参数
	for _, param := range route.Parameters {
		if param.In == ir.ParameterInPath {
			// 路径参数（必须）
			schema["properties"].(map[string]interface{})[param.Name] = param.Schema
			if param.Required {
				required = append(required, param.Name)
			}
		} else if param.In == ir.ParameterInQuery || param.In == ir.ParameterInHeader {
			// 查询参数和 Header 参数（可选，允许为空使用默认值）
			schema["properties"].(map[string]interface{})[param.Name] = param.Schema
			// 不添加到 required 中，允许为空
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

func generateResourceSuffix(params map[string]string) string {
	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s_%s", k, v))
	}
	return strings.Join(parts, "_")
}

type OpenAPIParameterizedResource struct {
	*OpenAPIResource
	params       map[string]string
	headerParams map[string]string
}

func NewOpenAPIParameterizedResource(
	name string,
	description string,
	route ir.HTTPRoute,
	client HTTPClient,
	baseURL string,
	params map[string]string,
) *OpenAPIParameterizedResource {
	resource := NewOpenAPIResource(name, description, route, client, baseURL)

	return &OpenAPIParameterizedResource{
		OpenAPIResource: resource,
		params:          params,
	}
}

func (pr *OpenAPIParameterizedResource) Read(ctx context.Context) (string, error) {
	reqURL, err := pr.buildParameterizedURL()
	if err != nil {
		return "", err
	}

	return pr.readFromParameterizedURL(ctx, reqURL)
}

func (pr *OpenAPIParameterizedResource) buildParameterizedURL() (string, error) {
	urlPath := pr.route.Path

	// 处理路径参数
	for paramName, paramValue := range pr.params {
		placeholder := fmt.Sprintf("{%s}", paramName)
		if strings.Contains(urlPath, placeholder) {
			urlPath = strings.ReplaceAll(urlPath, placeholder, paramValue)
		}
	}

	// 构建基础 URL
	var fullURL string
	if pr.baseURL != "" {
		baseURL := strings.TrimSuffix(pr.baseURL, "/")
		urlPath = strings.TrimPrefix(urlPath, "/")
		fullURL = fmt.Sprintf("%s/%s", baseURL, urlPath)
	} else {
		fullURL = urlPath
	}

	// 处理查询参数和 Header 参数（保持参数顺序）
	var queryParts []string
	headerParams := make(map[string]string)

	for _, param := range pr.route.Parameters {
		if param.In == ir.ParameterInQuery {
			// 尝试使用原始名称，如果不存在则尝试 sanitized 名称（下划线版本）
			paramValue, exists := pr.params[param.Name]
			if !exists {
				// 尝试 sanitized 版本（破折号替换为下划线）
				sanitizedName := strings.ReplaceAll(param.Name, "-", "_")
				paramValue, exists = pr.params[sanitizedName]
			}
			if exists && paramValue != "" {
				queryParts = append(queryParts, fmt.Sprintf("%s=%s", param.Name, paramValue))
			}
			// 如果参数为空，不添加到查询字符串中（使用默认值）
		} else if param.In == ir.ParameterInHeader {
			// 尝试使用原始名称，如果不存在则尝试 sanitized 名称（下划线版本）
			paramValue, exists := pr.params[param.Name]
			if !exists {
				// 尝试 sanitized 版本（破折号替换为下划线）
				sanitizedName := strings.ReplaceAll(param.Name, "-", "_")
				paramValue, exists = pr.params[sanitizedName]
			}
			if exists && paramValue != "" {
				headerParams[param.Name] = paramValue
			}
			// Header 参数不添加到 URL 查询字符串中
		}
	}

	// 添加查询参数到 URL
	if len(queryParts) > 0 {
		fullURL += "?" + strings.Join(queryParts, "&")
	}

	// 将 Header 参数存储到上下文中，供后续 HTTP 请求使用
	if len(headerParams) > 0 {
		// 这里需要将 Header 参数传递给 HTTP 请求
		// 我们可以在 OpenAPIParameterizedResource 中添加一个字段来存储 Header 参数
		pr.headerParams = headerParams
	}

	return fullURL, nil
}

func (pr *OpenAPIParameterizedResource) readFromParameterizedURL(ctx context.Context, reqURL string) (string, error) {
	// reqURL 已经在 buildParameterizedURL() 中构建好了完整 URL，直接使用
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", err
	}

	// 添加 MCP Headers
	if mcpHeaders := internal.GetMCPHeaders(ctx); mcpHeaders != nil {
		for k, v := range mcpHeaders {
			req.Header.Set(k, v)
		}
	}

	// 添加 Header 参数
	for headerName, headerValue := range pr.headerParams {
		req.Header.Set(headerName, headerValue)
	}

	resp, err := pr.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "json") {
		var jsonResult interface{}
		if json.Unmarshal(body, &jsonResult) == nil {
			prettyJSON, err := json.MarshalIndent(jsonResult, "", "  ")
			if err == nil {
				return string(prettyJSON), nil
			}
		}
	}

	return string(body), nil
}
