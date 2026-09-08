package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// excluded are the routes the HTTP metrics ignore. A Prometheus scrape hits
// /metrics every ~15s and the orchestrator polls /health as often; counting
// either would bury the real request rate under traffic that is not real.
var excluded = map[string]bool{
	"/metrics": true,
	"/health":  true,
}

// routePattern is the value used for the `path` label. It is the Gin route
// pattern (`/api/resource/find/:idResource`), never the request's real URL, so
// every ID under one route collapses to a single time series. A request that
// matched no route has an empty FullPath (it is a 404); those are labelled
// "unmatched" rather than leaking the raw path, which would be one new series
// per probed URL — unbounded cardinality.
func routePattern(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return "unmatched"
}

// Middleware records request count, duration and in-flight count. Install it in
// the router's global middleware chain, before the routes are registered, so it
// wraps every handler.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if excluded[c.FullPath()] {
			c.Next()
			return
		}

		start := time.Now()
		RequestsInFlight.Inc()
		// defer so the gauge is still decremented if a handler panics.
		defer RequestsInFlight.Dec()

		c.Next()

		path := routePattern(c)
		RequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
		RequestsTotal.WithLabelValues(c.Request.Method, path, strconv.Itoa(c.Writer.Status())).Inc()
	}
}

// Install mounts the /metrics scrape endpoint and pre-seeds the request counter
// for every route the engine serves. Call it once, at the end of a service's
// router setup, after every route is registered — Middleware() must already be
// in the engine's global chain by then.
//
// /metrics is mounted outside any auth group on purpose: Prometheus scrapes it
// unauthenticated from inside the cluster. Middleware() skips it, so scrapes do
// not inflate the request metrics.
//
// Pre-seeding matters more than it looks. A Prometheus counter that has never
// been touched is not a zero — it is an absent time series, and a dashboard
// querying it shows "No data". That is ambiguous: broken query, broken scrape,
// or a healthy service that simply has not served that route yet. Seeding every
// route at 0 makes absence mean one thing only: something is broken. Do not
// delete the pre-seed loop as pointless — the whole point is the series that
// would otherwise be missing.
//
// Excluded routes (/metrics, /health) are skipped: Middleware never records
// them, so a seeded series would stay at 0 forever and mislead.
func Install(r *gin.Engine) {
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	for _, route := range r.Routes() {
		if excluded[route.Path] {
			continue
		}
		RequestsTotal.WithLabelValues(route.Method, route.Path, "200").Add(0)
	}
}
