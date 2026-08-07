package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// defaultPollInterval is used when Client.PollInterval is zero.
	defaultPollInterval = 2 * time.Second
	// maxImagePolls caps the number of task-status polls to avoid infinite loops.
	maxImagePolls = 60
)

// imageInput is the DashScope text2image request input.
type imageInput struct {
	Prompt string `json:"prompt"`
}

// imageParameters holds the generation parameters.
type imageParameters struct {
	N    int    `json:"n"`
	Size string `json:"size"`
}

// imageRequest is the async image-synthesis request body.
type imageRequest struct {
	Model      string          `json:"model"`
	Input      imageInput      `json:"input"`
	Parameters imageParameters `json:"parameters"`
}

// imageOutput captures the fields we need from DashScope's output object,
// shared by both the submit response and the task-status response.
type imageOutput struct {
	TaskID     string `json:"task_id"`
	TaskStatus string `json:"task_status"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Results    []struct {
		URL string `json:"url"`
	} `json:"results"`
}

// imageResponse wraps the output object.
type imageResponse struct {
	Output imageOutput `json:"output"`
}

// GenerateImage submits an async text2image task to DashScope and polls the
// task endpoint until it succeeds or fails, returning the first result URL.
//
// refImageURLs is accepted now to satisfy the Task 7.2 contract, but reference
// images are not yet forwarded to the wan text2image endpoint ("先按无参考图跑通").
// TODO(task-7.2): forward refImageURLs (wan series input.ref_img) once the
// reference-image flow is wired up.
//
// It never logs the API key and respects ctx cancellation.
func (c *Client) GenerateImage(ctx context.Context, prompt string, refImageURLs []string) (string, error) {
	_ = refImageURLs // deferred to Task 7.2; see doc comment above.

	if err := ctx.Err(); err != nil {
		return "", err
	}

	taskID, err := c.submitImageTask(ctx, prompt)
	if err != nil {
		return "", err
	}
	return c.pollImageTask(ctx, taskID)
}

// submitImageTask POSTs the async image-synthesis request and returns the task id.
func (c *Client) submitImageTask(ctx context.Context, prompt string) (string, error) {
	payload, err := json.Marshal(imageRequest{
		Model:      c.ImageModel,
		Input:      imageInput{Prompt: prompt},
		Parameters: imageParameters{N: 1, Size: "1024*1024"},
	})
	if err != nil {
		return "", fmt.Errorf("ai: marshal image request: %w", err)
	}

	url := c.ImageBaseURL + "/services/aigc/text2image/image-synthesis"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ai: build image submit request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: image submit request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: read image submit response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai: image submit returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed imageResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("ai: decode image submit response: %w", err)
	}
	if parsed.Output.TaskID == "" {
		return "", fmt.Errorf("ai: image submit response missing task_id")
	}
	return parsed.Output.TaskID, nil
}

// pollImageTask polls the task endpoint until the task succeeds or fails, or the
// poll cap is exceeded, or ctx is cancelled.
func (c *Client) pollImageTask(ctx context.Context, taskID string) (string, error) {
	interval := c.PollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}
	url := c.ImageBaseURL + "/tasks/" + taskID

	for poll := 0; poll < maxImagePolls; poll++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		out, err := c.fetchTaskStatus(ctx, url)
		if err != nil {
			return "", err
		}

		switch out.TaskStatus {
		case "SUCCEEDED":
			if len(out.Results) == 0 || out.Results[0].URL == "" {
				return "", fmt.Errorf("ai: image task %s succeeded but returned no url", taskID)
			}
			return out.Results[0].URL, nil
		case "FAILED":
			return "", fmt.Errorf("ai: image task %s failed: %s %s", taskID, out.Code, out.Message)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
	}

	return "", fmt.Errorf("ai: image task %s timed out after %d polls", taskID, maxImagePolls)
}

// fetchTaskStatus performs a single GET on the task endpoint.
func (c *Client) fetchTaskStatus(ctx context.Context, url string) (imageOutput, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return imageOutput{}, fmt.Errorf("ai: build image poll request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return imageOutput{}, fmt.Errorf("ai: image poll request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return imageOutput{}, fmt.Errorf("ai: read image poll response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return imageOutput{}, fmt.Errorf("ai: image poll returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed imageResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return imageOutput{}, fmt.Errorf("ai: decode image poll response: %w", err)
	}
	return parsed.Output, nil
}
