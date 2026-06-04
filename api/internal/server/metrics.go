package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTP request metrics, scraped by Prometheus at /metrics.
var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trenches_http_requests_total",
		Help: "Total HTTP requests handled, by method, route and status code.",
	}, []string{"method", "route", "code"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "trenches_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, by method and route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	httpInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "trenches_http_requests_in_flight",
		Help: "Number of HTTP requests currently being served.",
	})

	// Bumped by the /events endpoint for each browser telemetry event.
	webEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trenches_web_events_total",
		Help: "Total web (browser) telemetry events received, by type.",
	}, []string{"type"})
)

// metricsMiddleware records request count and latency. The matched chi route
// pattern (e.g. "/api/me") is the label, which keeps cardinality bounded even
// under path parameters.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		httpInFlight.Inc()
		defer httpInFlight.Dec()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		httpRequests.WithLabelValues(r.Method, route, strconv.Itoa(ww.Status())).Inc()
		httpDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}
