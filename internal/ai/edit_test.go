package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEditImageParsesResultURL(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/services/aigc/multimodal-generation/generation") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-secret" {
			t.Errorf("missing/invalid auth header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-DashScope-Async") != "" {
			t.Errorf("edit must be synchronous, got async header")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"image":"http://img/x.png"}]}}]}}`))
	}))
	defer srv.Close()

	c := &Client{Key: "sk-secret", ImageBaseURL: srv.URL, EditModel: "qwen-image-edit"}
	url, err := c.EditImage(context.Background(), "画成吉卜力风格", []string{"data:image/png;base64,AAAA"})
	if err != nil {
		t.Fatalf("EditImage: %v", err)
	}
	if url != "http://img/x.png" {
		t.Fatalf("wrong url: %s", url)
	}

	// Assert body carries model + image + text.
	if gotBody["model"] != "qwen-image-edit" {
		t.Fatalf("wrong model in body: %v", gotBody["model"])
	}
	input, _ := gotBody["input"].(map[string]any)
	messages, _ := input["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	msg, _ := messages[0].(map[string]any)
	content, _ := msg["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected 2 content parts (image+text), got %d", len(content))
	}
	first, _ := content[0].(map[string]any)
	if first["image"] != "data:image/png;base64,AAAA" {
		t.Fatalf("image part wrong: %v", first)
	}
	last, _ := content[1].(map[string]any)
	if last["text"] != "画成吉卜力风格" {
		t.Fatalf("text part wrong: %v", last)
	}
}

func TestEditImageNon2xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{Key: "sk-secret", ImageBaseURL: srv.URL, EditModel: "qwen-image-edit"}
	_, err := c.EditImage(context.Background(), "p", []string{"data:image/png;base64,AA"})
	if err == nil {
		t.Fatal("expected error on non-2xx")
	}
	if strings.Contains(err.Error(), "sk-secret") {
		t.Fatalf("error leaked key: %v", err)
	}
}

func TestEditImageEmptyContentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[]}}]}}`))
	}))
	defer srv.Close()

	c := &Client{Key: "sk-secret", ImageBaseURL: srv.URL, EditModel: "qwen-image-edit"}
	if _, err := c.EditImage(context.Background(), "p", []string{"data:image/png;base64,AA"}); err == nil {
		t.Fatal("expected error on empty content")
	}
}

func TestEditImageMissingImageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"text":"no image here"}]}}]}}`))
	}))
	defer srv.Close()

	c := &Client{Key: "sk-secret", ImageBaseURL: srv.URL, EditModel: "qwen-image-edit"}
	if _, err := c.EditImage(context.Background(), "p", []string{"data:image/png;base64,AA"}); err == nil {
		t.Fatal("expected error on missing image field")
	}
}
