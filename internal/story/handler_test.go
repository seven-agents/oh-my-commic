package story

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/auth"
)

// mount builds a chi router that injects userID into the context (standing in
// for auth.RequireUser) and mounts the story handler.
func mount(env *storyTestEnv, userID int64) http.Handler {
	h := NewHandler(env.svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithUserID(req.Context(), userID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	h.Mount(r)
	return r
}

func TestConverseHandlerOK(t *testing.T) {
	env := newStoryTestEnv(t, "你好呀")
	ch := env.newChapter(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/"+itoa(ch.ID)+"/converse",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 %d, body=%s", w.Code, w.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["reply"] != "你好呀" {
		t.Fatalf("reply 错: %q", out["reply"])
	}
}

func TestConverseHandlerCrossUser404(t *testing.T) {
	env := newStoryTestEnv(t, "hi")
	ch := env.newChapter(t, 1)
	srv := mount(env, 2) // user 2 hits user 1's chapter

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/"+itoa(ch.ID)+"/converse",
		strings.NewReader(`{"messages":[]}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("跨用户应 404, got %d", w.Code)
	}
}

func TestConverseHandlerBadJSON400(t *testing.T) {
	env := newStoryTestEnv(t, "hi")
	ch := env.newChapter(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/"+itoa(ch.ID)+"/converse",
		strings.NewReader(`{bad`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("坏 JSON 应 400, got %d", w.Code)
	}
}

func TestConverseHandlerBadID400(t *testing.T) {
	env := newStoryTestEnv(t, "hi")
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/0/converse",
		strings.NewReader(`{"messages":[]}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 ID 应 400, got %d", w.Code)
	}
}

func TestStoryboardHandlerOK(t *testing.T) {
	body := "[{\"caption\":\"a\",\"characterIds\":[],\"sceneId\":0,\"imagePrompt\":\"x\"}]"
	env := newStoryTestEnv(t, body)
	ch := env.newChapter(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/"+itoa(ch.ID)+"/storyboard",
		strings.NewReader(`{"messages":[],"panelCount":1}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 %d, body=%s", w.Code, w.Body.String())
	}
}

func TestStoryboardHandlerAIError502(t *testing.T) {
	env := newStoryTestEnv(t, "无法生成分镜") // no JSON array -> parse error
	ch := env.newChapter(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/"+itoa(ch.ID)+"/storyboard",
		strings.NewReader(`{"messages":[],"panelCount":1}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("AI 错误应 502, got %d", w.Code)
	}
	// The generic message must not leak the key or raw upstream detail.
	if strings.Contains(w.Body.String(), "sk-x") {
		t.Fatal("响应泄露了 API key")
	}
}

// itoa formats an int64 chapter id for URL construction.
func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
