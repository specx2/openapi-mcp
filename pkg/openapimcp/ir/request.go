package ir

type RequestBodyInfo struct {
	Required       bool
	ContentSchemas map[string]Schema
	Description    string
}