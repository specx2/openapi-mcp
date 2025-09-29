package openapimcp

import (
	"net/http"
	"time"
)

// HTTPClientConfig describes reusable HTTP client settings for OpenAPI MCP tooling.
type HTTPClientConfig struct {
	BaseURL string
	Timeout time.Duration
	Headers http.Header
}
