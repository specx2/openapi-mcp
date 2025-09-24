package ir

type RequestBodyInfo struct {
	Required       bool
	ContentSchemas map[string]Schema
	Encodings      map[string]map[string]EncodingInfo
	Description    string
}

type EncodingInfo struct {
	ContentType   string
	Style         string
	Explode       *bool
	AllowReserved bool
}
