package render

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/asset"
	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/panel"
	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// errGen is a sentinel generator error carrying a would-be-sensitive substring,
// used to assert the handler's generic 502 never leaks upstream detail.
var errGen = errors.New("secret-upstream boom sk-leak")

// pngBytes is a minimal valid 1x1 PNG. http.DetectContentType sniffs it as
// image/png so the stored file gets a .png extension.
var pngBytes = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

// fakeGen is a test ImageGenerator implementing BOTH methods. It records the
// prompt and refs it received (race-safe), which method was called, and returns
// a preconfigured URL and error.
type fakeGen struct {
	mu       sync.Mutex
	url      string
	err      error
	prompt   string
	refs     []string
	calls    int // total calls across both methods
	t2iCalls int // GenerateImage (text2image fallback)
	refCalls int // RenderWithRefs (multi-image edit)
}

// GenerateImage is the text2image fallback path.
func (g *fakeGen) GenerateImage(_ context.Context, prompt string, refs []string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	g.t2iCalls++
	g.prompt = prompt
	g.refs = refs
	return g.url, g.err
}

// RenderWithRefs is the multi-image edit path.
func (g *fakeGen) RenderWithRefs(_ context.Context, prompt string, refs []string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	g.refCalls++
	g.prompt = prompt
	g.refs = refs
	return g.url, g.err
}

func (g *fakeGen) lastPrompt() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.prompt
}

func (g *fakeGen) lastRefs() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.refs
}

func (g *fakeGen) counts() (total, t2i, ref int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls, g.t2iCalls, g.refCalls
}

// renderTestEnv bundles a render.Service wired to real asset/chapter/panel
// services over an in-memory DB, a fake image generator, and an httptest image
// server serving PNG bytes.
type renderTestEnv struct {
	svc      *Service
	gen      *fakeGen
	panels   *panel.Service
	chapters *chapter.Service
	assets   *asset.Service
	books    *book.Repo
	store    storage.Local
	db       *sql.DB
	imgSrv   *httptest.Server
}

// newRenderTestEnv opens an in-memory DB, seeds two users, wires the full stack,
// and stands up an httptest server that serves pngBytes. The fake generator is
// pointed at that server's URL. store roots media under a temp dir.
func newRenderTestEnv(t *testing.T) *renderTestEnv {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	seedUsers(t, d, 2)

	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	t.Cleanup(imgSrv.Close)

	bookRepo := book.NewRepo(d)
	chapterSvc := chapter.NewService(chapter.NewRepo(d), bookRepo)
	panelSvc := panel.NewService(panel.NewRepo(d), chapterSvc)
	assetSvc := asset.NewService(asset.NewRepo(d), bookRepo)
	store := storage.Local{Root: t.TempDir()}

	gen := &fakeGen{url: imgSrv.URL + "/image.png"}
	svc := NewService(gen, panelSvc, chapterSvc, assetSvc, store, imgSrv.Client(), 4)

	return &renderTestEnv{
		svc:      svc,
		gen:      gen,
		panels:   panelSvc,
		chapters: chapterSvc,
		assets:   assetSvc,
		books:    bookRepo,
		store:    store,
		db:       d,
		imgSrv:   imgSrv,
	}
}

// seedUsers inserts n users so books referencing user ids 1..n satisfy the FK.
func seedUsers(t *testing.T, d *sql.DB, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		if _, err := d.Exec(
			`INSERT INTO users (nickname, password_hash) VALUES (?, ?)`,
			"user"+string(rune('0'+i)), "hash",
		); err != nil {
			t.Fatalf("seed user %d: %v", i, err)
		}
	}
}

// seededPanel bundles the ids produced by seedPanel for a test to render.
type seededPanel struct {
	bookID  int64
	panelID int64
	charID  int64
	sceneID int64
}

// seedPanel creates, for userID: a book, one character, one scene, a chapter, and
// a single panel referencing that character and scene. It returns the ids.
func (e *renderTestEnv) seedPanel(t *testing.T, userID int64) seededPanel {
	t.Helper()
	b, err := e.books.Create(userID, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	// Store real local reference images so RenderPanel can read them back and
	// base64-encode them (the /media URL is what buildPrompt collects).
	charRef, err := e.store.SaveBytes(b.ID, ".png", pngBytes)
	if err != nil {
		t.Fatalf("save char ref: %v", err)
	}
	sceneRef, err := e.store.SaveBytes(b.ID, ".png", pngBytes)
	if err != nil {
		t.Fatalf("save scene ref: %v", err)
	}
	ch, err := e.assets.CreateCharacter(userID, b.ID, models.Character{
		Name:        "小龙",
		Gender:      "男",
		Age:         "7",
		Personality: "勇敢",
		ImageURL:    charRef,
	})
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	sc, err := e.assets.CreateScene(userID, b.ID, models.Scene{
		Name:        "森林",
		Description: "阳光洒落的树林",
		ImageURL:    sceneRef,
	})
	if err != nil {
		t.Fatalf("create scene: %v", err)
	}
	chap, err := e.chapters.CreateChapter(userID, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	panels, err := e.panels.ReplacePanels(userID, chap.ID, []models.Panel{
		{Caption: "小龙在森林里奔跑", CharacterIDs: []int64{ch.ID}, SceneID: sc.ID},
	})
	if err != nil {
		t.Fatalf("replace panels: %v", err)
	}
	return seededPanel{bookID: b.ID, panelID: panels[0].ID, charID: ch.ID, sceneID: sc.ID}
}

// TestRenderPanelHappyPath verifies a full successful render: prompt built,
// image downloaded and stored under /media/{bookID}/, status advanced to done.
func TestRenderPanelHappyPath(t *testing.T) {
	env := newRenderTestEnv(t)
	sp := env.seedPanel(t, 1)

	updated, err := env.svc.RenderPanel(context.Background(), 1, sp.panelID)
	if err != nil {
		t.Fatalf("render panel: %v", err)
	}
	if updated.Status != "done" {
		t.Fatalf("status should be done, got %q", updated.Status)
	}
	if updated.ImageURL == "" {
		t.Fatalf("image url should be set")
	}
	wantPrefix := "/media/" + strconv.FormatInt(sp.bookID, 10)
	if !strings.HasPrefix(updated.ImageURL, wantPrefix) {
		t.Fatalf("image url %q should be under %q", updated.ImageURL, wantPrefix)
	}
	if !strings.HasSuffix(updated.ImageURL, ".png") {
		t.Fatalf("image url %q should end in .png", updated.ImageURL)
	}

	// Prompt must contain the style prefix, the caption, and the matched
	// character name.
	prompt := env.gen.lastPrompt()
	if !strings.Contains(prompt, stylePrefix) {
		t.Fatalf("prompt missing style prefix: %q", prompt)
	}
	if !strings.Contains(prompt, "小龙在森林里奔跑") {
		t.Fatalf("prompt missing caption: %q", prompt)
	}
	if !strings.Contains(prompt, "小龙") {
		t.Fatalf("prompt missing character name: %q", prompt)
	}

	// With ≥1 matched reference, the multi-image edit path must be used (not
	// the text2image fallback).
	total, t2i, ref := env.gen.counts()
	if ref != 1 || t2i != 0 || total != 1 {
		t.Fatalf("expected exactly one RenderWithRefs call, got total=%d t2i=%d ref=%d", total, t2i, ref)
	}

	// References are forwarded as base64 data: URIs (character + scene => 2).
	refs := env.gen.lastRefs()
	if len(refs) != 2 {
		t.Fatalf("expected 2 reference data URIs (char + scene), got %d: %v", len(refs), refs)
	}
	for i, r := range refs {
		if !strings.HasPrefix(r, "data:image/") || !strings.Contains(r, ";base64,") {
			t.Fatalf("ref[%d] is not a base64 data URI: %.40q", i, r)
		}
	}
}

// TestRenderPanelNoRefsFallsBackToText2Image verifies a panel with no matched
// characters or scene uses GenerateImage (text2image) rather than the edit
// endpoint, and still completes.
func TestRenderPanelNoRefsFallsBackToText2Image(t *testing.T) {
	env := newRenderTestEnv(t)
	b, err := env.books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	chap, err := env.chapters.CreateChapter(1, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	panels, err := env.panels.ReplacePanels(1, chap.ID, []models.Panel{
		{Caption: "空旷的天空"}, // no CharacterIDs, no SceneID
	})
	if err != nil {
		t.Fatalf("replace panels: %v", err)
	}

	updated, err := env.svc.RenderPanel(context.Background(), 1, panels[0].ID)
	if err != nil {
		t.Fatalf("render panel: %v", err)
	}
	if updated.Status != "done" {
		t.Fatalf("status should be done, got %q", updated.Status)
	}
	total, t2i, ref := env.gen.counts()
	if t2i != 1 || ref != 0 || total != 1 {
		t.Fatalf("expected exactly one GenerateImage fallback, got total=%d t2i=%d ref=%d", total, t2i, ref)
	}
	if len(env.gen.lastRefs()) != 0 {
		t.Fatalf("text2image fallback should receive no refs, got %v", env.gen.lastRefs())
	}
}

// TestRenderPanelCapsRefs verifies that when more references match than maxRefs,
// only maxRefs data URIs are forwarded (characters first, so priority is kept).
func TestRenderPanelCapsRefs(t *testing.T) {
	env := newRenderTestEnv(t)
	// Rebuild the service with a tight cap of 2 references.
	env.svc = NewService(env.gen, env.panels, env.chapters, env.assets, env.store, env.imgSrv.Client(), 2)

	b, err := env.books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	// Five characters, each with a stored local reference image + a scene: 6
	// candidate refs, capped to 2.
	charIDs := make([]int64, 0, 5)
	for i := 0; i < 5; i++ {
		ref, err := env.store.SaveBytes(b.ID, ".png", pngBytes)
		if err != nil {
			t.Fatalf("save char ref: %v", err)
		}
		c, err := env.assets.CreateCharacter(1, b.ID, models.Character{
			Name:     "角色" + strconv.Itoa(i),
			ImageURL: ref,
		})
		if err != nil {
			t.Fatalf("create character: %v", err)
		}
		charIDs = append(charIDs, c.ID)
	}
	sceneRef, err := env.store.SaveBytes(b.ID, ".png", pngBytes)
	if err != nil {
		t.Fatalf("save scene ref: %v", err)
	}
	sc, err := env.assets.CreateScene(1, b.ID, models.Scene{Name: "场景", ImageURL: sceneRef})
	if err != nil {
		t.Fatalf("create scene: %v", err)
	}
	chap, err := env.chapters.CreateChapter(1, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	panels, err := env.panels.ReplacePanels(1, chap.ID, []models.Panel{
		{Caption: "众人集合", CharacterIDs: charIDs, SceneID: sc.ID},
	})
	if err != nil {
		t.Fatalf("replace panels: %v", err)
	}

	if _, err := env.svc.RenderPanel(context.Background(), 1, panels[0].ID); err != nil {
		t.Fatalf("render panel: %v", err)
	}
	refs := env.gen.lastRefs()
	if len(refs) != 2 {
		t.Fatalf("expected refs capped to 2, got %d", len(refs))
	}
	_, t2i, ref := env.gen.counts()
	if ref != 1 || t2i != 0 {
		t.Fatalf("expected RenderWithRefs path, got t2i=%d ref=%d", t2i, ref)
	}
}

// TestRenderPanelGenError verifies a generator error resets status to "failed"
// and returns the error.
func TestRenderPanelGenError(t *testing.T) {
	env := newRenderTestEnv(t)
	env.gen.err = errors.New("upstream boom")
	sp := env.seedPanel(t, 1)

	if _, err := env.svc.RenderPanel(context.Background(), 1, sp.panelID); err == nil {
		t.Fatal("expected error from generator failure")
	}

	got, err := env.panels.GetPanel(1, sp.panelID)
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("status should be failed, got %q", got.Status)
	}
}

// TestRenderPanelDownloadError verifies a non-2xx download resets status to
// "failed" and returns an error.
func TestRenderPanelDownloadError(t *testing.T) {
	env := newRenderTestEnv(t)
	// Point the generator at a URL the image server answers with 404.
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	t.Cleanup(badSrv.Close)
	env.gen.url = badSrv.URL + "/missing.png"
	sp := env.seedPanel(t, 1)

	if _, err := env.svc.RenderPanel(context.Background(), 1, sp.panelID); err == nil {
		t.Fatal("expected error from download failure")
	}
	got, err := env.panels.GetPanel(1, sp.panelID)
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("status should be failed, got %q", got.Status)
	}
}

// TestRenderPanelCrossUser verifies user 2 cannot render user 1's panel; the
// gate surfaces render.ErrNotFound and the generator is never called.
func TestRenderPanelCrossUser(t *testing.T) {
	env := newRenderTestEnv(t)
	sp := env.seedPanel(t, 1)

	if _, err := env.svc.RenderPanel(context.Background(), 2, sp.panelID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user render should return ErrNotFound, got %v", err)
	}
	if env.gen.calls != 0 {
		t.Fatalf("generator must not be called on cross-user render, calls=%d", env.gen.calls)
	}
}

// TestRenderPanelUnknown verifies an unknown panel id returns ErrNotFound.
func TestRenderPanelUnknown(t *testing.T) {
	env := newRenderTestEnv(t)
	if _, err := env.svc.RenderPanel(context.Background(), 1, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown panel render should return ErrNotFound, got %v", err)
	}
}
