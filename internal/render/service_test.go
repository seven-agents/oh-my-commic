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

// fakeGen is a test ImageGenerator. It records the prompt and refs it received
// (race-safe), how many times it was called, and returns a preconfigured URL and
// error. Seedream handles both text2image (no refs) and image-edit (refs) via a
// single method, so the fake has one call path.
type fakeGen struct {
	mu     sync.Mutex
	url    string
	err    error
	prompt string
	refs   []string
	calls  int
}

// SeedreamImage is the single image-generation path (refs present or empty).
func (g *fakeGen) SeedreamImage(_ context.Context, prompt string, refs []string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
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

func (g *fakeGen) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
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
	svc := NewService(gen, panelSvc, chapterSvc, assetSvc, bookRepo, store, imgSrv.Client(), 4)

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

// seedCoverPanel creates, for userID: a book, its cover chapter (via
// EnsureCover), and a single panel under that cover chapter. It returns the ids.
func (e *renderTestEnv) seedCoverPanel(t *testing.T, userID int64) seededPanel {
	t.Helper()
	b, err := e.books.Create(userID, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	cover, err := e.chapters.EnsureCover(userID, b.ID)
	if err != nil {
		t.Fatalf("ensure cover: %v", err)
	}
	panels, err := e.panels.ReplacePanels(userID, cover.ID, []models.Panel{
		{Caption: "这本书的封面"},
	})
	if err != nil {
		t.Fatalf("replace panels: %v", err)
	}
	return seededPanel{bookID: b.ID, panelID: panels[0].ID}
}

// TestRenderCoverPanelSyncsBookCover verifies that rendering a panel of a cover
// chapter mirrors the stored image onto the book's cover_url.
func TestRenderCoverPanelSyncsBookCover(t *testing.T) {
	env := newRenderTestEnv(t)
	sp := env.seedCoverPanel(t, 1)

	updated, err := env.svc.RenderPanel(context.Background(), 1, sp.panelID)
	if err != nil {
		t.Fatalf("render cover panel: %v", err)
	}

	b, err := env.books.Get(1, sp.bookID)
	if err != nil {
		t.Fatalf("get book: %v", err)
	}
	if b.CoverURL != updated.ImageURL {
		t.Fatalf("封面章渲染后 book.coverUrl 应同步为图片 %q, got %q", updated.ImageURL, b.CoverURL)
	}
	if b.CoverURL == "" {
		t.Fatalf("book.coverUrl 不应为空")
	}
}

// TestRenderNormalPanelDoesNotTouchCover verifies that rendering a panel of a
// non-cover chapter leaves the book's cover_url untouched.
func TestRenderNormalPanelDoesNotTouchCover(t *testing.T) {
	env := newRenderTestEnv(t)
	sp := env.seedPanel(t, 1)

	before, err := env.books.Get(1, sp.bookID)
	if err != nil {
		t.Fatalf("get book before: %v", err)
	}
	if _, err := env.svc.RenderPanel(context.Background(), 1, sp.panelID); err != nil {
		t.Fatalf("render panel: %v", err)
	}
	after, err := env.books.Get(1, sp.bookID)
	if err != nil {
		t.Fatalf("get book after: %v", err)
	}
	if after.CoverURL != before.CoverURL {
		t.Fatalf("普通章渲染不应改动 book.coverUrl, before=%q after=%q", before.CoverURL, after.CoverURL)
	}
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
	// The no-text constraint must be present so the model doesn't stamp garbled
	// letters/watermarks into the image.
	if !strings.Contains(prompt, "不要出现任何文字") {
		t.Fatalf("prompt missing no-text constraint: %q", prompt)
	}
	if !strings.Contains(prompt, "小龙在森林里奔跑") {
		t.Fatalf("prompt missing caption: %q", prompt)
	}
	if !strings.Contains(prompt, "小龙") {
		t.Fatalf("prompt missing character name: %q", prompt)
	}
	// Each reference image must be explicitly bound to its subject, in order
	// (character first, scene last), so the multi-image model maps them right.
	if !strings.Contains(prompt, "参考图1=角色小龙") {
		t.Fatalf("prompt missing reference-image binding for the character: %q", prompt)
	}
	if !strings.Contains(prompt, "参考图2=场景") {
		t.Fatalf("prompt missing reference-image binding for the scene: %q", prompt)
	}

	// Exactly one Seedream call regardless of ref count.
	if got := env.gen.count(); got != 1 {
		t.Fatalf("expected exactly one SeedreamImage call, got %d", got)
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

// TestRenderPanelNoRefsUsesText2Image verifies a panel with no matched
// characters or scene still renders via Seedream with an empty ref list
// (Seedream does text2image when no image refs are supplied).
func TestRenderPanelNoRefsUsesText2Image(t *testing.T) {
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
	if got := env.gen.count(); got != 1 {
		t.Fatalf("expected exactly one SeedreamImage call, got %d", got)
	}
	if len(env.gen.lastRefs()) != 0 {
		t.Fatalf("text2image (no refs) should receive no refs, got %v", env.gen.lastRefs())
	}
}

// TestRenderPanelCapsRefs verifies that when more references match than maxRefs,
// only maxRefs data URIs are forwarded (characters first, so priority is kept).
func TestRenderPanelCapsRefs(t *testing.T) {
	env := newRenderTestEnv(t)
	// Rebuild the service with a tight cap of 2 references.
	env.svc = NewService(env.gen, env.panels, env.chapters, env.assets, env.books, env.store, env.imgSrv.Client(), 2)

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
	if got := env.gen.count(); got != 1 {
		t.Fatalf("expected exactly one SeedreamImage call, got %d", got)
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
