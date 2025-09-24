package main

import (
	"log"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp"
)

func main() {
	// Example OpenAPI spec (simplified petstore)
	openAPISpec := `{
		"openapi": "3.0.0",
		"info": {
			"title": "Pet Store",
			"version": "1.0.0"
		},
		"paths": {
			"/pets": {
				"get": {
					"operationId": "listPets",
					"summary": "List all pets",
					"tags": ["pets"],
					"parameters": [
						{
							"name": "limit",
							"in": "query",
							"schema": {
								"type": "integer",
								"maximum": 100
							}
						}
					],
					"responses": {
						"200": {
							"description": "A list of pets",
							"content": {
								"application/json": {
									"schema": {
										"type": "array",
										"items": {
											"type": "object",
											"properties": {
												"id": {"type": "integer"},
												"name": {"type": "string"},
												"tag": {"type": "string"}
											}
										}
									}
								}
							}
						}
					}
				},
				"post": {
					"operationId": "createPet",
					"summary": "Create a pet",
					"tags": ["pets"],
					"requestBody": {
						"required": true,
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"required": ["name"],
									"properties": {
										"name": {"type": "string"},
										"tag": {"type": "string"}
									}
								}
							}
						}
					},
					"responses": {
						"201": {
							"description": "Pet created",
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"properties": {
											"id": {"type": "integer"},
											"name": {"type": "string"},
											"tag": {"type": "string"}
										}
									}
								}
							}
						}
					}
				}
			},
			"/pets/{petId}": {
				"get": {
					"operationId": "getPet",
					"summary": "Info for a specific pet",
					"tags": ["pets"],
					"parameters": [
						{
							"name": "petId",
							"in": "path",
							"required": true,
							"schema": {
								"type": "integer"
							}
						}
					],
					"responses": {
						"200": {
							"description": "Expected response to a valid request",
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"properties": {
											"id": {"type": "integer"},
											"name": {"type": "string"},
											"tag": {"type": "string"}
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

	// Create OpenAPI MCP server
	server, err := openapimcp.NewServer([]byte(openAPISpec),
		openapimcp.WithBaseURL("https://petstore.swagger.io/v1"),
		openapimcp.WithServerInfo("petstore-mcp", "1.0.0"),
	)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// In a real implementation, you would serve via different transports:
	//
	// For STDIO (CLI usage):
	// server.Serve(server.NewStdioTransport())
	//
	// For HTTP (web usage):
	// server.Serve(server.NewHTTPTransport(":8080"))
	//
	// For now, just demonstrate the server was created successfully
	log.Printf("OpenAPI MCP Server created successfully!")
	log.Printf("Server instance: %T", server)
	log.Printf("OpenAPI MCP Server created and ready to serve!")

	// Show what would be available as tools/resources
	log.Println("\nThis server would provide the following MCP capabilities:")
	log.Println("Tools:")
	log.Println("  - listPets: List all pets (with optional limit parameter)")
	log.Println("  - createPet: Create a new pet (requires name, optional tag)")
	log.Println("Resources/Resource Templates:")
	log.Println("  - pets/{petId}: Get specific pet information")

	log.Println("\nExample usage in an MCP client:")
	log.Println("  Tool call: {'name': 'listPets', 'arguments': {'limit': 10}}")
	log.Println("  Tool call: {'name': 'createPet', 'arguments': {'name': 'Fluffy', 'tag': 'cat'}}")
	log.Println("  Resource: resource://pets/123")
}
