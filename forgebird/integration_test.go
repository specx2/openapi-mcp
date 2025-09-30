package forgebird

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	srv "github.com/mark3labs/mcp-go/server"
	"github.com/specx2/mcp-forgebird/core"
	"github.com/specx2/mcp-forgebird/core/interfaces"
)

// TestServerIntegration 测试完整的服务器集成流程
func TestServerIntegration(t *testing.T) {
	// OpenAPI spec: 包含 GET 和 POST 请求
	spec := []byte(`{
		"openapi": "3.0.0",
		"info": {
			"title": "Users API",
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
						},
						{
							"name": "limit",
							"in": "query",
							"schema": { "type": "integer" }
						}
					],
					"responses": {
						"200": {
							"description": "Success",
							"content": {
								"application/json": {
									"schema": {
										"type": "array",
										"items": {
											"type": "object",
											"properties": {
												"id": { "type": "string" },
												"name": { "type": "string" }
											}
										}
									}
								}
							}
						}
					}
				},
				"post": {
					"operationId": "createUser",
					"summary": "Create a user",
					"requestBody": {
						"required": true,
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"name": { "type": "string" },
										"email": { "type": "string" }
									},
									"required": ["name"]
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
			},
			"/users/{id}": {
				"get": {
					"operationId": "getUserById",
					"summary": "Get user by ID",
					"parameters": [
						{
							"name": "id",
							"in": "path",
							"required": true,
							"schema": { "type": "string" }
						}
					],
					"responses": {
						"200": {
							"description": "Success"
						}
					}
				}
			}
		}
	}`)

	// 1. 创建 Pipeline
	pipeline := NewPipeline()
	t.Log("✓ Pipeline 创建成功")

	// 2. 创建 Forgebird 并转换 spec
	fb := core.NewForgebird(pipeline)
	config := interfaces.ConversionConfig{
		Name:    "users-api",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
		Spec: interfaces.SpecConfig{
			Version:    "3.0.0",
			Validation: true,
		},
		Mapping: interfaces.MappingConfig{
			Rules: []interfaces.MappingRule{}, // 使用默认映射（包含一对多映射）
		},
		Output: interfaces.OutputConfig{
			IncludeMetadata: true,
		},
	}

	components, err := fb.ConvertSpec(spec, config)
	if err != nil {
		t.Fatalf("❌ 转换失败: %v", err)
	}
	t.Logf("✓ Spec 转换成功，生成 %d 个组件", len(components))

	// 统计组件类型
	toolCount := 0
	resourceCount := 0
	templateCount := 0

	for _, component := range components {
		switch component.GetType() {
		case interfaces.MCPTypeTool:
			toolCount++
			t.Logf("  - Tool: %s", component.GetName())
		case interfaces.MCPTypeResource:
			resourceCount++
			t.Logf("  - Resource: %s", component.GetName())
		case interfaces.MCPTypeResourceTemplate:
			templateCount++
			t.Logf("  - ResourceTemplate: %s", component.GetName())
		}
	}

	t.Logf("\n统计:")
	t.Logf("  Tools: %d", toolCount)
	t.Logf("  Resources: %d", resourceCount)
	t.Logf("  ResourceTemplates: %d", templateCount)

	// 验证一对多映射：2个GET请求应该各生成1个Tool + 1个ResourceTemplate，1个POST生成1个Tool
	// 期望: 3个Tool + 2个ResourceTemplate = 5个组件
	if len(components) != 5 {
		t.Errorf("❌ 预期 5 个组件，实际 %d 个", len(components))
	}
	if toolCount != 3 {
		t.Errorf("❌ 预期 3 个 Tool，实际 %d 个", toolCount)
	}
	if templateCount != 2 {
		t.Errorf("❌ 预期 2 个 ResourceTemplate，实际 %d 个", templateCount)
	}

	// 3. 创建 MCP Server 并注册组件
	server := srv.NewMCPServer("test-openapi-server", "1.0.0")
	t.Log("\n✓ MCP Server 创建成功")

	err = RegisterComponents(server, components)
	if err != nil {
		t.Fatalf("❌ 注册组件失败: %v", err)
	}
	t.Log("✓ 组件注册成功")

	// 4. 验证 Tools 是否注册成功
	registeredTools := server.ListTools()
	registeredToolCount := len(registeredTools)
	t.Logf("\n已注册的 Tools (%d):", registeredToolCount)
	for name, tool := range registeredTools {
		t.Logf("  - %s: %s", name, tool.Tool.Description[:50]+"...")
	}

	if registeredToolCount != toolCount {
		t.Errorf("❌ 预期注册 %d 个 Tool，实际注册 %d 个", toolCount, registeredToolCount)
	}

	// 5. 通过 resources/list 验证资源是否注册成功
	listResourcesMessage := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "resources/list"
	}`

	resourceResponse := server.HandleMessage(context.Background(), []byte(listResourcesMessage))
	if resourceResponse != nil {
		resourceResponseJSON, _ := json.MarshalIndent(resourceResponse, "", "  ")
		t.Logf("\n=== resources/list 响应 ===\n%s", string(resourceResponseJSON))

		// 验证响应中包含资源
		resp, ok := resourceResponse.(mcp.JSONRPCResponse)
		if !ok {
			t.Errorf("❌ 响应类型错误：%T", resourceResponse)
		} else {
			listResult, ok := resp.Result.(mcp.ListResourcesResult)
			if !ok {
				t.Errorf("❌ 结果类型错误：%T", resp.Result)
			} else {
				resourceListCount := len(listResult.Resources)
				t.Logf("\n✓ resources/list 返回 %d 个资源", resourceListCount)

				// 应该有 2 个资源（对应 2 个 ResourceTemplate）
				if resourceListCount != 2 {
					t.Errorf("❌ 预期 2 个资源，实际 %d 个", resourceListCount)
				}

				// 验证资源的 URI 和 Name
				for _, res := range listResult.Resources {
					t.Logf("  - Resource: uri=%s, name=%s", res.URI, res.Name)
				}
			}
		}
	}

	// 6. 验证 ResourceTemplates 是否注册成功（通过 MCP 协议）
	listTemplatesMessage := `{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "resources/templates/list"
	}`

	templateResponse := server.HandleMessage(context.Background(), []byte(listTemplatesMessage))
	if templateResponse == nil {
		t.Fatal("❌ 列出资源模板失败：响应为空")
	}

	templateResp, ok := templateResponse.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("❌ 响应类型错误：%T", templateResponse)
	}

	listResult, ok := templateResp.Result.(mcp.ListResourceTemplatesResult)
	if !ok {
		t.Fatalf("❌ 结果类型错误：%T", templateResp.Result)
	}

	registeredTemplateCount := len(listResult.ResourceTemplates)
	t.Logf("\n已注册的 ResourceTemplates (%d):", registeredTemplateCount)

	foundUsersTemplate := false
	foundUserByIdTemplate := false

	for _, template := range listResult.ResourceTemplates {
		uriTemplate := template.URITemplate.Template.Raw()
		t.Logf("  - %s: URI Template=%s", template.Name, uriTemplate)

		// 验证查询参数是否在 URI Template 中
		if template.Name == "List_all_users" && !strings.Contains(uriTemplate, "{?") {
			t.Errorf("❌ URI Template 应该包含查询参数 {?page,limit}，但实际是: %s", uriTemplate)
		}

		// 检查 /users 的模板（现在已修复，name 是操作名）
		if template.Name == "List_all_users" {
			foundUsersTemplate = true
		}

		// 检查 /users/{id} 的模板（现在已修复，name 是操作名）
		if template.Name == "Get_user_by_ID" {
			foundUserByIdTemplate = true
		}
	}

	if registeredTemplateCount != templateCount {
		t.Errorf("❌ 预期注册 %d 个 ResourceTemplate，实际注册 %d 个", templateCount, registeredTemplateCount)
	}

	if !foundUsersTemplate {
		t.Error("❌ 未找到 List_all_users 资源模板")
	}
	if !foundUserByIdTemplate {
		t.Error("❌ 未找到 Get_user_by_ID 资源模板")
	}

	// 输出详细的模板信息
	t.Log("\n=== ResourceTemplate 详情 ===")
	for i, template := range listResult.ResourceTemplates {
		templateJSON, _ := json.MarshalIndent(template, "", "  ")
		t.Logf("\nTemplate %d:\n%s", i+1, string(templateJSON))
	}

	// 测试通过
	if registeredToolCount == toolCount && registeredTemplateCount == templateCount &&
		foundUsersTemplate && foundUserByIdTemplate {
		t.Log("\n✅ 集成测试通过！")
		t.Log("   - GET 请求成功生成 Tool 和 ResourceTemplate")
		t.Log("   - POST 请求成功生成 Tool")
		t.Log("   - 所有组件成功注册到 MCP Server")
		t.Log("   - ResourceTemplates 同时注册为 Resource（可通过 resources/list 访问）")
		t.Log("   - ResourceTemplates 的 URI 和 Name 正确（修复了参数顺序）")
	}
}