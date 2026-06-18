// Package metrics holds the integrations service's Prometheus metrics, scraped at
// /metrics on both the server and the workers (design-doc 0010/0012).
//
// Cardinality has a hard split (design-doc 0012, "Observability"): you cannot emit
// per-connection time series for 50,000 connections, so the only permitted labels are
// connector / region / tenant_tier. connection_id is NEVER a label — per-connection
// detail lives in the sync_run lineage, queried on demand. We alert on OUTCOMES (the
// freshness / backlog SLIs) and diagnose with MECHANISM (durations, rate-limits,
// adaptive L). The outcome series carry the page; the mechanism series explain it.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// --- Outcome SLIs (alerting) -------------------------------------------------
//
// These are what we page on. They are aggregate gauges set by the backlog exporter
// (RunExporter) from Postgres + Temporal every cycle — not incremented inline.

var (
	// SyncFreshnessSeconds is the max sync lag (now − last_success_at) across a
	// connector's connections in a region — the freshness SLO (design-doc 0012). A
	// climbing value is the page: deltas are not landing.
	SyncFreshnessSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hivebook_sync_freshness_seconds",
		Help: "Max sync lag (now − last_success_at) across a connector's connections, by connector and region.",
	}, []string{"connector", "region"})

	// SyncBacklogUnits is the count of SELECTED non-terminal sync_unit rows by
	// connector — the backlog SLO (design-doc 0012). It is the work owed, independent
	// of whether any worker is draining it.
	SyncBacklogUnits = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hivebook_sync_backlog_units",
		Help: "Selected non-terminal sync_unit rows owed, by connector.",
	}, []string{"connector"})

	// SyncConnections gauges live connection counts by connector and status
	// (connected | needs_reauth | error | …) — the fleet's at-a-glance health.
	SyncConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hivebook_sync_connections",
		Help: "Live connections by connector and status.",
	}, []string{"connector", "status"})
)

// --- Mechanism (diagnosis) ---------------------------------------------------
//
// These explain a page after the fact: healthy backoff (L dropping because GitHub is
// throttling us) is the system working, not an incident. They are emitted inline by
// the activities/connectors, except AdmissionLimit which the exporter sets.

var (
	// SyncUnitDuration is the wall time of one SyncUnit drain, by connector.
	SyncUnitDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "hivebook_sync_unit_duration_seconds",
		Help:    "SyncUnit drain wall time in seconds, by connector.",
		Buckets: prometheus.ExponentialBuckets(0.5, 2, 10), // 0.5s … ~256s
	}, []string{"connector"})

	// ArtifactsIngested counts raw artifacts written (after dedup), by connector.
	ArtifactsIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hivebook_artifacts_ingested_total",
		Help: "Raw artifacts written to the corpus, by connector.",
	}, []string{"connector"})

	// SourceRateLimited counts times a source throttled us, by connector. A spike here
	// paired with L dropping is healthy backoff (don't page); paired with backlog
	// climbing and L stuck at 1 is starvation (do page) — the outcome SLI disambiguates.
	SourceRateLimited = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hivebook_source_rate_limited_total",
		Help: "Source rate-limit responses, by connector.",
	}, []string{"connector"})

	// AdmissionLimit gauges the current adaptive AIMD limit L averaged over a
	// connector's credential buckets (ADR-0036). Set by the exporter. L stuck at 1
	// with backlog climbing is the starvation signal.
	AdmissionLimit = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hivebook_admission_limit",
		Help: "Average current adaptive admission limit L, by connector.",
	}, []string{"connector"})

	// TokenRefresh counts OAuth token refresh outcomes (ok | error), by connector.
	TokenRefresh = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hivebook_token_refresh_total",
		Help: "OAuth token refresh attempts, by connector and result.",
	}, []string{"connector", "result"})

	// DiscoveryDegraded counts mark-and-sweep passes the degraded-discovery guard
	// rejected (empty or a suspicious drop) — no sweep ran, the corpus is intact
	// (ADR-0037). A spike means a source is flapping or a credential is dying.
	DiscoveryDegraded = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hivebook_discovery_degraded_total",
		Help: "Discovery passes rejected by the degraded-discovery guard, by connector.",
	}, []string{"connector"})

	// PurgeDeletes counts erasure-outbox object deletes by outcome (deleted | failed |
	// orphaned). deleted includes 404-already-gone; failed rows stay purge_pending for
	// the next pass (the outbox invariant); orphaned is a manifest row with no object
	// key (ADR-0039).
	PurgeDeletes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hivebook_purge_deletes_total",
		Help: "Erasure-outbox object deletes, by outcome.",
	}, []string{"outcome"})

	// PurgeSLABreaches gauges the count of purge_pending artifacts past the GDPR SLA
	// still not deleted — the page signal (ADR-0039). A non-zero value is an incident:
	// a delete is stuck and the contractual window is closing. Set by the exporter.
	PurgeSLABreaches = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hivebook_purge_sla_breaches",
		Help: "purge_pending artifacts past the GDPR erasure SLA still not deleted.",
	})

	// TemporalBacklog gauges the approximate pending workflow+activity task count per
	// Temporal task queue (design-doc 0012). THIS is the KEDA scale signal: it is
	// server-side, so a triggered sync with zero workers still shows backlog and scales
	// up — the property that kills the old River/KEDA scale-to-zero deadlock. Set by the
	// exporter from Temporal's DescribeTaskQueue.
	TemporalBacklog = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hivebook_temporal_backlog",
		Help: "Approximate pending Temporal task count, by task queue (the KEDA scale signal).",
	}, []string{"queue"})
)

// ConnectorCapHits counts times a connector hit a v0.1 sync cap (so truncation is
// never silent — review #17). Defined in the connectors package's view via this
// shared var.
var ConnectorCapHits = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "hivebook_connector_cap_hits_total",
	Help: "Times a connector hit a v0.1 sync cap, by connector and cap kind.",
}, []string{"connector", "kind"})
