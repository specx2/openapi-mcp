package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

type OpenAPI30Parser struct {
	document           libopenapi.Document
	model              *libopenapi.DocumentModel[v3.Document]
	components         map[string]ir.Schema
	callbackComponents map[string]*v3.Callback
	pathItemComponents map[string]*v3.PathItem
}

func NewOpenAPI30Parser() *OpenAPI30Parser {
	return &OpenAPI30Parser{
		components:         make(map[string]ir.Schema),
		callbackComponents: make(map[string]*v3.Callback),
		pathItemComponents: make(map[string]*v3.PathItem),
	}
}

func (p *OpenAPI30Parser) ParseSpec(spec []byte) ([]ir.HTTPRoute, error) {
	var err error
	for k := range p.components {
		delete(p.components, k)
	}
	for k := range p.callbackComponents {
		delete(p.callbackComponents, k)
	}
	for k := range p.pathItemComponents {
		delete(p.pathItemComponents, k)
	}
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

	if doc.Components != nil {
		if doc.Components.Schemas != nil {
			for name, schema := range doc.Components.Schemas.FromOldest() {
				p.components[name] = p.convertSchema(schema.Schema())
			}
		}
		if doc.Components.Callbacks != nil {
			for name, cb := range doc.Components.Callbacks.FromOldest() {
				if cb != nil {
					p.callbackComponents[name] = cb
				}
			}
		}
		if doc.Components.PathItems != nil {
			for name, pathItem := range doc.Components.PathItems.FromOldest() {
				if pathItem != nil {
					p.pathItemComponents[name] = pathItem
				}
			}
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
				Extensions:     convertExtensionsMap(operation.Extensions),
				OpenAPIVersion: "3.0",
			}

			if operation.RequestBody != nil {
				route.RequestBody = p.convertRequestBody(operation.RequestBody)
			}

			if callbacks := p.convertCallbacks(operation.Callbacks); len(callbacks) > 0 {
				route.Callbacks = callbacks
			}

			route.SchemaDefs = p.buildSchemaDefinitions()

			routes = append(routes, route)
		}
	}

	return routes, nil
}

func (p *OpenAPI30Parser) convertParameters(params []*v3.Parameter) []ir.ParameterInfo {
	var result []ir.ParameterInfo
	for _, param := range params {
		if param == nil {
			continue
		}

		paramInfo := ir.ParameterInfo{
			Name:            param.Name,
			In:              param.In,
			Required:        param.Required != nil && *param.Required,
			Description:     param.Description,
			Style:           param.Style,
			AllowReserved:   param.AllowReserved,
			Deprecated:      param.Deprecated,
			AllowEmptyValue: param.AllowEmptyValue,
		}

		if param.Explode != nil {
			paramInfo.Explode = param.Explode
		}

		if param.Schema != nil {
			paramInfo.Schema = p.convertSchema(param.Schema.Schema())
		}

		if param.Example != nil {
			paramInfo.Example = extractExampleValue(param.Example)
		}

		if examples := convertExamplesMap(param.Examples); len(examples) > 0 {
			paramInfo.Examples = examples
		}

		if extensions := convertExtensionsMap(param.Extensions); len(extensions) > 0 {
			paramInfo.Extensions = extensions
		}

		result = append(result, paramInfo)
	}
	return result
}

func (p *OpenAPI30Parser) convertRequestBody(requestBody *v3.RequestBody) *ir.RequestBodyInfo {
	if requestBody == nil {
		return nil
	}

	info := &ir.RequestBodyInfo{
		Required:       requestBody.Required != nil && *requestBody.Required,
		Description:    requestBody.Description,
		ContentSchemas: make(map[string]ir.Schema),
		Encodings:      make(map[string]map[string]ir.EncodingInfo),
	}

	if extensions := convertExtensionsMap(requestBody.Extensions); len(extensions) > 0 {
		info.Extensions = extensions
	}

	if requestBody.Content != nil {
		for mediaType, mediaTypeObj := range requestBody.Content.FromOldest() {
			info.ContentOrder = append(info.ContentOrder, mediaType)
			if mediaTypeObj == nil {
				continue
			}
			var converted ir.Schema
			if mediaTypeObj.Schema != nil {
				converted = p.convertSchema(mediaTypeObj.Schema.Schema())
				info.ContentSchemas[mediaType] = converted
			}
			if encodings := convertEncodings(mediaTypeObj.Encoding, true); len(encodings) > 0 {
				info.Encodings[mediaType] = encodings
			}
			if converted != nil {
				if def, ok := converted["default"]; ok {
					if info.MediaDefaults == nil {
						info.MediaDefaults = make(map[string]interface{})
					}
					info.MediaDefaults[mediaType] = cloneAny(def)
				}
			}
			if mediaTypeObj.Example != nil {
				if info.MediaExamples == nil {
					info.MediaExamples = make(map[string]interface{})
				}
				if example := extractExampleValue(mediaTypeObj.Example); example != nil {
					info.MediaExamples[mediaType] = example
				}
			}
			if examples := convertExamplesMap(mediaTypeObj.Examples); len(examples) > 0 {
				if info.MediaExampleSets == nil {
					info.MediaExampleSets = make(map[string]map[string]interface{})
				}
				info.MediaExampleSets[mediaType] = examples
			}
			if extensions := convertExtensionsMap(mediaTypeObj.Extensions); len(extensions) > 0 {
				if info.MediaExtensions == nil {
					info.MediaExtensions = make(map[string]map[string]interface{})
				}
				info.MediaExtensions[mediaType] = extensions
			}
		}
	}

	return info
}

func (p *OpenAPI30Parser) convertResponses(responses *v3.Responses) map[string]ir.ResponseInfo {
	result := make(map[string]ir.ResponseInfo)
	if responses == nil {
		return result
	}

	for status, response := range responses.Codes.FromOldest() {
		if response == nil {
			continue
		}

		respInfo := ir.ResponseInfo{
			Description:      response.Description,
			ContentSchemas:   make(map[string]ir.Schema),
			MediaExamples:    make(map[string]interface{}),
			MediaExampleSets: make(map[string]map[string]interface{}),
			MediaExtensions:  make(map[string]map[string]interface{}),
		}

		if extensions := convertExtensionsMap(response.Extensions); len(extensions) > 0 {
			respInfo.Extensions = extensions
		}

		if response.Content != nil {
			for mediaType, mediaTypeObj := range response.Content.FromOldest() {
				if mediaTypeObj == nil {
					continue
				}
				if mediaTypeObj.Schema != nil {
					respInfo.ContentSchemas[mediaType] = p.convertSchema(mediaTypeObj.Schema.Schema())
				}
				if mediaTypeObj.Example != nil {
					if example := extractExampleValue(mediaTypeObj.Example); example != nil {
						respInfo.MediaExamples[mediaType] = example
					}
				}
				if examples := convertExamplesMap(mediaTypeObj.Examples); len(examples) > 0 {
					respInfo.MediaExampleSets[mediaType] = examples
				}
				if extensions := convertExtensionsMap(mediaTypeObj.Extensions); len(extensions) > 0 {
					respInfo.MediaExtensions[mediaType] = extensions
				}
			}
		}
		if len(respInfo.MediaExamples) == 0 {
			respInfo.MediaExamples = nil
		}
		if len(respInfo.MediaExampleSets) == 0 {
			respInfo.MediaExampleSets = nil
		}
		if len(respInfo.MediaExtensions) == 0 {
			respInfo.MediaExtensions = nil
		}

		result[status] = respInfo
	}

	if responses.Default != nil {
		respInfo := ir.ResponseInfo{
			Description:      responses.Default.Description,
			ContentSchemas:   make(map[string]ir.Schema),
			MediaExamples:    make(map[string]interface{}),
			MediaExampleSets: make(map[string]map[string]interface{}),
			MediaExtensions:  make(map[string]map[string]interface{}),
		}

		if extensions := convertExtensionsMap(responses.Default.Extensions); len(extensions) > 0 {
			respInfo.Extensions = extensions
		}

		if responses.Default.Content != nil {
			for mediaType, mediaTypeObj := range responses.Default.Content.FromOldest() {
				if mediaTypeObj == nil {
					continue
				}
				if mediaTypeObj.Schema != nil {
					respInfo.ContentSchemas[mediaType] = p.convertSchema(mediaTypeObj.Schema.Schema())
				}
				if mediaTypeObj.Example != nil {
					if example := extractExampleValue(mediaTypeObj.Example); example != nil {
						respInfo.MediaExamples[mediaType] = example
					}
				}
				if examples := convertExamplesMap(mediaTypeObj.Examples); len(examples) > 0 {
					respInfo.MediaExampleSets[mediaType] = examples
				}
				if extensions := convertExtensionsMap(mediaTypeObj.Extensions); len(extensions) > 0 {
					respInfo.MediaExtensions[mediaType] = extensions
				}
			}
		}
		if len(respInfo.MediaExamples) == 0 {
			respInfo.MediaExamples = nil
		}
		if len(respInfo.MediaExampleSets) == 0 {
			respInfo.MediaExampleSets = nil
		}
		if len(respInfo.MediaExtensions) == 0 {
			respInfo.MediaExtensions = nil
		}

		result["default"] = respInfo
	}

	return result
}

func (p *OpenAPI30Parser) convertCallbacks(callbacks *orderedmap.Map[string, *v3.Callback]) []ir.CallbackInfo {
	if callbacks == nil || callbacks.Len() == 0 {
		return nil
	}

	var infos []ir.CallbackInfo
	for name, callback := range callbacks.FromOldest() {
		if callback == nil {
			continue
		}

		resolved := p.resolveCallback(callback)
		if resolved == nil {
			resolved = callback
		}

		extensions := convertExtensionsMap(callback.Extensions)
		if low := callback.GoLow(); low != nil {
			if lowExt := convertLowExtensionsMap(low.Extensions); len(lowExt) > 0 {
				if extensions == nil {
					extensions = make(map[string]interface{}, len(lowExt))
				}
				for k, v := range lowExt {
					if _, exists := extensions[k]; !exists {
						extensions[k] = v
					}
				}
			}
		}
		if resolved != callback {
			if resolvedExt := convertExtensionsMap(resolved.Extensions); len(resolvedExt) > 0 {
				if extensions == nil {
					extensions = make(map[string]interface{}, len(resolvedExt))
				}
				for k, v := range resolvedExt {
					if _, exists := extensions[k]; !exists {
						extensions[k] = v
					}
				}
			}
			if low := resolved.GoLow(); low != nil {
				if lowExt := convertLowExtensionsMap(low.Extensions); len(lowExt) > 0 {
					if extensions == nil {
						extensions = make(map[string]interface{}, len(lowExt))
					}
					for k, v := range lowExt {
						if _, exists := extensions[k]; !exists {
							extensions[k] = v
						}
					}
				}
			}
		}

		info := ir.CallbackInfo{
			Name:       name,
			Extensions: extensions,
		}

		if resolved.Expression != nil {
			for expression, pathItem := range resolved.Expression.FromOldest() {
				if expression != "" {
					if info.Expression == "" {
						info.Expression = expression
					} else if !strings.Contains(info.Expression, expression) {
						info.Expression += ", " + expression
					}
				}
				if pathItem == nil {
					continue
				}
				resolvedPath := p.resolveCallbackPathItem(pathItem)
				if resolvedPath == nil {
					continue
				}
				info.Operations = append(info.Operations, p.convertCallbackOperations(resolvedPath)...)
			}
		}

		if info.Expression == "" {
			info.Expression = name
		}

		if len(info.Operations) == 0 && len(info.Extensions) == 0 {
			continue
		}
		infos = append(infos, info)
	}

	if len(infos) == 0 {
		return nil
	}
	return infos
}

func (p *OpenAPI30Parser) convertCallbackOperations(pathItem *v3.PathItem) []ir.CallbackOperation {
	var ops []ir.CallbackOperation
	if pathItem == nil {
		return ops
	}

	operationMap := map[string]*v3.Operation{
		"GET":     pathItem.Get,
		"POST":    pathItem.Post,
		"PUT":     pathItem.Put,
		"DELETE":  pathItem.Delete,
		"PATCH":   pathItem.Patch,
		"HEAD":    pathItem.Head,
		"OPTIONS": pathItem.Options,
		"TRACE":   pathItem.Trace,
	}

	for method, operation := range operationMap {
		if operation == nil {
			continue
		}
		callbackOp := ir.CallbackOperation{
			Method:      method,
			Summary:     operation.Summary,
			Description: operation.Description,
			RequestBody: p.convertRequestBody(operation.RequestBody),
			Responses:   p.convertResponses(operation.Responses),
			Extensions:  convertExtensionsMap(operation.Extensions),
		}
		ops = append(ops, callbackOp)
	}

	return ops
}

func (p *OpenAPI30Parser) resolveCallback(callback *v3.Callback) *v3.Callback {
	if callback == nil {
		return nil
	}
	low := callback.GoLow()
	if low == nil || !low.IsReference() {
		return callback
	}
	ref := low.GetReference()
	if ref == "" {
		return callback
	}
	if strings.HasPrefix(ref, "#/components/callbacks/") {
		name := strings.TrimPrefix(ref, "#/components/callbacks/")
		if resolved, ok := p.callbackComponents[name]; ok && resolved != nil {
			return resolved
		}
	}
	return callback
}

func (p *OpenAPI30Parser) resolveCallbackPathItem(pathItem *v3.PathItem) *v3.PathItem {
	if pathItem == nil {
		return nil
	}
	low := pathItem.GoLow()
	if low == nil || !low.IsReference() {
		return pathItem
	}
	ref := low.GetReference()
	if ref == "" {
		return pathItem
	}
	if strings.HasPrefix(ref, "#/components/pathItems/") {
		name := strings.TrimPrefix(ref, "#/components/pathItems/")
		if resolved, ok := p.pathItemComponents[name]; ok && resolved != nil {
			return resolved
		}
	}
	return pathItem
}

func (p *OpenAPI30Parser) convertSchema(schema interface{}) ir.Schema {
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

	return ConvertToJSONSchema(schemaMap, true)
}

func (p *OpenAPI30Parser) buildSchemaDefinitions() ir.Schema {
	if len(p.components) == 0 {
		return nil
	}

	return ir.Schema{"$defs": p.components}
}

func (p *OpenAPI30Parser) ResolveReference(ref string) (ir.Schema, error) {
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

func (p *OpenAPI30Parser) GetVersion() string {
	// libopenapi doesn't expose OpenAPI version directly, return default
	return "3.0"
}

func (p *OpenAPI30Parser) Validate() error {
	if p.document == nil {
		return fmt.Errorf("no document loaded")
	}

	// libopenapi validation is done during parsing
	// Return nil as validation errors would have been caught earlier
	return nil
}
