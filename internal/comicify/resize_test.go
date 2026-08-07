package comicify

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
)

// makePNG builds an in-memory PNG of the given dimensions with a simple pattern
// (so the encoder does real work).
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

func TestResizeReferenceDownscalesLargeImage(t *testing.T) {
	raw := makePNG(t, 1600, 1200)
	out, mime := resizeReference(raw, "image/png")

	if mime != "image/jpeg" {
		t.Fatalf("expected downscaled mime image/jpeg, got %s", mime)
	}
	if bytes.Equal(out, raw) {
		t.Fatal("expected downscaled bytes to differ from original")
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode result config: %v", err)
	}
	if cfg.Width > maxRefDimension || cfg.Height > maxRefDimension {
		t.Fatalf("result %dx%d exceeds cap %d", cfg.Width, cfg.Height, maxRefDimension)
	}
	// Larger side should be pinned to the cap.
	if cfg.Width != maxRefDimension {
		t.Fatalf("expected width pinned to cap %d, got %d", maxRefDimension, cfg.Width)
	}
	// Aspect ratio preserved within rounding (4:3 -> 1600/1200 == 768/576).
	srcRatio := 1600.0 / 1200.0
	dstRatio := float64(cfg.Width) / float64(cfg.Height)
	if math.Abs(srcRatio-dstRatio) > 0.02 {
		t.Fatalf("aspect ratio drift: src %.4f dst %.4f", srcRatio, dstRatio)
	}
}

func TestResizeReferenceLeavesSmallImageUntouched(t *testing.T) {
	raw := makePNG(t, 300, 300)
	out, mime := resizeReference(raw, "image/png")

	if mime != "image/png" {
		t.Fatalf("expected original mime image/png, got %s", mime)
	}
	if !bytes.Equal(out, raw) {
		t.Fatal("small image should be returned unchanged (no upscale/re-encode)")
	}
}

func TestResizeReferenceFallsBackOnUndecodable(t *testing.T) {
	raw := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	out, mime := resizeReference(raw, "image/png")

	if mime != "image/png" {
		t.Fatalf("expected fallback mime image/png, got %s", mime)
	}
	if !bytes.Equal(out, raw) {
		t.Fatal("undecodable input should fall back to original bytes")
	}
}

func TestResizeReferencePortraitPinsHeight(t *testing.T) {
	raw := makePNG(t, 1000, 2000)
	out, _ := resizeReference(raw, "image/png")

	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode result config: %v", err)
	}
	if cfg.Height != maxRefDimension {
		t.Fatalf("expected height pinned to cap %d, got %d", maxRefDimension, cfg.Height)
	}
	if cfg.Width > maxRefDimension {
		t.Fatalf("width %d exceeds cap", cfg.Width)
	}
}
