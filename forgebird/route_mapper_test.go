package forgebird

import (
	"strings"
	"testing"

	"github.com/specx2/mcp-forgebird/core/interfaces"
)

// TestOneToManyMapping 测试一对多映射：GET 请求同时生成 Tool 和 ResourceTemplate
func TestOneToManyMapping(t *testing.T) {
	// 创建一个简单的 OpenAPI spec
	spec := []byte(`{
		"openapi": "3.0.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/users": {
				"get": {
					"operationId": "listUsers",
					"summary": "List all users",
					"parameters": [
						{
							"name": "page",
							"in": "query",
							"schema": { "type": "integer" }
						}
					],
					"responses": {
						"200": {
							"description": "Success"
						}
					}
				},
				"post": {
					"operationId": "createUser",
					"summary": "Create a user",
					"requestBody": {
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"name": { "type": "string" }
									}
								}
							}
						}
					},
					"responses": {
						"201": {
							"description": "Created"
						}
					}
				}
			}
		}
	}`)

	// 解析规范
	parser := NewParser(interfaces.ConversionConfig{})
	operations, err := parser.ParseSpec(spec)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 创建 Mapper 并设置自定义映射函数
	mapper := NewOpenAPIRouteMapper()
	mapper = mapper.WithMapFunc(func(operation interfaces.Operation) (*interfaces.MappingDecision, error) {
		metadata := operation.GetMetadata()
		if metadata == nil {
			return nil, nil
		}

		// 只对 GET 请求额外生成 ResourceTemplate
		if strings.ToUpper(metadata.Method) == "GET" {
			return &interfaces.MappingDecision{
				MCPType: interfaces.MCPTypeResourceTemplate,
				Name:    "",
				Tags:    nil,
				Exclude: false,
			}, nil
		}

		return nil, nil
	})

	// 执行映射
	mappedOps, err := mapper.MapOperations(operations)
	if err != nil {
		t.Fatalf("映射失败: %v", err)
	}

	// 统计各类型组件数量
	toolCount := 0
	templateCount := 0
	getToolFound := false
	getTemplateFound := false
	postToolFound := false

	for _, mapped := range mappedOps {
		metadata := mapped.Operation.GetMetadata()
		if metadata == nil {
			continue
		}

		switch mapped.MCPType {
		case interfaces.MCPTypeTool:
			toolCount++
			if strings.ToUpper(metadata.Method) == "GET" {
				getToolFound = true
			}
			if strings.ToUpper(metadata.Method) == "POST" {
				postToolFound = true
			}
		case interfaces.MCPTypeResourceTemplate:
			templateCount++
			if strings.ToUpper(metadata.Method) == "GET" {
				getTemplateFound = true
			}
		}
	}

	// 验证结果
	t.Logf("总映射数: %d", len(mappedOps))
	t.Logf("Tool 数量: %d", toolCount)
	t.Logf("ResourceTemplate 数量: %d", templateCount)

	// 断言：应该有 3 个映射结果
	if len(mappedOps) != 3 {
		t.Errorf("预期 3 个映射结果，实际 %d 个", len(mappedOps))
	}

	// 断言：应该有 2 个 Tool (GET + POST)
	if toolCount != 2 {
		t.Errorf("预期 2 个 Tool，实际 %d 个", toolCount)
	}

	// 断言：应该有 1 个 ResourceTemplate (GET)
	if templateCount != 1 {
		t.Errorf("预期 1 个 ResourceTemplate，实际 %d 个", templateCount)
	}

	// 断言：GET 请求生成了 Tool
	if !getToolFound {
		t.Error("GET 请求没有生成 Tool")
	}

	// 断言：GET 请求生成了 ResourceTemplate
	if !getTemplateFound {
		t.Error("GET 请求没有生成 ResourceTemplate")
	}

	// 断言：POST 请求生成了 Tool
	if !postToolFound {
		t.Error("POST 请求没有生成 Tool")
	}

	// 输出详细信息
	t.Log("\n=== 映射详情 ===")
	for i, mapped := range mappedOps {
		metadata := mapped.Operation.GetMetadata()
		method := "UNKNOWN"
		if metadata != nil {
			method = metadata.Method
		}
		t.Logf("  %d. [%s] %s -> %s", i+1, method, mapped.Name, mapped.MCPType)
	}
}

// TestWithoutCustomFunc 测试不使用自定义函数时的默认行为
func TestWithoutCustomFunc(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.0.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/users": {
				"get": {
					"operationId": "listUsers",
					"summary": "List users"
				}
			}
		}
	}`)

	parser := NewParser(interfaces.ConversionConfig{})
	operations, err := parser.ParseSpec(spec)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 创建 Mapper，不设置自定义映射函数
	mapper := NewOpenAPIRouteMapper()

	// 执行映射
	mappedOps, err := mapper.MapOperations(operations)
	if err != nil {
		t.Fatalf("映射失败: %v", err)
	}

	// 验证结果：应该只有 1 个映射（默认规则）
	if len(mappedOps) != 1 {
		t.Errorf("预期 1 个映射结果，实际 %d 个", len(mappedOps))
	}

	// 默认规则应该生成 Tool
	if len(mappedOps) > 0 && mappedOps[0].MCPType != interfaces.MCPTypeTool {
		t.Errorf("预期类型为 Tool，实际为 %s", mappedOps[0].MCPType)
	}
}