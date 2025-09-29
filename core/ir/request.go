package ir

type RequestBodyInfo struct {
	Required         bool
	ContentSchemas   map[string]Schema
	ContentOrder     []string
	Encodings        map[string]map[string]EncodingInfo
	Description      string
	MediaExamples    map[string]interface{}
	MediaExampleSets map[string]map[string]interface{}
	MediaDefaults    map[string]interface{}
	MediaExtensions  map[string]map[string]interface{}
	Extensions       map[string]interface{}
}

type EncodingInfo struct {
	ContentType   string
	Style         string
	Explode       *bool
	AllowReserved bool
	Headers       map[string]HeaderInfo
	Extensions    map[string]interface{}
}

type HeaderInfo struct {
	Name            string
	Description     string
	Required        bool
	Deprecated      bool
	AllowEmptyValue bool
	Style           string
	Explode         bool
	AllowReserved   bool
	Schema          Schema
	Example         interface{}
	Examples        map[string]interface{}
	Extensions      map[string]interface{}
}
