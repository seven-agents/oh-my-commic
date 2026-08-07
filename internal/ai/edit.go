package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// editContentPart is one part of a multimodal message: either an image
// reference (data URI or URL) or a text instruction. Exactly one field is set
// per part; the empty field is omitted from the JSON so DashScope sees a clean
// {"image":...} or {"text":...} object.
type editContentPart struct {
	Image string `json:"image,omitempty"`
	Text  string `json:"text,omitempty"`
}

// editMessage is a single chat message carrying multimodal content parts.
type editMessage struct {
	Role    string            `json:"role"`
	Content []editContentPart `json:"content"`
}

// editInput wraps the message list for the multimodal-generation request.
type editInput struct {
	Messages []editMessage `json:"messages"`
}

// editParameters holds the image-edit generation parameters.
type editParameters struct {
	N         int  `json:"n"`
	Watermark bool `json:"watermark"`
}

// editRequest is the synchronous image-edit request body.
type editRequest struct {
	Model      string         `json:"model"`
	Input      editInput      `json:"input"`
	Parameters editParameters `json:"parameters"`
}

// editResponse captures the fields we need from the multimodal-generation
// response: output.choices[0].message.content[0].image.
type editResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content []struct {
					Image string `json:"image"`
				} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
}

// EditImage redraws the reference image(s) using the qwen-image-edit model
// (c.EditModel) and returns the URL of the generated (stylized) image. The call
// is synchronous. Used by the comic-ification flow.
//
// refImageDataURIs are sent verbatim as the image content parts (typically
// data:{mime};base64,... URIs because the local upload is not reachable by
// DashScope). prompt is appended as a trailing text part.
//
// It never logs or embeds the API key in errors and respects ctx cancellation.
func (c *Client) EditImage(ctx context.Context, prompt string, refImageDataURIs []string) (string, error) {
	return c.multimodalEdit(ctx, c.EditModel, prompt, refImageDataURIs)
}

// RenderWithRefs renders a panel from prompt plus one or more reference images
// (matched characters and scene) using the multi-image edit model
// (c.RenderModel, e.g. qwen-image-edit-plus). Behaves like EditImage but on the
// render model, so panel consistency is driven by the actual locked reference
// images rather than text alone.
//
// refImageDataURIs must contain at least one image; the edit endpoint requires
// an input image (the caller falls back to text2image when there are none).
func (c *Client) RenderWithRefs(ctx context.Context, prompt string, refImageDataURIs []string) (string, error) {
	return c.multimodalEdit(ctx, c.RenderModel, prompt, refImageDataURIs)
}

// multimodalEdit performs the synchronous multimodal-generation (image-edit)
// request against the given model with the supplied reference image data URIs
// and a trailing text prompt, returning the produced image URL.
//
// It never logs or embeds the API key in errors and respects ctx cancellation.
func (c *Client) multimodalEdit(ctx context.Context, model, prompt string, refImageDataURIs []string) (string, error) {
	content := make([]editContentPart, 0, len(refImageDataURIs)+1)
	for _, uri := range refImageDataURIs {
		content = append(content, editContentPart{Image: uri})
	}
	content = append(content, editContentPart{Text: prompt})

	payload, err := json.Marshal(editRequest{
		Model:      model,
		Input:      editInput{Messages: []editMessage{{Role: "user", Content: content}}},
		Parameters: editParameters{N: 1, Watermark: false},
	})
	if err != nil {
		return "", fmt.Errorf("ai: marshal edit request: %w", err)
	}

	url := c.ImageBaseURL + "/services/aigc/multimodal-generation/generation"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ai: build edit request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: edit request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: read edit response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai: edit returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed editResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("ai: decode edit response: %w", err)
	}
	if len(parsed.Output.Choices) == 0 {
		return "", fmt.Errorf("ai: edit response missing choices")
	}
	content0 := parsed.Output.Choices[0].Message.Content
	if len(content0) == 0 || content0[0].Image == "" {
		return "", fmt.Errorf("ai: edit response missing image")
	}
	return content0[0].Image, nil
}
