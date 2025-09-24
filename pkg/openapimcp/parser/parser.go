package parser

import (
	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

type OpenAPIParser interface {
	ParseSpec(spec []byte) ([]ir.HTTPRoute, error)
	ResolveReference(ref string) (ir.Schema, error)
	GetVersion() string
	Validate() error
}

type ParseError struct {
	Message string
	Path    string
	Line    int
	Column  int
}

func (e ParseError) Error() string {
	if e.Path != "" {
		return e.Path + ": " + e.Message
	}
	return e.Message
}