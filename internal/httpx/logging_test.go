package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequestLoggerCapturesStatus verifies the wrapper records the status code a
// handler writes (here 418) so it can be logged. The log line itself is not
// asserted; only that the captured status is correct and forwarded downstream.
func TestRequestLoggerCapturesStatus(t *testing.T) {
	var captured int
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec, ok := w.(*statusRecorder)
		if !ok {
			t.Fatalf("期望 statusRecorder, 实际 %T", w)
		}
		w.WriteHeader(http.StatusTeapot)
		captured = rec.status
	})

	h := requestLogger(inner)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/anything", nil))

	if captured != http.StatusTeapot {
		t.Fatalf("wrapper 未捕获状态码: 期望 418, 实际 %d", captured)
	}
	if rr.Code != http.StatusTeapot {
		t.Fatalf("状态码未透传给底层 writer: 期望 418, 实际 %d", rr.Code)
	}
}
