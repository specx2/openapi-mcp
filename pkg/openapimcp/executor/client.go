package executor

import (
	"net/http"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type DefaultHTTPClient struct {
	client  *http.Client
	headers http.Header
}

func NewDefaultHTTPClient() *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client:  &http.Client{},
		headers: make(http.Header),
	}
}

func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	for key, values := range c.headers {
		for _, value := range values {
			if req.Header.Get(key) == "" {
				req.Header.Add(key, value)
			}
		}
	}
	return c.client.Do(req)
}

func (c *DefaultHTTPClient) WithTimeout(timeout time.Duration) *DefaultHTTPClient {
	c.client.Timeout = timeout
	return c
}

func (c *DefaultHTTPClient) WithHeaders(headers http.Header) *DefaultHTTPClient {
	for key, values := range headers {
		for _, value := range values {
			c.headers.Add(key, value)
		}
	}
	return c
}

func (c *DefaultHTTPClient) Headers() http.Header {
	return c.headers.Clone()
}

func (c *DefaultHTTPClient) Client() *http.Client {
	return c.client
}
