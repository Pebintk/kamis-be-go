package database

import (
	"database/sql"
	"errors"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// instrumentKey names the per-query start time stashed on the GORM instance
// between the before- and after-callbacks. It is instance-scoped, so concurrent
// queries do not clobber each other's timing.
const instrumentKey = "metrics:start"

// instrument registers GORM callbacks that time every query, and a collector
// that reports the connection pool's stats. Doing it here, on the one shared
// Connect path, instruments all seven services at once — no per-repository or
// per-call-site code.
//
// A callback pair is registered on each GORM operation family. before stamps a
// start time; after observes the elapsed time and whether the operation carried
// an error. GORM has no wildcard "every operation" hook, so each family is
// wired explicitly — the list is fixed and small.
func instrument(db *gorm.DB) error {
	// reg is a GORM callback (*callback).Register bound method value. GORM's
	// processor/callback types are unexported, so the callbacks are wired
	// through this func type rather than by naming them.
	type reg = func(name string, fn func(*gorm.DB)) error

	// wire registers the start/observe pair for one operation family. GORM has
	// no wildcard "every operation" hook, so each family is wired explicitly —
	// the list is fixed and small.
	wire := func(op string, before, after reg) error {
		if err := before("metrics:before_"+op, startTimer); err != nil {
			return err
		}
		return after("metrics:after_"+op, observer(op))
	}

	c := db.Callback()
	for _, f := range []struct {
		op            string
		before, after reg
	}{
		{"create", c.Create().Before("gorm:create").Register, c.Create().After("gorm:create").Register},
		{"query", c.Query().Before("gorm:query").Register, c.Query().After("gorm:query").Register},
		{"update", c.Update().Before("gorm:update").Register, c.Update().After("gorm:update").Register},
		{"delete", c.Delete().Before("gorm:delete").Register, c.Delete().After("gorm:delete").Register},
		{"row", c.Row().Before("gorm:row").Register, c.Row().After("gorm:row").Register},
		{"raw", c.Raw().Before("gorm:raw").Register, c.Raw().After("gorm:raw").Register},
	} {
		if err := wire(f.op, f.before, f.after); err != nil {
			return err
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	// Pool exhaustion never shows up in query or HTTP metrics — it surfaces only
	// as latency, while requests queue for a free connection. These gauges make
	// it directly visible. MustRegister would panic if a second Connect ran in
	// one process; Register tolerates it (only profile+tests open two).
	_ = prometheus.Register(newPoolCollector(sqlDB))
	return nil
}

func startTimer(db *gorm.DB) {
	db.InstanceSet(instrumentKey, time.Now())
}

// observer returns the after-callback for one operation family. It reads the
// start time, records duration and a success/error status. A "record not found"
// is not a failure — it is an ordinary query result — so it counts as success.
func observer(op string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		status := "success"
		if db.Error != nil && !errors.Is(db.Error, gorm.ErrRecordNotFound) {
			status = "error"
		}
		metrics.DBQueriesTotal.WithLabelValues(op, status).Inc()

		if v, ok := db.InstanceGet(instrumentKey); ok {
			if start, ok := v.(time.Time); ok {
				metrics.DBQueryDuration.WithLabelValues(op).Observe(time.Since(start).Seconds())
			}
		}
	}
}

// poolCollector reports sql.DB pool stats. It reads them at scrape time rather
// than caching, so a value is never stale.
type poolCollector struct {
	db   *sql.DB
	open *prometheus.Desc
	use  *prometheus.Desc
	idle *prometheus.Desc
	wait *prometheus.Desc
}

func newPoolCollector(db *sql.DB) *poolCollector {
	return &poolCollector{
		db:   db,
		open: prometheus.NewDesc("db_pool_open_connections", "Open connections in the pool (in use + idle).", nil, nil),
		use:  prometheus.NewDesc("db_pool_in_use_connections", "Connections currently in use.", nil, nil),
		idle: prometheus.NewDesc("db_pool_idle_connections", "Idle connections in the pool.", nil, nil),
		wait: prometheus.NewDesc("db_pool_wait_total", "Total number of connections waited for (pool exhaustion).", nil, nil),
	}
}

func (p *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.open
	ch <- p.use
	ch <- p.idle
	ch <- p.wait
}

func (p *poolCollector) Collect(ch chan<- prometheus.Metric) {
	s := p.db.Stats()
	ch <- prometheus.MustNewConstMetric(p.open, prometheus.GaugeValue, float64(s.OpenConnections))
	ch <- prometheus.MustNewConstMetric(p.use, prometheus.GaugeValue, float64(s.InUse))
	ch <- prometheus.MustNewConstMetric(p.idle, prometheus.GaugeValue, float64(s.Idle))
	ch <- prometheus.MustNewConstMetric(p.wait, prometheus.CounterValue, float64(s.WaitCount))
}
