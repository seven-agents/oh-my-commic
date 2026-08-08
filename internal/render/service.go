// Package render orchestrates single-panel image generation: it builds a
// Ghibli-style prompt from a panel's caption plus its matched characters and
// scene, calls an image generator, downloads the produced image, stores it
// locally under the owning book, and writes the local URL back onto the panel.
//
// Every operation is ownership-gated (panel → chapter → book): a caller can only
// render a panel that (transitively) belongs to them; any cross-user or unknown
// panel surfaces as ErrNotFound so existence never leaks. On any generation or
// download failure the panel's status is reset to "failed" (never left
// "rendering"), and upstream/API detail is wrapped but never surfaced to clients
// by the handler.
package render

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/seven-agents/oh-my-commic/internal/asset"
	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/imageutil"
	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/panel"
	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// ErrNotFound is returned when the target panel does not exist or is not owned by
// the caller. It mirrors panel.ErrNotFound so the handler can map it to 404.
var ErrNotFound = errors.New("not found")

// ErrInsufficientCredits is returned when the caller's credit balance is too low
// to pay for the image generation. The handler maps it to HTTP 402.
var ErrInsufficientCredits = errors.New("insufficient credits")

// stylePrefix is the fixed Ghibli / Miyazaki storybook art-direction prefix
// prepended to every generated prompt. Kept as a constant so the visual style
// stays consistent across every panel of every book. The no-text constraint
// keeps the image model from stamping garbled letters/watermarks into the art.
const stylePrefix = "吉卜力/宫崎骏风格：手绘水彩、暖色调、柔和光影、圆润造型、亲子友好绘本风。画面中不要出现任何文字、字母、数字或水印。画面内容铺满整幅，四周不要任何边框、画框、描边或色条。"

// maxDownloadBytes caps how many bytes we read from the remote image to bound
// memory and guard against a hostile or misbehaving upstream (~15MB).
const maxDownloadBytes = 15 << 20

// statusRendering marks a panel whose image generation is in progress.
const statusRendering = "rendering"

// statusFailed marks a panel whose image generation failed.
const statusFailed = "failed"

// defaultMaxRefs caps reference images per render when NewService is given a
// non-positive maxRefs (defensive fallback so the model is never over-fed).
const defaultMaxRefs = 10

// defaultCost is the credit charge per render when NewService is given a
// non-positive cost (defensive fallback so a render is never accidentally free).
const defaultCost = 1

// modelMaxRefs is the hard upper bound the image model accepts. Seedream 4.0
// accepts up to 10 reference images per request, so we clamp to this regardless
// of how maxRefs is configured — over-feeding causes a 400 → render failure.
const modelMaxRefs = 10

// ImageGenerator produces a remote image URL from a prompt plus zero or more
// reference images. It is satisfied by *ai.Client via SeedreamImage. The Service
// depends on this narrow interface (defined where it is used) so tests can inject
// a fake without any real network calls.
//
// refImageDataURIs are base64 data: URIs of the matched characters/scene driving
// visual consistency; an empty list yields pure text-to-image.
type ImageGenerator interface {
	SeedreamImage(ctx context.Context, prompt string, refImageDataURIs []string) (string, error)
}

// CreditLedger charges and refunds a user's image-generation credits. It is
// satisfied by *auth.UserRepo (Spend/Refund). Defined here (where it is used) so
// the render Service depends only on the narrow interface and tests can inject a
// fake without any database.
//
// Spend deducts cost credits atomically, returning ok=false when the balance is
// insufficient (no charge made). Refund returns amount credits, used to undo a
// prior successful Spend when the paid-for generation ultimately fails.
type CreditLedger interface {
	Spend(userID int64, cost int) (bool, error)
	Refund(userID int64, amount int) error
}

// CoverSetter sets a book's cover image URL, scoped to the owning user. It is
// satisfied by *book.Repo via SetCover. The Service depends on this narrow
// interface (defined where it is used) so a cover chapter's rendered panel can
// be mirrored onto book.cover_url without a hard dependency on book internals.
type CoverSetter interface {
	SetCover(userID, bookID int64, coverURL string) (models.Book, error)
}

// Service orchestrates the render-one-panel flow.
type Service struct {
	gen      ImageGenerator
	panels   *panel.Service
	chapters *chapter.Service
	assets   *asset.Service
	cover    CoverSetter
	ledger   CreditLedger
	cost     int
	store    storage.Local
	http     *http.Client
	maxRefs  int
}

// NewService wires a Service to its collaborators. httpClient is used only to
// download the generated image; callers should set a sane timeout on it.
// maxRefs caps how many reference images are forwarded to the multi-image edit
// model; a non-positive value falls back to defaultMaxRefs.
// ledger charges cost credits before each generation and refunds on failure; a
// non-positive cost falls back to defaultCost so a misconfiguration never makes
// rendering free.
func NewService(
	gen ImageGenerator,
	panels *panel.Service,
	chapters *chapter.Service,
	assets *asset.Service,
	cover CoverSetter,
	ledger CreditLedger,
	cost int,
	store storage.Local,
	httpClient *http.Client,
	maxRefs int,
) *Service {
	if maxRefs <= 0 {
		maxRefs = defaultMaxRefs
	}
	if maxRefs > modelMaxRefs {
		maxRefs = modelMaxRefs // model hard limit: never send more than 10 images
	}
	if cost <= 0 {
		cost = defaultCost
	}
	return &Service{
		gen:      gen,
		panels:   panels,
		chapters: chapters,
		assets:   assets,
		cover:    cover,
		ledger:   ledger,
		cost:     cost,
		store:    store,
		http:     httpClient,
		maxRefs:  maxRefs,
	}
}

// RenderPanel generates, downloads, stores, and writes back the image for a
// single panel owned by userID, returning the updated panel with its local
// ImageURL set and status "done".
//
// Steps: ownership-gated load → resolve chapter/book → build prompt + reference
// images from the matched characters and scene → mark status "rendering" → call
// the generator → download the image (size-capped, ctx-aware) → store under the
// book → set the panel image (status → "done"). On any generator or download
// failure the panel status is set to "failed" and a wrapped error is returned.
func (s *Service) RenderPanel(ctx context.Context, userID, panelID int64) (models.Panel, error) {
	p, err := s.panels.GetPanel(userID, panelID)
	if err != nil {
		if errors.Is(err, panel.ErrNotFound) {
			return models.Panel{}, ErrNotFound
		}
		return models.Panel{}, fmt.Errorf("render panel %d: load panel: %w", panelID, err)
	}

	ch, err := s.chapters.GetChapter(userID, p.ChapterID)
	if err != nil {
		if errors.Is(err, chapter.ErrNotFound) {
			return models.Panel{}, ErrNotFound
		}
		return models.Panel{}, fmt.Errorf("render panel %d: load chapter: %w", panelID, err)
	}

	prompt, refs, err := s.buildPrompt(userID, ch.BookID, p)
	if err != nil {
		return models.Panel{}, fmt.Errorf("render panel %d: build prompt: %w", panelID, err)
	}

	// Charge the caller BEFORE spending any image-API quota. The deduction is
	// atomic: an insufficient balance is rejected here (generator never called),
	// and a successful charge is refunded if any later step fails.
	ok, err := s.ledger.Spend(userID, s.cost)
	if err != nil {
		return models.Panel{}, fmt.Errorf("render panel %d: spend credits: %w", panelID, err)
	}
	if !ok {
		return models.Panel{}, ErrInsufficientCredits
	}

	if _, err := s.panels.SetPanelStatus(userID, panelID, statusRendering); err != nil {
		s.refund(userID)
		return models.Panel{}, fmt.Errorf("render panel %d: set rendering: %w", panelID, err)
	}

	// Convert the local reference URLs into base64 data URIs the model can
	// consume (the image API cannot reach our /media host). Characters come first
	// in refs, scene last, so a simple slice cap preserves character priority.
	// Seedream handles both cases: an empty list is pure text-to-image, a
	// non-empty list drives the render from the reference images.
	dataURIs := s.refDataURIs(refs)

	remoteURL, err := s.gen.SeedreamImage(ctx, prompt, dataURIs)
	if err != nil {
		s.refund(userID)
		s.markFailed(userID, panelID)
		return models.Panel{}, fmt.Errorf("render panel %d: generate image: %w", panelID, err)
	}

	imgBytes, err := s.download(ctx, remoteURL)
	if err != nil {
		s.refund(userID)
		s.markFailed(userID, panelID)
		return models.Panel{}, fmt.Errorf("render panel %d: download image: %w", panelID, err)
	}

	ext := extForBytes(imgBytes)
	localURL, err := s.store.SaveBytes(ch.BookID, ext, imgBytes)
	if err != nil {
		s.refund(userID)
		s.markFailed(userID, panelID)
		return models.Panel{}, fmt.Errorf("render panel %d: save image: %w", panelID, err)
	}

	updated, err := s.panels.SetPanelImage(userID, panelID, localURL)
	if err != nil {
		return models.Panel{}, fmt.Errorf("render panel %d: set image: %w", panelID, err)
	}

	// If this panel belongs to a cover chapter, mirror its image onto the book's
	// cover_url. This is a best-effort side sync: a failure here must never fail
	// the render (the panel is already saved), so log and continue.
	if ch.IsCover {
		if _, err := s.cover.SetCover(userID, ch.BookID, localURL); err != nil {
			log.Printf("render: sync cover for book %d failed: %v", ch.BookID, err)
		}
	}
	return updated, nil
}

// refund best-effort returns the credits charged for a render that failed after
// the charge. A refund error is only logged (never surfaced): the caller is
// already returning the primary failure and must not have it masked, and the
// user losing at most one credit is preferable to failing the request twice.
func (s *Service) refund(userID int64) {
	if err := s.ledger.Refund(userID, s.cost); err != nil {
		log.Printf("render: refund %d credits to user %d failed: %v", s.cost, userID, err)
	}
}

// markFailed best-effort resets the panel status to "failed". Any error here is
// intentionally ignored: the caller is already returning the primary failure and
// must not have it masked by a secondary status-write error.
func (s *Service) markFailed(userID, panelID int64) {
	_, _ = s.panels.SetPanelStatus(userID, panelID, statusFailed)
}

// refDataURIs reads each local reference URL, downscales it, and encodes it as a
// base64 data URI suitable for the Seedream image model. A reference that
// cannot be read is skipped (logged, not fatal) so one bad asset never aborts
// the whole render. The result is capped at s.maxRefs, preserving input order
// (characters first, scene last) so character consistency is prioritized.
func (s *Service) refDataURIs(refs []string) []string {
	out := make([]string, 0, len(refs))
	for _, url := range refs {
		if len(out) >= s.maxRefs {
			break
		}
		raw, ext, err := s.store.ReadByURL(url)
		if err != nil {
			// Skip an unreadable reference rather than failing the render.
			log.Printf("render: skip unreadable reference image: %v", err)
			continue
		}
		resized, mime := imageutil.ResizeForReference(raw, imageutil.MimeFromExt(ext))
		out = append(out, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(resized))
	}
	return out
}

// buildPrompt loads the book's characters and scenes, selects the ones matched
// by the panel (character ids ∈ p.CharacterIDs, scene id == p.SceneID), and
// assembles the generation prompt and the reference image URL list.
func (s *Service) buildPrompt(userID, bookID int64, p models.Panel) (string, []string, error) {
	characters, err := s.assets.ListCharacters(userID, bookID)
	if err != nil {
		return "", nil, fmt.Errorf("list characters: %w", err)
	}
	scenes, err := s.assets.ListScenes(userID, bookID)
	if err != nil {
		return "", nil, fmt.Errorf("list scenes: %w", err)
	}

	matchedChars := matchCharacters(characters, p.CharacterIDs)
	matchedScene, hasScene := matchScene(scenes, p.SceneID)

	var b strings.Builder
	b.WriteString(stylePrefix)
	if p.Caption != "" {
		b.WriteString("画面：")
		b.WriteString(p.Caption)
		b.WriteString("。")
	}
	if p.Location != "" {
		b.WriteString("地点：")
		b.WriteString(p.Location)
		b.WriteString("。")
	}
	for _, c := range matchedChars {
		b.WriteString(characterSummary(c, p.CharExpressions[c.ID]))
	}
	if hasScene {
		b.WriteString(sceneSummary(matchedScene))
	}
	if p.Event != "" {
		b.WriteString("事件：")
		b.WriteString(p.Event)
		b.WriteString("。")
	}
	if p.ImagePrompt != "" {
		b.WriteString("补充：")
		b.WriteString(p.ImagePrompt)
		b.WriteString("。")
	}

	// Collect reference images together with a human label, in the SAME order,
	// so we can bind each numbered reference to its subject in the prompt.
	refs := make([]string, 0, len(matchedChars)+1)
	labels := make([]string, 0, len(matchedChars)+1)
	for _, c := range matchedChars {
		if c.ImageURL != "" {
			refs = append(refs, c.ImageURL)
			labels = append(labels, "角色"+c.Name)
		}
	}
	if hasScene && matchedScene.ImageURL != "" {
		refs = append(refs, matchedScene.ImageURL)
		labels = append(labels, "场景"+matchedScene.Name)
	}

	// Explicitly bind each numbered reference image to its subject. Passing
	// several images without saying which is which lets the multi-image edit
	// model swap or blend them, so the drawn characters drift from their locked
	// reference. Numbering the references and naming each one keeps every
	// character faithful to its own indexed image.
	if len(labels) > 0 {
		b.WriteString("本次提供了 ")
		b.WriteString(strconv.Itoa(len(labels)))
		b.WriteString(" 张参考图，请严格按参考图还原对应对象的样貌，画面中每个角色都要与它自己的参考图保持一致：")
		for i, label := range labels {
			b.WriteString(fmt.Sprintf("参考图%d=%s；", i+1, label))
		}
	}

	return b.String(), refs, nil
}

// download fetches remoteURL with the injected client, honoring ctx and capping
// the read at maxDownloadBytes. A non-2xx status or read failure is an error.
func (s *Service) download(ctx context.Context, remoteURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	// Read at most maxDownloadBytes+1 so we can detect an over-cap body.
	limited := io.LimitReader(resp.Body, maxDownloadBytes+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("empty image body")
	}
	if int64(len(b)) > maxDownloadBytes {
		return nil, fmt.Errorf("image exceeds %d bytes", maxDownloadBytes)
	}
	return b, nil
}

// matchCharacters returns the characters whose ids appear in want, preserving
// the book's character order.
func matchCharacters(all []models.Character, want []int64) []models.Character {
	if len(want) == 0 {
		return nil
	}
	wantSet := make(map[int64]bool, len(want))
	for _, id := range want {
		wantSet[id] = true
	}
	out := make([]models.Character, 0, len(want))
	for _, c := range all {
		if wantSet[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

// matchScene returns the scene with id sceneID and true, or a zero scene and
// false when sceneID is 0 or no such scene exists.
func matchScene(all []models.Scene, sceneID int64) (models.Scene, bool) {
	if sceneID == 0 {
		return models.Scene{}, false
	}
	for _, sc := range all {
		if sc.ID == sceneID {
			return sc, true
		}
	}
	return models.Scene{}, false
}

// characterSummary renders a short, non-empty-field summary of a character,
// including the panel-specific expression when one is provided.
func characterSummary(c models.Character, expression string) string {
	var b strings.Builder
	b.WriteString("角色")
	b.WriteString(c.Name)
	parts := make([]string, 0, 4)
	if c.Gender != "" {
		parts = append(parts, "性别"+c.Gender)
	}
	if c.Age != "" {
		parts = append(parts, "年龄"+c.Age)
	}
	if c.Personality != "" {
		parts = append(parts, "性格"+c.Personality)
	}
	if strings.TrimSpace(expression) != "" {
		parts = append(parts, "表情"+expression)
	}
	if len(parts) > 0 {
		b.WriteString("（")
		b.WriteString(strings.Join(parts, "、"))
		b.WriteString("）")
	}
	b.WriteString("。")
	return b.String()
}

// sceneSummary renders a short summary of a scene.
func sceneSummary(sc models.Scene) string {
	var b strings.Builder
	b.WriteString("场景")
	b.WriteString(sc.Name)
	if sc.Description != "" {
		b.WriteString("（")
		b.WriteString(sc.Description)
		b.WriteString("）")
	}
	b.WriteString("。")
	return b.String()
}

// extForBytes maps the sniffed content type to a stored image extension,
// defaulting to .png for anything we do not explicitly recognize.
func extForBytes(b []byte) string {
	switch http.DetectContentType(b) {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/png":
		return ".png"
	default:
		return ".png"
	}
}
