package openapimcp

import (
	"net/http"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/executor"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/factory"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/mapper"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/parser"
)

type ServerOptions struct {
	HTTPClient    executor.HTTPClient
	HTTPConfig    *HTTPClientConfig
	BaseURL       string
	RouteMaps     []mapper.RouteMap
	RouteMapFunc  mapper.RouteMapFunc
	CustomNames   map[string]string
	ComponentFunc factory.ComponentFunc
	Parser        parser.OpenAPIParser
	ServerName    string
	ServerVersion string
}

func defaultServerOptions() *ServerOptions {
	return &ServerOptions{
		HTTPClient:    executor.NewDefaultHTTPClient(),
		HTTPConfig:    &HTTPClientConfig{Headers: make(http.Header)},
		RouteMaps:     mapper.DefaultRouteMappings(),
		ServerName:    "openapi-mcp-server",
		ServerVersion: "1.0.0",
	}
}

type ServerOption func(*ServerOptions)

func WithHTTPClient(client executor.HTTPClient) ServerOption {
	return func(opts *ServerOptions) {
		opts.HTTPClient = client
	}
}

func WithHTTPClientConfig(cfg *HTTPClientConfig) ServerOption {
	return func(opts *ServerOptions) {
		opts.HTTPConfig = cfg
	}
}

func WithBaseURL(url string) ServerOption {
	return func(opts *ServerOptions) {
		opts.BaseURL = url
	}
}

func WithRouteMaps(maps []mapper.RouteMap) ServerOption {
	return func(opts *ServerOptions) {
		opts.RouteMaps = maps
	}
}

func WithRouteMapFunc(fn mapper.RouteMapFunc) ServerOption {
	return func(opts *ServerOptions) {
		opts.RouteMapFunc = fn
	}
}

func WithCustomNames(names map[string]string) ServerOption {
	return func(opts *ServerOptions) {
		opts.CustomNames = names
	}
}

func WithComponentFunc(fn factory.ComponentFunc) ServerOption {
	return func(opts *ServerOptions) {
		opts.ComponentFunc = fn
	}
}

func WithParser(p parser.OpenAPIParser) ServerOption {
	return func(opts *ServerOptions) {
		opts.Parser = p
	}
}

func WithServerInfo(name, version string) ServerOption {
	return func(opts *ServerOptions) {
		opts.ServerName = name
		opts.ServerVersion = version
	}
}
