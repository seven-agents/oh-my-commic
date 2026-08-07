package comicify

import (
	"context"
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

// fakeEditor records the prompt + refs it was called with and returns a fixed
// remote URL (pointing at the test image server).
type fakeEditor struct {
	remoteURL string
	gotPrompt string
	gotRefs   []string
}

func (f *fakeEditor) EditImage(_ context.Context, prompt string, refs []string) (string, error) {
	f.gotPrompt = prompt
	f.gotRefs = refs
	return f.remoteURL, nil
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
	svc := NewService(editor, store, imgSrv.Client())

	c := models.Character{Name: "阿黄", Gender: "男孩", Personality: "勇敢"}
	newURL, err := svc.Character(context.Background(), 3, c, srcURL)
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

func TestSceneComicifies(t *testing.T) {
	store := storage.Local{Root: t.TempDir()}
	srcURL, err := store.SaveBytes(8, ".png", pngBytes)
	if err != nil {
		t.Fatal(err)
	}

	imgSrv := newImageServer(t)
	defer imgSrv.Close()

	editor := &fakeEditor{remoteURL: imgSrv.URL + "/bg.png"}
	svc := NewService(editor, store, imgSrv.Client())

	sc := models.Scene{Name: "魔法森林", Description: "夜晚发光的蘑菇"}
	newURL, err := svc.Scene(context.Background(), 8, sc, srcURL)
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
