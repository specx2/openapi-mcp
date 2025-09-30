package forgebird

import (
	"fmt"
	"strings"

	"github.com/specx2/mcp-forgebird/core/interfaces"
)

// openAPIMCPDescriptorStrategy 是 openapi-mcp 专用的 ComponentDescriptorStrategy
// 支持 RFC 6570 URI Template 语法，包括查询参数和 Header 参数
type openAPIMCPDescriptorStrategy struct{}

// NewOpenAPIMCPDescriptorStrategy 创建 openapi-mcp 的描述符策略
func NewOpenAPIMCPDescriptorStrategy() interfaces.ComponentDescriptorStrategy {
	return openAPIMCPDescriptorStrategy{}
}

func (openAPIMCPDescriptorStrategy) Describe(operation interfaces.Operation, mcpType interfaces.MCPType) string {
	description := operation.GetDescription()
	if description == "" {
		switch mcpType {
		case interfaces.MCPTypeTool:
			description = fmt.Sprintf("Execute %s operation", operation.GetName())
		case interfaces.MCPTypeResource:
			description = fmt.Sprintf("Access %s resource", operation.GetName())
		case interfaces.MCPTypeResourceTemplate:
			description = fmt.Sprintf("Access %s resource template", operation.GetName())
		}
	}

	if tags := operation.GetTags(); len(tags) > 0 {
		description += fmt.Sprintf("\nTags: %s", strings.Join(tags, ", "))
	}

	if metadata := operation.GetMetadata(); metadata != nil && len(metadata.Examples) > 0 {
		description += "\n\nExamples:"
		for _, example := range metadata.Examples {
			if example.Description != "" {
				description += fmt.Sprintf("\n- %s: %s", example.Name, example.Description)
			}
		}
	}

	return description
}

func (openAPIMCPDescriptorStrategy) BuildResourceURI(operation interfaces.Operation) string {
	metadata := operation.GetMetadata()
	if metadata != nil && metadata.Path != "" {
		return fmt.Sprintf("resource://%s", strings.TrimPrefix(metadata.Path, "/"))
	}
	return fmt.Sprintf("resource://%s", operation.GetName())
}

// BuildResourceTemplateURI 构建资源模板 URI，包含 RFC 6570 查询参数
// 根据 MCP 协议标准，URI 应该包含 resource:// 前缀
func (openAPIMCPDescriptorStrategy) BuildResourceTemplateURI(operation interfaces.Operation) string {
	metadata := operation.GetMetadata()
	if metadata == nil || metadata.Path == "" {
		return "resource://" + operation.GetName()
	}

	base := strings.TrimPrefix(metadata.Path, "/")

	// 尝试从 HTTPOperation 获取参数信息
	httpOp, ok := operation.(interfaces.HTTPOperation)
	if !ok {
		return "resource://" + base
	}

	descriptor := httpOp.GetHTTPRequestDescriptor()
	if descriptor == nil || len(descriptor.Parameters) == 0 {
		return "resource://" + base
	}

	// 收集查询参数和 Header 参数
	var queryParams []string
	var headerParams []string

	for _, param := range descriptor.Parameters {
		if param.In == interfaces.ParameterInQuery {
			queryParams = append(queryParams, sanitizeParamName(param.Name))
		} else if param.In == interfaces.ParameterInHeader {
			// Header 参数使用特殊前缀，并将破折号替换为下划线（RFC 6570 不允许破折号）
			headerParams = append(headerParams, "__header__"+sanitizeParamName(param.Name))
		}
	}

	// 使用 RFC 6570 URI Template 语法添加查询参数: users{?page,limit,__header__Authorization}
	if len(queryParams) > 0 || len(headerParams) > 0 {
		var allParams []string
		allParams = append(allParams, queryParams...)
		allParams = append(allParams, headerParams...)
		base += "{?" + strings.Join(allParams, ",") + "}"
	}

	return "resource://" + base
}

// sanitizeParamName 将参数名转换为 RFC 6570 兼容格式
// RFC 6570 变量名只允许: ALPHA / DIGIT / "_" / pct-encoded
func sanitizeParamName(name string) string {
	// 将破折号替换为下划线
	return strings.ReplaceAll(name, "-", "_")
}

func (openAPIMCPDescriptorStrategy) BuildMeta(operation interfaces.Operation, mcpType interfaces.MCPType, tags []string) map[string]interface{} {
	meta := make(map[string]interface{})
	meta["component_type"] = string(mcpType)
	meta["operation_id"] = operation.GetID()

	if len(tags) > 0 {
		meta["tags"] = tags
	}

	if metadata := operation.GetMetadata(); metadata != nil {
		opMeta := make(map[string]interface{})
		if metadata.Method != "" {
			opMeta["method"] = metadata.Method
		}
		if metadata.Path != "" {
			opMeta["path"] = metadata.Path
		}
		opMeta["is_async"] = metadata.IsAsync
		opMeta["is_readonly"] = metadata.IsReadOnly
		opMeta["is_idempotent"] = metadata.IsIdempotent
		if metadata.Deprecated {
			opMeta["deprecated"] = true
		}
		meta["operation"] = opMeta
	}

	if extensions := operation.GetExtensions(); len(extensions) > 0 {
		meta["extensions"] = extensions
	}

	return meta
}