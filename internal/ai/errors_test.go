package ai

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestClassifyStatus verifies the status-code → sentinel mapping: 429 →
// ErrRateLimited, 5xx → ErrUpstreamUnavailable, everything else → nil (caller
// falls back to its generic status error).
func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		name string
		code int
		want error
	}{
		{"rate limited 429", 429, ErrRateLimited},
		{"server 500", 500, ErrUpstreamUnavailable},
		{"server 502", 502, ErrUpstreamUnavailable},
		{"server 503", 503, ErrUpstreamUnavailable},
		{"server 599", 599, ErrUpstreamUnavailable},
		{"ok 200", 200, nil},
		{"bad request 400", 400, nil},
		{"unauthorized 401", 401, nil},
		{"not found 404", 404, nil},
		{"other 4xx 418", 418, nil},
		{"below server 499", 499, nil},
		{"above server 600", 600, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyStatus(tc.code)
			if !errors.Is(got, tc.want) {
				t.Fatalf("classifyStatus(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

// timeoutErr is a net.Error whose Timeout() is true, standing in for a network
// timeout without an ErrDeadlineExceeded chain.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// nonTimeoutNetErr is a net.Error whose Timeout() is false (e.g. connection
// refused): it must NOT be classified as a timeout.
type nonTimeoutNetErr struct{}

func (nonTimeoutNetErr) Error() string   { return "connection refused" }
func (nonTimeoutNetErr) Timeout() bool   { return false }
func (nonTimeoutNetErr) Temporary() bool { return false }

// timeoutRoundTripper is an http.RoundTripper that always fails with a timing-out
// net.Error, letting Chat/SeedreamImage timeout classification be tested without
// a real server or network wait. The http.Client wraps this in a *url.Error whose
// Timeout() delegates here, so classifyTransport still recognizes it.
type timeoutRoundTripper struct{}

func (timeoutRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, timeoutErr{}
}

// TestClassifyTransport verifies timeouts (context deadline or a timing-out
// net.Error, including when wrapped) become ErrUpstreamTimeout, while other
// errors pass through unchanged with their chain intact.
func TestClassifyTransport(t *testing.T) {
	plain := errors.New("some non-timeout failure")

	t.Run("nil stays nil", func(t *testing.T) {
		if got := classifyTransport(nil); got != nil {
			t.Fatalf("classifyTransport(nil) = %v, want nil", got)
		}
	})

	t.Run("context deadline exceeded", func(t *testing.T) {
		if got := classifyTransport(context.DeadlineExceeded); !errors.Is(got, ErrUpstreamTimeout) {
			t.Fatalf("deadline exceeded not classified as timeout: %v", got)
		}
	})

	t.Run("wrapped context deadline exceeded", func(t *testing.T) {
		wrapped := fmt.Errorf("dial: %w", context.DeadlineExceeded)
		if got := classifyTransport(wrapped); !errors.Is(got, ErrUpstreamTimeout) {
			t.Fatalf("wrapped deadline not classified as timeout: %v", got)
		}
	})

	t.Run("net timeout error", func(t *testing.T) {
		var ne net.Error = timeoutErr{}
		if got := classifyTransport(ne); !errors.Is(got, ErrUpstreamTimeout) {
			t.Fatalf("net timeout not classified as timeout: %v", got)
		}
	})

	t.Run("wrapped net timeout error", func(t *testing.T) {
		wrapped := fmt.Errorf("do request: %w", net.Error(timeoutErr{}))
		if got := classifyTransport(wrapped); !errors.Is(got, ErrUpstreamTimeout) {
			t.Fatalf("wrapped net timeout not classified as timeout: %v", got)
		}
	})

	t.Run("non-timeout net error passes through", func(t *testing.T) {
		var ne net.Error = nonTimeoutNetErr{}
		got := classifyTransport(ne)
		if errors.Is(got, ErrUpstreamTimeout) {
			t.Fatalf("non-timeout net error wrongly classified as timeout: %v", got)
		}
		if got != ne {
			t.Fatalf("non-timeout net error not passed through: %v", got)
		}
	})

	t.Run("plain error passes through unchanged", func(t *testing.T) {
		got := classifyTransport(plain)
		if !errors.Is(got, plain) {
			t.Fatalf("plain error chain broken: %v", got)
		}
		if errors.Is(got, ErrUpstreamTimeout) {
			t.Fatalf("plain error wrongly classified as timeout: %v", got)
		}
	})

	// Guard the intent: a genuinely slow dial produces a real net timeout that
	// must classify as ErrUpstreamTimeout.
	t.Run("real dial timeout", func(t *testing.T) {
		// 203.0.113.0/24 (TEST-NET-3) is reserved and unroutable, so the dial
		// hangs until the deadline fires — a real net timeout.
		_, err := net.DialTimeout("tcp", "203.0.113.1:9", 50*time.Millisecond)
		if err == nil {
			t.Skip("dial unexpectedly succeeded; skipping timeout check")
		}
		var ne net.Error
		if !errors.As(err, &ne) || !ne.Timeout() {
			t.Skipf("dial error was not a timeout (%v); environment-dependent", err)
		}
		if got := classifyTransport(err); !errors.Is(got, ErrUpstreamTimeout) {
			t.Fatalf("real dial timeout not classified: %v", got)
		}
	})
}
