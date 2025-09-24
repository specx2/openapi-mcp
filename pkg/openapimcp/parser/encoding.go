package parser

import (
	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"

	"github.com/yourusername/openapi-mcp/pkg/openapimcp/ir"
)

func convertEncodings(encodings *orderedmap.Map[string, *v3.Encoding]) map[string]ir.EncodingInfo {
	if encodings == nil {
		return nil
	}

	result := make(map[string]ir.EncodingInfo)
	for name, encoding := range encodings.FromOldest() {
		if encoding == nil {
			continue
		}

		info := ir.EncodingInfo{
			ContentType:   encoding.ContentType,
			Style:         encoding.Style,
			AllowReserved: encoding.AllowReserved,
		}
		if encoding.Explode != nil {
			explode := *encoding.Explode
			info.Explode = &explode
		}

		result[name] = info
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
