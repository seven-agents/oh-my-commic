// Package contract runs an AI-free end-to-end walk of the real HTTP surface and
// validates every response against the authoritative OpenAPI contract at
// docs/openapi.yaml. It assembles the same handler/service/repository stack as
// cmd/server, but injects a fake image generator so no network calls or real API
// keys are required — the test must pass with an empty environment.
//
// The walk exercises: registration, login (cookie), GET /me (credits==100),
// book + character + chapter + panel CRUD, cross-user isolation (404), and the
// 402 insufficient-credit path on panel render (the fake generator must never be
// called). Each response is checked with openapi3filter.ValidateResponse so a
// drifted handler (or a stale openapi.yaml) fails the build.
package contract

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/seven-agents/oh-my-commic/internal/ai"
	"github.com/seven-agents/oh-my-commic/internal/asset"
	"github.com/seven-agents/oh-my-commic/internal/auth"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/comicify"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/httpx"
	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/panel"
	"github.com/seven-agents/oh-my-commic/internal/render"
	"github.com/seven-agents/oh-my-commic/internal/storage"
	"github.com/seven-agents/oh-my-commic/internal/story"
)

// fakeGenerator is an ImageGenerator/ImageEditor that records whether it was
// ever called. In the AI-free flow it must never be invoked; the render 402 path
// asserts calls==0 (Spend fails before the generator runs).
type fakeGenerator struct{ calls int }

func (f *fakeGenerator) SeedreamImage(_ context.Context, _ string, _ []string) (string, error) {
	f.calls++
	return "", nil
}

// testApp bundles the running server with the contract validator and the raw DB
// (used only to force the credit balance for the 402 case).
type testApp struct {
	server    *httptest.Server
	validator *contractValidator
	db        *sql.DB
	gen       *fakeGenerator
}

// newTestApp wires the full backend stack against a temp SQLite DB and temp data
// dir, injecting a fake image generator so no real AI is reached.
func newTestApp(t *testing.T) *testApp {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "contract.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	media := storage.Local{Root: t.TempDir()}
	sess := auth.NewSession(d)
	userRepo := auth.NewUserRepo(d)

	// SignupCredits fixed at 100 so the /me credits assertion is deterministic.
	authSvc := auth.NewService(userRepo, sess, 100)
	authHandler := auth.NewHandler(authSvc)

	bookRepo := book.NewRepo(d)
	bookHandler := book.NewHandler(book.NewService(bookRepo))

	gen := &fakeGenerator{}

	// Comicify is wired but never exercised here (no image uploads), so the fake
	// editor is safe. cost=1 mirrors production defaults.
	comicSvc := comicify.NewService(gen, userRepo, 1, media, http.DefaultClient)

	assetSvc := asset.NewService(asset.NewRepo(d), bookRepo)
	assetHandler := asset.NewHandler(assetSvc, media, comicSvc)

	chapterSvc := chapter.NewService(chapter.NewRepo(d), bookRepo)
	chapterHandler := chapter.NewHandler(chapterSvc)

	panelSvc := panel.NewService(panel.NewRepo(d), chapterSvc)
	panelHandler := panel.NewHandler(panelSvc)

	// The story handler needs a concrete *ai.Client to construct; the AI-free
	// flow never invokes it, so a zero-value client (no key) is fine.
	storySvc := story.NewService(&ai.Client{}, assetSvc, chapterSvc, panelSvc)
	storyHandler := story.NewHandler(storySvc)

	renderSvc := render.NewService(
		gen, panelSvc, chapterSvc, assetSvc, bookRepo,
		userRepo, 1, media, http.DefaultClient, 10,
	)
	renderHandler := render.NewHandler(renderSvc)

	router := httpx.NewRouter(httpx.Deps{
		Session: sess,
		Auth:    authHandler,
		Book:    bookHandler,
		Asset:   assetHandler,
		Chapter: chapterHandler,
		Panel:   panelHandler,
		Story:   storyHandler,
		Render:  renderHandler,
		Media:   media.Handler(),
		Static:  nil,
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &testApp{
		server:    srv,
		validator: newContractValidator(t),
		db:        d,
		gen:       gen,
	}
}

// contractValidator loads the OpenAPI document and routes/validates responses.
type contractValidator struct {
	t      *testing.T
	doc    *openapi3.T
	router routers.Router
}

func newContractValidator(t *testing.T) *contractValidator {
	t.Helper()
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("load openapi: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("openapi self-validation: %v", err)
	}
	r, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}
	return &contractValidator{t: t, doc: doc, router: r}
}

// validate checks that the response for req/resp conforms to the contract. It
// buffers and restores the response body so the caller can still read it.
func (v *contractValidator) validate(req *http.Request, resp *http.Response, body []byte) {
	v.t.Helper()

	route, pathParams, err := v.router.FindRoute(req)
	if err != nil {
		v.t.Fatalf("no contract route for %s %s: %v", req.Method, req.URL.Path, err)
	}

	reqInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			// Cookie auth is validated at the app layer; the contract only
			// declares the scheme. Accept it here without a real check.
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
	}

	respInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 resp.StatusCode,
		Header:                 resp.Header,
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true,
			AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
		},
	}
	respInput.SetBodyBytes(body)

	if err := openapi3filter.ValidateResponse(context.Background(), respInput); err != nil {
		v.t.Fatalf("contract violation for %s %s -> %d: %v\nbody: %s",
			req.Method, req.URL.Path, resp.StatusCode, err, string(body))
	}
}

// call issues an HTTP request, validates the response against the contract, and
// returns the status code and body bytes. cookie may be empty.
func (a *testApp) call(t *testing.T, method, path, cookie string, body any) (int, []byte) {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, a.server.URL+path, reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := a.server.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	// Rebuild an equivalent *http.Request (without the server URL host) for the
	// contract router, which matches on path + method.
	valReq, _ := http.NewRequest(method, path, nil)
	a.validator.validate(valReq, resp, raw)

	return resp.StatusCode, raw
}

// TestContractEndToEnd walks the AI-free flow and validates every response.
func TestContractEndToEnd(t *testing.T) {
	app := newTestApp(t)

	// --- register user A ---
	code, _ := app.call(t, http.MethodPost, "/api/v1/register", "",
		map[string]string{"nickname": "小明", "password": "pw123456"})
	if code != http.StatusCreated {
		t.Fatalf("register: want 201, got %d", code)
	}

	// --- login user A (capture cookie) ---
	cookie := app.login(t, "小明", "pw123456")

	// --- GET /me: credits must be 100 ---
	code, body := app.call(t, http.MethodGet, "/api/v1/me", cookie, nil)
	if code != http.StatusOK {
		t.Fatalf("me: want 200, got %d: %s", code, body)
	}
	var me models.User
	mustJSON(t, body, &me)
	if me.Credits != 100 {
		t.Fatalf("me: want credits 100, got %d", me.Credits)
	}

	// --- create a book ---
	code, body = app.call(t, http.MethodPost, "/api/v1/books", cookie,
		map[string]string{"title": "星星的故事"})
	if code != http.StatusCreated {
		t.Fatalf("create book: want 201, got %d: %s", code, body)
	}
	var b models.Book
	mustJSON(t, body, &b)

	// --- list + get book ---
	if code, _ = app.call(t, http.MethodGet, "/api/v1/books", cookie, nil); code != http.StatusOK {
		t.Fatalf("list books: want 200, got %d", code)
	}
	bookPath := "/api/v1/books/" + itoa(b.ID)
	if code, _ = app.call(t, http.MethodGet, bookPath, cookie, nil); code != http.StatusOK {
		t.Fatalf("get book: want 200, got %d", code)
	}

	// --- create a character (no image upload -> comicify never runs) ---
	code, body = app.call(t, http.MethodPost, bookPath+"/characters", cookie,
		map[string]string{"type": "character", "name": "狐狸", "description": "聪明"})
	if code != http.StatusCreated {
		t.Fatalf("create character: want 201, got %d: %s", code, body)
	}
	var char models.Character
	mustJSON(t, body, &char)

	// --- regenerate a character with no locked image -> 400 (validated envelope) ---
	// This exercises the new regenerate contract path without any AI call.
	code, body = app.call(t, http.MethodPost,
		bookPath+"/characters/"+itoa(char.ID)+"/regenerate", cookie, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("regenerate without image: want 400, got %d: %s", code, body)
	}
	if app.gen.calls != 0 {
		t.Fatalf("regenerate must not call generator without a local image, calls=%d", app.gen.calls)
	}

	// --- create a chapter ---
	code, body = app.call(t, http.MethodPost, bookPath+"/chapters", cookie,
		map[string]string{"title": "第一章"})
	if code != http.StatusCreated {
		t.Fatalf("create chapter: want 201, got %d: %s", code, body)
	}
	var ch models.Chapter
	mustJSON(t, body, &ch)
	chPath := "/api/v1/chapters/" + itoa(ch.ID)

	// --- replace panels (bulk) so we have a panel to render ---
	code, body = app.call(t, http.MethodPut, chPath+"/panels", cookie,
		[]models.Panel{{Content: "小狐狸出门", Caption: "出发"}})
	if code != http.StatusOK {
		t.Fatalf("replace panels: want 200, got %d: %s", code, body)
	}
	var panels []models.Panel
	mustJSON(t, body, &panels)
	if len(panels) != 1 {
		t.Fatalf("want 1 panel, got %d", len(panels))
	}
	panelID := panels[0].ID

	// --- list panels ---
	if code, _ = app.call(t, http.MethodGet, chPath+"/panels", cookie, nil); code != http.StatusOK {
		t.Fatalf("list panels: want 200, got %d", code)
	}

	// --- isolation: user B cannot see A's book -> 404 (validated envelope) ---
	if code, _ = app.call(t, http.MethodPost, "/api/v1/register", "",
		map[string]string{"nickname": "小红", "password": "pw654321"}); code != http.StatusCreated {
		t.Fatalf("register B: want 201, got %d", code)
	}
	cookieB := app.login(t, "小红", "pw654321")
	code, body = app.call(t, http.MethodGet, bookPath, cookieB, nil)
	if code != http.StatusNotFound {
		t.Fatalf("cross-user get: want 404, got %d: %s", code, body)
	}
	var isoErr map[string]string
	mustJSON(t, body, &isoErr)
	if isoErr["error"] == "" {
		t.Fatalf("cross-user 404 missing error envelope: %s", body)
	}

	// --- 402: force A's credits to 0, render must be 402 and NOT call generator ---
	if _, err := app.db.Exec("UPDATE users SET credits = 0 WHERE nickname = ?", "小明"); err != nil {
		t.Fatalf("zero credits: %v", err)
	}
	renderPath := "/api/v1/panels/" + itoa(panelID) + "/render"
	code, body = app.call(t, http.MethodPost, renderPath, cookie, nil)
	if code != http.StatusPaymentRequired {
		t.Fatalf("render with 0 credits: want 402, got %d: %s", code, body)
	}
	if app.gen.calls != 0 {
		t.Fatalf("image generator must not be called on 402, calls=%d", app.gen.calls)
	}
}

// login posts credentials and returns the session cookie header value.
func (a *testApp) login(t *testing.T, nickname, password string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, a.server.URL+"/api/v1/login",
		bytes.NewReader(mustMarshal(t, map[string]string{"nickname": nickname, "password": password})))
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.server.Client().Do(req)
	if err != nil {
		t.Fatalf("login do: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	// Validate the login response against the contract too.
	valReq, _ := http.NewRequest(http.MethodPost, "/api/v1/login", nil)
	a.validator.validate(valReq, resp, raw)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: want 200, got %d: %s", resp.StatusCode, raw)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			return c.Name + "=" + c.Value
		}
	}
	t.Fatal("login: no session cookie")
	return ""
}

func mustJSON(t *testing.T, data []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("unmarshal %T: %v (body: %s)", dst, err, string(data))
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
