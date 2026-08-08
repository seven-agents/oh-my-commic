package ai

import (
	"context"
	"errors"
	"net"
	"net/http"
)

// Upstream-error sentinels. Handlers map these (via errors.Is on the wrapped
// error chain) to distinct HTTP statuses and friendly Chinese messages. They
// classify upstream failures WITHOUT ever carrying the upstream response body
// or the API key: only the HTTP status code / transport condition is inspected.
var (
	// ErrRateLimited marks an upstream 429: the request was throttled, either
	// because we are sending too fast or the account quota is tight.
	ErrRateLimited = errors.New("ai: upstream rate limited")

	// ErrUpstreamTimeout marks a timed-out upstream call: the request context
	// deadline was exceeded or the underlying network operation timed out.
	ErrUpstreamTimeout = errors.New("ai: upstream timeout")

	// ErrUpstreamUnavailable marks an upstream 5xx: the provider is failing or
	// temporarily down.
	ErrUpstreamUnavailable = errors.New("ai: upstream unavailable")
)

// classifyStatus maps an upstream HTTP status code to a sentinel error, or nil
// when the status does not warrant a specific classification. Callers wrap the
// returned sentinel with %w so errors.Is can recover it; a nil return means the
// caller should fall back to its existing generic status error.
//
//   - 429      → ErrRateLimited
//   - 5xx      → ErrUpstreamUnavailable
//   - anything else (including other 4xx) → nil
func classifyStatus(code int) error {
	switch {
	case code == http.StatusTooManyRequests:
		return ErrRateLimited
	case code >= 500 && code <= 599:
		return ErrUpstreamUnavailable
	default:
		return nil
	}
}

// classifyTransport inspects a transport-level error from an HTTP round trip. If
// it represents a timeout (a context deadline exceeded, or a net.Error whose
// Timeout() is true) it returns ErrUpstreamTimeout so callers can wrap it with
// %w. Otherwise it returns err unchanged, preserving the original chain.
func classifyTransport(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrUpstreamTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrUpstreamTimeout
	}
	return err
}
