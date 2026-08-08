// Package imageutil provides small, dependency-light helpers for preparing
// image bytes before they are sent to an image model as a reference.
package imageutil

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

// maxRefDimension caps the larger side of the reference image sent to an
// image-edit model. A full-resolution upload (1024px+) makes the request slow
// enough to exceed the edit-client timeout; downscaling the *input* reference
// keeps the round-trip fast while leaving the model output at full resolution.
const maxRefDimension = 768

// jpegQuality is used when re-encoding a downscaled reference. JPEG is fine for
// a throwaway reference image and keeps the base64 payload small.
const jpegQuality = 85

// ResizeForReference decodes raw and, if its larger dimension exceeds
// maxRefDimension, downscales it (preserving aspect ratio, high-quality
// CatmullRom) and re-encodes it as JPEG. It returns the (possibly new) bytes
// and the MIME type to advertise in the data URI.
//
// It never fails the caller: if raw cannot be decoded (unknown format, corrupt
// data) or re-encoding hiccups, it falls back to the original bytes and the
// caller-provided origMime.
func ResizeForReference(raw []byte, origMime string) (out []byte, mime string) {
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

// maxWebDimension caps the larger side of an image served to the browser. A
// generated comic panel is 2048px (~1.5MB); on a phone/laptop screen 1280px is
// visually indistinguishable but a fraction of the bytes.
const maxWebDimension = 1280

// webJPEGQuality re-encodes downscaled web images. 80 keeps comic art clean
// while cutting size hard versus the model's near-lossless output.
const webJPEGQuality = 80

// CompressForWeb downscales an oversized image and re-encodes it as JPEG for
// delivery to the browser, shrinking transfer size without touching the stored
// original. It only transforms images whose larger side exceeds
// maxWebDimension — small images (avatars, icons) are returned untouched so a
// PNG with transparency is never flattened into JPEG. changed reports whether a
// new, smaller encoding was produced.
//
// Like ResizeForReference it never fails the caller: an undecodable or
// re-encode-failing input falls back to the original bytes with changed=false.
func CompressForWeb(raw []byte, origMime string) (out []byte, mime string, changed bool) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return raw, origMime, false
	}

	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return raw, origMime, false
	}

	// Within the cap: leave it alone (no upscaling, no needless PNG→JPEG).
	if w <= maxWebDimension && h <= maxWebDimension {
		return raw, origMime, false
	}

	newW, newH := scaledDimensions(w, h, maxWebDimension)
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: webJPEGQuality}); err != nil {
		return raw, origMime, false
	}
	return buf.Bytes(), "image/jpeg", true
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

// MimeFromExt maps a stored file extension (with leading dot, any case) to its
// image MIME type, defaulting to image/png for anything unrecognized.
func MimeFromExt(ext string) string {
	switch ext {
	case ".jpg", ".jpeg", ".JPG", ".JPEG":
		return "image/jpeg"
	case ".webp", ".WEBP":
		return "image/webp"
	case ".png", ".PNG":
		return "image/png"
	default:
		return "image/png"
	}
}
