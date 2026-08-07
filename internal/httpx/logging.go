package httpx

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder wraps an http.ResponseWriter to capture the status code written
// by a handler so it can be logged. It defaults to 200, matching net/http's
// implicit status when a handler writes a body without calling WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader records the status code and forwards it to the underlying writer.
func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Write ensures a handler that writes a body without an explicit WriteHeader is
// still recorded as a 200, mirroring net/http's own behaviour.
func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(b)
}

// Flush forwards to the underlying writer when it supports flushing, preserving
// streaming responses (e.g. Server-Sent Events) through the wrapper.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// requestLogger logs one line per request: method, path, status, and duration.
//
// It deliberately logs only the URL path (never RawQuery, which may carry
// secrets) and never the Cookie/Authorization headers or request/response
// bodies. This keeps the access log free of credentials while still surfacing
// failures in the server log.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}
