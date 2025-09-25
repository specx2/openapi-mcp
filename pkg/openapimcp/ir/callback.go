package ir

type CallbackInfo struct {
	Name       string
	Expression string
	Operations []CallbackOperation
	Extensions map[string]interface{}
}

type CallbackOperation struct {
	Method      string
	Summary     string
	Description string
	RequestBody *RequestBodyInfo
	Responses   map[string]ResponseInfo
	Extensions  map[string]interface{}
}
