# OpenAPI-MCP 优化记录与对比分析

## 当前改进（本次由 Codex 完成）
- 修复参数冲突检测：恢复并增强 `combineSchemas`，确保路径/查询参数与请求体字段冲突时使用 `__{location}` 后缀，并补全 `ParamMapping.OriginalName`。参见 `pkg/openapimcp/factory/schema.go`、`pkg/openapimcp/factory/factory.go`。
- 确保请求体字段不会被丢弃：`RequestBuilder` 现在对未映射的字段自动归入请求体，同时保留显式的 `body` 映射，解决 Cursor 版本导致请求体为空的问题。参见 `pkg/openapimcp/executor/builder.go`。
- 支持非对象请求体与内容类型：当 OpenAPI 描述的 `requestBody` 不是对象时，使用 `body` 字段映射并按内容类型编码（JSON / x-www-form-urlencoded），同时在构建请求时自动设置 `Content-Type`。参见 `pkg/openapimcp/factory/schema.go`、`pkg/openapimcp/executor/builder.go`。
- 查询参数与 Cookie 序列化增强：统一通过 `ParamMapping` 处理 `deepObject`、`spaceDelimited`、`pipeDelimited`、`explode=false` 等样式，并补充 Cookie 写入；新测试覆盖路径替换、深度对象、原始体写入。参见 `pkg/openapimcp/executor/builder.go`、`test/parameter_collision_test.go`。
- 错误处理对齐 fastmcp：`ResponseProcessor` 与 `ErrorHandler` 现会解析 HTTP 状态码、格式化 JSON 错误体并返回结构化内容，同时区分网络错误的可重试性。参见 `pkg/openapimcp/executor/processor.go`、`pkg/openapimcp/executor/error_handler.go`、`pkg/openapimcp/executor/error_handler_test.go`。
- Schema 合并增强：可选参数自动允许 `null`、请求体/参数均使用深拷贝，且 `$defs` 仅保留实际引用的定义，避免多余 schema 噪音。参见 `pkg/openapimcp/factory/schema.go`、`pkg/openapimcp/factory/schema_test.go`。
- HTTP 客户端配置：新增 `HTTPClientConfig`，构建 `Server` 时可注入 BaseURL、默认 Headers 与 Timeout，并在 `DefaultHTTPClient` 内统一应用。参见 `pkg/openapimcp/options.go`、`pkg/openapimcp/server.go`、`pkg/openapimcp/executor/client.go`、`pkg/openapimcp/server_client_test.go`。
- 暴露 `OpenAPITool.ParameterMappings()` 便于调试与测试。
- 新增覆盖测试 `test/parameter_collision_test.go`，验证参数冲突、无冲突场景以及构建出的 HTTP 请求（路径替换、深度对象序列化和请求体写入）。
- 移除误提交的可执行二进制 `improved_usage` 并写入 `.gitignore`，保持仓库整洁。
- 更新示例 `examples/improved_usage/main.go` 使能力说明与实际实现一致。

## 与 fastmcp 的主要差距
对照 `/tmp/fastmcp/src/fastmcp/experimental/server/openapi` 及其 utilities：
1. **请求构建能力**：fastmcp 使用 `RequestDirector` 与 openapi-core 生成 HTTP 请求，自动处理参数风格、`requestBody` 组合、`nullable` 等复杂场景。Go 版本仍依赖手写 `RequestBuilder`，无法解析 `allOf` / `oneOf`、多内容类型、`encoding` 或 cookie/headers 参数。建议：移植一个 `RequestDirector` 风格的组件，或在现有 `RequestBuilder` 中补齐这些分支逻辑。
2. **错误处理**：✅ 已对齐。`ErrorHandler` 现在会解析 HTTP 状态、格式化 JSON 错误体并区分可重试错误，行为与 fastmcp `OpenAPITool.run` 中的处理一致。
3. **Schema 处理**：基本可空化与 `$defs` 剪裁已实现，仍需覆盖 `allOf/oneOf` 合并、复杂 `$ref`（含外部引用）与 nullable 转换的剩余边界场景，可继续对齐 fastmcp `_combine_schemas_and_map_params` 的完整行为。
4. **超时与客户端设置**：✅ 已对齐。可通过 `WithHTTPClient` / `WithHTTPClientConfig` 注入自定义 `http.Client`、默认 Headers、超时与 BaseURL，`DefaultHTTPClient` 会自动附加这些配置。
5. **高级路由/命名功能**：fastmcp 提供 `route_map_fn`、`ComponentFn` 与 `mcp_names` 的冲突检测；当前仅支持基础的 `RouteMap` 与简单计数命名。后续可补充重复命名冲突检测以及 tags 聚合。
6. **响应 Schema 对齐**：fastmcp 会在非对象响应时自动包裹 `result`，并携带 `$defs`。Go 版本虽然有 `WrapNonObjectSchema`，但对于 OpenAPI 3.0 的 `nullable`/`anyOf` 转换和 `$defs` 裁剪仍欠缺一致性。

## 建议的后续路线
1. **实现请求指挥器**（高优先级）：仿照 fastmcp `RequestDirector`，构建一个专门的序列化与 URL 生成层，逐步取代手写逻辑，并复用当前 `ParamMapping`。
2. **完善错误处理链路**：在 `OpenAPITool.Run` 中根据 HTTP 状态构造 `HTTPError`，同时扩展 `ResponseProcessor` 支持返回结构化 JSON（例如 `{ "code": ..., "detail": ... }`）。
3. **Schema 组合与 `$defs` 管理**：把 fastmcp 的 `_replace_ref_with_defs`、引用裁剪、nullable 兼容迁移到 `parser` 层，减少最终 schema 体积并提升兼容性。
4. **可配置客户端与重试**：允许通过选项传入 `*http.Client`、超时、重试策略，或提供装饰器接入 mcp-go 的传输选项。
5. **测试补充**：
   - 添加端到端测试覆盖 POST/GET/DELETE + 多种参数风格。
   - 模拟非 2xx 响应并校验错误消息。
   - 覆盖 OpenAPI 3.0 & 3.1 的 nullable/`allOf`。
6. **文档与示例**：在 README/示例中明确已完成与待开发特性，便于贡献者对照 fastmcp。

> 以上差异分析基于 fastmcp 最新 experimental/openapi 分支与本仓库当前实现的逐文件对比，建议按照优先级从请求构建与错误处理着手，逐步实现与 fastmcp 的能力对齐。
