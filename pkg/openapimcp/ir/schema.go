package ir

type Schema map[string]interface{}

func (s Schema) Type() string {
	if t, ok := s["type"].(string); ok {
		return t
	}
	return ""
}

func (s Schema) Properties() map[string]Schema {
	props := make(map[string]Schema)
	if p, ok := s["properties"].(map[string]interface{}); ok {
		for k, v := range p {
			if schema, ok := v.(map[string]interface{}); ok {
				props[k] = schema
				continue
			}
			if nested, ok := v.(Schema); ok {
				props[k] = nested
			}
		}
	}
	return props
}

func (s Schema) Required() []string {
	if req, ok := s["required"].([]interface{}); ok {
		result := make([]string, 0, len(req))
		for _, r := range req {
			if str, ok := r.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

func (s Schema) Definitions() map[string]Schema {
	defs := make(map[string]Schema)
	if d, ok := s["$defs"].(map[string]interface{}); ok {
		for k, v := range d {
			if schema, ok := v.(map[string]interface{}); ok {
				defs[k] = schema
			}
		}
	}
	return defs
}
