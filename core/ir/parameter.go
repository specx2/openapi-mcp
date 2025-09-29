package ir

type ParameterInfo struct {
	Name            string
	In              string
	Required        bool
	Schema          Schema
	Description     string
	Explode         *bool
	Style           string
	AllowReserved   bool
	Deprecated      bool
	AllowEmptyValue bool
	Example         interface{}
	Examples        map[string]interface{}
	Extensions      map[string]interface{}
}

const (
	ParameterInPath   = "path"
	ParameterInQuery  = "query"
	ParameterInHeader = "header"
	ParameterInCookie = "cookie"
)
