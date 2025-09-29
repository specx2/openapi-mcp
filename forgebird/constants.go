package forgebird

const (
	SpecTypeOpenAPI = "openapi"
	SpecVersion30   = "3.0"
	SpecVersion31   = "3.1"
)

var (
	supportedExtensions   = []string{".yaml", ".yml", ".json"}
	supportedContentTypes = []string{
		"application/yaml",
		"application/x-yaml",
		"application/json",
		"text/yaml",
		"text/x-yaml",
	}
)
