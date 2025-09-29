package parser

import (
	"encoding/json"

	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"

	"github.com/specx2/openapi-mcp/core/ir"
)

func convertEncodings(encodings *orderedmap.Map[string, *v3.Encoding], isOpenAPI30 bool) map[string]ir.EncodingInfo {
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
		if headers := convertEncodingHeaders(encoding.Headers, isOpenAPI30); len(headers) > 0 {
			info.Headers = headers
		}

		result[name] = info
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func convertEncodingHeaders(headers *orderedmap.Map[string, *v3.Header], isOpenAPI30 bool) map[string]ir.HeaderInfo {
	if headers == nil || headers.Len() == 0 {
		return nil
	}

	result := make(map[string]ir.HeaderInfo)
	for headerName, header := range headers.FromOldest() {
		if header == nil {
			continue
		}

		headerInfo := ir.HeaderInfo{
			Name:            headerName,
			Description:     header.Description,
			Required:        header.Required,
			Deprecated:      header.Deprecated,
			AllowEmptyValue: header.AllowEmptyValue,
			Style:           header.Style,
			Explode:         header.Explode,
			AllowReserved:   header.AllowReserved,
		}

		if header.Schema != nil {
			if schema := header.Schema.Schema(); schema != nil {
				var schemaMap map[string]interface{}
				data, err := json.Marshal(schema)
				if err == nil {
					if err := json.Unmarshal(data, &schemaMap); err == nil {
						headerInfo.Schema = ConvertToJSONSchema(schemaMap, isOpenAPI30)
					}
				}
			}
		}

		if header.Example != nil {
			headerInfo.Example = extractExampleValue(header.Example)
		}

		if examples := convertExamplesMap(header.Examples); len(examples) > 0 {
			headerInfo.Examples = examples
		}

		if extensions := convertExtensionsMap(header.Extensions); len(extensions) > 0 {
			headerInfo.Extensions = extensions
		}

		result[headerName] = headerInfo
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
