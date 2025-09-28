package factory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/specx2/openapi-mcp/pkg/openapimcp/ir"
)

func (cf *ComponentFactory) formatDescription(route ir.HTTPRoute) string {
	var parts []string

	if route.Description != "" {
		parts = append(parts, route.Description)
	} else if route.Summary != "" {
		parts = append(parts, route.Summary)
	} else {
		parts = append(parts, fmt.Sprintf("%s %s", route.Method, route.Path))
	}

	if paramSection := formatParameterSection(route.Parameters); paramSection != "" {
		parts = append(parts, paramSection)
	}

	if route.RequestBody != nil && route.RequestBody.Description != "" {
		parts = append(parts, fmt.Sprintf("**Request Body:** %s", route.RequestBody.Description))
	}

	if route.RequestBody != nil {
		if summary := summarizeRequestBodyText(*route.RequestBody); summary != "" {
			parts = append(parts, "**Request Body Schema:** "+summary)
		}
		if ext := summarizeExtensions(route.RequestBody.Extensions); ext != "" {
			parts = append(parts, "**Request Body Extensions:** "+ext)
		}
	}

	if extSummary := summarizeExtensions(route.Extensions); extSummary != "" {
		parts = append(parts, "**Extensions:** "+extSummary)
	}

	if len(route.Responses) > 0 {
		if responseSection := cf.formatResponseSection(route); responseSection != "" {
			parts = append(parts, responseSection)
		}
	}

	if len(route.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("**Tags:** %s", strings.Join(route.Tags, ", ")))
	}

	if callbackSection := formatCallbacks(route.Callbacks); callbackSection != "" {
		parts = append(parts, callbackSection)
	}

	return strings.Join(parts, "\n\n")
}

func formatParameterSection(params []ir.ParameterInfo) string {
	if len(params) == 0 {
		return ""
	}

	grouped := map[string][]string{
		ir.ParameterInPath:   nil,
		ir.ParameterInQuery:  nil,
		ir.ParameterInHeader: nil,
		ir.ParameterInCookie: nil,
	}

	for _, param := range params {
		line := formatParameterLine(param)
		if line == "" {
			continue
		}
		grouped[param.In] = append(grouped[param.In], line)
	}

	var sections []string
	if lines := grouped[ir.ParameterInPath]; len(lines) > 0 {
		sections = append(sections, "**Path Parameters:**\n"+strings.Join(lines, "\n"))
	}
	if lines := grouped[ir.ParameterInQuery]; len(lines) > 0 {
		sections = append(sections, "**Query Parameters:**\n"+strings.Join(lines, "\n"))
	}
	if lines := grouped[ir.ParameterInHeader]; len(lines) > 0 {
		sections = append(sections, "**Header Parameters:**\n"+strings.Join(lines, "\n"))
	}
	if lines := grouped[ir.ParameterInCookie]; len(lines) > 0 {
		sections = append(sections, "**Cookie Parameters:**\n"+strings.Join(lines, "\n"))
	}

	if len(sections) == 0 {
		return ""
	}

	return strings.Join(sections, "\n\n")
}

func formatParameterLine(param ir.ParameterInfo) string {
	if param.Name == "" {
		return ""
	}
	label := fmt.Sprintf("- %s", param.Name)
	if param.Required {
		label += " (required)"
	}

	var details []string
	if desc := strings.TrimSpace(param.Description); desc != "" {
		details = append(details, desc)
	}

	if summary, schemaType := summarizeSchema(param.Schema); summary != "" {
		details = append(details, summary)
	} else if schemaType != "" {
		details = append(details, fmt.Sprintf("type: %s", schemaType))
	}

	if param.Schema != nil {
		if def, ok := param.Schema["default"]; ok {
			if formatted := formatExample(def); formatted != "" {
				details = append(details, "default: "+formatted)
			}
		}
	}

	if param.Example != nil {
		if formatted := formatExample(param.Example); formatted != "" {
			details = append(details, "example: "+formatted)
		}
	}

	if len(param.Examples) > 0 {
		keys := make([]string, 0, len(param.Examples))
		for name := range param.Examples {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		if sample := param.Examples[keys[0]]; sample != nil {
			if formatted := formatExample(sample); formatted != "" {
				details = append(details, "example: "+formatted)
			}
		}
	}

	var styleHints []string
	if param.Style != "" {
		styleHints = append(styleHints, fmt.Sprintf("style=%s", param.Style))
	}
	if param.Explode != nil {
		styleHints = append(styleHints, fmt.Sprintf("explode=%t", *param.Explode))
	}
	if param.AllowReserved {
		styleHints = append(styleHints, "allowReserved")
	}
	if param.AllowEmptyValue {
		styleHints = append(styleHints, "allowEmptyValue")
	}
	if param.Deprecated {
		styleHints = append(styleHints, "deprecated")
	}
	if len(styleHints) > 0 {
		details = append(details, strings.Join(styleHints, ", "))
	}

	if ext := summarizeExtensions(param.Extensions); ext != "" {
		details = append(details, "extensions: "+ext)
	}

	if len(details) > 0 {
		label += ": " + strings.Join(details, "; ")
	}

	return label
}

func (cf *ComponentFactory) formatResponseSection(route ir.HTTPRoute) string {
	if len(route.Responses) == 0 {
		return ""
	}

	statuses := make([]string, 0, len(route.Responses))
	for status := range route.Responses {
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		pi, oi := responsePriority(statuses[i])
		pj, oj := responsePriority(statuses[j])
		if pi != pj {
			return pi < pj
		}
		return oi < oj
	})

	var responseParts []string
	for _, status := range statuses {
		response := route.Responses[status]
		line := fmt.Sprintf("- %s", status)
		if response.Description != "" {
			line += fmt.Sprintf(": %s", response.Description)
		}
		if summary := summarizeResponseContent(response); summary != "" {
			if response.Description != "" {
				line += fmt.Sprintf(" %s", summary)
			} else {
				line += fmt.Sprintf(": %s", summary)
			}
		}
		if example := pickResponseExample(response); example != "" {
			line += fmt.Sprintf(" (Example: %s)", example)
		}
		if ext := summarizeExtensions(response.Extensions); ext != "" {
			line += fmt.Sprintf(" [Extensions: %s]", ext)
		}
		responseParts = append(responseParts, line)
	}

	if len(responseParts) == 0 {
		return ""
	}

	return "**Responses:**\n" + strings.Join(responseParts, "\n")
}

func responsePriority(status string) (int, int) {
	if status == "default" {
		return 4, 0
	}
	if code, err := strconv.Atoi(status); err == nil {
		switch {
		case code >= 200 && code < 300:
			return 0, code
		case code >= 100 && code < 200:
			return 1, code
		case code >= 300 && code < 400:
			return 2, code
		default:
			return 3, code
		}
	}
	return 5, 0
}

func summarizeResponseContent(resp ir.ResponseInfo) string {
	mediaType, schema := selectPreferredMedia(resp.ContentSchemas)
	if schema == nil {
		if mediaType != "" {
			return fmt.Sprintf("returns %s", mediaType)
		}
		return ""
	}

	summary, schemaType := summarizeSchema(schema)
	var builder strings.Builder
	if summary != "" {
		builder.WriteString("returns ")
		builder.WriteString(summary)
	} else if schemaType != "" {
		builder.WriteString("returns ")
		builder.WriteString(schemaType)
	}
	if builder.Len() == 0 {
		return ""
	}
	if mediaType != "" {
		builder.WriteString(fmt.Sprintf(" [%s]", mediaType))
	}
	return builder.String()
}

func summarizeRequestBodyText(body ir.RequestBodyInfo) string {
	mediaType, schema := selectPreferredMedia(body.ContentSchemas)
	if schema == nil {
		return ""
	}
	summary, schemaType := summarizeSchema(schema)
	text := summary
	if text == "" {
		text = schemaType
	}
	if text == "" {
		return ""
	}
	if mediaType != "" {
		text = fmt.Sprintf("%s [%s]", text, mediaType)
	}
	if example := pickExampleForMedia(mediaType, body.MediaExamples, body.MediaExampleSets); example != "" {
		text = fmt.Sprintf("%s (Example: %s)", text, example)
	}
	return text
}

func pickResponseExample(resp ir.ResponseInfo) string {
	mediaType, _ := selectPreferredMedia(resp.ContentSchemas)
	if mediaType == "" {
		for mt := range resp.ContentSchemas {
			mediaType = mt
			break
		}
	}
	if mediaType == "" {
		return ""
	}
	return pickExampleForMedia(mediaType, resp.MediaExamples, resp.MediaExampleSets)
}

func pickExampleForMedia(mediaType string, examples map[string]interface{}, exampleSets map[string]map[string]interface{}) string {
	if mediaType == "" {
		return ""
	}
	if examples != nil {
		if example, ok := examples[mediaType]; ok {
			if formatted := formatExample(example); formatted != "" {
				return formatted
			}
		}
	}
	if exampleSets != nil {
		if set, ok := exampleSets[mediaType]; ok {
			for _, example := range set {
				if formatted := formatExample(example); formatted != "" {
					return formatted
				}
			}
		}
	}
	return ""
}

func summarizeExtensions(ext map[string]interface{}) string {
	if len(ext) == 0 {
		return ""
	}
	keys := make([]string, 0, len(ext))
	for k := range ext {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		v := ext[k]
		formatted := formatExtensionValue(v)
		parts = append(parts, fmt.Sprintf("%s=%s", k, formatted))
	}
	return strings.Join(parts, ", ")
}

func formatExtensionValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		s := string(data)
		if len(s) > 120 {
			return s[:117] + "..."
		}
		return s
	}
}

func formatCallbacks(callbacks []ir.CallbackInfo) string {
	if len(callbacks) == 0 {
		return ""
	}
	var sections []string
	for _, cb := range callbacks {
		var lines []string
		for _, op := range cb.Operations {
			line := fmt.Sprintf("%s", op.Method)
			if op.Summary != "" {
				line += fmt.Sprintf(" - %s", op.Summary)
			} else if op.Description != "" {
				line += fmt.Sprintf(" - %s", op.Description)
			}
			if ext := summarizeExtensions(op.Extensions); ext != "" {
				line += fmt.Sprintf(" [%s]", ext)
			}
			lines = append(lines, line)
		}
		label := cb.Expression
		if cb.Name != "" {
			if label == "" {
				label = cb.Name
			} else if cb.Name != label {
				label = fmt.Sprintf("%s (%s)", cb.Name, label)
			}
		}
		if label == "" {
			label = "callback"
		}
		line := fmt.Sprintf("- %s", label)
		if len(lines) > 0 {
			line += "\n  " + strings.Join(lines, "\n  ")
		}
		if ext := summarizeExtensions(cb.Extensions); ext != "" {
			line += fmt.Sprintf("\n  Extensions: %s", ext)
		}
		sections = append(sections, line)
	}
	return "**Callbacks:**\n" + strings.Join(sections, "\n")
}

func selectPreferredMedia(contentSchemas map[string]ir.Schema) (string, ir.Schema) {
	if len(contentSchemas) == 0 {
		return "", nil
	}
	bestsType := ""
	bests := ir.Schema(nil)
	bestScore := -1
	for mediaType, schema := range contentSchemas {
		score := mediaPreferenceScore(mediaType)
		if score > bestScore || (score == bestScore && mediaType < bestsType) {
			bestScore = score
			bestsType = mediaType
			bests = schema
		}
	}
	return bestsType, bests
}

func mediaPreferenceScore(mediaType string) int {
	mediaType = strings.ToLower(mediaType)
	switch {
	case strings.Contains(mediaType, "json"):
		return 3
	case strings.Contains(mediaType, "yaml"):
		return 2
	case strings.HasPrefix(mediaType, "text/"):
		return 1
	default:
		return 0
	}
}

func summarizeSchema(schema ir.Schema) (string, string) {
	if schema == nil {
		return "", ""
	}
	typ := schema.Type()
	if typ == "" {
		if _, ok := schema["type"].([]interface{}); ok {
			typ = "multi"
		}
	}
	props := schema.Properties()
	if len(props) == 0 {
		if typ == "array" {
			if items, ok := schema["items"].(map[string]interface{}); ok {
				if t, _ := items["type"].(string); t != "" {
					typ = fmt.Sprintf("array of %s", t)
				}
			}
		}
		variants := summarizeVariants(schema)
		return variants, typ
	}

	keys := make([]string, 0, len(props))
	for name := range props {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	if len(keys) > 6 {
		keys = keys[:6]
	}
	var fields []string
	for _, name := range keys {
		prop := props[name]
		propType := prop.Type()
		if propType == "" {
			if items, ok := prop["items"].(map[string]interface{}); ok {
				if t, _ := items["type"].(string); t != "" {
					propType = fmt.Sprintf("array of %s", t)
				}
			}
		}
		if propType == "" {
			propType = "value"
		}
		fields = append(fields, fmt.Sprintf("%s (%s)", name, propType))
	}

	summary := fmt.Sprintf("object fields: %s", strings.Join(fields, ", "))
	if variants := summarizeVariants(schema); variants != "" {
		summary = fmt.Sprintf("%s; %s", summary, variants)
	}
	return summary, "object"
}

func formatExample(example interface{}) string {
	if example == nil {
		return ""
	}
	data, err := json.Marshal(example)
	if err != nil {
		return ""
	}
	str := string(data)
	const maxLen = 160
	if len(str) > maxLen {
		return str[:maxLen-3] + "..."
	}
	return str
}

func summarizeVariants(schema ir.Schema) string {
	if schema == nil {
		return ""
	}
	if variants := extractVariantLabels(schema, "oneOf"); len(variants) > 0 {
		return fmt.Sprintf("variants: oneOf(%s)", strings.Join(variants, ", "))
	}
	if variants := extractVariantLabels(schema, "anyOf"); len(variants) > 0 {
		return fmt.Sprintf("variants: anyOf(%s)", strings.Join(variants, ", "))
	}
	return ""
}

func extractVariantLabels(schema ir.Schema, key string) []string {
	list, ok := schema[key].([]interface{})
	if !ok || len(list) == 0 {
		return nil
	}
	labels := make([]string, 0, len(list))
	for _, item := range list {
		variant := toSchema(item)
		label := strings.TrimSpace(variantTitle(variant))
		if label == "" {
			if t := variant.Type(); t != "" {
				label = t
			}
		}
		if label == "" {
			labels = append(labels, "variant")
		} else {
			labels = append(labels, label)
		}
	}
	return labels
}

func variantTitle(schema ir.Schema) string {
	if schema == nil {
		return ""
	}
	if title, ok := schema["title"].(string); ok && title != "" {
		return title
	}
	if desc, ok := schema["description"].(string); ok && desc != "" {
		return desc
	}
	return ""
}
