// Package metrics exposes Prometheus instrumentation shared by every KAMIS
// service. The HTTP metrics are collected by a Gin middleware (see
// middleware.go) so that adding them to a service is one line in its router,
// not per-handler bookkeeping.
//
// Cardinality is the failure mode that kills a Prometheus server, and it does
// so slowly enough that nobody notices until it is bad. Every label here is
// bounded: `method` is the small set of HTTP verbs, `path` is the Gin route
// pattern (`/api/resource/find/:idResource`, never the request's real ID), and
// `status` is the small set of codes. Never add a label whose values are
// open-ended — a user ID, an email, a raw URL, a request body.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// No `service` label: Prometheus attaches a `job` label at scrape time, so
// carrying the service name in the metric as well would just be redundant.
var (
	// RequestsTotal counts finished HTTP requests. `path` is the route
	// pattern, so all IDs under one route share a single series.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests processed, by method, route pattern and status code.",
	}, []string{"method", "path", "status"})

	// RequestDuration measures handler latency. Default buckets
	// (5ms..10s) fit an internal JSON API; the slowest route is the supplier
	// detail fan-out, which stays inside the top bucket, so there is no reason
	// to override them.
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, by method and route pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// RequestsInFlight is the number of requests currently being served. It is
	// unlabelled: a single number per process is what reveals a pile-up.
	RequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Number of HTTP requests currently being served.",
	})

	// DBQueriesTotal counts finished GORM queries. `operation` is the GORM
	// callback family (create/query/update/delete/row/raw) — a fixed, tiny set
	// — and `status` is success/error. No SQL text or table name as a label:
	// either would be unbounded.
	DBQueriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "db_queries_total",
		Help: "Total database queries executed, by operation and status.",
	}, []string{"operation", "status"})

	// DBQueryDuration measures GORM query latency by operation.
	DBQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "Database query latency in seconds, by operation.",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})
)
