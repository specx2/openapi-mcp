package openapimcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/executor"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/factory"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/mapper"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/parser"
)

type Server struct {
	mcpServer *server.MCPServer
	parser    parser.OpenAPIParser
	mapper    *mapper.RouteMapper
	factory   *factory.ComponentFactory
	options   *ServerOptions
}

func prepareHTTPClient(opts *ServerOptions) (executor.HTTPClient, *HTTPClientConfig) {
	client, ok := opts.HTTPClient.(*executor.DefaultHTTPClient)
	config := opts.HTTPConfig
	if !ok {
		if config == nil {
			return opts.HTTPClient, &HTTPClientConfig{Headers: make(http.Header)}
		}
		if config.Headers == nil {
			config.Headers = make(http.Header)
		}
		return opts.HTTPClient, config
	}

	if config == nil {
		config = &HTTPClientConfig{Headers: make(http.Header)}
	} else {
		if config.Headers == nil {
			config.Headers = make(http.Header)
		}
	}

	if config.Timeout > 0 {
		client.WithTimeout(config.Timeout)
	}
	if len(config.Headers) > 0 {
		client.WithHeaders(config.Headers)
	}

	return client, config
}

func NewServer(spec []byte, opts ...ServerOption) (*Server, error) {
	options := defaultServerOptions()
	for _, opt := range opts {
		opt(options)
	}

	p, err := parser.NewParser(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	routes, err := p.ParseSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}

	client, clientConfig := prepareHTTPClient(options)
	options.HTTPClient = client
	if clientConfig != nil && options.BaseURL == "" && clientConfig.BaseURL != "" {
		options.BaseURL = clientConfig.BaseURL
	}

	m := mapper.NewRouteMapper(options.RouteMaps)
	if options.RouteMapFunc != nil {
		m = m.WithMapFunc(options.RouteMapFunc)
	}

	f := factory.NewComponentFactory(options.HTTPClient, options.BaseURL)
	if options.CustomNames != nil {
		f = f.WithCustomNames(options.CustomNames)
	}
	if options.ComponentFunc != nil {
		f = f.WithComponentFunc(options.ComponentFunc)
	}

	mcpServer := server.NewMCPServer(
		options.ServerName,
		options.ServerVersion,
	)

	s := &Server{
		mcpServer: mcpServer,
		parser:    p,
		mapper:    m,
		factory:   f,
		options:   options,
	}

	if err := s.registerComponents(routes); err != nil {
		return nil, fmt.Errorf("failed to register components: %w", err)
	}

	return s, nil
}

func (s *Server) registerComponents(routes []ir.HTTPRoute) error {
	mappedRoutes := s.mapper.MapRoutes(routes)

	components, err := s.factory.CreateComponents(mappedRoutes)
	if err != nil {
		return err
	}

	for _, component := range components {
		switch c := component.(type) {
		case *executor.OpenAPITool:
			s.mcpServer.AddTool(c.Tool(), s.createToolHandler(c))

		case *executor.OpenAPIResource:
			s.mcpServer.AddResource(c.Resource(), s.createResourceHandler(c))

		case *executor.OpenAPIResourceTemplate:
			s.mcpServer.AddResourceTemplate(c.Template(), s.createResourceTemplateHandler(c))
		}
	}

	return nil
}

func (s *Server) createToolHandler(tool *executor.OpenAPITool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return tool.Run(ctx, request)
	}
}

func (s *Server) createResourceHandler(resource *executor.OpenAPIResource) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		content, err := resource.Read(ctx)
		if err != nil {
			return nil, err
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      resource.Resource().URI,
				MIMEType: "application/json",
				Text:     content,
			},
		}, nil
	}
}

func (s *Server) createResourceTemplateHandler(template *executor.OpenAPIResourceTemplate) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		params := extractParametersFromURI(request.Params.URI, template.Template().URITemplate.Raw())

		// Create a parameterized resource instance to handle the request
		paramResource := executor.NewOpenAPIParameterizedResource(
			template.Template().Name,
			template.Template().Description,
			template.GetRoute(),
			template.GetClient(),
			template.GetBaseURL(),
			params,
		)

		content, err := paramResource.Read(ctx)
		if err != nil {
			return nil, err
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "application/json",
				Text:     content,
			},
		}, nil
	}
}

func (s *Server) Serve(transport interface{}) error {
	// The actual serving depends on the transport type
	// This would be implemented based on the specific transport being used
	return nil
}

func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

func extractParametersFromURI(uri, template string) map[string]string {
	params := make(map[string]string)

	clean := func(value string) []string {
		if value == "" {
			return nil
		}

		if idx := strings.IndexAny(value, "?#"); idx >= 0 {
			value = value[:idx]
		}

		value = strings.TrimPrefix(value, "resource://")
		value = strings.Trim(value, "/")
		if value == "" {
			return nil
		}
		return strings.Split(value, "/")
	}

	actualSegments := clean(uri)
	templateSegments := clean(template)
	if len(actualSegments) == 0 || len(templateSegments) == 0 {
		return params
	}

	if len(actualSegments) != len(templateSegments) {
		return params
	}

	for idx, segment := range templateSegments {
		if len(segment) < 2 || segment[0] != '{' || segment[len(segment)-1] != '}' {
			continue
		}

		name := strings.TrimSpace(segment[1 : len(segment)-1])
		if name == "" {
			continue
		}

		value, err := url.PathUnescape(actualSegments[idx])
		if err != nil {
			value = actualSegments[idx]
		}

		params[name] = value
	}

	return params
}
