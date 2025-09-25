package ir

type ResponseInfo struct {
	Description      string
	ContentSchemas   map[string]Schema
	MediaExamples    map[string]interface{}
	MediaExampleSets map[string]map[string]interface{}
	MediaExtensions  map[string]map[string]interface{}
	Extensions       map[string]interface{}
}
