# OpenAPI MCP

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

OpenAPI MCP 是一个 Go 框架，可以将 OpenAPI 规范（Swagger）转换为 MCP（模型上下文协议）服务器。它通过模型上下文协议实现现有 REST API 与 AI 模型的无缝集成。

[English Documentation](README.md)

## 🚀 特性

- **多协议支持**：支持 OpenAPI 3.0 和 3.1 规范
- **基于 Forgebird 构建**：利用强大的 [mcp-forgebird](https://github.com/specx2/mcp-forgebird) 框架
- **灵活映射**：将 OpenAPI 操作转换为 MCP 工具（Tools）、资源（Resources）或资源模板（ResourceTemplates）
- **双传输模式**：支持 stdio（CLI）和 SSE（HTTP 服务器）模式
- **RFC 6570 URI 模板**：完全支持参数化资源 URI
- **自定义认证**：可插拔的 HTTP 客户端用于自定义认证逻辑
- **多规范支持**：同时加载和服务多个 OpenAPI 规范

## 📦 安装

```bash
go get github.com/specx2/openapi-mcp
```

## 🎯 快速开始

### 基本用法

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
    // 加载 OpenAPI 规范
    specBytes, err := os.ReadFile("petstore.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // 创建带自定义映射策略的 Forgebird 管道
    pipeline := forgebird.NewPipeline()
    fb := core.NewForgebird(pipeline)

    // 将 OpenAPI 规范转换为 MCP 组件
    components, err := fb.ConvertSpec(specBytes, interfaces.ConversionConfig{
        BaseURL: "https://petstore.swagger.io/v1",
        Timeout: 15,
        Spec:    interfaces.SpecConfig{SpecURL: "petstore.yaml"},
    })
    if err != nil {
        log.Fatal(err)
    }

    // 创建并注册 MCP 服务器
    mserver := mcpsrv.NewMCPServer("petstore-mcp", "1.0.0")
    if err := forgebird.RegisterComponents(mserver, components); err != nil {
        log.Fatal(err)
    }

    // 以 stdio 模式启动服务器
    stdio := mcpsrv.NewStdioServer(mserver)
    stdio.Listen(context.Background(), os.Stdin, os.Stdout)
}
```

### 使用 CLI

```bash
# stdio 模式（默认）
openapi-mcp -spec petstore.yaml -base-url https://api.example.com

# SSE 模式（HTTP 服务器）
openapi-mcp -spec petstore.yaml -base-url https://api.example.com -sse -sse-addr :8080

# 多个规范文件
openapi-mcp -spec spec1.yaml -spec spec2.yaml -base-url https://api.example.com

# 自定义日志输出
openapi-mcp -spec petstore.yaml -log-output server.log -log-tee-console
```

## 🏗️ 架构

```
┌─────────────────┐
│ OpenAPI 规范    │
│ (YAML/JSON)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Forgebird       │
│ 管道            │
│ - 解析器        │
│ - 路由映射器    │
│ - 工厂          │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ MCP 组件        │
│ - 工具          │
│ - 资源          │
│ - 模板          │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ MCP 服务器      │
│ (mcp-go)        │
└─────────────────┘
```

### 核心层次

1. **解析层** (`forgebird/parser.go`)
   - OpenAPI 规范解析和验证
   - 支持 OpenAPI 3.0 和 3.1
   - Schema 引用解析

2. **映射层** (`forgebird/route_mapper.go`)
   - 将 OpenAPI 操作转换为 MCP 组件类型
   - 默认规则：GET 请求 → 工具 + 资源模板，其他请求 → 工具
   - 通过管道配置自定义映射规则

3. **工厂层** (`mcp-forgebird/core/factory`)
   - 生成 MCP 工具、资源和资源模板定义
   - Schema 组合和参数冲突处理
   - 从 OpenAPI Schema 生成 JSON Schema

4. **执行层** (`core/executor`)
   - 从 MCP 工具调用构建 HTTP 请求
   - 参数序列化（路径、查询、头部、请求体）
   - 响应处理和验证

5. **注册层** (`forgebird/api.go`)
   - 向 mcp-go 服务器注册 MCP 组件
   - 处理工具执行和资源获取
   - 资源模板的 URI 模板匹配

## 🎨 映射策略

### 默认映射（一对多）

默认情况下，GET 请求生成工具和资源模板，其他方法仅生成工具：

```yaml
GET /pets/{id}          → 工具: get_api_pets_id + 资源模板: resource://api/pets/{id}{?param1,param2}
POST /pets              → 工具: post_api_pets
PUT /pets/{id}          → 工具: put_api_pets_id
DELETE /pets/{id}       → 工具: delete_api_pets_id
```

### 自定义映射

您可以在管道中自定义映射策略：

```go
pipeline := forgebird.NewPipeline()

// 自定义路由映射器
customMapper := &forgebird.RouteMapper{
    // 您的自定义映射逻辑
}
pipeline.SetRouteMapper(customMapper)
```

## 🔧 高级用法

### 自定义认证

```go
package main

import (
    "net/http"

    "github.com/specx2/openapi-mcp/core/executor"
    "github.com/specx2/openapi-mcp/forgebird"
)

// 带认证的自定义 HTTP 客户端
type AuthClient struct {
    client *http.Client
    apiKey string
}

func (c *AuthClient) Do(req *http.Request) (*http.Response, error) {
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    return c.client.Do(req)
}

func main() {
    // 创建认证客户端
    authClient := &AuthClient{
        client: &http.Client{Timeout: 15 * time.Second},
        apiKey: "your-api-key",
    }

    // 用 DefaultHTTPClient 包装
    httpClient := executor.NewDefaultHTTPClientFrom(authClient)

    // 使用自定义客户端注册组件
    forgebird.RegisterComponents(
        mserver,
        components,
        forgebird.WithHTTPClient(httpClient),
    )
}
```

### 多规范服务器

```go
// 加载多个规范
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

### SSE 模式与自定义配置

```go
// 创建带自定义选项的 SSE 服务器
sseServer := mcpsrv.NewSSEServer(
    mserver,
    mcpsrv.WithBaseURL("https://example.com"),
    mcpsrv.WithKeepAlive(true),
)

// 在自定义端口上启动
if err := sseServer.Start(":8080"); err != nil {
    log.Fatal(err)
}
```

## 📁 项目结构

```
openapi-mcp/
├── cmd/
│   └── openapi-mcp/          # CLI 应用程序
│       ├── main.go            # 入口点
│       └── server/            # 服务器实现
├── core/
│   ├── executor/              # 请求执行层
│   │   ├── builder.go         # HTTP 请求构建
│   │   ├── processor.go       # 响应处理
│   │   ├── tool.go            # 工具执行器
│   │   ├── resource.go        # 资源执行器
│   │   └── template.go        # 资源模板执行器
│   ├── factory/               # 组件工厂
│   │   ├── factory.go         # 组件生成
│   │   ├── schema.go          # Schema 处理
│   │   └── naming.go          # 名称生成
│   ├── mapper/                # 路由映射
│   │   ├── mapper.go          # 核心映射逻辑
│   │   └── defaults.go        # 默认映射
│   ├── parser/                # OpenAPI 解析
│   │   ├── parser.go          # 主解析器
│   │   ├── openapi30.go       # OpenAPI 3.0 支持
│   │   └── openapi31.go       # OpenAPI 3.1 支持
│   └── server.go              # 主服务器实现
├── forgebird/                 # Forgebird 集成
│   ├── api.go                 # 注册 API
│   ├── parser.go              # Forgebird 解析器适配器
│   ├── route_mapper.go        # Forgebird 路由映射器
│   ├── descriptor_strategy.go # URI 模板生成
│   └── operation.go           # 操作包装器
├── examples/
│   └── basic/                 # 使用示例
└── test/
    └── _gigasdk/              # 集成测试
        └── cmd/server/        # 带认证的测试服务器
```

## 🧪 测试

```bash
# 运行所有测试
go test ./...

# 运行带覆盖率的测试
go test -cover ./...

# 运行特定测试
go test -v ./core/executor/...
```

## 📚 示例

查看 [examples](examples/) 目录获取完整示例：

- [基本用法](examples/basic/main.go) - 简单的 petstore 示例
- [GigaSDK 集成](test/_gigasdk/cmd/server/main.go) - 带自定义认证的真实集成案例

## 🤝 贡献

欢迎贡献！请随时提交 Pull Request。

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 🔗 相关项目

- [mcp-forgebird](https://github.com/specx2/mcp-forgebird) - MCP 组件生成的底层框架
- [mcp-go](https://github.com/mark3labs/mcp-go) - 模型上下文协议的 Go 实现
- [fastmcp](https://github.com/jlowin/fastmcp) - Python FastMCP 框架（灵感来源）

## 📖 文档

- [架构设计](docs/ARCHITECTURE.md) - 详细的架构文档
- [API 参考](https://pkg.go.dev/github.com/specx2/openapi-mcp) - Go 包文档
- [MCP 规范](https://spec.modelcontextprotocol.io/) - 模型上下文协议规范

## 🙏 致谢

特别感谢：
- [mcp-go](https://github.com/mark3labs/mcp-go) 团队提供的出色 MCP 实现
- [libopenapi](https://github.com/pb33f/libopenapi) 提供的 OpenAPI 解析功能
- [fastmcp](https://github.com/jlowin/fastmcp) 提供的设计灵感