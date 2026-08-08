package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatParsesContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-x" {
			t.Fatal("缺 Bearer")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "qwen-plus") {
			t.Fatalf("body 缺 model: %s", body)
		}
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("body 非合法 JSON: %v", err)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"你好"}}]}`))
	}))
	defer ts.Close()
	c := Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	got, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "hi"}})
	if err != nil || got != "你好" {
		t.Fatalf("解析失败: %q %v", got, err)
	}
}

func TestChatNon2xxReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer ts.Close()
	c := Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("期望 non-2xx 报错")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("错误应含状态码: %v", err)
	}
}

// TestChatNon2xxNeverLeaksBody verifies the upstream response body is never
// embedded in the error (it may echo the Bearer key), only the status code.
func TestChatNon2xxNeverLeaksBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"leaked-secret-body sk-x"}`))
	}))
	defer ts.Close()
	c := Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("期望 non-2xx 报错")
	}
	if strings.Contains(err.Error(), "leaked-secret-body") {
		t.Fatalf("错误泄露了上游 body: %v", err)
	}
}

// TestChatClassifiesStatus verifies a 429 wraps ErrRateLimited and a 5xx wraps
// ErrUpstreamUnavailable, while other non-2xx carry no sentinel.
func TestChatClassifiesStatus(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusInternalServerError, ErrUpstreamUnavailable},
		{http.StatusServiceUnavailable, ErrUpstreamUnavailable},
		{http.StatusBadRequest, nil},
	}
	for _, tc := range cases {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.code)
		}))
		c := Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
		_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "hi"}})
		ts.Close()
		if err == nil {
			t.Fatalf("code %d: 期望报错", tc.code)
		}
		if tc.want == nil {
			if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrUpstreamUnavailable) {
				t.Fatalf("code %d: 不应带 sentinel: %v", tc.code, err)
			}
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Fatalf("code %d: 期望 %v, got %v", tc.code, tc.want, err)
		}
	}
}

// TestChatTimeoutClassified verifies a transport-level timeout surfaces as
// ErrUpstreamTimeout through the wrapped chain. A stub RoundTripper returns a
// timing-out net.Error so no real server or network wait is needed.
func TestChatTimeoutClassified(t *testing.T) {
	c := Client{
		Key:         "sk-x",
		TextBaseURL: "http://upstream.invalid",
		TextModel:   "qwen-plus",
		HTTP:        &http.Client{Transport: timeoutRoundTripper{}},
	}
	_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("期望超时报错")
	}
	if !errors.Is(err, ErrUpstreamTimeout) {
		t.Fatalf("期望 ErrUpstreamTimeout, got %v", err)
	}
}

func TestChatEmptyChoicesReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer ts.Close()
	c := Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus", HTTP: ts.Client()}
	_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("期望空 choices 报错")
	}
}

func TestChatEmptyMessagesReturnsError(t *testing.T) {
	c := Client{Key: "sk-x", TextBaseURL: "http://unused", TextModel: "qwen-plus"}
	_, err := c.Chat(context.Background(), nil)
	if err == nil {
		t.Fatal("期望空 messages 报错")
	}
}

func TestChatDefaultsHTTPClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()
	c := Client{Key: "sk-x", TextBaseURL: ts.URL, TextModel: "qwen-plus"}
	got, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "hi"}})
	if err != nil || got != "ok" {
		t.Fatalf("默认 HTTP client 失败: %q %v", got, err)
	}
}
