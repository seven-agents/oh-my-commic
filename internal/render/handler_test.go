package render

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
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// mount builds a chi router that injects userID into the context (standing in
// for auth.RequireUser) and mounts the render handler.
func mount(env *renderTestEnv, userID int64) http.Handler {
	h := NewHandler(env.svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithUserID(req.Context(), userID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Route("/api/v1", func(v1 chi.Router) { h.Mount(v1) })
	return r
}

// TestRenderHandlerOK verifies the owner gets 200 with the updated panel JSON.
func TestRenderHandlerOK(t *testing.T) {
	env := newRenderTestEnv(t)
	sp := env.seedPanel(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/panels/"+strconv.FormatInt(sp.panelID, 10)+"/render", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body=%s", w.Code, w.Body.String())
	}
	var out models.Panel
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Status != "done" || out.ImageURL == "" {
		t.Fatalf("expected rendered panel, got %+v", out)
	}
}

// TestRenderHandlerCrossUser404 verifies user 2 rendering user 1's panel gets a
// 404 (via the ownership gate) and no upstream detail leaks.
func TestRenderHandlerCrossUser404(t *testing.T) {
	env := newRenderTestEnv(t)
	sp := env.seedPanel(t, 1)
	srv := mount(env, 2) // user 2 hits user 1's panel

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/panels/"+strconv.FormatInt(sp.panelID, 10)+"/render", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-user should 404, got %d", w.Code)
	}
}

// TestRenderHandlerBadID400 verifies a non-positive id yields 400.
func TestRenderHandlerBadID400(t *testing.T) {
	env := newRenderTestEnv(t)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/panels/0/render", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad id should 400, got %d", w.Code)
	}
}

// TestRenderHandlerInsufficientCredits402 verifies an empty balance surfaces as
// a 402 Payment Required with a friendly message and never calls the generator.
func TestRenderHandlerInsufficientCredits402(t *testing.T) {
	env := newRenderTestEnv(t)
	env.ledger.balance = 0
	sp := env.seedPanel(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/panels/"+strconv.FormatInt(sp.panelID, 10)+"/render", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("insufficient credits should 402, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "积分不足") {
		t.Fatalf("body should carry a friendly credit message, got %s", w.Body.String())
	}
	if env.gen.count() != 0 {
		t.Fatalf("generator must not run on 402, calls=%d", env.gen.count())
	}
}

// TestRenderHandlerGenError502 verifies a generation failure surfaces as a
// generic 502 that leaks neither the API key nor upstream detail.
func TestRenderHandlerGenError502(t *testing.T) {
	env := newRenderTestEnv(t)
	env.gen.err = errGen
	sp := env.seedPanel(t, 1)
	srv := mount(env, 1)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/panels/"+strconv.FormatInt(sp.panelID, 10)+"/render", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("gen error should 502, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "sk-") || strings.Contains(w.Body.String(), "secret-upstream") {
		t.Fatal("response leaked upstream detail")
	}
}

// TestWriteRenderErrorClassifies verifies the AI-error sentinels map to their
// distinct HTTP statuses via the wrapped error chain, and the generic fallback
// stays 502. The sentinels are wrapped (as the service layer does with %w) so
// the test also proves errors.Is reaches through the chain.
func TestWriteRenderErrorClassifies(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"rate limited", fmt.Errorf("render: generate: %w", ai.ErrRateLimited), http.StatusTooManyRequests},
		{"timeout", fmt.Errorf("render: generate: %w", ai.ErrUpstreamTimeout), http.StatusGatewayTimeout},
		{"unavailable", fmt.Errorf("render: generate: %w", ai.ErrUpstreamUnavailable), http.StatusBadGateway},
		{"generic fallback", errGen, http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeRenderError(w, tc.err)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tc.want, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "sk-") || strings.Contains(w.Body.String(), "secret-upstream") {
				t.Fatal("response leaked upstream detail")
			}
		})
	}
}
