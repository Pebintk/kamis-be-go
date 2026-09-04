package database

import (
	"embed"
	"io/fs"
	"os"
	"testing"
)

//go:embed testdata/migrations/*.sql
var testMigrations embed.FS

// TestMigrate exercises the real goose runner against a real PostgreSQL, which
// is the only way to know the embedded SQL actually applies.
//
// It needs a database and so is skipped unless TEST_DATABASE_URL points at one;
// CI has no Postgres and skips it. To run it:
//
//	podman run -d --name kamis-pg -e POSTGRES_PASSWORD=kamis -e POSTGRES_USER=kamis \
//	  -p 55432:5432 postgres:16-alpine
//	TEST_DATABASE_URL='postgres://kamis:kamis@localhost:55432/postgres?sslmode=disable' \
//	  go test ./pkg/database/ -run TestMigrate -v
func TestMigrate(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run the migration tests")
	}

	db, err := Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}

	// Start from nothing, so a re-run of the suite is not affected by the last.
	for _, statement := range []string{
		"DROP TABLE IF EXISTS widgets",
		"DROP TABLE IF EXISTS goose_db_version",
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	// The service packages embed their migrations flat; this test's live under
	// testdata, so it is rooted to match.
	sub, err := fs.Sub(testMigrations, "testdata/migrations")
	if err != nil {
		t.Fatal(err)
	}

	if err := Migrate(db, sub); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	// Both migrations ran: the table exists and carries the column the second
	// one adds.
	var columns int64
	err = db.Raw(`SELECT count(*) FROM information_schema.columns
	              WHERE table_name = 'widgets'`).Scan(&columns).Error
	if err != nil {
		t.Fatal(err)
	}
	if columns != 3 {
		t.Errorf("widgets has %d columns, want 3 (id, name, colour)", columns)
	}

	// Running again is a no-op rather than an error, which is what makes it safe
	// to call on every start.
	if err := Migrate(db, sub); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	var version int64
	if err := db.Raw("SELECT max(version_id) FROM goose_db_version").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Errorf("schema version = %d, want 2", version)
	}
}
