package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGenerateImagePolls(t *testing.T) {
	var polls int
	mux := http.NewServeMux()
	mux.HandleFunc("/services/aigc/text2image/image-synthesis", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-DashScope-Async") != "enable" {
			t.Fatal("缺 async 头")
		}
		w.Write([]byte(`{"output":{"task_id":"t1","task_status":"PENDING"}}`))
	})
	mux.HandleFunc("/tasks/t1", func(w http.ResponseWriter, r *http.Request) {
		polls++
		if polls < 2 {
			w.Write([]byte(`{"output":{"task_status":"PENDING"}}`))
			return
		}
		w.Write([]byte(`{"output":{"task_status":"SUCCEEDED","results":[{"url":"http://img/x.png"}]}}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	c := Client{Key: "sk-x", ImageBaseURL: ts.URL, ImageModel: "wan2.2-t2i-plus", HTTP: ts.Client(), PollInterval: time.Millisecond}
	url, err := c.GenerateImage(context.Background(), "fox", nil)
	if err != nil || url != "http://img/x.png" {
		t.Fatalf("应拿到 url: %q %v", url, err)
	}
	if polls < 2 {
		t.Fatal("应至少轮询2次")
	}
}

func TestGenerateImageFailedStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/services/aigc/text2image/image-synthesis", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"output":{"task_id":"t1","task_status":"PENDING"}}`))
	})
	mux.HandleFunc("/tasks/t1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"output":{"task_status":"FAILED","code":"InternalError","message":"boom"}}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	c := Client{Key: "sk-x", ImageBaseURL: ts.URL, ImageModel: "wan2.2-t2i-plus", HTTP: ts.Client(), PollInterval: time.Millisecond}
	if _, err := c.GenerateImage(context.Background(), "fox", nil); err == nil {
		t.Fatal("FAILED 状态应返回错误")
	}
}

func TestGenerateImagePollCapExceeded(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/services/aigc/text2image/image-synthesis", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"output":{"task_id":"t1","task_status":"PENDING"}}`))
	})
	mux.HandleFunc("/tasks/t1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"output":{"task_status":"PENDING"}}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	c := Client{Key: "sk-x", ImageBaseURL: ts.URL, ImageModel: "wan2.2-t2i-plus", HTTP: ts.Client(), PollInterval: time.Nanosecond}
	if _, err := c.GenerateImage(context.Background(), "fox", nil); err == nil {
		t.Fatal("超出轮询上限应超时报错")
	}
}

func TestGenerateImageContextCancelled(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/services/aigc/text2image/image-synthesis", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"output":{"task_id":"t1","task_status":"PENDING"}}`))
	})
	mux.HandleFunc("/tasks/t1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"output":{"task_status":"PENDING"}}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	c := Client{Key: "sk-x", ImageBaseURL: ts.URL, ImageModel: "wan2.2-t2i-plus", HTTP: ts.Client(), PollInterval: 10 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GenerateImage(ctx, "fox", nil)
	if err != context.Canceled {
		t.Fatalf("ctx 取消应返回 context.Canceled, 得到 %v", err)
	}
}

func TestGenerateImageSubmitNon2xx(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/services/aigc/text2image/image-synthesis", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code":"InvalidApiKey"}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	c := Client{Key: "sk-x", ImageBaseURL: ts.URL, ImageModel: "wan2.2-t2i-plus", HTTP: ts.Client(), PollInterval: time.Millisecond}
	if _, err := c.GenerateImage(context.Background(), "fox", nil); err == nil {
		t.Fatal("提交非2xx应返回错误")
	}
}
