package ai

import (
	"context"
	"encoding/json"
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
