// Package migrations carries the purchase service's schema history, embedded in the
// binary so a deploy needs no separate tool or file copy.
package migrations

import "embed"

// FS holds the migration files, applied by database.Migrate.
//
//go:embed *.sql
var FS embed.FS
