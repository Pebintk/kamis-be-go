package database

import (
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

// migrationsDir is the root of the embedded filesystem each service hands over.
// Every service keeps its migrations flat in internal/migrations, so there is
// never a subdirectory to name.
const migrationsDir = "."

// Migrate applies a service's pending migrations and reports which ran.
//
// It replaces GORM's AutoMigrate, which inferred the schema from the structs at
// every start. That was right while the schema was still moving and nothing was
// stored, but it has no history, no down path, and no way to express anything
// the struct tags cannot say — a backfill, a rename that preserves data, a
// partial index. Migrations are ordinary SQL files, embedded in the binary, so a
// deploy carries its own schema and no separate tool has to reach the database.
//
// Each service owns its own database, so each keeps its own numbering.
func Migrate(db *gorm.DB, migrations fs.FS) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("access sql.DB: %w", err)
	}

	goose.SetBaseFS(migrations)
	goose.SetLogger(gooseLogger{})
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	before, err := goose.GetDBVersion(sqlDB)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	after, err := goose.GetDBVersion(sqlDB)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if before == after {
		slog.Info("schema is up to date", "version", after)
	} else {
		slog.Info("schema migrated", "from", before, "to", after)
	}
	return nil
}

// gooseLogger routes goose's own output through slog, so migration lines match
// the rest of the process instead of going to stdout unstructured.
type gooseLogger struct{}

func (gooseLogger) Printf(format string, v ...any) {
	slog.Info("goose: " + fmt.Sprintf(format, v...))
}

func (gooseLogger) Fatalf(format string, v ...any) {
	// goose calls Fatalf on an error it has already returned to us, so this must
	// not exit: the caller decides what a failed migration means.
	slog.Error("goose: " + fmt.Sprintf(format, v...))
}
