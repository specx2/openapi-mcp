package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/specx2/openapi-mcp/pkg/openapimcp/ir"
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
	pathParams := extractPathParameters(route)
	uriTemplate := buildURITemplate(name, pathParams)

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

func buildURITemplate(name string, pathParams []string) string {
	if len(pathParams) == 0 {
		return fmt.Sprintf("resource://%s", name)
	}

	templateVars := make([]string, len(pathParams))
	for i, param := range pathParams {
		templateVars[i] = fmt.Sprintf("{%s}", param)
	}

	return fmt.Sprintf("resource://%s/%s", name, strings.Join(templateVars, "/"))
}

func buildParametersSchema(route ir.HTTPRoute, pathParams []string) ir.Schema {
	schema := ir.Schema{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	var required []string

	for _, paramName := range pathParams {
		for _, param := range route.Parameters {
			if param.Name == paramName && param.In == ir.ParameterInPath {
				schema["properties"].(map[string]interface{})[paramName] = param.Schema
				if param.Required {
					required = append(required, paramName)
				}
				break
			}
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
	params map[string]string
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

	for paramName, paramValue := range pr.params {
		placeholder := fmt.Sprintf("{%s}", paramName)
		urlPath = strings.ReplaceAll(urlPath, placeholder, paramValue)
	}

	var fullURL string
	if pr.baseURL != "" {
		baseURL := strings.TrimSuffix(pr.baseURL, "/")
		urlPath = strings.TrimPrefix(urlPath, "/")
		fullURL = fmt.Sprintf("%s/%s", baseURL, urlPath)
	} else {
		fullURL = urlPath
	}

	return fullURL, nil
}

func (pr *OpenAPIParameterizedResource) readFromParameterizedURL(ctx context.Context, reqURL string) (string, error) {
	origURL := pr.baseURL
	origPath := pr.route.Path

	// Temporarily modify for parameterized request
	pr.baseURL = ""
	pr.route.Path = reqURL

	result, err := pr.OpenAPIResource.Read(ctx)

	// Restore original values
	pr.baseURL = origURL
	pr.route.Path = origPath

	return result, err
}
