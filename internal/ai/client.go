// Package ai provides clients for DashScope (Qwen) text and image generation
// via the OpenAI-compatible endpoint.
package ai

import (
	"net/http"
	"time"
)

// Client talks to DashScope's OpenAI-compatible API. The zero-value HTTP field
// falls back to http.DefaultClient at request time.
type Client struct {
	Key          string
	TextBaseURL  string
	ImageBaseURL string
	TextModel    string
	ImageModel   string
	EditModel    string
	RenderModel  string
	HTTP         *http.Client

	// PollInterval controls polling cadence for async image tasks (Task 7.1).
	PollInterval time.Duration
}

// httpClient returns the configured client or the shared default.
func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}
