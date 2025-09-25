package ir

type HTTPRoute struct {
	Path           string
	Method         string
	OperationID    string
	Summary        string
	Description    string
	Tags           []string
	Parameters     []ParameterInfo
	RequestBody    *RequestBodyInfo
	Responses      map[string]ResponseInfo
	SchemaDefs     Schema
	Extensions     map[string]interface{}
	OpenAPIVersion string
	ParameterMap   map[string]ParamMapping
	Callbacks      []CallbackInfo
}

type ParamMapping struct {
	OpenAPIName  string
	Location     string
	IsSuffixed   bool
	OriginalName string // 原始参数名
}
