# openapi-mcp Architecture Design

## 1. Executive Summary

This document outlines the architecture design for **openapi-mcp**, a Go framework that converts OpenAPI specifications into MCP (Model Context Protocol) servers. The framework is built on top of `mcp-go` and replicates the capabilities of Python's `fastmcp.FastMCPOpenAPI` implementation.

### 1.1 Design Principles

1. **Clean Architecture**: Clear separation between OpenAPI parsing, route mapping, and MCP component generation
2. **Type Safety**: Leverage Go's type system for compile-time guarantees
3. **Extensibility**: Pluggable components for custom parsing, mapping, and request handling
4. **Idiomatic Go**: Follow Go best practices rather than direct Python port
5. **Integration**: Seamless integration with mcp-go's existing infrastructure

### 1.2 Core Capabilities

- ✅ Parse OpenAPI 3.0/3.1 specifications
- ✅ Generate MCP Tools from HTTP operations
- ✅ Generate MCP Resources from static GET endpoints
- ✅ Generate MCP ResourceTemplates from parameterized GET endpoints
- ✅ Flexible route mapping with regex patterns, tags, and methods
- ✅ Comprehensive parameter handling (path, query, header, body)
- ✅ Schema conversion with collision detection
- ✅ Request execution with proper serialization
- ✅ Authentication and header propagation
- ✅ Error handling and validation

---

## 2. High-Level Architecture

### 2.1 Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     User Application                         │
│                                                              │
│  server := openapimcp.NewServer(spec, options...)           │
│  server.Start(ctx)                                          │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                  OpenAPI MCP Server                          │
│  ┌────────────────────────────────────────────────────┐     │
│  │              OpenAPI Parsing Layer                  │     │
│  │  ┌──────────────────────────────────────────────┐  │     │
│  │  │ OpenAPIParser (interface)                     │  │     │
│  │  │  - ParseSpec(spec) → []HTTPRoute             │  │     │
│  │  │  - ResolveReferences($ref) → Schema          │  │     │
│  │  └──────────────────────────────────────────────┘  │     │
│  │           ▲                    ▲                    │     │
│  │           │                    │                    │     │
│  │  ┌────────────────┐   ┌────────────────┐          │     │
│  │  │ OpenAPI30Parser│   │ OpenAPI31Parser│          │     │
│  │  └────────────────┘   └────────────────┘          │     │
│  └────────────────────────────────────────────────────┘     │
│                           │                                  │
│                           ▼                                  │
│  ┌────────────────────────────────────────────────────┐     │
│  │           Intermediate Representation               │     │
│  │  ┌──────────────────────────────────────────────┐  │     │
│  │  │ HTTPRoute                                     │  │     │
│  │  │  - Path, Method, OperationID                 │  │     │
│  │  │  - Parameters (path/query/header/cookie)     │  │     │
│  │  │  - RequestBody (schema + content types)      │  │     │
│  │  │  - Responses (status → schema + content)     │  │     │
│  │  │  - SchemaDefinitions ($defs)                 │  │     │
│  │  │  - Tags, Extensions                          │  │     │
│  │  └──────────────────────────────────────────────┘  │     │
│  └────────────────────────────────────────────────────┘     │
│                           │                                  │
│                           ▼                                  │
│  ┌────────────────────────────────────────────────────┐     │
│  │              Route Mapping Layer                    │     │
│  │  ┌──────────────────────────────────────────────┐  │     │
│  │  │ RouteMapper                                   │  │     │
│  │  │  - MapRoute(route) → MCPType                 │  │     │
│  │  │  - Matches: Method, PathPattern, Tags        │  │     │
│  │  └──────────────────────────────────────────────┘  │     │
│  │                                                      │     │
│  │  RouteMap:                                          │     │
│  │    - Methods: []string | "*"                        │     │
│  │    - PathPattern: regexp                            │     │
│  │    - Tags: []string (AND logic)                     │     │
│  │    - MCPType: Tool/Resource/ResourceTemplate/Exclude│     │
│  │    - MCPTags: []string (add to component)          │     │
│  └────────────────────────────────────────────────────┘     │
│                           │                                  │
│                           ▼                                  │
│  ┌────────────────────────────────────────────────────┐     │
│  │          Component Generation Layer                 │     │
│  │  ┌──────────────────────────────────────────────┐  │     │
│  │  │ ComponentFactory                              │  │     │
│  │  │  - CreateTool(route) → OpenAPITool           │  │     │
│  │  │  - CreateResource(route) → OpenAPIResource   │  │     │
│  │  │  - CreateTemplate(route) → OpenAPITemplate   │  │     │
│  │  │                                               │  │     │
│  │  │ Features:                                     │  │     │
│  │  │  - Schema combination (params + body)        │  │     │
│  │  │  - Collision detection & suffixing           │  │     │
│  │  │  - Output schema extraction & wrapping       │  │     │
│  │  │  - Name generation & deduplication           │  │     │
│  │  │  - Description enhancement                   │  │     │
│  │  └──────────────────────────────────────────────┘  │     │
│  └────────────────────────────────────────────────────┘     │
│                           │                                  │
│                           ▼                                  │
│  ┌────────────────────────────────────────────────────┐     │
│  │          Request Execution Layer                    │     │
│  │  ┌──────────────────────────────────────────────┐  │     │
│  │  │ RequestBuilder                                │  │     │
│  │  │  - BuildRequest(route, args) → *http.Request│  │     │
│  │  │                                               │  │     │
│  │  │ Process:                                      │  │     │
│  │  │  1. Unflatten arguments via param map        │  │     │
│  │  │  2. Substitute path parameters               │  │     │
│  │  │  3. Format query params (style/explode)      │  │     │
│  │  │  4. Build headers (params + MCP + client)    │  │     │
│  │  │  5. Serialize request body                   │  │     │
│  │  └──────────────────────────────────────────────┘  │     │
│  │                                                      │     │
│  │  ┌──────────────────────────────────────────────┐  │     │
│  │  │ HTTPClient (interface)                        │  │     │
│  │  │  - Do(req) → (*http.Response, error)        │  │     │
│  │  └──────────────────────────────────────────────┘  │     │
│  │                                                      │     │
│  │  ┌──────────────────────────────────────────────┐  │     │
│  │  │ ResponseProcessor                             │  │     │
│  │  │  - ProcessResponse(resp, schema) → Result    │  │     │
│  │  │                                               │  │     │
│  │  │ Logic:                                        │  │     │
│  │  │  - JSON parsing with wrapping detection      │  │     │
│  │  │  - Text/binary fallback                      │  │     │
│  │  │  - Error extraction from responses           │  │     │
│  │  └──────────────────────────────────────────────┘  │     │
│  └────────────────────────────────────────────────────┘     │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      mcp-go Framework                        │
│  ┌────────────────────────────────────────────────────┐     │
│  │ MCPServer                                           │     │
│  │  - AddTool(tool, handler)                          │     │
│  │  - AddResource(resource, handler)                  │     │
│  │  - AddResourceTemplate(template, handler)          │     │
│  │  - Serve(transport)                                │     │
│  └────────────────────────────────────────────────────┘     │
│                                                              │
│  ┌────────────────────────────────────────────────────┐     │
│  │ Transport Layer                                     │     │
│  │  - STDIO, HTTP/SSE, In-Process                     │     │
│  └────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Data Flow

```
OpenAPI Spec (JSON/YAML)
    ↓
[Parser] Parse & validate spec
    ↓
HTTPRoute[] (Intermediate Representation)
    ↓
[Mapper] Apply route mapping rules
    ↓
Mapped Routes (with MCPType)
    ↓
[Factory] Generate MCP components
    ↓
MCP Components (Tool/Resource/ResourceTemplate)
    ↓
[Registration] Register with mcp-go server
    ↓
MCP Server Ready
    ↓
[Runtime] Handle tool calls
    ↓
[Builder] Build HTTP request from args
    ↓
[Client] Execute HTTP request
    ↓
[Processor] Process HTTP response
    ↓
MCP ToolResult/ResourceContent
```

---

## 3. Core Components

### 3.1 OpenAPI Parsing Layer

#### 3.1.1 OpenAPIParser Interface

```go
type OpenAPIParser interface {
    // Parse OpenAPI spec into intermediate representation
    ParseSpec(spec []byte) ([]HTTPRoute, error)

    // Resolve $ref references
    ResolveReference(ref string) (Schema, error)

    // Get OpenAPI version
    GetVersion() string

    // Validate spec
    Validate() error
}
```

#### 3.1.2 HTTPRoute (Intermediate Representation)

```go
type HTTPRoute struct {
    Path            string
    Method          string
    OperationID     string
    Summary         string
    Description     string
    Tags            []string
    Parameters      []ParameterInfo
    RequestBody     *RequestBodyInfo
    Responses       map[string]ResponseInfo
    SchemaDefs      map[string]Schema
    Extensions      map[string]interface{}
    OpenAPIVersion  string

    // Generated during schema combination
    ParameterMap    map[string]ParamMapping
}

type ParameterInfo struct {
    Name        string
    In          string // "path", "query", "header", "cookie"
    Required    bool
    Schema      Schema
    Description string
    Explode     *bool
    Style       string // "simple", "form", "deepObject", etc.
}

type RequestBodyInfo struct {
    Required       bool
    ContentSchemas map[string]Schema // media_type -> schema
    Description    string
}

type ResponseInfo struct {
    Description    string
    ContentSchemas map[string]Schema // media_type -> schema
}

type ParamMapping struct {
    OpenAPIName string
    Location    string // "path", "query", "header", "body"
    IsSuffixed  bool
}

type Schema map[string]interface{} // JSON Schema
```

#### 3.1.3 Parser Implementations

**OpenAPI30Parser**: Handles OpenAPI 3.0.x
- Uses `nullable: true` for nullable fields
- Converts to JSON Schema `anyOf` pattern

**OpenAPI31Parser**: Handles OpenAPI 3.1.x
- Native JSON Schema support
- Uses `type: ["string", "null"]` for nullables

**Library Choice**: `github.com/pb33f/libopenapi` (comprehensive, supports 3.0 & 3.1)

### 3.2 Route Mapping Layer

#### 3.2.1 MCPType Enum

```go
type MCPType string

const (
    MCPTypeTool             MCPType = "tool"
    MCPTypeResource         MCPType = "resource"
    MCPTypeResourceTemplate MCPType = "resource_template"
    MCPTypeExclude          MCPType = "exclude"
)
```

#### 3.2.2 RouteMap Configuration

```go
type RouteMap struct {
    // HTTP methods to match ("*" for all)
    Methods []string

    // Regex pattern for path matching
    PathPattern *regexp.Regexp

    // Tags to match (AND logic - all must be present)
    Tags []string

    // MCP type to assign
    MCPType MCPType

    // Tags to add to generated component
    MCPTags []string
}
```

#### 3.2.3 RouteMapper

```go
type RouteMapper struct {
    routeMaps []RouteMap
    mapFunc   RouteMapFunc // Optional custom function
}

type RouteMapFunc func(route HTTPRoute, mappedType MCPType) *MCPType

func (rm *RouteMapper) MapRoute(route HTTPRoute) MCPType {
    // 1. Find first matching RouteMap
    for _, mapping := range rm.routeMaps {
        if rm.matches(route, mapping) {
            mappedType := mapping.MCPType

            // 2. Allow custom function to override
            if rm.mapFunc != nil {
                if override := rm.mapFunc(route, mappedType); override != nil {
                    mappedType = *override
                }
            }

            return mappedType
        }
    }

    // 3. Default to Tool
    return MCPTypeTool
}

func (rm *RouteMapper) matches(route HTTPRoute, mapping RouteMap) bool {
    // Check method
    if !containsOrWildcard(mapping.Methods, route.Method) {
        return false
    }

    // Check path pattern
    if !mapping.PathPattern.MatchString(route.Path) {
        return false
    }

    // Check tags (AND logic)
    if len(mapping.Tags) > 0 {
        routeTags := stringSet(route.Tags)
        for _, tag := range mapping.Tags {
            if !routeTags[tag] {
                return false
            }
        }
    }

    return true
}
```

### 3.3 Component Generation Layer

#### 3.3.1 ComponentFactory

```go
type ComponentFactory struct {
    client      HTTPClient
    nameCounter map[string]map[string]int // component_type -> name -> count
    customNames map[string]string         // operation_id -> custom_name
    componentFn ComponentFunc             // Optional customization
}

type ComponentFunc func(route HTTPRoute, component interface{})

func (cf *ComponentFactory) CreateTool(route HTTPRoute) (*OpenAPITool, error) {
    // 1. Combine schemas (parameters + body)
    inputSchema, paramMap, err := cf.combineSchemas(route)
    if err != nil {
        return nil, err
    }

    // 2. Extract output schema
    outputSchema, wrapResult := cf.extractOutputSchema(route)

    // 3. Generate unique name
    name := cf.generateName(route, "tool")

    // 4. Enhance description
    description := cf.formatDescription(route)

    // 5. Create OpenAPITool
    tool := &OpenAPITool{
        Tool: mcp.NewTool(name,
            mcp.WithDescription(description),
            mcp.WithString("inputSchema", inputSchema),
        ),
        route:        route,
        client:       cf.client,
        paramMap:     paramMap,
        outputSchema: outputSchema,
        wrapResult:   wrapResult,
    }

    // 6. Apply customization
    if cf.componentFn != nil {
        cf.componentFn(route, tool)
    }

    return tool, nil
}

func (cf *ComponentFactory) CreateResource(route HTTPRoute) (*OpenAPIResource, error) {
    // Similar pattern for static resources
}

func (cf *ComponentFactory) CreateResourceTemplate(route HTTPRoute) (*OpenAPIResourceTemplate, error) {
    // Similar pattern for parameterized resources
}
```

#### 3.3.2 Schema Combination Logic

```go
func (cf *ComponentFactory) combineSchemas(route HTTPRoute) (Schema, map[string]ParamMapping, error) {
    schema := Schema{
        "type":       "object",
        "properties": make(map[string]interface{}),
    }

    required := []string{}
    paramMap := make(map[string]ParamMapping)
    allNames := make(map[string]string) // name -> location

    // 1. Collect all parameter names
    for _, param := range route.Parameters {
        allNames[param.Name] = param.In
    }

    // 2. Check for collisions with request body
    bodyProps := make(map[string]bool)
    if route.RequestBody != nil {
        for _, schema := range route.RequestBody.ContentSchemas {
            if props, ok := schema["properties"].(map[string]interface{}); ok {
                for propName := range props {
                    bodyProps[propName] = true
                }
            }
        }
    }

    // 3. Add parameters with collision handling
    for _, param := range route.Parameters {
        paramName := param.Name

        // Detect collision
        if bodyProps[param.Name] {
            paramName = fmt.Sprintf("%s__%s", param.Name, param.In)
        }

        // Add to schema
        schema["properties"].(map[string]interface{})[paramName] = param.Schema

        if param.Required {
            required = append(required, paramName)
        }

        // Store mapping
        paramMap[paramName] = ParamMapping{
            OpenAPIName: param.Name,
            Location:    param.In,
            IsSuffixed:  paramName != param.Name,
        }
    }

    // 4. Add request body properties
    if route.RequestBody != nil {
        // Prefer application/json
        contentType := selectContentType(route.RequestBody.ContentSchemas)
        bodySchema := route.RequestBody.ContentSchemas[contentType]

        if props, ok := bodySchema["properties"].(map[string]interface{}); ok {
            for propName, propSchema := range props {
                schema["properties"].(map[string]interface{})[propName] = propSchema

                paramMap[propName] = ParamMapping{
                    OpenAPIName: propName,
                    Location:    "body",
                    IsSuffixed:  false,
                }
            }
        }

        if reqFields, ok := bodySchema["required"].([]string); ok {
            required = append(required, reqFields...)
        }
    }

    if len(required) > 0 {
        schema["required"] = required
    }

    // 5. Add schema definitions
    if len(route.SchemaDefs) > 0 {
        schema["$defs"] = route.SchemaDefs
    }

    return schema, paramMap, nil
}
```

#### 3.3.3 Output Schema Extraction

```go
func (cf *ComponentFactory) extractOutputSchema(route HTTPRoute) (Schema, bool) {
    // 1. Find success response
    var response ResponseInfo
    for _, status := range []string{"200", "201", "202", "204"} {
        if resp, ok := route.Responses[status]; ok {
            response = resp
            break
        }
    }

    if response.ContentSchemas == nil {
        return nil, false
    }

    // 2. Select content type (prefer JSON)
    contentType := selectContentType(response.ContentSchemas)
    schema := response.ContentSchemas[contentType]

    // 3. Check if wrapping needed
    wrapResult := false
    if schema["type"] != "object" {
        schema = Schema{
            "type": "object",
            "properties": map[string]interface{}{
                "result": schema,
            },
            "required": []string{"result"},
        }
        wrapResult = true
    }

    // 4. Add schema definitions
    if len(route.SchemaDefs) > 0 {
        schema["$defs"] = route.SchemaDefs
    }

    return schema, wrapResult
}
```

#### 3.3.4 Name Generation

```go
func (cf *ComponentFactory) generateName(route HTTPRoute, componentType string) string {
    var baseName string

    // 1. Check custom mapping
    if name, ok := cf.customNames[route.OperationID]; ok {
        baseName = name
    } else if route.OperationID != "" {
        // 2. Use operation ID (up to __)
        parts := strings.Split(route.OperationID, "__")
        baseName = parts[0]
    } else if route.Summary != "" {
        // 3. Slugify summary
        baseName = slugify(route.Summary)
    } else {
        // 4. Fallback: method_path
        baseName = fmt.Sprintf("%s_%s",
            strings.ToLower(route.Method),
            slugify(route.Path))
    }

    // 5. Truncate to 56 chars
    if len(baseName) > 56 {
        baseName = baseName[:56]
    }

    // 6. Deduplicate
    if cf.nameCounter[componentType] == nil {
        cf.nameCounter[componentType] = make(map[string]int)
    }

    cf.nameCounter[componentType][baseName]++
    count := cf.nameCounter[componentType][baseName]

    if count == 1 {
        return baseName
    }
    return fmt.Sprintf("%s_%d", baseName, count)
}

func slugify(text string) string {
    // Replace spaces/separators with underscores
    slug := regexp.MustCompile(`[\s\-\.]+`).ReplaceAllString(text, "_")
    // Remove non-alphanumeric except underscores
    slug = regexp.MustCompile(`[^a-zA-Z0-9_]`).ReplaceAllString(slug, "")
    // Remove multiple underscores
    slug = regexp.MustCompile(`_+`).ReplaceAllString(slug, "_")
    // Trim underscores
    return strings.Trim(slug, "_")
}
```

### 3.4 Request Execution Layer

#### 3.4.1 OpenAPITool Implementation

```go
type OpenAPITool struct {
    *mcp.Tool
    route        HTTPRoute
    client       HTTPClient
    paramMap     map[string]ParamMapping
    outputSchema Schema
    wrapResult   bool
}

func (t *OpenAPITool) Run(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // 1. Parse arguments
    args, err := parseArguments(request)
    if err != nil {
        return nil, err
    }

    // 2. Build HTTP request
    builder := NewRequestBuilder(t.route, t.paramMap)
    httpReq, err := builder.Build(ctx, args)
    if err != nil {
        return nil, err
    }

    // 3. Add MCP headers (from context)
    if mcpHeaders := GetMCPHeaders(ctx); mcpHeaders != nil {
        for k, v := range mcpHeaders {
            httpReq.Header.Set(k, v)
        }
    }

    // 4. Execute request
    resp, err := t.client.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("request error: %w", err)
    }
    defer resp.Body.Close()

    // 5. Process response
    processor := NewResponseProcessor(t.outputSchema, t.wrapResult, errorHandler)
    return processor.Process(resp)
}
```

#### 3.4.2 RequestBuilder

```go
type RequestBuilder struct {
    route    HTTPRoute
    paramMap map[string]ParamMapping
}

func (rb *RequestBuilder) Build(ctx context.Context, args map[string]interface{}) (*http.Request, error) {
    // 1. Unflatten arguments
    pathParams := make(map[string]string)
    queryParams := url.Values{}
    headerParams := make(map[string]string)
    bodyParams := make(map[string]interface{})

    for argName, argValue := range args {
        mapping, ok := rb.paramMap[argName]
        if !ok {
            continue
        }

        switch mapping.Location {
        case "path":
            pathParams[mapping.OpenAPIName] = fmt.Sprintf("%v", argValue)
        case "query":
            rb.addQueryParam(queryParams, mapping.OpenAPIName, argValue)
        case "header":
            headerParams[mapping.OpenAPIName] = fmt.Sprintf("%v", argValue)
        case "body":
            bodyParams[mapping.OpenAPIName] = argValue
        }
    }

    // 2. Build URL with path substitution
    urlPath := rb.route.Path
    for paramName, paramValue := range pathParams {
        urlPath = strings.ReplaceAll(urlPath, fmt.Sprintf("{%s}", paramName), paramValue)
    }

    reqURL, err := url.Parse(urlPath)
    if err != nil {
        return nil, err
    }
    reqURL.RawQuery = queryParams.Encode()

    // 3. Build request body
    var bodyReader io.Reader
    if len(bodyParams) > 0 {
        bodyJSON, err := json.Marshal(bodyParams)
        if err != nil {
            return nil, err
        }
        bodyReader = bytes.NewReader(bodyJSON)
    }

    // 4. Create request
    req, err := http.NewRequestWithContext(ctx, rb.route.Method, reqURL.String(), bodyReader)
    if err != nil {
        return nil, err
    }

    // 5. Set headers
    if len(bodyParams) > 0 {
        req.Header.Set("Content-Type", "application/json")
    }
    for name, value := range headerParams {
        req.Header.Set(name, value)
    }

    return req, nil
}

func (rb *RequestBuilder) addQueryParam(params url.Values, name string, value interface{}) {
    // Find parameter info for style/explode handling
    var paramInfo *ParameterInfo
    for i := range rb.route.Parameters {
        if rb.route.Parameters[i].Name == name && rb.route.Parameters[i].In == "query" {
            paramInfo = &rb.route.Parameters[i]
            break
        }
    }

    // Handle different parameter styles
    switch v := value.(type) {
    case []interface{}:
        rb.addArrayParam(params, name, v, paramInfo)
    case map[string]interface{}:
        rb.addObjectParam(params, name, v, paramInfo)
    default:
        params.Add(name, fmt.Sprintf("%v", value))
    }
}

func (rb *RequestBuilder) addArrayParam(params url.Values, name string, values []interface{}, info *ParameterInfo) {
    explode := true
    if info != nil && info.Explode != nil {
        explode = *info.Explode
    }

    if explode {
        // ?id=1&id=2&id=3
        for _, v := range values {
            params.Add(name, fmt.Sprintf("%v", v))
        }
    } else {
        // ?id=1,2,3
        strValues := make([]string, len(values))
        for i, v := range values {
            strValues[i] = fmt.Sprintf("%v", v)
        }
        params.Add(name, strings.Join(strValues, ","))
    }
}

func (rb *RequestBuilder) addObjectParam(params url.Values, name string, obj map[string]interface{}, info *ParameterInfo) {
    if info != nil && info.Style == "deepObject" {
        // ?filter[name]=john&filter[age]=30
        for key, value := range obj {
            params.Add(fmt.Sprintf("%s[%s]", name, key), fmt.Sprintf("%v", value))
        }
    } else {
        // Default: form style (explode)
        for key, value := range obj {
            params.Add(key, fmt.Sprintf("%v", value))
        }
    }
}
```

#### 3.4.3 ResponseProcessor

```go
type ResponseProcessor struct {
    outputSchema Schema
    wrapResult   bool
}

func (rp *ResponseProcessor) Process(resp *http.Response) (*mcp.CallToolResult, error) {
    // 1. Check status code
    if resp.StatusCode >= 400 {
        return rp.processError(resp)
    }

    // 2. Try JSON parsing
    var result interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
        return rp.processJSON(result)
    }

    // 3. Fallback to text
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            mcp.NewTextContent(string(body)),
        },
    }, nil
}

func (rp *ResponseProcessor) processJSON(result interface{}) (*mcp.CallToolResult, error) {
    // Check if wrapping needed
    if rp.wrapResult {
        return &mcp.CallToolResult{
            StructuredContent: map[string]interface{}{
                "result": result,
            },
        }, nil
    }

    // MCP requires object type for structured content
    if resultMap, ok := result.(map[string]interface{}); ok {
        return &mcp.CallToolResult{
            StructuredContent: resultMap,
        }, nil
    }

    // Wrap non-object results
    return &mcp.CallToolResult{
        StructuredContent: map[string]interface{}{
            "result": result,
        },
    }, nil
}

func (rp *ResponseProcessor) processError(resp *http.Response) (*mcp.CallToolResult, error) {
    var errorMsg string

    // Try to parse error as JSON
    var errorData map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&errorData); err == nil {
        errorJSON, _ := json.Marshal(errorData)
        errorMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(errorJSON))
    } else {
        body, _ := io.ReadAll(resp.Body)
        errorMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
    }

    return &mcp.CallToolResult{
        IsError: true,
        Content: []mcp.Content{
            mcp.NewTextContent(errorMsg),
        },
    }, nil
}
```

---

## 4. Package Structure

```
openapi-mcp/
├── go.mod
├── go.sum
├── README.md
├── docs/
│   ├── ARCHITECTURE.md (this file)
│   ├── DESIGN_PARSER.md
│   ├── DESIGN_MAPPER.md
│   ├── DESIGN_FACTORY.md
│   ├── DESIGN_EXECUTOR.md
│   └── examples/
│       ├── basic_usage.md
│       ├── route_mapping.md
│       └── customization.md
├── pkg/
│   └── openapimcp/
│       ├── server.go              // Main entry point
│       ├── options.go             // Configuration options
│       ├── parser/
│       │   ├── parser.go          // OpenAPIParser interface
│       │   ├── openapi30.go       // OpenAPI 3.0 parser
│       │   ├── openapi31.go       // OpenAPI 3.1 parser
│       │   ├── schema.go          // Schema utilities
│       │   └── resolver.go        // Reference resolution
│       ├── ir/
│       │   ├── route.go           // HTTPRoute and related types
│       │   ├── parameter.go       // ParameterInfo
│       │   ├── request.go         // RequestBodyInfo
│       │   └── response.go        // ResponseInfo
│       ├── mapper/
│       │   ├── mapper.go          // RouteMapper
│       │   ├── types.go           // MCPType, RouteMap
│       │   └── defaults.go        // Default mappings
│       ├── factory/
│       │   ├── factory.go         // ComponentFactory
│       │   ├── schema.go          // Schema combination logic
│       │   ├── naming.go          // Name generation
│       │   └── description.go     // Description formatting
│       ├── executor/
│       │   ├── tool.go            // OpenAPITool
│       │   ├── resource.go        // OpenAPIResource
│       │   ├── template.go        // OpenAPIResourceTemplate
│       │   ├── builder.go         // RequestBuilder
│       │   ├── processor.go       // ResponseProcessor
│       │   └── client.go          // HTTPClient interface
│       └── internal/
│           ├── context.go         // MCP context utilities
│           └── utils.go           // Common utilities
├── examples/
│   ├── basic/
│   │   └── main.go
│   ├── petstore/
│   │   └── main.go
│   └── custom_mapping/
│       └── main.go
└── test/
    ├── parser_test.go
    ├── mapper_test.go
    ├── factory_test.go
    ├── executor_test.go
    └── testdata/
        ├── petstore.json
        └── github.json
```

---

## 5. Usage Examples

### 5.1 Basic Usage

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/yourusername/openapi-mcp/pkg/openapimcp"
    "github.com/mark3labs/mcp-go/server"
)

func main() {
    // Load OpenAPI spec
    specData, err := os.ReadFile("petstore.json")
    if err != nil {
        log.Fatal(err)
    }

    // Create OpenAPI MCP server
    mcpServer, err := openapimcp.NewServer(specData)
    if err != nil {
        log.Fatal(err)
    }

    // Serve via STDIO
    if err := mcpServer.Serve(context.Background(), server.StdioTransport()); err != nil {
        log.Fatal(err)
    }
}
```

### 5.2 Custom Route Mapping

```go
import (
    "regexp"
    "github.com/yourusername/openapi-mcp/pkg/openapimcp"
    "github.com/yourusername/openapi-mcp/pkg/openapimcp/mapper"
)

func main() {
    specData, _ := os.ReadFile("api.json")

    mcpServer, err := openapimcp.NewServer(specData,
        openapimcp.WithRouteMaps([]mapper.RouteMap{
            // GET with path params → ResourceTemplate
            {
                Methods:     []string{"GET"},
                PathPattern: regexp.MustCompile(`.*\{.*\}.*`),
                MCPType:     mapper.MCPTypeResourceTemplate,
            },
            // GET without path params → Resource
            {
                Methods:     []string{"GET"},
                PathPattern: regexp.MustCompile(`.*`),
                MCPType:     mapper.MCPTypeResource,
            },
            // Admin routes → Exclude
            {
                Methods:     []string{"*"},
                PathPattern: regexp.MustCompile(`^/admin/.*`),
                MCPType:     mapper.MCPTypeExclude,
            },
        }),
    )

    // ...
}
```

### 5.3 Component Customization

```go
import (
    "github.com/yourusername/openapi-mcp/pkg/openapimcp"
    "github.com/yourusername/openapi-mcp/pkg/openapimcp/factory"
    "github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
    "github.com/mark3labs/mcp-go/mcp"
)

func main() {
    specData, _ := os.ReadFile("api.json")

    mcpServer, err := openapimcp.NewServer(specData,
        openapimcp.WithComponentFunc(func(route ir.HTTPRoute, component interface{}) {
            // Add cost annotation to admin operations
            if hasTag(route.Tags, "admin") {
                if tool, ok := component.(*executor.OpenAPITool); ok {
                    tool.Tool = tool.Tool.WithAnnotations(mcp.ToolAnnotations{
                        Cost: mcp.Float64Ptr(100.0),
                    })
                }
            }

            // Add custom description
            if tool, ok := component.(*executor.OpenAPITool); ok {
                tool.Tool = tool.Tool.WithDescription(
                    tool.Tool.Description + "\n\n⚠️ This operation requires authentication",
                )
            }
        }),
    )

    // ...
}
```

### 5.4 Custom HTTP Client

```go
import (
    "net/http"
    "time"

    "github.com/yourusername/openapi-mcp/pkg/openapimcp"
    "github.com/yourusername/openapi-mcp/pkg/openapimcp/executor"
)

func main() {
    // Custom HTTP client with timeout and auth
    httpClient := &http.Client{
        Timeout: 30 * time.Second,
        Transport: &authTransport{
            apiKey: "your-api-key",
            base:   http.DefaultTransport,
        },
    }

    specData, _ := os.ReadFile("api.json")

    mcpServer, err := openapimcp.NewServer(specData,
        openapimcp.WithHTTPClient(httpClient),
        openapimcp.WithBaseURL("https://api.example.com"),
    )

    // ...
}

type authTransport struct {
    apiKey string
    base   http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    req.Header.Set("Authorization", "Bearer "+t.apiKey)
    return t.base.RoundTrip(req)
}
```

---

## 6. Configuration Options

```go
type ServerOptions struct {
    // HTTP client for executing requests
    HTTPClient executor.HTTPClient

    // Base URL for API (overrides OpenAPI servers)
    BaseURL string

    // Route mapping configuration
    RouteMaps []mapper.RouteMap

    // Custom route mapping function
    RouteMapFunc mapper.RouteMapFunc

    // Custom component names (operationId -> name)
    CustomNames map[string]string

    // Component customization function
    ComponentFunc factory.ComponentFunc

    // OpenAPI parser (default: auto-detect)
    Parser parser.OpenAPIParser

    // Server info
    ServerName    string
    ServerVersion string
}

// Option functions
func WithHTTPClient(client executor.HTTPClient) ServerOption
func WithBaseURL(url string) ServerOption
func WithRouteMaps(maps []mapper.RouteMap) ServerOption
func WithRouteMapFunc(fn mapper.RouteMapFunc) ServerOption
func WithCustomNames(names map[string]string) ServerOption
func WithComponentFunc(fn factory.ComponentFunc) ServerOption
func WithParser(parser parser.OpenAPIParser) ServerOption
func WithServerInfo(name, version string) ServerOption
```

---

## 7. Testing Strategy

### 7.1 Unit Tests

- **Parser Layer**: Test OpenAPI 3.0/3.1 parsing, reference resolution, schema conversion
- **Mapper Layer**: Test route matching with various patterns, methods, tags
- **Factory Layer**: Test schema combination, collision detection, name generation
- **Executor Layer**: Test request building, parameter serialization, response processing

### 7.2 Integration Tests

- **Full Pipeline**: OpenAPI spec → MCP components → Request execution
- **Edge Cases**: Complex parameters, nested objects, array handling
- **Error Scenarios**: Invalid specs, network errors, API errors

### 7.3 Test Data

Use real-world OpenAPI specs:
- Petstore (official OpenAPI example)
- GitHub API
- Stripe API
- Custom test cases for edge scenarios

---

## 8. Implementation Phases

### Phase 1: Core Infrastructure (Week 1)
- ✅ Project setup and dependency management
- ✅ Define core interfaces and types
- ✅ Implement HTTPRoute IR
- ✅ Basic OpenAPI 3.0 parser
- ✅ Unit tests for parser

### Phase 2: Route Mapping (Week 2)
- ✅ Implement RouteMapper
- ✅ Route matching logic (method, pattern, tags)
- ✅ Default and custom mapping rules
- ✅ Unit tests for mapper

### Phase 3: Component Generation (Week 3)
- ✅ Implement ComponentFactory
- ✅ Schema combination with collision detection
- ✅ Output schema extraction
- ✅ Name generation and deduplication
- ✅ Unit tests for factory

### Phase 4: Request Execution (Week 4)
- ✅ Implement RequestBuilder
- ✅ Parameter serialization (path, query, header, body)
- ✅ Style/explode handling
- ✅ ResponseProcessor
- ✅ Unit tests for executor

### Phase 5: Integration & Polish (Week 5)
- ✅ OpenAPITool/Resource/ResourceTemplate implementation
- ✅ MCP server integration
- ✅ End-to-end integration tests
- ✅ Examples and documentation
- ✅ Performance optimization

### Phase 6: Advanced Features (Week 6)
- ✅ OpenAPI 3.1 support
- ✅ Enhanced error handling
- ✅ Authentication/security schemes
- ✅ Content negotiation
- ✅ Schema optimization

---

## 9. Dependencies

### Required

- **mcp-go**: MCP protocol implementation
  - `github.com/mark3labs/mcp-go`

- **OpenAPI Parser**: Spec parsing and validation
  - `github.com/pb33f/libopenapi` (recommended)
  - Alternative: `github.com/getkin/kin-openapi`

- **JSON Schema**: Schema validation
  - `github.com/santhosh-tekuri/jsonschema/v6`

### Optional

- **HTTP Client**: Enhanced HTTP functionality
  - stdlib `net/http` (sufficient for most cases)
  - `github.com/hashicorp/go-retryablehttp` (for retry logic)

- **Testing**: Mock servers and assertions
  - `github.com/stretchr/testify`
  - `github.com/jarcoal/httpmock`

---

## 10. Design Decisions & Rationale

### 10.1 Two-Phase Architecture

**Decision**: Separate OpenAPI parsing from MCP component generation via HTTPRoute IR

**Rationale**:
- **Flexibility**: Easy to swap parser implementations (libopenapi vs kin-openapi)
- **Testability**: Each phase can be tested independently
- **Clarity**: Clear separation of concerns
- **Extensibility**: Can add transformations at IR level

### 10.2 Interface-Based Design

**Decision**: Use interfaces for Parser, HTTPClient, RouteMapper

**Rationale**:
- **Testability**: Easy to mock dependencies
- **Extensibility**: Users can provide custom implementations
- **Go Idiomatic**: Follows Go's composition over inheritance

### 10.3 Collision Handling via Suffixing

**Decision**: Suffix non-body parameters with `__{location}` on collision

**Rationale**:
- **Clarity**: Clear which parameter is which in schema
- **Reversibility**: Bidirectional mapping via ParamMapping
- **Compatibility**: Matches FastMCP behavior
- **Documentation**: Schema description indicates suffixing

### 10.4 Functional Options Pattern

**Decision**: Use functional options for server configuration

**Rationale**:
- **Extensibility**: Easy to add new options without breaking API
- **Clarity**: Self-documenting option names
- **Flexibility**: Optional configurations with sensible defaults
- **Go Idiomatic**: Common pattern in Go libraries

### 10.5 Schema Wrapping for Non-Objects

**Decision**: Wrap non-object outputs in `{"result": ...}` structure

**Rationale**:
- **MCP Requirement**: MCP expects object-type structured content
- **Transparency**: Mark with flag to unwrap if needed
- **Compatibility**: Matches FastMCP behavior

---

## 11. Future Enhancements

### 11.1 Short-term (Next 3 months)

- **OpenAPI 2.0 (Swagger) Support**: Extend parser to handle Swagger specs
- **WebSocket Support**: Handle WebSocket operations in OpenAPI
- **Async API Support**: Extend to AsyncAPI for event-driven APIs
- **Enhanced Validation**: Strict schema validation with detailed errors
- **Rate Limiting**: Built-in rate limiting for API calls

### 11.2 Long-term (6-12 months)

- **Code Generation**: Generate Go client code from OpenAPI specs
- **Mock Server**: Auto-generate mock servers from specs
- **API Versioning**: Handle multiple API versions gracefully
- **GraphQL Support**: Convert GraphQL schemas to MCP
- **Plugin System**: Allow third-party plugins for custom behavior

---

## 12. Success Metrics

### 12.1 Functional Completeness

- ✅ All FastMCP OpenAPI features replicated
- ✅ Support for OpenAPI 3.0 and 3.1
- ✅ Comprehensive parameter handling
- ✅ Proper error handling and validation

### 12.2 Code Quality

- ✅ >80% test coverage
- ✅ Clean, idiomatic Go code
- ✅ Comprehensive documentation
- ✅ Zero critical security issues

### 12.3 Performance

- ✅ Parse 100-endpoint spec in <100ms
- ✅ Handle 1000+ requests/sec with proper client
- ✅ Low memory footprint (<50MB for typical usage)

### 12.4 Usability

- ✅ Simple API for common use cases
- ✅ Extensive customization options
- ✅ Clear examples and tutorials
- ✅ Active community support

---

## 13. Conclusion

This architecture provides a solid foundation for building a comprehensive OpenAPI-to-MCP framework in Go. The design emphasizes:

- **Clarity**: Clean separation of concerns
- **Extensibility**: Interface-based design for customization
- **Compatibility**: Feature parity with FastMCP
- **Go Idioms**: Following Go best practices

The phased implementation plan ensures steady progress with testable milestones. The architecture is designed to be maintainable, testable, and extensible for future enhancements.
