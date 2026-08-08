package comicify

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// pngBytes is a minimal payload that http.DetectContentType classifies as PNG
// (the 8-byte PNG signature is sufficient).
var pngBytes = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}

// makePNG builds an in-memory PNG of the given dimensions (so the encoder does
// real work) used to exercise the large-reference downscale path.
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// fakeEditor records the prompt + refs it was called with and returns a fixed
// remote URL (pointing at the test image server).
type fakeEditor struct {
	remoteURL string
	err       error
	gotPrompt string
	gotRefs   []string
}

func (f *fakeEditor) SeedreamImage(_ context.Context, prompt string, refs []string) (string, error) {
	f.gotPrompt = prompt
	f.gotRefs = refs
	if f.err != nil {
		return "", f.err
	}
	return f.remoteURL, nil
}

// fakeLedger is an in-memory CreditLedger for comicify tests. It records spend
// and refund counts and enforces the balance so an empty balance is rejected.
type fakeLedger struct {
	balance int
	spends  int
	refunds int
}

func (l *fakeLedger) Spend(_ int64, cost int) (bool, error) {
	l.spends++
	if l.balance < cost {
		return false, nil
	}
	l.balance -= cost
	return true, nil
}

func (l *fakeLedger) Refund(_ int64, amount int) error {
	l.refunds++
	l.balance += amount
	return nil
}

func newImageServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
}

func TestCharacterComicifies(t *testing.T) {
	store := storage.Local{Root: t.TempDir()}
	srcURL, err := store.SaveBytes(3, ".png", pngBytes)
	if err != nil {
		t.Fatal(err)
	}

	imgSrv := newImageServer(t)
	defer imgSrv.Close()

	editor := &fakeEditor{remoteURL: imgSrv.URL + "/x.png"}
	ledger := &fakeLedger{balance: 100}
	svc := NewService(editor, ledger, 1, store, imgSrv.Client())

	c := models.Character{Name: "阿黄", Gender: "男孩", Personality: "勇敢"}
	newURL, err := svc.Character(context.Background(), 7, 3, c, srcURL)
	if err != nil {
		t.Fatalf("Character: %v", err)
	}
	if !strings.HasPrefix(newURL, "/media/3/") {
		t.Fatalf("expected new /media/3 url, got %s", newURL)
	}
	if newURL == srcURL {
		t.Fatal("expected a new url distinct from source")
	}
	// The stylized bytes were saved and are readable back.
	if _, _, err := store.ReadByURL(newURL); err != nil {
		t.Fatalf("saved image not readable: %v", err)
	}
	// The editor received the source image as a base64 data URI plus a prompt
	// carrying the metadata.
	if len(editor.gotRefs) != 1 || !strings.HasPrefix(editor.gotRefs[0], "data:image/png;base64,") {
		t.Fatalf("editor ref not a png data uri: %v", editor.gotRefs)
	}
	if !strings.Contains(editor.gotPrompt, "阿黄") {
		t.Fatalf("prompt missing character name: %s", editor.gotPrompt)
	}
}

func TestCharacterDownscalesLargeReference(t *testing.T) {
	store := storage.Local{Root: t.TempDir()}
	large := makePNG(t, 1600, 1200)
	srcURL, err := store.SaveBytes(4, ".png", large)
	if err != nil {
		t.Fatal(err)
	}

	imgSrv := newImageServer(t)
	defer imgSrv.Close()

	editor := &fakeEditor{remoteURL: imgSrv.URL + "/x.png"}
	ledger := &fakeLedger{balance: 100}
	svc := NewService(editor, ledger, 1, store, imgSrv.Client())

	c := models.Character{Name: "小蓝", Gender: "女孩", Personality: "好奇"}
	if _, err := svc.Character(context.Background(), 7, 4, c, srcURL); err != nil {
		t.Fatalf("Character: %v", err)
	}
	// A large decodable reference is downscaled and re-encoded as JPEG.
	if len(editor.gotRefs) != 1 || !strings.HasPrefix(editor.gotRefs[0], "data:image/jpeg;base64,") {
		t.Fatalf("expected downscaled jpeg data uri, got prefix of: %.40q", editor.gotRefs)
	}
}

func TestSceneComicifies(t *testing.T) {
	store := storage.Local{Root: t.TempDir()}
	srcURL, err := store.SaveBytes(8, ".png", pngBytes)
	if err != nil {
		t.Fatal(err)
	}

	imgSrv := newImageServer(t)
	defer imgSrv.Close()

	editor := &fakeEditor{remoteURL: imgSrv.URL + "/bg.png"}
	ledger := &fakeLedger{balance: 100}
	svc := NewService(editor, ledger, 1, store, imgSrv.Client())

	sc := models.Scene{Name: "魔法森林", Description: "夜晚发光的蘑菇"}
	newURL, err := svc.Scene(context.Background(), 7, 8, sc, srcURL)
	if err != nil {
		t.Fatalf("Scene: %v", err)
	}
	if !strings.HasPrefix(newURL, "/media/8/") {
		t.Fatalf("expected new /media/8 url, got %s", newURL)
	}
	if !strings.Contains(editor.gotPrompt, "魔法森林") {
		t.Fatalf("prompt missing scene name: %s", editor.gotPrompt)
	}
	if !strings.Contains(editor.gotPrompt, "不要出现任何人物") {
		t.Fatalf("scene prompt should exclude characters: %s", editor.gotPrompt)
	}
}

// TestComicifyChargesOnSuccess verifies a successful comicify deducts one credit
// and never refunds.
func TestComicifyChargesOnSuccess(t *testing.T) {
	store := storage.Local{Root: t.TempDir()}
	srcURL, err := store.SaveBytes(3, ".png", pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	imgSrv := newImageServer(t)
	defer imgSrv.Close()

	editor := &fakeEditor{remoteURL: imgSrv.URL + "/x.png"}
	ledger := &fakeLedger{balance: 5}
	svc := NewService(editor, ledger, 1, store, imgSrv.Client())

	if _, err := svc.Character(context.Background(), 7, 3, models.Character{Name: "阿黄"}, srcURL); err != nil {
		t.Fatalf("Character: %v", err)
	}
	if ledger.spends != 1 || ledger.refunds != 0 || ledger.balance != 4 {
		t.Fatalf("success should spend once, not refund, balance 4: %+v", ledger)
	}
}

// TestComicifyInsufficientCreditsRejects verifies an empty balance returns
// ErrInsufficientCredits and never calls the image editor.
func TestComicifyInsufficientCreditsRejects(t *testing.T) {
	store := storage.Local{Root: t.TempDir()}
	srcURL, err := store.SaveBytes(3, ".png", pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	imgSrv := newImageServer(t)
	defer imgSrv.Close()

	editor := &fakeEditor{remoteURL: imgSrv.URL + "/x.png"}
	ledger := &fakeLedger{balance: 0}
	svc := NewService(editor, ledger, 1, store, imgSrv.Client())

	_, err = svc.Character(context.Background(), 7, 3, models.Character{Name: "阿黄"}, srcURL)
	if !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("empty balance should return ErrInsufficientCredits, got %v", err)
	}
	if editor.gotRefs != nil {
		t.Fatal("image editor must not be called when credits are insufficient")
	}
	if ledger.refunds != 0 {
		t.Fatalf("a rejected charge must not refund, refunds=%d", ledger.refunds)
	}
}

// TestComicifyRefundsOnEditError verifies an image-editor failure refunds the
// charged credit so the balance is restored.
func TestComicifyRefundsOnEditError(t *testing.T) {
	store := storage.Local{Root: t.TempDir()}
	srcURL, err := store.SaveBytes(3, ".png", pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	imgSrv := newImageServer(t)
	defer imgSrv.Close()

	editor := &fakeEditor{err: errors.New("upstream boom")}
	ledger := &fakeLedger{balance: 5}
	svc := NewService(editor, ledger, 1, store, imgSrv.Client())

	if _, err := svc.Character(context.Background(), 7, 3, models.Character{Name: "阿黄"}, srcURL); err == nil {
		t.Fatal("expected error from editor failure")
	}
	if ledger.spends != 1 || ledger.refunds != 1 || ledger.balance != 5 {
		t.Fatalf("edit failure should spend then refund, balance restored to 5: %+v", ledger)
	}
}
