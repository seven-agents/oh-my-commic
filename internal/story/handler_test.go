package story

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/seven-agents/oh-my-commic/internal/ai"
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

const storyboardChatPath = "/storyboard-chat"

func TestStoryboardChatHandlerOK(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"你好呀","panels":[{"content":"一个安静的清晨"}]}`)
	ch := env.newChapter(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/"+itoa(ch.ID)+storyboardChatPath,
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}],"panelCount":3}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 %d, body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Reply  string `json:"reply"`
		Panels []struct {
			Content string `json:"content"`
		} `json:"panels"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Reply != "你好呀" {
		t.Fatalf("reply 错: %q", out.Reply)
	}
	if len(out.Panels) != 1 || out.Panels[0].Content != "一个安静的清晨" {
		t.Fatalf("panels 错: %+v", out.Panels)
	}
}

// TestProcessPanelHandlerOK verifies POST /api/panels/{id}/process decomposes a
// seeded content-only panel and returns the updated structured panel.
func TestProcessPanelHandlerOK(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"ok","panels":[{"content":"小狐狸在森林里"}]}`)
	ch := env.newChapter(t, 1)
	_, seeded, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil || len(seeded) != 1 {
		t.Fatalf("seed: %v (%d)", err, len(seeded))
	}
	env.setContent(`{"location":"森林","sceneId":0,"characters":[],"event":"漫步","caption":"走走","imagePrompt":"forest"}`)

	srv := mount(env, 1)
	req := httptest.NewRequest(http.MethodPost, "/api/panels/"+itoa(seeded[0].ID)+"/process", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 %d, body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Content  string `json:"content"`
		Location string `json:"location"`
		Event    string `json:"event"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Content != "小狐狸在森林里" || out.Location != "森林" || out.Event != "漫步" {
		t.Fatalf("process 结果错: %+v", out)
	}
}

// TestProcessPanelHandlerCrossUser404 verifies a user cannot process another
// user's panel.
func TestProcessPanelHandlerCrossUser404(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"ok","panels":[{"content":"一格"}]}`)
	ch := env.newChapter(t, 1)
	_, seeded, err := env.svc.StoryboardChat(1, ch.ID, nil, 0)
	if err != nil || len(seeded) != 1 {
		t.Fatalf("seed: %v (%d)", err, len(seeded))
	}

	srv := mount(env, 2)
	req := httptest.NewRequest(http.MethodPost, "/api/panels/"+itoa(seeded[0].ID)+"/process", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("跨用户应 404, got %d", w.Code)
	}
}

func TestStoryboardChatHandlerCrossUser404(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"hi","panels":[]}`)
	ch := env.newChapter(t, 1)
	srv := mount(env, 2) // user 2 hits user 1's chapter

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/"+itoa(ch.ID)+storyboardChatPath,
		strings.NewReader(`{"messages":[]}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("跨用户应 404, got %d", w.Code)
	}
}

func TestStoryboardChatHandlerBadJSON400(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"hi","panels":[]}`)
	ch := env.newChapter(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/"+itoa(ch.ID)+storyboardChatPath,
		strings.NewReader(`{bad`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("坏 JSON 应 400, got %d", w.Code)
	}
}

func TestStoryboardChatHandlerBadID400(t *testing.T) {
	env := newStoryTestEnv(t, `{"reply":"hi","panels":[]}`)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/0"+storyboardChatPath,
		strings.NewReader(`{"messages":[]}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 ID 应 400, got %d", w.Code)
	}
}

func TestStoryboardChatHandlerAIError502(t *testing.T) {
	env := newStoryTestEnv(t, "无法生成分镜") // no JSON object -> parse error
	ch := env.newChapter(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/chapters/"+itoa(ch.ID)+storyboardChatPath,
		strings.NewReader(`{"messages":[]}`))
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

// TestWriteStoryErrorClassifies verifies the AI-error sentinels map to their
// distinct HTTP statuses via the wrapped error chain, with the generic fallback
// staying 502.
func TestWriteStoryErrorClassifies(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not found", ErrNotFound, http.StatusNotFound},
		{"rate limited", fmt.Errorf("story: chat: %w", ai.ErrRateLimited), http.StatusTooManyRequests},
		{"timeout", fmt.Errorf("story: chat: %w", ai.ErrUpstreamTimeout), http.StatusGatewayTimeout},
		{"unavailable", fmt.Errorf("story: chat: %w", ai.ErrUpstreamUnavailable), http.StatusBadGateway},
		{"generic fallback", fmt.Errorf("story: parse: boom"), http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeStoryError(w, tc.err)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// itoa formats an int64 chapter id for URL construction.
func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
