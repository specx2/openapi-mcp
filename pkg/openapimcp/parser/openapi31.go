package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

type OpenAPI31Parser struct {
	document   libopenapi.Document
	model      *libopenapi.DocumentModel[v3.Document]
	components map[string]ir.Schema
}

func NewOpenAPI31Parser() *OpenAPI31Parser {
	return &OpenAPI31Parser{
		components: make(map[string]ir.Schema),
	}
}

func (p *OpenAPI31Parser) ParseSpec(spec []byte) ([]ir.HTTPRoute, error) {
	var err error
	p.document, err = libopenapi.NewDocument(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	p.model, err = p.document.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("failed to build v3 model: %w", err)
	}

	doc := p.model.Model
	var routes []ir.HTTPRoute

	if doc.Components != nil && doc.Components.Schemas != nil {
		for name, schema := range doc.Components.Schemas.FromOldest() {
			p.components[name] = p.convertSchema(schema.Schema())
		}
	}

	if doc.Paths == nil {
		return routes, nil
	}

	for path, pathItem := range doc.Paths.PathItems.FromOldest() {
		if pathItem == nil {
			continue
		}

		commonParams := p.convertParameters(pathItem.Parameters)

		operations := map[string]*v3.Operation{
			"GET":     pathItem.Get,
			"POST":    pathItem.Post,
			"PUT":     pathItem.Put,
			"DELETE":  pathItem.Delete,
			"PATCH":   pathItem.Patch,
			"HEAD":    pathItem.Head,
			"OPTIONS": pathItem.Options,
			"TRACE":   pathItem.Trace,
		}

		for method, operation := range operations {
			if operation == nil {
				continue
			}

			route := ir.HTTPRoute{
				Path:           path,
				Method:         method,
				OperationID:    operation.OperationId,
				Summary:        operation.Summary,
				Description:    operation.Description,
				Tags:           operation.Tags,
				Parameters:     append(commonParams, p.convertParameters(operation.Parameters)...),
				Responses:      p.convertResponses(operation.Responses),
				Extensions:     make(map[string]interface{}), // TODO: Convert orderedmap to map[string]interface{}
				OpenAPIVersion: "3.1",
			}

			if operation.RequestBody != nil {
				route.RequestBody = p.convertRequestBody(operation.RequestBody)
			}

			route.SchemaDefs = p.buildSchemaDefinitions()

			routes = append(routes, route)
		}
	}

	return routes, nil
}

func (p *OpenAPI31Parser) convertParameters(params []*v3.Parameter) []ir.ParameterInfo {
	var result []ir.ParameterInfo
	for _, param := range params {
		if param == nil {
			continue
		}

		paramInfo := ir.ParameterInfo{
			Name:        param.Name,
			In:          param.In,
			Required:    param.Required != nil && *param.Required,
			Description: param.Description,
			Style:       param.Style,
		}

		if param.Explode != nil {
			paramInfo.Explode = param.Explode
		}

		if param.Schema != nil {
			paramInfo.Schema = p.convertSchema(param.Schema.Schema())
		}

		result = append(result, paramInfo)
	}
	return result
}

func (p *OpenAPI31Parser) convertRequestBody(requestBody *v3.RequestBody) *ir.RequestBodyInfo {
	if requestBody == nil {
		return nil
	}

	info := &ir.RequestBodyInfo{
		Required:       requestBody.Required != nil && *requestBody.Required,
		Description:    requestBody.Description,
		ContentSchemas: make(map[string]ir.Schema),
	}

	if requestBody.Content != nil {
		for mediaType, mediaTypeObj := range requestBody.Content.FromOldest() {
			if mediaTypeObj != nil && mediaTypeObj.Schema != nil {
				info.ContentSchemas[mediaType] = p.convertSchema(mediaTypeObj.Schema.Schema())
			}
		}
	}

	return info
}

func (p *OpenAPI31Parser) convertResponses(responses *v3.Responses) map[string]ir.ResponseInfo {
	result := make(map[string]ir.ResponseInfo)
	if responses == nil {
		return result
	}

	for status, response := range responses.Codes.FromOldest() {
		if response == nil {
			continue
		}

		respInfo := ir.ResponseInfo{
			Description:    response.Description,
			ContentSchemas: make(map[string]ir.Schema),
		}

		if response.Content != nil {
			for mediaType, mediaTypeObj := range response.Content.FromOldest() {
				if mediaTypeObj != nil && mediaTypeObj.Schema != nil {
					respInfo.ContentSchemas[mediaType] = p.convertSchema(mediaTypeObj.Schema.Schema())
				}
			}
		}

		result[status] = respInfo
	}

	if responses.Default != nil {
		respInfo := ir.ResponseInfo{
			Description:    responses.Default.Description,
			ContentSchemas: make(map[string]ir.Schema),
		}

		if responses.Default.Content != nil {
			for mediaType, mediaTypeObj := range responses.Default.Content.FromOldest() {
				if mediaTypeObj != nil && mediaTypeObj.Schema != nil {
					respInfo.ContentSchemas[mediaType] = p.convertSchema(mediaTypeObj.Schema.Schema())
				}
			}
		}

		result["default"] = respInfo
	}

	return result
}

func (p *OpenAPI31Parser) convertSchema(schema interface{}) ir.Schema {
	if schema == nil {
		return nil
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil
	}

	return ConvertToJSONSchema(schemaMap, false)
}

func (p *OpenAPI31Parser) buildSchemaDefinitions() ir.Schema {
	if len(p.components) == 0 {
		return nil
	}

	return ir.Schema{"$defs": p.components}
}

func (p *OpenAPI31Parser) ResolveReference(ref string) (ir.Schema, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("unsupported reference format: %s", ref)
	}

	if strings.HasPrefix(ref, "#/components/schemas/") {
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		if schema, ok := p.components[name]; ok {
			return schema, nil
		}
		return nil, fmt.Errorf("schema not found: %s", name)
	}

	return nil, fmt.Errorf("unsupported reference path: %s", ref)
}

func (p *OpenAPI31Parser) GetVersion() string {
	// libopenapi doesn't expose OpenAPI version directly, return default
	return "3.1"
}

func (p *OpenAPI31Parser) Validate() error {
	if p.document == nil {
		return fmt.Errorf("no document loaded")
	}

	// libopenapi validation is done during parsing
	// Return nil as validation errors would have been caught earlier
	return nil
}