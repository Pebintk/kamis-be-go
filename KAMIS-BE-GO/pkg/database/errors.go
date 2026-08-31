package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Domain-level data errors. Repositories return these so the service layer can
// branch on what happened without importing gorm or knowing Postgres codes.
var (
	// ErrNotFound reports that a queried row does not exist.
	ErrNotFound = errors.New("record not found")

	// ErrDuplicate reports a unique-constraint violation.
	ErrDuplicate = errors.New("duplicate key")
)

// unique_violation, from the PostgreSQL error-code table.
const uniqueViolation = "23505"

// Translate maps driver and ORM errors onto the domain errors above, leaving
// anything else untouched. Detection goes through the typed *pgconn.PgError
// rather than matching on the message text, which varies by server version and
// locale.
func Translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return ErrDuplicate
	}
	return err
}
