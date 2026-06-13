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
		Name: "hivebook_http_requests_total",
		Help: "Total HTTP requests handled, by method, route and status code.",
	}, []string{"method", "route", "code"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "hivebook_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, by method and route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	httpInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hivebook_http_requests_in_flight",
		Help: "Number of HTTP requests currently being served.",
	})

	// Bumped by the /events endpoint for each browser telemetry event.
	webEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hivebook_web_events_total",
		Help: "Total web (browser) telemetry events received, by type.",
	}, []string{"type"})

	// Per-RPC metrics, recorded by the Connect metrics interceptor. The HTTP
	// middleware can't see individual RPCs — the chi mount collapses them all to
	// one wildcard route — so per-procedure latency and error rate live here.
	rpcRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hivebook_rpc_requests_total",
		Help: "Total Connect RPCs handled, by procedure and result code.",
	}, []string{"procedure", "code"})

	rpcDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "hivebook_rpc_request_duration_seconds",
		Help:    "Connect RPC latency in seconds, by procedure.",
		Buckets: prometheus.DefBuckets,
	}, []string{"procedure"})
)

// metricsMiddleware records HTTP request count and latency, labelled by the
// matched chi route pattern (so cardinality stays bounded). For the mounted
// Connect handler that pattern is a single wildcard — per-procedure RPC metrics
// come from the Connect interceptor instead (rpcRequests / rpcDuration).
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
