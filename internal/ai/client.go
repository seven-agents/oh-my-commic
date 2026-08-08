// Package ai provides clients for DashScope (Qwen) text and image generation
// via the OpenAI-compatible endpoint.
package ai

import (
	"net/http"
)

// Client talks to DashScope's OpenAI-compatible API (text) and Volcano Ark's
// Seedream image API (images). The zero-value HTTP field falls back to
// http.DefaultClient at request time.
type Client struct {
	// DashScope text (Qwen chat) config.
	Key         string
	TextBaseURL string
	TextModel   string

	// Volcano Ark Seedream 4.0 image config.
	ArkKey          string
	SeedreamModel   string
	SeedreamBaseURL string

	HTTP *http.Client
}

// httpClient returns the configured client or the shared default.
func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}
