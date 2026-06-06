package server

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

// dbStatsCollector exposes the database/sql connection-pool stats as Prometheus
// gauges. It reads stats lazily on each scrape (no background goroutine).
type dbStatsCollector struct {
	stats func() sql.DBStats

	open         *prometheus.Desc
	inUse        *prometheus.Desc
	idle         *prometheus.Desc
	maxOpen      *prometheus.Desc
	waitCount    *prometheus.Desc
	waitDuration *prometheus.Desc
}

func newDBStatsCollector(stats func() sql.DBStats) *dbStatsCollector {
	ns := "hivebook_db"
	d := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(ns+"_"+name, help, nil, nil)
	}
	return &dbStatsCollector{
		stats:        stats,
		open:         d("open_connections", "Number of established connections (in use + idle)."),
		inUse:        d("in_use_connections", "Number of connections currently in use."),
		idle:         d("idle_connections", "Number of idle connections."),
		maxOpen:      d("max_open_connections", "Maximum number of open connections allowed."),
		waitCount:    d("wait_count_total", "Total number of connections waited for."),
		waitDuration: d("wait_duration_seconds_total", "Total time blocked waiting for a connection."),
	}
}

func (c *dbStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.open
	ch <- c.inUse
	ch <- c.idle
	ch <- c.maxOpen
	ch <- c.waitCount
	ch <- c.waitDuration
}

func (c *dbStatsCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.stats()
	g := func(desc *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v)
	}
	g(c.open, float64(s.OpenConnections))
	g(c.inUse, float64(s.InUse))
	g(c.idle, float64(s.Idle))
	g(c.maxOpen, float64(s.MaxOpenConnections))
	// Counters, but exported as the current cumulative value.
	ch <- prometheus.MustNewConstMetric(c.waitCount, prometheus.CounterValue, float64(s.WaitCount))
	ch <- prometheus.MustNewConstMetric(c.waitDuration, prometheus.CounterValue, s.WaitDuration.Seconds())
}
