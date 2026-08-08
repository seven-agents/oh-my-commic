package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// seedreamSize is the requested output resolution for Seedream 4.0 panels.
const seedreamSize = "2048x2048"

// seedreamRequest is the Volcano Ark Seedream image-generation request body.
// Image is a list of base64 data-URI reference images (0..10); it is omitted
// entirely for pure text-to-image (no refs).
type seedreamRequest struct {
	Model              string   `json:"model"`
	Prompt             string   `json:"prompt"`
	Image              []string `json:"image,omitempty"`
	Size               string   `json:"size"`
	SequentialImageGen string   `json:"sequential_image_generation"`
	ResponseFormat     string   `json:"response_format"`
	Watermark          bool     `json:"watermark"`
}

// seedreamResponse captures the produced image URL at data[0].url.
type seedreamResponse struct {
	Data []struct {
		URL string `json:"url"`
	} `json:"data"`
}

// SeedreamImage generates an image with Volcano Ark's Seedream 4.0 model. It is
// synchronous: it POSTs to {SeedreamBaseURL}/images/generations and returns the
// produced image URL (data[0].url).
//
// refImageDataURIs are base64 data: URIs of the reference images (0..10). When
// none are given the "image" field is omitted and Seedream performs pure
// text-to-image. The reference count is expected to be pre-capped by callers.
//
// It never logs or embeds the API key in errors and respects ctx cancellation.
func (c *Client) SeedreamImage(ctx context.Context, prompt string, refImageDataURIs []string) (string, error) {
	reqBody := seedreamRequest{
		Model:              c.SeedreamModel,
		Prompt:             prompt,
		Size:               seedreamSize,
		SequentialImageGen: "disabled",
		ResponseFormat:     "url",
		Watermark:          false,
	}
	if len(refImageDataURIs) > 0 {
		reqBody.Image = refImageDataURIs
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ai: marshal seedream request: %w", err)
	}

	url := c.SeedreamBaseURL + "/images/generations"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ai: build seedream request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.ArkKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		// Classify a timeout so handlers can map it to a 504; otherwise the
		// original transport error is preserved through the %w chain.
		return "", fmt.Errorf("ai: seedream request failed: %w", classifyTransport(err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: read seedream response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Deliberately omit the response body from the error: the request carried
		// the Bearer key and a misbehaving upstream could echo it back, so only
		// the status code is surfaced. A 429/5xx additionally carries a sentinel
		// (via %w) so handlers can distinguish rate-limit / upstream-down.
		if sentinel := classifyStatus(resp.StatusCode); sentinel != nil {
			return "", fmt.Errorf("ai: seedream returned status %d: %w", resp.StatusCode, sentinel)
		}
		return "", fmt.Errorf("ai: seedream returned status %d", resp.StatusCode)
	}

	var parsed seedreamResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("ai: decode seedream response: %w", err)
	}
	if len(parsed.Data) == 0 || parsed.Data[0].URL == "" {
		return "", fmt.Errorf("ai: seedream response missing image url")
	}
	return parsed.Data[0].URL, nil
}
