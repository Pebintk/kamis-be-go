package database

import (
	"os"
	"testing"

	"github.com/karina/kamis-be-go/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestQueryMetricsRecorded proves the GORM callbacks registered by Connect
// actually fire: a real query moves db_queries_total. It needs a database, so
// it is skipped unless TEST_DATABASE_URL points at one (see TestMigrate for the
// podman one-liner).
func TestQueryMetricsRecorded(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run the database metrics test")
	}

	db, err := Connect(dsn) // registers the callbacks + pool collector
	if err != nil {
		t.Fatal(err)
	}

	// A temp table, so the test needs no migration and cleans itself up with the
	// session. CREATE runs through the "raw" callback.
	if err := db.Exec("CREATE TEMP TABLE metrics_probe (id int)").Error; err != nil {
		t.Fatal(err)
	}

	before := testutil.ToFloat64(metrics.DBQueriesTotal.WithLabelValues("query", "success"))

	// A SELECT through the model path fires the "query" callback.
	var rows []map[string]any
	if err := db.Table("metrics_probe").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}

	after := testutil.ToFloat64(metrics.DBQueriesTotal.WithLabelValues("query", "success"))
	if after <= before {
		t.Errorf("db_queries_total{operation=query,status=success} did not increase: %v -> %v", before, after)
	}
}
