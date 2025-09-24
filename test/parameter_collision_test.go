package test

import (
	"net/http"
	"testing"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/executor"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/factory"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

// MockHTTPClient 用于测试
type MockHTTPClient struct{}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// 返回一个模拟的 HTTP 响应
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
	}, nil
}

func TestParameterCollisionDetection(t *testing.T) {
	// 创建测试用的 HTTPRoute，包含参数冲突
	route := ir.HTTPRoute{
		Path:        "/users/{id}",
		Method:      "PUT",
		OperationID: "updateUser",
		Parameters: []ir.ParameterInfo{
			{
				Name:     "id",
				In:       "path",
				Required: true,
				Schema:   ir.Schema{"type": "integer"},
			},
			{
				Name:     "id",
				In:       "query",
				Required: false,
				Schema:   ir.Schema{"type": "integer"},
			},
		},
		RequestBody: &ir.RequestBodyInfo{
			Required: true,
			ContentSchemas: map[string]ir.Schema{
				"application/json": {
					"type": "object",
					"properties": map[string]interface{}{
						"id":   ir.Schema{"type": "integer"},
						"name": ir.Schema{"type": "string"},
					},
					"required": []string{"name"},
				},
			},
		},
	}

	// 创建 ComponentFactory
	mockClient := &MockHTTPClient{}
	cf := factory.NewComponentFactory(mockClient, "https://api.example.com")

	// 测试参数冲突检测
	paramMap := cf.DetectParameterCollisions(route)

	// 验证结果
	expectedMappings := map[string]ir.ParamMapping{
		"id__path": {
			OpenAPIName:  "id",
			Location:     "path",
			IsSuffixed:   true,
			OriginalName: "id",
		},
		"id__query": {
			OpenAPIName:  "id",
			Location:     "query",
			IsSuffixed:   true,
			OriginalName: "id",
		},
	}

	for expectedName, expectedMapping := range expectedMappings {
		if mapping, exists := paramMap[expectedName]; !exists {
			t.Errorf("Expected parameter mapping for %s not found", expectedName)
		} else {
			if mapping.OpenAPIName != expectedMapping.OpenAPIName {
				t.Errorf("Expected OpenAPIName %s, got %s", expectedMapping.OpenAPIName, mapping.OpenAPIName)
			}
			if mapping.Location != expectedMapping.Location {
				t.Errorf("Expected Location %s, got %s", expectedMapping.Location, mapping.Location)
			}
			if mapping.IsSuffixed != expectedMapping.IsSuffixed {
				t.Errorf("Expected IsSuffixed %v, got %v", expectedMapping.IsSuffixed, mapping.IsSuffixed)
			}
		}
	}

	// 验证请求体属性没有被映射（因为它们不冲突）
	if _, exists := paramMap["name"]; exists {
		t.Error("Request body property 'name' should not be in parameter map")
	}
}

func TestNoParameterCollision(t *testing.T) {
	// 创建没有参数冲突的 HTTPRoute
	route := ir.HTTPRoute{
		Path:        "/users/{userId}",
		Method:      "POST",
		OperationID: "createUser",
		Parameters: []ir.ParameterInfo{
			{
				Name:     "userId",
				In:       "path",
				Required: true,
				Schema:   ir.Schema{"type": "integer"},
			},
			{
				Name:     "limit",
				In:       "query",
				Required: false,
				Schema:   ir.Schema{"type": "integer"},
			},
		},
		RequestBody: &ir.RequestBodyInfo{
			Required: true,
			ContentSchemas: map[string]ir.Schema{
				"application/json": {
					"type": "object",
					"properties": map[string]interface{}{
						"name":  ir.Schema{"type": "string"},
						"email": ir.Schema{"type": "string"},
					},
				},
			},
		},
	}

	mockClient := &MockHTTPClient{}
	cf := factory.NewComponentFactory(mockClient, "https://api.example.com")

	paramMap := cf.DetectParameterCollisions(route)

	// 验证没有后缀的参数映射
	expectedMappings := map[string]ir.ParamMapping{
		"userId": {
			OpenAPIName:  "userId",
			Location:     "path",
			IsSuffixed:   false,
			OriginalName: "userId",
		},
		"limit": {
			OpenAPIName:  "limit",
			Location:     "query",
			IsSuffixed:   false,
			OriginalName: "limit",
		},
	}

	for expectedName, expectedMapping := range expectedMappings {
		if mapping, exists := paramMap[expectedName]; !exists {
			t.Errorf("Expected parameter mapping for %s not found", expectedName)
		} else {
			if mapping.IsSuffixed {
				t.Errorf("Parameter %s should not be suffixed", expectedName)
			}
			// 验证其他属性
			if mapping.OpenAPIName != expectedMapping.OpenAPIName {
				t.Errorf("Expected OpenAPIName %s, got %s", expectedMapping.OpenAPIName, mapping.OpenAPIName)
			}
			if mapping.Location != expectedMapping.Location {
				t.Errorf("Expected Location %s, got %s", expectedMapping.Location, mapping.Location)
			}
		}
	}
}

func TestParameterMappingInTool(t *testing.T) {
	// 创建带有参数冲突的 OpenAPITool
	route := ir.HTTPRoute{
		Path:        "/users/{id}",
		Method:      "PUT",
		OperationID: "updateUser",
		Parameters: []ir.ParameterInfo{
			{
				Name:     "id",
				In:       "path",
				Required: true,
				Schema:   ir.Schema{"type": "integer"},
			},
		},
		RequestBody: &ir.RequestBodyInfo{
			Required: true,
			ContentSchemas: map[string]ir.Schema{
				"application/json": {
					"type": "object",
					"properties": map[string]interface{}{
						"id":   ir.Schema{"type": "integer"},
						"name": ir.Schema{"type": "string"},
					},
				},
			},
		},
	}

	paramMap := map[string]ir.ParamMapping{
		"id__path": {
			OpenAPIName:  "id",
			Location:     "path",
			IsSuffixed:   true,
			OriginalName: "id",
		},
		"name": {
			OpenAPIName:  "name",
			Location:     "body",
			IsSuffixed:   false,
			OriginalName: "name",
		},
	}

	mockClient := &MockHTTPClient{}
	tool := executor.NewOpenAPITool(
		"updateUser",
		"Update user",
		ir.Schema{},
		nil,
		false,
		route,
		mockClient,
		"https://api.example.com",
		paramMap,
	)

	// 测试参数映射
	args := map[string]interface{}{
		"id__path": 123,
		"name":     "John Doe",
	}

	mappedArgs, err := tool.MapParameters(args)
	if err != nil {
		t.Fatalf("Failed to map parameters: %v", err)
	}

	// 验证映射结果
	if mappedArgs["id"] != 123 {
		t.Errorf("Expected id=123, got %v", mappedArgs["id"])
	}
	if mappedArgs["name"] != "John Doe" {
		t.Errorf("Expected name='John Doe', got %v", mappedArgs["name"])
	}

	// 验证后缀参数不存在于映射结果中
	if _, exists := mappedArgs["id__path"]; exists {
		t.Error("Suffixed parameter 'id__path' should not be in mapped args")
	}
}
