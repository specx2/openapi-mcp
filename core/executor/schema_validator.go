package executor

import (
	"bytes"
	"encoding/json"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/specx2/openapi-mcp/core/ir"
)

func compileJSONSchema(raw json.RawMessage) *jsonschema.Schema {
	if raw == nil {
		return nil
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", bytes.NewReader(raw)); err != nil {
		return nil
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil
	}
	return schema
}

func compileIRSchema(schema ir.Schema) *jsonschema.Schema {
	if schema == nil {
		return nil
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	return compileJSONSchema(data)
}
