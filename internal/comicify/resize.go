package comicify

import (
	"bytes"
	"image"
	"image/jpeg"

	// Register decoders for the formats we accept as reference input. WebP is
	// decode-only, which is all we need here.
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// maxRefDimension caps the larger side of the reference image sent to the
// editor. A full-resolution upload (1024px+) makes the qwen-image-edit request
// slow enough to exceed the edit-client timeout; downscaling the *input*
// reference keeps the round-trip fast while leaving the model output at full
// resolution.
const maxRefDimension = 768

// jpegQuality is used when re-encoding a downscaled reference. JPEG is fine for
// a throwaway reference image and keeps the base64 payload small.
const jpegQuality = 85

// resizeReference decodes raw and, if its larger dimension exceeds
// maxRefDimension, downscales it (preserving aspect ratio, high-quality
// CatmullRom) and re-encodes it as JPEG. It returns the (possibly new) bytes
// and the MIME type to advertise in the data URI.
//
// It never fails the caller: if raw cannot be decoded (unknown format, corrupt
// data) or re-encoding hiccups, it falls back to the original bytes and the
// caller-provided origMime.
func resizeReference(raw []byte, origMime string) (out []byte, mime string) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return raw, origMime
	}

	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return raw, origMime
	}

	// Already within the cap: keep the original bytes+mime (no upscaling).
	if w <= maxRefDimension && h <= maxRefDimension {
		return raw, origMime
	}

	newW, newH := scaledDimensions(w, h, maxRefDimension)
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return raw, origMime
	}
	return buf.Bytes(), "image/jpeg"
}

// scaledDimensions returns the width/height that fit the larger side of w x h
// into cap while preserving aspect ratio. At least 1px per side is guaranteed.
func scaledDimensions(w, h, cap int) (int, int) {
	if w >= h {
		nh := int(float64(h) * float64(cap) / float64(w))
		if nh < 1 {
			nh = 1
		}
		return cap, nh
	}
	nw := int(float64(w) * float64(cap) / float64(h))
	if nw < 1 {
		nw = 1
	}
	return nw, cap
}
