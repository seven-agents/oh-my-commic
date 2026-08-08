package storage

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

// bigJPEG builds an in-memory JPEG of the given dimensions with a gradient so
// the encoder produces realistic (non-trivial) bytes.
func bigJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func smallPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func getMedia(t *testing.T, h http.Handler, url string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestHandlerCompressesLargeImage(t *testing.T) {
	s := Local{Root: t.TempDir()}
	raw := bigJPEG(t, 2048, 2048)
	url, err := s.SaveBytes(1, ".jpg", raw)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	h := s.Handler()

	resp := getMedia(t, h, url)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("content-type = %q, want image/jpeg", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=2592000" {
		t.Fatalf("cache-control = %q", cc)
	}
	body := readAll(t, resp)
	if len(body) >= len(raw) {
		t.Fatalf("expected compressed body smaller than %d, got %d", len(raw), len(body))
	}
	// Result must be a valid, downscaled image.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode served body: %v", err)
	}
	if cfg.Width > 1280 || cfg.Height > 1280 {
		t.Fatalf("served %dx%d exceeds web cap 1280", cfg.Width, cfg.Height)
	}
}

func TestHandlerFullParamServesOriginal(t *testing.T) {
	s := Local{Root: t.TempDir()}
	raw := bigJPEG(t, 2048, 2048)
	url, err := s.SaveBytes(1, ".jpg", raw)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	h := s.Handler()

	resp := getMedia(t, h, url+"?full=1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := readAll(t, resp)
	if !bytes.Equal(body, raw) {
		t.Fatalf("?full=1 should serve the original %d bytes, got %d", len(raw), len(body))
	}
}

func TestHandlerLeavesSmallImageUntouched(t *testing.T) {
	s := Local{Root: t.TempDir()}
	raw := smallPNG(t, 200, 200)
	url, err := s.SaveBytes(1, ".png", raw)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	h := s.Handler()

	resp := getMedia(t, h, url)
	body := readAll(t, resp)
	if !bytes.Equal(body, raw) {
		t.Fatal("small PNG should be served unchanged (no PNG->JPEG flattening)")
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=2592000" {
		t.Fatalf("cache-control = %q", cc)
	}
}

func readAll(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return buf.Bytes()
}
