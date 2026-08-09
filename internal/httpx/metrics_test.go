package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// scrape performs GET /metrics against the router and returns the body.
func scrape(t *testing.T, router http.Handler) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

func TestMetricsEndpointExposesInstruments(t *testing.T) {
	env := newTestRouter(t)

	// Drive one request so the counter/histogram have a sample to expose.
	hreq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	hrec := httptest.NewRecorder()
	env.router.ServeHTTP(hrec, hreq)
	if hrec.Code != http.StatusOK {
		t.Fatalf("health = %d, want 200", hrec.Code)
	}

	body := scrape(t, env.router)

	for _, want := range []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"omc_registered_users",
		"go_goroutines", // Go collector wired
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/metrics missing %q\n---\n%s", want, body)
		}
	}

	// The health request must be recorded with its route pattern + 200 status,
	// proving the middleware labels by matched route (not raw path).
	if !strings.Contains(body, `route="/api/health"`) || !strings.Contains(body, `status="200"`) {
		t.Fatalf("health request not recorded with route/status labels\n---\n%s", body)
	}
}

func TestMetricsUserCountReflectsRegistrations(t *testing.T) {
	env := newTestRouter(t)

	// No users yet.
	if got := scrape(t, env.router); !strings.Contains(got, "omc_registered_users 0") {
		t.Fatalf("fresh DB should report omc_registered_users 0\n---\n%s", got)
	}

	// Register one user through the real handler.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register", strings.NewReader(registerBody("kid1", env.code)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}

	// The gauge queries the DB on scrape, so it now reflects the new user.
	if got := scrape(t, env.router); !strings.Contains(got, "omc_registered_users 1") {
		t.Fatalf("after one registration want omc_registered_users 1\n---\n%s", got)
	}
}

// TestMetricsMultipleRoutersNoPanic guards the per-router registry: building two
// routers (as tests do) must not panic on duplicate metric registration.
func TestMetricsMultipleRoutersNoPanic(t *testing.T) {
	_ = newTestRouter(t)
	_ = newTestRouter(t)
}
