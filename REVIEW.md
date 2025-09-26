# REVIEW.md

本节记录对 Cursor 提交（commit `f1f5ab8`) 的复查结果以及随后完成的修正。

## 发现的问题
- `pkg/openapimcp/factory/schema.go` 与 `factory.go`：`combineSchemas` 被拆分后仅返回 schema，不再为请求体字段生成 `ParamMapping`，同时 `DetectParameterCollisions` 改为独立函数并未覆盖请求体。最终导致请求体字段在构建请求时全部丢失。
- `pkg/openapimcp/executor/tool.go`：新增的 `MapParameters` 会将带后缀的参数名（例如 `id__path`）重新映射成 `id`，但 `RequestBuilder` 仍以后缀名为键，直接造成路径/查询参数无法命中。
- `test/parameter_collision_test.go`：新测试仅覆盖手工构造的映射，并没有走完整工厂流程，未能暴露上述两个缺陷。
- `pkg/openapimcp/executor/parameter_serializer.go`：实现未被调用，实际序列化逻辑仍旧不足。
- 根目录意外提交 `improved_usage` 可执行文件，污染仓库。
- `examples/improved_usage/main.go` 声明的部分能力（错误处理、完整序列化）与实际代码不符。
- `pkg/openapimcp/server.go`：`extractParametersFromURI` 为空实现，`resource_template` 无法解析 URI 中的路径参数，导致模板资源与 fastmcp 行为不一致。
- `pkg/openapimcp/executor/builder.go`：缺少必填路径参数校验，且在 multipart/form-data / x-www-form-urlencoded 仅包含单字段时误判为原始体，破坏 fastmcp 的表单兼容性。

## 修复摘要
- 恢复并增强 `combineSchemas` -> 现返回 schema 与完整 `ParamMapping`，同时持久化到 `OpenAPITool`，解决请求体丢失与冲突处理问题。
- 移除 `MapParameters`，让 `RequestBuilder` 直接消费真实参数名，并对未映射字段回退到请求体。
- 扩展查询参数序列化逻辑以支持 `deepObject`、`spaceDelimited`、`pipeDelimited` 等样式。
- 重写参数冲突测试，覆盖工厂建模与请求构建全流程。
- 删除误提交二进制并更新 `.gitignore`，调整示例文案以反映真实能力。
- 实现资源模板 URI 参数反解析，补齐 `resource_template` 按 URI 提取路径参数的能力。
- 为请求构建补充必填路径参数校验，并在表单/Multipart 单字段场景保持结构化编码，避免误降级为原始体。
- 扩展路径参数序列化，支持 `simple`/`label`/`matrix` 样式及 `explode` 配置，生成的 URL 与 fastmcp 保持一致，同时补充相应单测。
- 统一 query/header/cookie 参数序列化，引入 `param_encoder` 支持 `allowReserved`、Cookie 默认 explode 与标量规范化，并依据请求体内容自动选择 JSON/form/multipart/text/octet-stream，新增对应回归测试。
- 参数元数据保留：解析阶段会提取参数的 `example`/`examples`/`deprecated` 等元信息，并在 schema 组合时写回属性，确保工具描述与 fastmcp 一致呈现示例与弃用标识。
- 请求体示例同步：解析器抓取每种内容类型的示例与示例集，`combineSchemas` 在展开 body 字段时赋予相应的 `example`/`examples`，避免 Cursor 版本遗漏请求体示例信息。
- 请求体默认值同步：保留 media `default` 并在构建请求时自动填充缺省字段/原始体，防止 Cursor 版本丢失 fastmcp 默认值行为。
- Encoding 元数据应用：解析 `encoding.headers`，构建 multipart/form-data 部分时自动附加自定义头部，恢复 fastmcp RequestDirector 对高级编码的支持。
- 工具 Meta 输出：将 OpenAPI method/path/请求体编码等信息写入 MCP `_meta.openapi`，方便调试与扩展字段的自定义消费。
- 响应描述增强：描述字符串会加入主要响应的结构摘要与示例，减少 fastmcp 与本实现之间的使用差距。
- Schema 变体保留：保留并标准化 `oneOf`/`anyOf` 组合结构，描述和 meta 都会提示可选分支，避免 Cursor 版本丢失关键信息。
- 请求参数校验：新增 JSON Schema 验证，阻止 Cursor 版本未捕获的非法参数进入请求链路。
- 响应校验：成功响应通过 schema 校验，规避 fastmcp 已覆盖的响应偏差问题。

以上修正均已在 `OPTIMIZE.md` 中记录，后续优化方向亦已列出。

## 新增改进（对齐 fastmcp x-* 与回调能力）
- 统一参数/请求体/响应的扩展字段保留：`ir.ParameterInfo`、`ir.RequestBodyInfo`、`ir.ResponseInfo` 增补 `Extensions`，描述渲染会在路径/查询参数与请求体片段中展示 `x-*` 信息，工具 `_meta.openapi` 同步输出，确保与 fastmcp 对齐的可见性。
- 回调引用解析增强：`parser/openapi30.go` 与 `openapi31.go` 缓存 `components.callbacks`、`components.pathItems`，解析时解析 `$ref` 并合并扩展字段，生成的 `CallbackInfo` 附带名称与表达式，描述与 meta 均可直观展示。
- 描述生成升级：新增参数分组摘要、请求体扩展提示与响应扩展标识，补齐 fastmcp `format_description_with_responses` 的信息密度。
- 回归测试补充：`factory/description_test.go` 验证扩展字段输出，`parser/openapi_callbacks_test.go` 覆盖回调组件 `$ref` 场景，`go test ./...` 全量通过。
- 组件命名器增强：`ComponentFactory` 支持 `CustomNames` 按 `operationId` 或 `METHOD /path` 覆盖组件名称，空摘要时回退 `method_path`，并在重名时自动追加后缀；新增 `factory/naming_test.go` 确保覆盖。
- 内容类型协商改进：`RequestBodyInfo` 记录媒体类型顺序，`RequestBuilder` 会优先选择 `application/json`/`*+json`、`multipart/form-data`、`text/plain` 等语义更合适的内容类型，并识别 `_rawBody` 的文本/JSON/二进制场景；新增 `executor/builder_test.go` 针对媒体类型优先级的回归测试。
- Schema 解析升级：引入 `schema_resolver` 与 `schema_converter`，支持外部/相对 `$ref`、跨文件 `$defs`、`discriminator`、`not` 等高级 JSON Schema 特性，新增 `WithSpecURL` 以显式指定基路径并通过 `schema_resolution_test.go` 覆盖复杂引用场景。
- 路由映射增强：`RouteMapper` 现在支持全局标签、匹配映射自带标签以及 `RouteMapFunc` 返回完整决策（类型/标签/注解），与 fastmcp 的 `DEFAULT_ROUTE_MAPPINGS`+`route_map_fn` 行为保持一致，并将聚合标签写入 `_meta.tags` 方便客户端消费。
- 注解提示对齐：`executor.NewOpenAPITool` 根据 HTTP 动词推导默认 `ToolAnnotation`（如 GET 自动标记为只读、幂等），同时允许映射或自定义函数覆盖，并传递到 MCP `annotations` 字段。
- 响应体验增强：`ResponseProcessor` 统一构造 `_meta`（状态码、状态文本、请求上下文与响应首部），在 204 / 空响应与非 JSON 降级时仍返回结构化结果；`extractOutputSchema` 会在需要包裹时添加 `x-fastmcp-wrap-result` 扩展，`RequestBuilder` 也会基于成功响应声明推导 `Accept` 头，均有对应单测覆盖。
- MCP Forgebird 脚手架对齐：抽象出的 schema 解析链路迁移至 `core/schema`，补全 `$ref`/`nullable`/`discriminator` 等高级 JSON Schema 支持并在 `DefaultComponentFactory` 中统一执行校验/裁剪；同时新增操作中间件、组件钩子与可插拔路由映射器，整体执行链与 fastmcp 的 RequestDirector/RouteMap 架构保持一致。参见 `../mcp-forgebird/core/schema/resolver.go:1`、`../mcp-forgebird/core/schema/converter.go:1`、`../mcp-forgebird/core/schema/processor.go:10`、`../mcp-forgebird/core/factory/factory.go:72`、`../mcp-forgebird/core/forgebird.go:12`、`../mcp-forgebird/core/interfaces/plugin.go:128`，配套测试 `../mcp-forgebird/core/schema/processor_test.go:1`、`../mcp-forgebird/core/factory/factory_test.go:1`、`../mcp-forgebird/core/forgebird_test.go:234` 已覆盖。
- 工具注解策略抽象：新增 `ToolAnnotationStrategy` 接口，`DefaultComponentFactory` 与 `Forgebird` 支持自定义注解策略，不再强绑 HTTP 语义，默认实现依旧提供动词到 hint 的映射。参见 `../mcp-forgebird/core/interfaces/factory.go:12`、`../mcp-forgebird/core/factory/annotation_strategy.go:1`、`../mcp-forgebird/core/forgebird.go:20`，测试 `../mcp-forgebird/core/factory/factory_test.go:73`、`../mcp-forgebird/core/forgebird_test.go:333`。
- 组件描述与元数据策略抽象：开放 `ComponentDescriptorStrategy` 接口，`DefaultComponentFactory`/`Forgebird` 可注入自定义描述、URI 及元数据生成逻辑，默认策略改为协议无关。参见 `../mcp-forgebird/core/interfaces/factory.go:23`、`../mcp-forgebird/core/factory/component_descriptor_strategy.go:1`、`../mcp-forgebird/core/factory/factory.go:19`、`../mcp-forgebird/core/forgebird.go:22`，测试 `../mcp-forgebird/core/factory/factory_test.go:115`、`../mcp-forgebird/core/forgebird_test.go:368`。
- 默认路由映射去 HTTP 化：`GetDefaultMappingRules` 现为空集合，新增 `GetHTTPMappingRules` 和 `mapper.NewHTTPRouteMapper` 供需要 HTTP 语义的插件使用，文档同步示例。参见 `../mcp-forgebird/core/interfaces/mapper.go:1`、`../mcp-forgebird/core/mapper/http_route_mapper.go:1`、`../mcp-forgebird/core/mapper/mapper.go:12`。
