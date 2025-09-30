# OpenAPI MCP

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

OpenAPI MCP is a Go framework that converts OpenAPI specifications (Swagger) into MCP (Model Context Protocol) servers. It enables seamless integration of existing REST APIs with AI models through the Model Context Protocol.

[中文文档](README_CN.md)

## 🚀 Features

- **Multiple Protocol Support**: OpenAPI 3.0 and 3.1 specifications
- **Built on Forgebird**: Leverages the powerful [mcp-forgebird](https://github.com/specx2/mcp-forgebird) framework
- **Flexible Mapping**: Convert OpenAPI operations to MCP Tools, Resources, or ResourceTemplates
- **Dual Transport Modes**: Support for stdio (CLI) and SSE (HTTP server) modes
- **RFC 6570 URI Templates**: Full support for parameterized resource URIs
- **Custom Authentication**: Pluggable HTTP client for custom authentication logic
- **Multi-Spec Support**: Load and serve multiple OpenAPI specifications simultaneously

## 📦 Installation

```bash
go get github.com/specx2/openapi-mcp
```

## 🎯 Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "os"

    mcpsrv "github.com/mark3labs/mcp-go/server"
    "github.com/specx2/openapi-mcp/forgebird"
    "github.com/specx2/mcp-forgebird/core"
    "github.com/specx2/mcp-forgebird/core/interfaces"
)

func main() {
    // Load OpenAPI specification
    specBytes, err := os.ReadFile("petstore.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Create Forgebird pipeline with custom mapping strategy
    pipeline := forgebird.NewPipeline()
    fb := core.NewForgebird(pipeline)

    // Convert OpenAPI spec to MCP components
    components, err := fb.ConvertSpec(specBytes, interfaces.ConversionConfig{
        BaseURL: "https://petstore.swagger.io/v1",
        Timeout: 15,
        Spec:    interfaces.SpecConfig{SpecURL: "petstore.yaml"},
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create and register MCP server
    mserver := mcpsrv.NewMCPServer("petstore-mcp", "1.0.0")
    if err := forgebird.RegisterComponents(mserver, components); err != nil {
        log.Fatal(err)
    }

    // Start server in stdio mode
    stdio := mcpsrv.NewStdioServer(mserver)
    stdio.Listen(context.Background(), os.Stdin, os.Stdout)
}
```

### Using the CLI

```bash
# stdio mode (default)
openapi-mcp -spec petstore.yaml -base-url https://api.example.com

# SSE mode (HTTP server)
openapi-mcp -spec petstore.yaml -base-url https://api.example.com -sse -sse-addr :8080

# Multiple specs
openapi-mcp -spec spec1.yaml -spec spec2.yaml -base-url https://api.example.com

# With custom logging
openapi-mcp -spec petstore.yaml -log-output server.log -log-tee-console
```

## 🏗️ Architecture

```
┌─────────────────┐
│ OpenAPI Spec    │
│ (YAML/JSON)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Forgebird       │
│ Pipeline        │
│ - Parser        │
│ - RouteMapper   │
│ - Factory       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ MCP Components  │
│ - Tools         │
│ - Resources     │
│ - Templates     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ MCP Server      │
│ (mcp-go)        │
└─────────────────┘
```

### Core Layers

1. **Parser Layer** (`forgebird/parser.go`)
   - OpenAPI specification parsing and validation
   - Support for both OpenAPI 3.0 and 3.1
   - Schema reference resolution

2. **Mapping Layer** (`forgebird/route_mapper.go`)
   - Convert OpenAPI operations to MCP component types
   - Default: GET requests → Tool + ResourceTemplate, Others → Tool
   - Customizable mapping rules via pipeline configuration

3. **Factory Layer** (`mcp-forgebird/core/factory`)
   - Generate MCP Tool, Resource, and ResourceTemplate definitions
   - Schema combination and parameter collision handling
   - JSON Schema generation from OpenAPI schemas

4. **Executor Layer** (`core/executor`)
   - HTTP request construction from MCP tool calls
   - Parameter serialization (path, query, header, body)
   - Response processing and validation

5. **Registration Layer** (`forgebird/api.go`)
   - Register MCP components with mcp-go server
   - Handle Tool execution and Resource fetching
   - URI template matching for ResourceTemplates

## 🎨 Mapping Strategies

### Default Mapping (One-to-Many)

By default, GET requests generate both a Tool and a ResourceTemplate, while other methods generate only Tools:

```yaml
GET /pets/{id}          → Tool: get_api_pets_id + ResourceTemplate: resource://api/pets/{id}{?param1,param2}
POST /pets              → Tool: post_api_pets
PUT /pets/{id}          → Tool: put_api_pets_id
DELETE /pets/{id}       → Tool: delete_api_pets_id
```

### Custom Mapping

You can customize the mapping strategy in the pipeline:

```go
pipeline := forgebird.NewPipeline()

// Customize the route mapper
customMapper := &forgebird.RouteMapper{
    // Your custom mapping logic
}
pipeline.SetRouteMapper(customMapper)
```

## 🔧 Advanced Usage

### Custom Authentication

```go
package main

import (
    "net/http"

    "github.com/specx2/openapi-mcp/core/executor"
    "github.com/specx2/openapi-mcp/forgebird"
)

// Custom HTTP client with authentication
type AuthClient struct {
    client *http.Client
    apiKey string
}

func (c *AuthClient) Do(req *http.Request) (*http.Response, error) {
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    return c.client.Do(req)
}

func main() {
    // Create authenticated client
    authClient := &AuthClient{
        client: &http.Client{Timeout: 15 * time.Second},
        apiKey: "your-api-key",
    }

    // Wrap with DefaultHTTPClient
    httpClient := executor.NewDefaultHTTPClientFrom(authClient)

    // Register components with custom client
    forgebird.RegisterComponents(
        mserver,
        components,
        forgebird.WithHTTPClient(httpClient),
    )
}
```

### Multi-Spec Server

```go
// Load multiple specs
specs := []string{
    "users-api.yaml",
    "products-api.yaml",
    "orders-api.yaml",
}

for _, specPath := range specs {
    specBytes, _ := os.ReadFile(specPath)
    components, _ := fb.ConvertSpec(specBytes, config)
    forgebird.RegisterComponents(mserver, components)
}
```

### SSE Mode with Custom Configuration

```go
// Create SSE server with custom options
sseServer := mcpsrv.NewSSEServer(
    mserver,
    mcpsrv.WithBaseURL("https://example.com"),
    mcpsrv.WithKeepAlive(true),
)

// Start on custom port
if err := sseServer.Start(":8080"); err != nil {
    log.Fatal(err)
}
```

## 📁 Project Structure

```
openapi-mcp/
├── cmd/
│   └── openapi-mcp/          # CLI application
│       ├── main.go            # Entry point
│       └── server/            # Server implementation
├── core/
│   ├── executor/              # Request execution layer
│   │   ├── builder.go         # HTTP request construction
│   │   ├── processor.go       # Response processing
│   │   ├── tool.go            # Tool executor
│   │   ├── resource.go        # Resource executor
│   │   └── template.go        # ResourceTemplate executor
│   ├── factory/               # Component factory
│   │   ├── factory.go         # Component generation
│   │   ├── schema.go          # Schema processing
│   │   └── naming.go          # Name generation
│   ├── mapper/                # Route mapping
│   │   ├── mapper.go          # Core mapper logic
│   │   └── defaults.go        # Default mappings
│   ├── parser/                # OpenAPI parsing
│   │   ├── parser.go          # Main parser
│   │   ├── openapi30.go       # OpenAPI 3.0 support
│   │   └── openapi31.go       # OpenAPI 3.1 support
│   └── server.go              # Main server implementation
├── forgebird/                 # Forgebird integration
│   ├── api.go                 # Registration API
│   ├── parser.go              # Forgebird parser adapter
│   ├── route_mapper.go        # Forgebird route mapper
│   ├── descriptor_strategy.go # URI template generation
│   └── operation.go           # Operation wrapper
├── examples/
│   └── basic/                 # Usage examples
└── test/
    └── _gigasdk/              # Integration tests
        └── cmd/server/        # Test server with auth
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -v ./core/executor/...
```

## 📚 Examples

See the [examples](examples/) directory for complete examples:

- [Basic Usage](examples/basic/main.go) - Simple petstore example
- [GigaSDK Integration](test/_gigasdk/cmd/server/main.go) - Real-world integration with custom authentication

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 Related Projects

- [mcp-forgebird](https://github.com/specx2/mcp-forgebird) - The underlying framework for MCP component generation
- [mcp-go](https://github.com/mark3labs/mcp-go) - Go implementation of Model Context Protocol
- [fastmcp](https://github.com/jlowin/fastmcp) - Python FastMCP framework (inspiration)

## 📖 Documentation

- [Architecture Design](docs/ARCHITECTURE.md) - Detailed architecture documentation
- [API Reference](https://pkg.go.dev/github.com/specx2/openapi-mcp) - Go package documentation
- [MCP Specification](https://spec.modelcontextprotocol.io/) - Model Context Protocol specification

## 🙏 Acknowledgments

Special thanks to:
- [mcp-go](https://github.com/mark3labs/mcp-go) team for the excellent MCP implementation
- [libopenapi](https://github.com/pb33f/libopenapi) for OpenAPI parsing capabilities
- [fastmcp](https://github.com/jlowin/fastmcp) for design inspiration