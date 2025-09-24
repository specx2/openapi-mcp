package ir

type ParameterInfo struct {
	Name        string
	In          string
	Required    bool
	Schema      Schema
	Description string
	Explode     *bool
	Style       string
}

const (
	ParameterInPath   = "path"
	ParameterInQuery  = "query"
	ParameterInHeader = "header"
	ParameterInCookie = "cookie"
)