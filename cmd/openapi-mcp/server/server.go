package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	mcpsrv "github.com/mark3labs/mcp-go/server"
	"github.com/specx2/mcp-forgebird/core"
	"github.com/specx2/mcp-forgebird/core/interfaces"
	executorpkg "github.com/specx2/openapi-mcp/core/executor"
	openapiparser "github.com/specx2/openapi-mcp/core/parser"
	forgebird "github.com/specx2/openapi-mcp/forgebird"
)

// Spec describes an OpenAPI document to register with the server.
type Spec struct {
	// Path to the specification file (absolute preferred) used for resolving references.
	Path string
	// Data holds the raw OpenAPI document bytes.
	Data []byte
	// Version optionally overrides the detected OpenAPI version.
	Version string
}

// Options controls server construction.
type Options struct {
	Specs         []Spec
	BaseURL       string
	Timeout       time.Duration
	Headers       http.Header
	ServerName    string
	ServerVersion string
	GlobalTags    []string
	CustomNames   map[string]string
}

// Server wraps an MCP server populated through the OpenAPI plugin.
type Server struct {
	options    Options
	mcpServer  *mcpsrv.MCPServer
	httpClient *executorpkg.DefaultHTTPClient
}

// New constructs a new Server instance using the supplied options.
func New(opts Options) (*Server, error) {
	if len(opts.Specs) == 0 {
		return nil, fmt.Errorf("at least one spec must be provided")
	}

	timeout := opts.Timeout
	if timeout < 0 {
		timeout = 0
	}

	httpClient := executorpkg.NewDefaultHTTPClient()
	if timeout > 0 {
		httpClient.WithTimeout(timeout)
	}
	if len(opts.Headers) > 0 {
		httpClient.WithHeaders(opts.Headers)
	}

	serverName := opts.ServerName
	if serverName == "" {
		serverName = "openapi-mcp"
	}
	serverVersion := opts.ServerVersion
	if serverVersion == "" {
		serverVersion = "0.1.0"
	}

	pipeline := forgebird.NewPipeline()
	fb := core.NewForgebird(pipeline)

	mcpServer := mcpsrv.NewMCPServer(serverName, serverVersion)

	server := &Server{
		options:    opts,
		mcpServer:  mcpServer,
		httpClient: httpClient,
	}

	// Convert and register each spec
	for _, spec := range opts.Specs {
		if err := server.registerSpec(fb, spec); err != nil {
			return nil, err
		}
	}

	return server, nil
}

// MCPServer exposes the underlying MCP server.
func (s *Server) MCPServer() *mcpsrv.MCPServer {
	return s.mcpServer
}

func (s *Server) registerSpec(fb *core.DefaultForgebird, spec Spec) error {
	data := spec.Data
	if len(data) == 0 {
		return fmt.Errorf("spec %s contains no data", spec.Path)
	}

	absPath := spec.Path
	if absPath != "" {
		if !filepath.IsAbs(absPath) {
			resolved, err := filepath.Abs(absPath)
			if err == nil {
				absPath = resolved
			}
		}
		absPath = filepath.ToSlash(absPath)
	}

	version := spec.Version
	if version == "" {
		if detected, err := openapiparser.DetectOpenAPIVersion(data); err == nil {
			version = detected
		}
	}

	conversionConfig := interfaces.ConversionConfig{
		BaseURL: s.options.BaseURL,
		Timeout: int(s.options.Timeout.Seconds()),
		Spec: interfaces.SpecConfig{
			Version: version,
			SpecURL: absPath,
		},
		Mapping: interfaces.MappingConfig{
			GlobalTags:  append([]string(nil), s.options.GlobalTags...),
			CustomNames: s.options.CustomNames,
		},
		Output: interfaces.OutputConfig{
			IncludeMetadata:   true,
			IncludeExtensions: true,
		},
	}

	components, err := fb.ConvertSpec(data, conversionConfig)
	if err != nil {
		return fmt.Errorf("failed to convert spec %s: %w", spec.Path, err)
	}

	// 使用 forgebird 的默认注册能力，零自定义 handler
	return forgebird.RegisterComponents(
		s.mcpServer,
		components,
		forgebird.WithBaseURL(s.options.BaseURL),
		forgebird.WithHTTPClient(s.httpClient),
	)
}
