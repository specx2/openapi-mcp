package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/yaml"
)

// schemaResolver resolves JSON pointers and remote references for schemas.
type schemaResolver struct {
	root        map[string]interface{}
	baseURL     *url.URL
	client      *http.Client
	cache       map[string]map[string]interface{}
	nameCache   map[string]string
	nameCounter map[string]int
	mu          sync.Mutex
}

func newSchemaResolver(spec []byte, specURL string) (*schemaResolver, error) {
	root, err := decodeDocumentToMap(spec)
	if err != nil {
		return nil, err
	}

	var base *url.URL
	if specURL != "" {
		base, err = url.Parse(specURL)
		if err != nil {
			return nil, fmt.Errorf("invalid spec url %q: %w", specURL, err)
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}

	return &schemaResolver{
		root:        root,
		baseURL:     base,
		client:      client,
		cache:       make(map[string]map[string]interface{}),
		nameCache:   make(map[string]string),
		nameCounter: make(map[string]int),
	}, nil
}

func (r *schemaResolver) register(ref, name string, schema map[string]interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name != "" {
		r.nameCache[ref] = name
	}
	if schema != nil {
		r.cache[ref] = schema
	}
}

func (r *schemaResolver) getDefinitionName(ref string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name, ok := r.nameCache[ref]
	return name, ok
}

func (r *schemaResolver) resolveRef(ref string) (string, map[string]interface{}, error) {
	if ref == "" {
		return "", nil, errors.New("empty reference")
	}

	r.mu.Lock()
	if schema, ok := r.cache[ref]; ok {
		name := r.nameCache[ref]
		r.mu.Unlock()
		return name, cloneGenericMap(schema), nil
	}
	r.mu.Unlock()

	var (
		schema map[string]interface{}
		err    error
	)

	if strings.HasPrefix(ref, "#/") || ref == "#" {
		schema, err = r.resolveLocal(strings.TrimPrefix(ref, "#"))
	} else {
		schema, err = r.resolveRemote(ref)
	}
	if err != nil {
		return "", nil, err
	}

	name := r.generateNameForRef(ref)

	r.mu.Lock()
	r.cache[ref] = schema
	r.nameCache[ref] = name
	r.mu.Unlock()

	return name, cloneGenericMap(schema), nil
}

func (r *schemaResolver) resolveLocal(pointer string) (map[string]interface{}, error) {
	value, err := navigateJSONPointer(r.root, pointer)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve local ref '#%s': %w", pointer, err)
	}
	schema, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("reference '#%s' does not resolve to an object", pointer)
	}
	return cloneGenericMap(schema), nil
}

func (r *schemaResolver) resolveRemote(ref string) (map[string]interface{}, error) {
	parsed, err := url.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("invalid reference %q: %w", ref, err)
	}

	var fragment string
	if parsed.Fragment != "" {
		fragment = parsed.Fragment
		parsed.Fragment = ""
	}

	absolute := parsed
	if r.baseURL != nil && !parsed.IsAbs() {
		absolute = r.baseURL.ResolveReference(parsed)
	}

	if !absolute.IsAbs() {
		return nil, fmt.Errorf("unable to resolve relative reference %q: base URL unknown", ref)
	}

	r.mu.Lock()
	doc, ok := r.cache[absolute.String()]
	r.mu.Unlock()

	if !ok {
		data, err := r.fetchResource(absolute)
		if err != nil {
			return nil, err
		}
		doc, err = decodeDocumentToMap(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse referenced document %q: %w", absolute.String(), err)
		}
		r.mu.Lock()
		r.cache[absolute.String()] = doc
		r.mu.Unlock()
	}

	if fragment == "" {
		return cloneGenericMap(doc), nil
	}

	value, err := navigateJSONPointer(doc, fragment)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve fragment '#%s' in %q: %w", fragment, absolute.String(), err)
	}
	schema, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("reference %q does not resolve to an object", ref)
	}
	return cloneGenericMap(schema), nil
}

func (r *schemaResolver) fetchResource(u *url.URL) ([]byte, error) {
	switch u.Scheme {
	case "http", "https":
		resp, err := r.client.Get(u.String())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %q: %w", u.String(), err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("failed to fetch %q: status %s", u.String(), resp.Status)
		}
		return io.ReadAll(resp.Body)
	case "file":
		return os.ReadFile(u.Path)
	case "":
		// treat as file path
		return os.ReadFile(u.Path)
	default:
		return nil, fmt.Errorf("unsupported URI scheme %q in reference %q", u.Scheme, u.String())
	}
}

func (r *schemaResolver) generateNameForRef(ref string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if name, ok := r.nameCache[ref]; ok {
		return name
	}

	base := deriveRefBase(ref)
	count := r.nameCounter[base]
	if count > 0 {
		base = fmt.Sprintf("%s_%d", base, count+1)
	}
	r.nameCounter[deriveRefBase(ref)] = count + 1
	r.nameCache[ref] = base
	return base
}

func decodeDocumentToMap(data []byte) (map[string]interface{}, error) {
	var jsonBytes []byte
	if json.Valid(data) {
		jsonBytes = data
	} else {
		converted, err := yaml.YAMLToJSON(data)
		if err != nil {
			return nil, err
		}
		jsonBytes = converted
	}
	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// navigateJSONPointer resolves a JSON pointer (without leading '#') within the provided document.
func navigateJSONPointer(doc interface{}, pointer string) (interface{}, error) {
	if pointer == "" {
		return doc, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		pointer = "/" + pointer
	}
	tokens := strings.Split(pointer, "/")[1:]
	current := doc
	for _, rawToken := range tokens {
		token := strings.ReplaceAll(strings.ReplaceAll(rawToken, "~1", "/"), "~0", "~")
		switch node := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = node[token]
			if !ok {
				return nil, fmt.Errorf("pointer segment %q not found", token)
			}
		case []interface{}:
			idx, err := parseArrayIndex(token, len(node))
			if err != nil {
				return nil, err
			}
			current = node[idx]
		default:
			return nil, fmt.Errorf("invalid node encountered while resolving pointer segment %q", token)
		}
	}
	return current, nil
}
