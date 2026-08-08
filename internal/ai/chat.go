package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Msg is a single chat message in OpenAI chat-completions format.
type Msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatMaxTokens caps the model's output length. It must be generous enough that
// a multi-panel structured storyboard JSON is never truncated mid-object (a
// truncated response is invalid JSON and fails downstream parsing).
const chatMaxTokens = 8192

// chatRequest is the OpenAI chat-completions request body.
type chatRequest struct {
	Model     string `json:"model"`
	Messages  []Msg  `json:"messages"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

// chatResponse captures the fields we need from the response.
type chatResponse struct {
	Choices []struct {
		Message Msg `json:"message"`
	} `json:"choices"`
}

// Chat sends messages to {TextBaseURL}/chat/completions and returns the content
// of the first choice. It never logs the API key.
func (c *Client) Chat(ctx context.Context, messages []Msg) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("ai: chat requires at least one message")
	}

	payload, err := json.Marshal(chatRequest{Model: c.TextModel, Messages: messages, MaxTokens: chatMaxTokens})
	if err != nil {
		return "", fmt.Errorf("ai: marshal chat request: %w", err)
	}

	url := c.TextBaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ai: build chat request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		// Classify a timeout so handlers can map it to a 504; otherwise the
		// original transport error is preserved through the %w chain.
		return "", fmt.Errorf("ai: chat request failed: %w", classifyTransport(err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: read chat response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Never surface the upstream body: the request carried the Bearer key and
		// a misbehaving upstream could echo it back, so only the status code is
		// reported. A 429/5xx additionally carries a sentinel (via %w) so handlers
		// can distinguish rate-limit / upstream-down.
		if sentinel := classifyStatus(resp.StatusCode); sentinel != nil {
			return "", fmt.Errorf("ai: chat returned status %d: %w", resp.StatusCode, sentinel)
		}
		return "", fmt.Errorf("ai: chat returned status %d", resp.StatusCode)
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("ai: decode chat response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("ai: chat response contained no choices")
	}

	return parsed.Choices[0].Message.Content, nil
}
