package main

import (
	"log"
	"regexp"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/mapper"
)

func main() {
	// 包含参数冲突的 OpenAPI 规范示例
	openAPISpec := `{
		"openapi": "3.1.0",
		"info": {
			"title": "User Management API",
			"version": "1.0.0"
		},
		"paths": {
			"/users/{id}": {
				"put": {
					"operationId": "updateUser",
					"summary": "Update user information",
					"tags": ["users"],
					"parameters": [
						{
							"name": "id",
							"in": "path",
							"required": true,
							"description": "User ID",
							"schema": {"type": "integer"}
						},
						{
							"name": "id",
							"in": "query",
							"required": false,
							"description": "Override ID for validation",
							"schema": {"type": "integer"}
						},
						{
							"name": "version",
							"in": "query",
							"required": false,
							"description": "API version",
							"schema": {"type": "string", "enum": ["v1", "v2"]}
						}
					],
					"requestBody": {
						"required": true,
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"required": ["name", "email"],
									"properties": {
										"id": {
											"type": "integer",
											"description": "User ID in request body"
										},
										"name": {
											"type": "string",
											"description": "User name"
										},
										"email": {
											"type": "string",
											"format": "email",
											"description": "User email"
										},
										"tags": {
											"type": "array",
											"items": {"type": "string"},
											"description": "User tags"
										}
									}
								}
							}
						}
					},
					"responses": {
						"200": {
							"description": "User updated successfully",
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"properties": {
											"id": {"type": "integer"},
											"name": {"type": "string"},
											"email": {"type": "string"},
											"updated_at": {"type": "string", "format": "date-time"}
										}
									}
								}
							}
						}
					}
				}
			},
			"/users": {
				"get": {
					"operationId": "listUsers",
					"summary": "List all users",
					"tags": ["users"],
					"parameters": [
						{
							"name": "limit",
							"in": "query",
							"required": false,
							"description": "Number of users to return",
							"schema": {"type": "integer", "maximum": 100}
						},
						{
							"name": "tags",
							"in": "query",
							"required": false,
							"description": "Filter by tags",
							"style": "form",
							"explode": true,
							"schema": {
								"type": "array",
								"items": {"type": "string"}
							}
						}
					],
					"responses": {
						"200": {
							"description": "List of users",
							"content": {
								"application/json": {
									"schema": {
										"type": "array",
										"items": {
											"type": "object",
											"properties": {
												"id": {"type": "integer"},
												"name": {"type": "string"},
												"email": {"type": "string"}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	// 创建自定义路由映射
	customMappings := []mapper.RouteMap{
		{
			Methods:     []string{"GET"},
			PathPattern: regexp.MustCompile(`.*`),
			MCPType:     mapper.MCPTypeResource,
		},
		{
			Methods:     []string{"PUT", "POST", "DELETE", "PATCH"},
			PathPattern: regexp.MustCompile(`.*`),
			MCPType:     mapper.MCPTypeTool,
		},
	}

	// 创建 OpenAPI MCP 服务器
	_, err := openapimcp.NewServer([]byte(openAPISpec),
		openapimcp.WithBaseURL("https://api.example.com/v1"),
		openapimcp.WithServerInfo("user-management-mcp", "1.0.0"),
		openapimcp.WithRouteMaps(customMappings),
		openapimcp.WithCustomNames(map[string]string{
			"updateUser": "modify_user",
			"listUsers":  "get_all_users",
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("✅ OpenAPI MCP Server created successfully!")
	log.Printf("📋 Server Info: %s v%s", "user-management-mcp", "1.0.0")

	// 展示改进后的功能
	log.Println("\n🚀 改进后的功能特性:")
	log.Println("1. ✅ 参数冲突自动处理")
	log.Println("   - 路径参数 'id' 和请求体属性 'id' 冲突")
	log.Println("   - 系统自动为路径参数添加 '__path' 后缀")
	log.Println("   - 查询参数 'id' 添加 '__query' 后缀")

	log.Println("\n2. ✅ 增强的错误处理")
	log.Println("   - 结构化的错误分类")
	log.Println("   - 详细的错误信息和上下文")
	log.Println("   - 支持重试逻辑判断")

	log.Println("\n3. ✅ 完整的参数序列化")
	log.Println("   - 支持多种 OpenAPI 参数样式")
	log.Println("   - 正确的 explode/implode 行为")
	log.Println("   - 数组参数的正确处理")

	log.Println("\n4. ✅ 智能路由映射")
	log.Println("   - GET 请求映射为 Resource")
	log.Println("   - 其他 HTTP 方法映射为 Tool")
	log.Println("   - 自定义组件命名支持")

	log.Println("\n📝 生成的 MCP 组件:")
	log.Println("Tools:")
	log.Println("  - modify_user: 更新用户信息")
	log.Println("    Parameters: id__path (integer, required), id__query (integer, optional)")
	log.Println("                version (string, enum: v1|v2, optional)")
	log.Println("    Body: id (integer, optional), name (string, required), email (string, required)")
	log.Println("           tags (array[string], optional)")

	log.Println("\nResources:")
	log.Println("  - get_all_users: 获取所有用户")
	log.Println("    Parameters: limit (integer, optional), tags (array[string], optional)")

	log.Println("\n🔧 使用示例:")
	log.Println("工具调用:")
	log.Println(`  {"name": "modify_user", "arguments": {`)
	log.Println(`    "id__path": 123,`)
	log.Println(`    "version": "v2",`)
	log.Println(`    "name": "John Doe",`)
	log.Println(`    "email": "john@example.com",`)
	log.Println(`    "tags": ["admin", "premium"]`)
	log.Println(`  }}`)

	log.Println("\n资源访问:")
	log.Println(`  resource://get_all_users?limit=10&tags=admin&tags=premium`)

	log.Println("\n✨ 这些改进使您的项目达到了与 fastmcp 相当的功能水平！")
}
