# REVIEW.md

本节记录对 Cursor 提交（commit `f1f5ab8`) 的复查结果以及随后完成的修正。

## 发现的问题
- `pkg/openapimcp/factory/schema.go` 与 `factory.go`：`combineSchemas` 被拆分后仅返回 schema，不再为请求体字段生成 `ParamMapping`，同时 `DetectParameterCollisions` 改为独立函数并未覆盖请求体。最终导致请求体字段在构建请求时全部丢失。
- `pkg/openapimcp/executor/tool.go`：新增的 `MapParameters` 会将带后缀的参数名（例如 `id__path`）重新映射成 `id`，但 `RequestBuilder` 仍以后缀名为键，直接造成路径/查询参数无法命中。
- `test/parameter_collision_test.go`：新测试仅覆盖手工构造的映射，并没有走完整工厂流程，未能暴露上述两个缺陷。
- `pkg/openapimcp/executor/parameter_serializer.go`：实现未被调用，实际序列化逻辑仍旧不足。
- 根目录意外提交 `improved_usage` 可执行文件，污染仓库。
- `examples/improved_usage/main.go` 声明的部分能力（错误处理、完整序列化）与实际代码不符。

## 修复摘要
- 恢复并增强 `combineSchemas` -> 现返回 schema 与完整 `ParamMapping`，同时持久化到 `OpenAPITool`，解决请求体丢失与冲突处理问题。
- 移除 `MapParameters`，让 `RequestBuilder` 直接消费真实参数名，并对未映射字段回退到请求体。
- 扩展查询参数序列化逻辑以支持 `deepObject`、`spaceDelimited`、`pipeDelimited` 等样式。
- 重写参数冲突测试，覆盖工厂建模与请求构建全流程。
- 删除误提交二进制并更新 `.gitignore`，调整示例文案以反映真实能力。

以上修正均已在 `OPTIMIZE.md` 中记录，后续优化方向亦已列出。
