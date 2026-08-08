// Package comicify redraws a raw uploaded asset image into a locked, Ghibli /
// Miyazaki storybook-style reference image via an image-edit model.
//
// The flow for both characters and scenes is: read the local upload → encode it
// as a base64 data URI (the image API cannot reach a localhost /media URL) → build a
// style + metadata prompt → call the image editor → download the produced image
// (size-capped, ctx-aware) → store it under the same book → return the new local
// /media URL. The returned URL replaces the asset's imageUrl and becomes the
// consistency reference used by later panel rendering.
package comicify

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/seven-agents/oh-my-commic/internal/imageutil"
	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/storage"
)

// maxDownloadBytes caps how many bytes we read from the produced image to bound
// memory against a hostile or misbehaving upstream (~15MB).
const maxDownloadBytes = 15 << 20

// defaultCost is the credit charge per comicify when NewService is given a
// non-positive cost (defensive fallback so a comicify is never accidentally free).
const defaultCost = 1

// ErrInsufficientCredits is returned when the caller's credit balance is too low
// to pay for the comic-ification. The HTTP handler maps it to 402.
var ErrInsufficientCredits = errors.New("insufficient credits")

// ImageEditor redraws reference images into a new stylized image and returns its
// URL. It is satisfied by *ai.Client via SeedreamImage. Defined here (where it is
// used) so tests can inject a fake without any real network calls.
type ImageEditor interface {
	SeedreamImage(ctx context.Context, prompt string, refImageDataURIs []string) (string, error)
}

// CreditLedger charges and refunds a user's image-generation credits. It is
// satisfied by *auth.UserRepo (Spend/Refund). Defined here (where it is used) so
// the comicify Service depends only on the narrow interface and tests can inject
// a fake without any database.
type CreditLedger interface {
	Spend(userID int64, cost int) (bool, error)
	Refund(userID int64, amount int) error
}

// Service orchestrates the comic-ification of an uploaded asset image.
type Service struct {
	editor ImageEditor
	ledger CreditLedger
	cost   int
	store  storage.Local
	http   *http.Client
}

// NewService wires a Service to the image editor, credit ledger, storage, and
// the HTTP client used to download the produced image (callers should set a sane
// timeout). cost is charged up front per comic-ification and refunded on
// failure; a non-positive cost falls back to defaultCost.
func NewService(editor ImageEditor, ledger CreditLedger, cost int, store storage.Local, httpClient *http.Client) *Service {
	if cost <= 0 {
		cost = defaultCost
	}
	return &Service{editor: editor, ledger: ledger, cost: cost, store: store, http: httpClient}
}

// Character comic-ifies the uploaded image at srcURL for character c, storing the
// stylized result under bookID and returning its new /media URL. userID is
// charged one image credit up front (refunded on failure); an insufficient
// balance returns ErrInsufficientCredits without calling the image API.
func (s *Service) Character(ctx context.Context, userID, bookID int64, c models.Character, srcURL string) (string, error) {
	return s.run(ctx, userID, bookID, srcURL, characterPrompt(c))
}

// Scene comic-ifies the uploaded image at srcURL for scene sc, storing the
// stylized result under bookID and returning its new /media URL. userID is
// charged one image credit up front (refunded on failure); an insufficient
// balance returns ErrInsufficientCredits without calling the image API.
func (s *Service) Scene(ctx context.Context, userID, bookID int64, sc models.Scene, srcURL string) (string, error) {
	return s.run(ctx, userID, bookID, srcURL, scenePrompt(sc))
}

// run performs the shared read → charge → encode → edit → download → store
// pipeline. The credit charge happens after the source is read (so an unreadable
// upload never spends quota) but before the image API call; any failure after a
// successful charge triggers a best-effort refund.
func (s *Service) run(ctx context.Context, userID, bookID int64, srcURL, prompt string) (string, error) {
	rawBytes, ext, err := s.store.ReadByURL(srcURL)
	if err != nil {
		return "", fmt.Errorf("comicify: read source: %w", err)
	}

	ok, err := s.ledger.Spend(userID, s.cost)
	if err != nil {
		return "", fmt.Errorf("comicify: spend credits: %w", err)
	}
	if !ok {
		return "", ErrInsufficientCredits
	}

	// Downscale the reference before base64/SeedreamImage: a full-resolution upload
	// makes the edit request slow enough to time out. The model output stays
	// full-resolution; only this input reference is shrunk. A decode failure
	// falls back to the original bytes so a resize hiccup never fails comicify.
	refBytes, refMime := imageutil.ResizeForReference(rawBytes, imageutil.MimeFromExt(ext))
	dataURI := "data:" + refMime + ";base64," + base64.StdEncoding.EncodeToString(refBytes)

	remoteURL, err := s.editor.SeedreamImage(ctx, prompt, []string{dataURI})
	if err != nil {
		s.refund(userID)
		return "", fmt.Errorf("comicify: edit image: %w", err)
	}

	imgBytes, err := s.download(ctx, remoteURL)
	if err != nil {
		s.refund(userID)
		return "", fmt.Errorf("comicify: download result: %w", err)
	}

	localURL, err := s.store.SaveBytes(bookID, extForBytes(imgBytes), imgBytes)
	if err != nil {
		s.refund(userID)
		return "", fmt.Errorf("comicify: save result: %w", err)
	}
	return localURL, nil
}

// refund best-effort returns the credits charged for a comicify that failed
// after the charge. A refund error is only logged (never surfaced) so the
// primary failure is not masked.
func (s *Service) refund(userID int64) {
	if err := s.ledger.Refund(userID, s.cost); err != nil {
		log.Printf("comicify: refund %d credits to user %d failed: %v", s.cost, userID, err)
	}
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
