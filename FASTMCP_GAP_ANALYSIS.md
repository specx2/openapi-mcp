# FastMCP 对齐差距分析

本文记录了当前 Go 版 openapi-mcp 与 Python fastmcp `FastMCPOpenAPI` / `experimental.server.openapi` 实现之间尚未覆盖的能力、存在的代码质量隐患，以及建议的改进优先级。分析基于仓库 `commit HEAD` 与 fastmcp 源码逐段对照。

## 概览

- ✅ **已基本对齐**：路由解析、工具/资源/模板生成、基础参数映射、错误返回包装以及路径参数的 `simple/label/matrix` 序列化。
- ⚠️ **仍有明显差距**：请求构建器缺乏 openapi-core 级别的序列化与验证、Schema 解析尚未覆盖 `discriminator`/外部 `$ref` 等进阶特性、编码/内容类型选择策略仍待增强、工具执行链缺少上下游钩子。
- 📉 **质量隐患**：重复/未使用代码、巨型函数、`TODO` 未落地、测试覆盖面不均衡。

## 详细差距

### 请求构建与序列化
- `pkg/openapimcp/executor/builder.go:214` 已能依据 `requestBody.content` 自动在 JSON / form / multipart / text / octet-stream 之间选择，保留 OpenAPI 声明顺序并优先匹配 `application/json` / `*+json` 媒体类型，同时注入默认值、encoding headers、`_contentType`/`_rawBody` 覆盖；但仍缺少 fastmcp `RequestDirector` 的 `discriminator` 分支推断与 schema 级参数修正（如自动补齐缺失字段）。
- `pkg/openapimcp/executor/param_encoder.go:16` 覆盖 `form`/`simple`/`label`/`matrix`/`spaceDelimited`/`pipeDelimited`/`deepObject`，且支持 `allowReserved` 与 Cookie 默认 explode；仍未支持 header/cookie 针对 vendor-specific style 的自定义钩子，与 fastmcp 仍有轻微差距。
- `pkg/openapimcp/executor/tool.go:69` 已在调用前通过 JSON Schema 校验入参，但缺少 fastmcp 借助 openapi-core 的格式验证/`nullable` 错误定位；需继续扩展校验细粒度（如 pattern、format）。
- `pkg/openapimcp/executor/parameter_serializer.go:1` 未接入主流程，建议合并入 `param_encoder.go` 以避免重复实现。
- `_contentType` / `_rawBody` 仍需调用方手动覆盖；可考虑提供更友好的 API（例如依据 schema 自动识别二进制流）来进一步靠拢 fastmcp 无感化的体验。

### Schema 解析与合成
- `pkg/openapimcp/factory/schema.go:24`：`combineSchemas` 现在会保留 `oneOf`/`anyOf` 并标准化 `$defs`，同时在描述与 meta 中提示变体；但仍缺 `discriminator`/`not` 处理以及跨组合的 `required` 合并策略。
- `pkg/openapimcp/parser/openapi30.go:221`：输出与请求体 `$defs` 会按引用裁剪，但依旧无法解析外部 `$ref`、`link` 及跨文件组件；可参考 fastmcp 使用 `jsonschema_path` 的做法补齐。
- `pkg/openapimcp/parser/openapi30.go:124`：✅ 参数/请求体/响应的 `example`、`examples`、`default`、`encoding.headers` 与 `x-*` 扩展均已保留，描述与 `_meta.openapi` 同步呈现；仍需处理 `discriminator`、`callbacks` 以外的复杂扩展（如 `x-jsonschema-` 系列）与 Schema combination 的 edge cases。
- ✅ `pkg/openapimcp/parser/openapi_callbacks_test.go` 证明当前实现已解析回调 `$ref` 并输出 `CallbackInfo`，与 fastmcp 对齐。

### 工具执行链能力
- `pkg/openapimcp/executor/tool.go:40` 已支持根据 HTTP 动词推导默认 `ToolAnnotation` 并接受路由映射覆盖，同时在 `_meta.tags` 和 `_meta.openapi` 中曝光扩展/回调信息；仍缺少 fastmcp 的执行前/执行后钩子与自定义 serializer 注入机制。
- `pkg/openapimcp/factory/description.go:30` 现在会将主要响应、示例与扩展渲染进描述，但与 fastmcp 相比仍缺少错误响应摘要及多语言格式化。
- `pkg/openapimcp/executor/processor.go:55` 已对 JSON 错误体结构化输出并执行响应校验，不过在非 JSON (如 CSV/Binary) 且存在 output schema 时仍需更好的降级策略。

-### Server 与路由能力
- `pkg/openapimcp/server.go:80`：✅ `RouteMapper` 现提供 `WithMapFunc`、全局标签与注解克隆，行为与 fastmcp `route_map_fn` 相当；命名器也支持 `CustomNames` 覆盖 `operationId` 与 `METHOD /path`，并在重名时追加后缀。仍缺乏跨组件的统一命名策略与按语义自动生成名称的能力。
- ✅ `ServerOptions.ComponentFunc` 已透传到所有组件，便于注入日志/监控；后续仍可借鉴 fastmcp 的错误隔离包装。
- 资源模板目前仅支持路径变量回填（`pkg/openapimcp/server.go:146`），缺少查询参数/矩阵变量映射；可参考 fastmcp 的 RequestDirector 在模板读取时构建完整 URL。

### 测试与验证
- `test/parameter_collision_test.go` 已覆盖路径样式、allowReserved、form/multipart、内容类型自选等核心场景，但仍缺少 header/cookie 特殊样式、错误路径（如 schema 校验失败）以及带身份验证/扩展头的集成测试。fastmcp 在 `tests/server/openapi` 中覆盖了 description 传播、结果包装、结构化错误等。
- 缺乏端到端集成测试（HTTP mock server + openapi spec），而 fastmcp 在 `test_openapi_compatibility.py` 中验证完整 CRUD 场景。

## 代码质量观察
- `pkg/openapimcp/executor/builder.go` 近 350 行，职责过多（参数分类、序列化、请求构造），应拆分成 `Serializer` / `BodyEncoder` / `URLBuilder` 模块并单测覆盖。
- 重复/未使用代码：`executor/parameter_serializer.go`、`paramMap` 中 `IsSuffixed` 与 `OriginalName` 在构建链以外未再使用，可收敛接口。
- 多处 `TODO`（如 `pkg/openapimcp/parser/openapi30.go:78` 的 extensions 转换）长期未处理，导致扩展字段丢失。
- 错误处理使用 `fmt.Errorf` 拼接字符串，缺少错误类型区分，后续难以做重试/分类处理。

## 建议的下一步
1. **引入请求指挥器**：抽象 `RequestBuilder` 为独立 `Director`，支持多 content-type/encoding、参数 style/explode 全面实现，并在构建后与 schema 校验结果绑定（可参考 fastmcp `RequestDirector`）。
2. **强化 schema 组合**：补齐 `discriminator` / `not` / 外部 `$ref` 的处理逻辑，并在裁剪 `$defs` 时支持跨文件引用及复杂示例复用。
3. **完善测试矩阵**：模仿 fastmcp `test_openapi_compatibility`，以真实 spec 驱动回归；针对 header/cookie/allowReserved 等样式补齐单元测试。
4. **瘦身 RequestBuilder**：拆分函数与冗余代码，引入共享的序列化工具，而非散落在 builder 内的多段逻辑。
5. **文档同步**：在 README/OPTIMIZE 中持续维护实现 vs 计划清单，确保使用者了解当前限制。

以上差距整理可作为后续迭代的工作待办，优先从请求构建与 schema 解析补齐核心功能，再逐步提升文档与测试深度。
