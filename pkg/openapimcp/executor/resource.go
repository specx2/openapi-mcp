package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/internal"
)

type OpenAPIResource struct {
	resource mcp.Resource
	route    ir.HTTPRoute
	client   HTTPClient
	baseURL  string
}

func NewOpenAPIResource(
	name string,
	description string,
	route ir.HTTPRoute,
	client HTTPClient,
	baseURL string,
) *OpenAPIResource {
	uri := fmt.Sprintf("resource://%s", name)

	resource := mcp.NewResource(
		uri,
		name,
		mcp.WithResourceDescription(description),
		mcp.WithMIMEType("application/json"),
	)

	return &OpenAPIResource{
		resource: resource,
		route:    route,
		client:   client,
		baseURL:  baseURL,
	}
}

func (r *OpenAPIResource) Resource() mcp.Resource {
	return r.resource
}

func (r *OpenAPIResource) SetResource(resource mcp.Resource) {
	r.resource = resource
}

func (r *OpenAPIResource) Read(ctx context.Context) (string, error) {
	reqURL, err := r.buildURL()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", err
	}

	if mcpHeaders := internal.GetMCPHeaders(ctx); mcpHeaders != nil {
		for k, v := range mcpHeaders {
			req.Header.Set(k, v)
		}
	}

	resp, err := r.client.Do(req)
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

func (r *OpenAPIResource) buildURL() (string, error) {
	urlPath := r.route.Path

	var fullURL string
	if r.baseURL != "" {
		baseURL := strings.TrimSuffix(r.baseURL, "/")
		urlPath = strings.TrimPrefix(urlPath, "/")
		fullURL = fmt.Sprintf("%s/%s", baseURL, urlPath)
	} else {
		fullURL = urlPath
	}

	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	return parsedURL.String(), nil
}