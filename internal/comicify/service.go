// Package comicify redraws a raw uploaded asset image into a locked, Ghibli /
// Miyazaki storybook-style reference image via an image-edit model.
//
// The flow for both characters and scenes is: read the local upload → encode it
// as a base64 data URI (DashScope cannot reach a localhost /media URL) → build a
// style + metadata prompt → call the image editor → download the produced image
// (size-capped, ctx-aware) → store it under the same book → return the new local
// /media URL. The returned URL replaces the asset's imageUrl and becomes the
// consistency reference used by later panel rendering.
package comicify

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// maxDownloadBytes caps how many bytes we read from the produced image to bound
// memory against a hostile or misbehaving upstream (~15MB).
const maxDownloadBytes = 15 << 20

// ImageEditor redraws reference images into a new stylized image and returns its
// URL. It is satisfied by *ai.Client. Defined here (where it is used) so tests
// can inject a fake without any real network calls.
type ImageEditor interface {
	EditImage(ctx context.Context, prompt string, refImageDataURIs []string) (string, error)
}

// Service orchestrates the comic-ification of an uploaded asset image.
type Service struct {
	editor ImageEditor
	store  storage.Local
	http   *http.Client
}

// NewService wires a Service to the image editor, storage, and the HTTP client
// used to download the produced image (callers should set a sane timeout).
func NewService(editor ImageEditor, store storage.Local, httpClient *http.Client) *Service {
	return &Service{editor: editor, store: store, http: httpClient}
}

// Character comic-ifies the uploaded image at srcURL for character c, storing the
// stylized result under bookID and returning its new /media URL.
func (s *Service) Character(ctx context.Context, bookID int64, c models.Character, srcURL string) (string, error) {
	return s.run(ctx, bookID, srcURL, characterPrompt(c))
}

// Scene comic-ifies the uploaded image at srcURL for scene sc, storing the
// stylized result under bookID and returning its new /media URL.
func (s *Service) Scene(ctx context.Context, bookID int64, sc models.Scene, srcURL string) (string, error) {
	return s.run(ctx, bookID, srcURL, scenePrompt(sc))
}

// run performs the shared read → encode → edit → download → store pipeline.
func (s *Service) run(ctx context.Context, bookID int64, srcURL, prompt string) (string, error) {
	rawBytes, ext, err := s.store.ReadByURL(srcURL)
	if err != nil {
		return "", fmt.Errorf("comicify: read source: %w", err)
	}

	dataURI := "data:" + mimeFromExt(ext) + ";base64," + base64.StdEncoding.EncodeToString(rawBytes)

	remoteURL, err := s.editor.EditImage(ctx, prompt, []string{dataURI})
	if err != nil {
		return "", fmt.Errorf("comicify: edit image: %w", err)
	}

	imgBytes, err := s.download(ctx, remoteURL)
	if err != nil {
		return "", fmt.Errorf("comicify: download result: %w", err)
	}

	localURL, err := s.store.SaveBytes(bookID, extForBytes(imgBytes), imgBytes)
	if err != nil {
		return "", fmt.Errorf("comicify: save result: %w", err)
	}
	return localURL, nil
}

// download fetches remoteURL honoring ctx and capping the read at
// maxDownloadBytes. A non-2xx status, empty body, or over-cap body is an error.
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

// mimeFromExt maps a stored file extension to its image MIME type, defaulting to
// image/png for anything unrecognized.
func mimeFromExt(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".png":
		return "image/png"
	default:
		return "image/png"
	}
}

// extForBytes maps the sniffed content type of downloaded bytes to a stored
// image extension, defaulting to .png.
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
