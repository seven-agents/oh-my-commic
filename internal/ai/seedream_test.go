package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSeedreamImageWithRefs verifies the request carries the Bearer key, the
// model, and the reference image array, and that data[0].url is parsed back.
func TestSeedreamImageWithRefs(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/images/generations") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer ark-secret" {
			t.Errorf("missing/invalid auth header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing json content-type: %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"http://img/x.png"}]}`))
	}))
	defer srv.Close()

	c := &Client{ArkKey: "ark-secret", SeedreamBaseURL: srv.URL, SeedreamModel: "doubao-seedream-4-0-250828"}
	refs := []string{"data:image/png;base64,AAAA", "data:image/jpeg;base64,BBBB"}
	url, err := c.SeedreamImage(context.Background(), "画成吉卜力风格", refs)
	if err != nil {
		t.Fatalf("SeedreamImage: %v", err)
	}
	if url != "http://img/x.png" {
		t.Fatalf("wrong url: %s", url)
	}

	if gotBody["model"] != "doubao-seedream-4-0-250828" {
		t.Fatalf("wrong model in body: %v", gotBody["model"])
	}
	if gotBody["prompt"] != "画成吉卜力风格" {
		t.Fatalf("wrong prompt in body: %v", gotBody["prompt"])
	}
	images, ok := gotBody["image"].([]any)
	if !ok || len(images) != 2 {
		t.Fatalf("expected 2 image refs, got %v", gotBody["image"])
	}
	if images[0] != "data:image/png;base64,AAAA" {
		t.Fatalf("first ref wrong: %v", images[0])
	}
}

// TestSeedreamImageNoRefsOmitsImage verifies pure text-to-image omits the
// "image" field entirely.
func TestSeedreamImageNoRefsOmitsImage(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"http://img/y.png"}]}`))
	}))
	defer srv.Close()

	c := &Client{ArkKey: "ark-secret", SeedreamBaseURL: srv.URL, SeedreamModel: "doubao-seedream-4-0-250828"}
	url, err := c.SeedreamImage(context.Background(), "纯文生图", nil)
	if err != nil {
		t.Fatalf("SeedreamImage: %v", err)
	}
	if url != "http://img/y.png" {
		t.Fatalf("wrong url: %s", url)
	}
	if _, present := gotBody["image"]; present {
		t.Fatalf("image field must be omitted when there are no refs, got %v", gotBody["image"])
	}
}

// TestSeedreamImageNon2xxError verifies a non-2xx response surfaces an error and
// never leaks the key.
func TestSeedreamImageNon2xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom ark-secret", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{ArkKey: "ark-secret", SeedreamBaseURL: srv.URL, SeedreamModel: "doubao-seedream-4-0-250828"}
	_, err := c.SeedreamImage(context.Background(), "p", []string{"data:image/png;base64,AA"})
	if err == nil {
		t.Fatal("expected error on non-2xx")
	}
	if strings.Contains(err.Error(), "ark-secret") {
		t.Fatalf("error leaked key: %v", err)
	}
}

// TestSeedreamImageClassifiesStatus verifies a 429 wraps ErrRateLimited and a
// 5xx wraps ErrUpstreamUnavailable, while other non-2xx carry no sentinel — and
// none of them leak the key.
func TestSeedreamImageClassifiesStatus(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusInternalServerError, ErrUpstreamUnavailable},
		{http.StatusBadGateway, ErrUpstreamUnavailable},
		{http.StatusBadRequest, nil},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom ark-secret", tc.code)
		}))
		c := &Client{ArkKey: "ark-secret", SeedreamBaseURL: srv.URL, SeedreamModel: "m"}
		_, err := c.SeedreamImage(context.Background(), "p", nil)
		srv.Close()
		if err == nil {
			t.Fatalf("code %d: expected error", tc.code)
		}
		if strings.Contains(err.Error(), "ark-secret") {
			t.Fatalf("code %d: error leaked key: %v", tc.code, err)
		}
		if tc.want == nil {
			if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrUpstreamUnavailable) {
				t.Fatalf("code %d: should carry no sentinel: %v", tc.code, err)
			}
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Fatalf("code %d: want %v, got %v", tc.code, tc.want, err)
		}
	}
}

// TestSeedreamImageTimeoutClassified verifies a transport-level timeout surfaces
// as ErrUpstreamTimeout. A stub RoundTripper returns a timing-out net.Error so
// no real server or network wait is needed.
func TestSeedreamImageTimeoutClassified(t *testing.T) {
	c := &Client{
		ArkKey:          "ark-secret",
		SeedreamBaseURL: "http://upstream.invalid",
		SeedreamModel:   "m",
		HTTP:            &http.Client{Transport: timeoutRoundTripper{}},
	}
	_, err := c.SeedreamImage(context.Background(), "p", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, ErrUpstreamTimeout) {
		t.Fatalf("expected ErrUpstreamTimeout, got %v", err)
	}
}

// TestSeedreamImageEmptyDataError verifies an empty data array is an error.
func TestSeedreamImageEmptyDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := &Client{ArkKey: "ark-secret", SeedreamBaseURL: srv.URL, SeedreamModel: "doubao-seedream-4-0-250828"}
	if _, err := c.SeedreamImage(context.Background(), "p", nil); err == nil {
		t.Fatal("expected error on empty data")
	}
}
