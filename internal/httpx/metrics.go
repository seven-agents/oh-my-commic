package httpx

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metricsPath is the (unversioned) scrape endpoint, mirroring /api/health.
const metricsPath = "/metrics"

// metrics holds the Prometheus registry and the request instruments. Each router
// owns its own registry (not the global default) so building multiple routers —
// in tests, say — never panics on duplicate registration.
type metrics struct {
	reg      *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// newMetrics builds the registry and registers the HTTP instruments, the Go
// runtime/process collectors, and (when non-nil) a live user-count gauge that
// queries userCount on each scrape.
func newMetrics(userCount func() (int, error)) *metrics {
	m := &metrics{
		reg: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests, labeled by method, matched route and status code.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labeled by method and matched route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
	}
	m.reg.MustRegister(m.requests, m.duration)
	m.reg.MustRegister(collectors.NewGoCollector())
	m.reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if userCount != nil {
		m.reg.MustRegister(newUserCountCollector(userCount))
	}
	return m
}

// handler serves the registry in the Prometheus text exposition format.
func (m *metrics) handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// middleware records one observation per request: latency into the histogram and
// a count into the counter, both labeled by the chi route pattern (e.g.
// "/api/v1/books/{id}") rather than the concrete path, so IDs never explode
// label cardinality. The scrape endpoint itself is not measured.
func (m *metrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == metricsPath {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// RoutePattern() is populated during chi's routing, so after ServeHTTP it
		// holds the matched template. Unmatched paths (SPA shell, static, 404s)
		// have no pattern; bucket them under "other" to keep cardinality bounded.
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "other"
		}
		m.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
	})
}

// userCountCollector exposes the registered-user total as a gauge, querying the
// database lazily on each scrape so the value is always current without a
// background goroutine. A query error is skipped rather than failing the whole
// scrape.
type userCountCollector struct {
	count func() (int, error)
	desc  *prometheus.Desc
}

func newUserCountCollector(count func() (int, error)) *userCountCollector {
	return &userCountCollector{
		count: count,
		desc:  prometheus.NewDesc("omc_registered_users", "Number of registered users.", nil, nil),
	}
}

func (c *userCountCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *userCountCollector) Collect(ch chan<- prometheus.Metric) {
	n, err := c.count()
	if err != nil {
		return
	}
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(n))
}
