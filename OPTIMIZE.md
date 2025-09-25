# OpenAPI-MCP 优化记录与对比分析

## 当前改进（本次由 Codex 完成）
- 修复参数冲突检测：恢复并增强 `combineSchemas`，确保路径/查询参数与请求体字段冲突时使用 `__{location}` 后缀，并补全 `ParamMapping.OriginalName`。参见 `pkg/openapimcp/factory/schema.go`、`pkg/openapimcp/factory/factory.go`。
- 确保请求体字段不会被丢弃：`RequestBuilder` 现在对未映射的字段自动归入请求体，同时保留显式的 `body` 映射，解决 Cursor 版本导致请求体为空的问题。参见 `pkg/openapimcp/executor/builder.go`。
- 支持非对象请求体与内容类型：当 OpenAPI 描述的 `requestBody` 不是对象时，使用 `body` 字段映射并按内容类型编码（JSON / x-www-form-urlencoded），同时在构建请求时自动设置 `Content-Type`。参见 `pkg/openapimcp/factory/schema.go`、`pkg/openapimcp/executor/builder.go`。
- 查询参数与 Cookie 序列化增强：统一通过 `ParamMapping` 处理 `deepObject`、`spaceDelimited`、`pipeDelimited`、`explode=false` 等样式，并补充 Cookie 写入；新测试覆盖路径替换、深度对象、原始体写入。参见 `pkg/openapimcp/executor/builder.go`、`test/parameter_collision_test.go`。
- 必填路径参数保障与表单兼容性：`RequestBuilder` 会在构建前检测所有必填路径参数是否提供，同时针对单字段 `multipart/form-data` 与 `application/x-www-form-urlencoded` 场景保持结构化编码，避免误判为原始体。参见 `pkg/openapimcp/executor/builder.go`、`test/parameter_collision_test.go`。
- 参数序列化指挥器：新增 `param_encoder.go`，所有 query/header/cookie 参数统一通过风格+explode 序列化并支持 `allowReserved`，与 fastmcp RequestDirector 行为一致。参见 `pkg/openapimcp/executor/param_encoder.go`、`pkg/openapimcp/executor/builder.go`。
- 请求体媒体类型选择：`RequestBuilder` 会根据 `requestBody.content` 中的可用媒体类型与传入数据自动挑选 JSON、表单、multipart、text/plain 或 octet-stream，并在缺省时回退到优先序。参见 `pkg/openapimcp/executor/builder.go`。
- Cookie 默认 explode 对齐：form 样式下 Cookie 参数默认逐项展开，与 OpenAPI 规范以及 fastmcp 行为一致；同时新增用例验证多值 cookie。参见 `pkg/openapimcp/executor/param_encoder.go`、`test/parameter_collision_test.go`。
- Schema 组合增强：参数描述会自动附带位置说明，请求体 description 得以保留，输出 schema 的 `$defs` 会按引用裁剪，只保留实际使用的 definitions。参见 `pkg/openapimcp/factory/schema.go`、`pkg/openapimcp/factory/schema_test.go`。
- 参数示例与弃用信息保留：解析器会提取 OpenAPI 参数的 `example`、`examples`、`deprecated`、`allowEmptyValue` 元数据并在合成 schema 时写回，便于工具描述与 fastmcp 一致呈现示例与提示。参见 `pkg/openapimcp/parser/openapi30.go`、`pkg/openapimcp/parser/openapi31.go`、`pkg/openapimcp/factory/schema.go`。
- 请求体示例对齐：解析器捕获每个 media type 的 `example`/`examples`，`combineSchemas` 会在展开请求体字段时同步填充示例，确保文档/工具能展示 fastmcp 同级的示例信息。参见 `pkg/openapimcp/ir/request.go`、`pkg/openapimcp/parser/openapi30.go`、`pkg/openapimcp/parser/openapi31.go`、`pkg/openapimcp/factory/schema.go`、`pkg/openapimcp/factory/schema_test.go`。
- 请求体默认值填充：保存每种媒体类型的 schema `default`，`RequestBuilder` 在缺省入参时自动注入默认字段/原始体，模拟 fastmcp `RequestDirector` 的默认值逻辑。参见 `pkg/openapimcp/ir/request.go`、`pkg/openapimcp/parser/openapi30.go`、`pkg/openapimcp/parser/openapi31.go`、`pkg/openapimcp/executor/builder.go`、`test/parameter_collision_test.go`。
- Encoding 元数据对齐：解析并保留 `encoding.headers` 与媒体 `x-*` 扩展，构建 multipart 请求时自动附加自定义头部与内容类型，覆盖 fastmcp 中的高级编码场景。参见 `pkg/openapimcp/ir/request.go`、`pkg/openapimcp/parser/encoding.go`、`pkg/openapimcp/executor/builder.go`、`test/parameter_collision_test.go`。
- 工具 meta 暴露 OpenAPI 元信息：`OpenAPITool` 会在 `_meta.openapi` 中公开 method/path/请求体 encodings 等上下文，方便外部系统复用扩展字段。参见 `pkg/openapimcp/executor/tool.go`、`pkg/openapimcp/executor/tool_meta_test.go`。
- 扩展字段可视化：`ir.ParameterInfo`/`ir.RequestBodyInfo`/`ir.ResponseInfo` 新增 `Extensions`，描述渲染与工具 meta 均会输出 `x-*` 值，保证与 fastmcp 下游体验一致。参见 `pkg/openapimcp/ir/*.go`、`pkg/openapimcp/factory/description.go`、`pkg/openapimcp/executor/tool.go`。
- 回调 `$ref` 支持：缓存 `components.callbacks` / `components.pathItems` 并解析 `$ref`，合并扩展后生成 `CallbackInfo`（含名称/表达式），描述与 `_meta.openapi` 同步呈现。参见 `pkg/openapimcp/parser/openapi30.go`、`pkg/openapimcp/parser/openapi31.go`、`pkg/openapimcp/executor/tool.go`。
- 描述增强：工具描述新增参数分组摘要、请求体/响应扩展提示及回调标签，使 `formatDescription` 产出与 fastmcp `format_description_with_responses` 对齐。参见 `pkg/openapimcp/factory/description.go`、`pkg/openapimcp/factory/description_test.go`。
- 回调回归测试：新增 `pkg/openapimcp/parser/openapi_callbacks_test.go` 覆盖回调组件引用场景，确保解析行为稳定。
- 路由映射高级特性：`RouteMapper` 支持全局标签、映射标签与 `RouteMapFunc` 返回 `RouteDecision`（类型/标签/注解），并自动附加默认兜底映射，生成的组件会携带聚合标签。参见 `pkg/openapimcp/mapper/mapper.go`、`pkg/openapimcp/mapper/types.go`、`pkg/openapimcp/server.go`、`pkg/openapimcp/mapper/mapper_test.go`。
- 工具注解/标签同步：`executor.NewOpenAPITool` 基于 HTTP 方法推导默认 `ToolAnnotation`（GET/HEAD 只读、DELETE/PUT 幂等等），允许路由映射覆盖，并把聚合标签写入 `_meta.tags`。参见 `pkg/openapimcp/executor/tool.go`、`pkg/openapimcp/executor/tool_meta_test.go`。
- 组件命名对齐 fastmcp：`ComponentFactory` 支持 `CustomNames` 以 `operationId` 或 `METHOD /path`、`method:/path` 键覆盖名称，并在 slug 为空时回退 `method_path`，对重复名称自动追加后缀；新增 `pkg/openapimcp/factory/naming_test.go` 确保自定义与冲突场景稳定。
- 内容类型协商增强：保留 OpenAPI `content` 的声明顺序并在请求构建时优先选择 `application/json` / `*+json`、`multipart`、`text` 等更符合语义的媒体类型，自动识别原始 JSON/文本/二进制输入；新增 `pkg/openapimcp/executor/builder_test.go` 验证高级选择逻辑。
- 响应描述增强：工具描述会自动附带主要响应的结构摘要与示例，便于调用方理解返回 payload。参见 `pkg/openapimcp/factory/description.go`、`pkg/openapimcp/factory/description_test.go`。
- 合成 schema 保留 oneOf/anyOf：`normalizeSchema` 会递归保留并标准化组合结构，描述输出会提示可选分支，与 fastmcp 的变体信息一致。参见 `pkg/openapimcp/factory/schema.go`、`pkg/openapimcp/factory/description.go`、`pkg/openapimcp/factory/description_test.go`。
- 输入参数校验：在构建请求前使用 `jsonschema` 对工具入参执行验证，提前给出 schema 级错误提示。参见 `pkg/openapimcp/executor/tool.go`、`pkg/openapimcp/executor/tool_validation_test.go`。
- 响应校验：成功响应会通过 JSON Schema 验证，避免返回体偏离 OpenAPI 描述。参见 `pkg/openapimcp/executor/processor.go`、`pkg/openapimcp/executor/error_handler_test.go`、`pkg/openapimcp/executor/schema_validator.go`。
- 路径参数序列化对齐规范：支持 `label`、`matrix`、`simple` 三种路径样式及 `explode` 变体，并确保映射顺序稳定，使生成的 URL 符合 fastmcp/openapi-core 行为。参见 `pkg/openapimcp/executor/builder.go`、`test/parameter_collision_test.go`。
- 错误处理对齐 fastmcp：`ResponseProcessor` 与 `ErrorHandler` 现会解析 HTTP 状态码、格式化 JSON 错误体并返回结构化内容，同时区分网络错误的可重试性。参见 `pkg/openapimcp/executor/processor.go`、`pkg/openapimcp/executor/error_handler.go`、`pkg/openapimcp/executor/error_handler_test.go`。
- Schema 合并增强：可选参数自动允许 `null`、请求体/参数均使用深拷贝，且 `$defs` 仅保留实际引用的定义，避免多余 schema 噪音。参见 `pkg/openapimcp/factory/schema.go`、`pkg/openapimcp/factory/schema_test.go`。
- 复合 schema 支持：`allOf` 会被展开合并，`required` 与属性集正确汇总，非对象请求体可根据 `title` 自动命名字段，与 fastmcp 的 `_combine_schemas_and_map_params` 行为保持一致。参见同上。
- HTTP 客户端配置：新增 `HTTPClientConfig`，构建 `Server` 时可注入 BaseURL、默认 Headers 与 Timeout，并在 `DefaultHTTPClient` 内统一应用。参见 `pkg/openapimcp/options.go`、`pkg/openapimcp/server.go`、`pkg/openapimcp/executor/client.go`、`pkg/openapimcp/server_client_test.go`。
- 暴露 `OpenAPITool.ParameterMappings()` 便于调试与测试。
- 新增覆盖测试 `test/parameter_collision_test.go`，验证参数冲突、无冲突场景以及构建出的 HTTP 请求（路径替换、深度对象序列化和请求体写入）。
- 路径样式单测补充：新增 label/matrix 样式用例，防止未来回 regress。参见 `test/parameter_collision_test.go`。
- allowReserved 与内容类型单测：新增查询参数保留字符、原始字符串/二进制体触发的媒体类型选择用例，覆盖 fastmcp 的高阶场景。参见 `test/parameter_collision_test.go`。
- 移除误提交的可执行二进制 `improved_usage` 并写入 `.gitignore`，保持仓库整洁。
- 更新示例 `examples/improved_usage/main.go` 使能力说明与实际实现一致。
- 资源模板参数解析：实现 URI 与模板之间的反向映射，使 `resource_template` 能根据调用 URI 自动填充路径参数，匹配 fastmcp 行为。参见 `pkg/openapimcp/server.go`、`pkg/openapimcp/server_client_test.go`。

## 与 fastmcp 的主要差距
对照 `/tmp/fastmcp/src/fastmcp/experimental/server/openapi` 及其 utilities：
1. **请求构建能力**：fastmcp 使用 `RequestDirector` 与 openapi-core 生成 HTTP 请求，自动处理多内容类型优先级、`encoding`、oneOf/anyOf 分支选择及 schema 级校验。Go 版本虽已覆盖 JSON/form/multipart/text/octet-stream、自定义 encoding 头与默认值填充，但尚未支持多媒体类型权重、`discriminator` 驱动的分支决策与基于 schema 的自动参数补齐，仍需引入更通用的指挥器组件。
2. **错误处理**：✅ 已对齐。`ErrorHandler` 解析 HTTP 状态、格式化 JSON 错误体并区分可重试错误，行为与 fastmcp `OpenAPITool.run` 一致。
3. **Schema 处理**：✅ `allOf`/`oneOf`/`anyOf` 正常保留并裁剪 `$defs`，但尚未解析外部 `$ref`、`discriminator`、`link` 以及跨组件的示例复用；需要扩展 parser 以覆盖这些高阶特性并完善 nullable 边界场景。
4. **超时与客户端设置**：✅ 已对齐。可通过 `WithHTTPClient` / `WithHTTPClientConfig` 注入自定义 `http.Client`、默认 Headers、超时与 BaseURL，`DefaultHTTPClient` 会自动附加这些配置。
5. **高级路由/命名功能**：✅ `RouteMapper` 现支持 `route_map_fn` 等价回调、全局标签与注解透传；命名器已支持 `CustomNames` 键映射与冲突后缀。仍缺少更智能的自动命名策略（按 tag/响应推导语义名称）。
6. **响应 Schema 对齐**：fastmcp 会在非对象响应时自动包裹 `result` 并复用 `$defs`。Go 版虽然提供 `wrapResult` 和响应校验，但 nullable/`anyOf` 与外部 `$ref` 剪裁仍需补完，尤其是多态响应的结构化提示。

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
