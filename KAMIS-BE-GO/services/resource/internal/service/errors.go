package service

import (
	"errors"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/database"
)

// InvalidError and NotFoundError are the shared pkg/apierr types, aliased under
// the names this package already uses. InvalidError is a caller mistake (400);
// NotFoundError is a missing resource (404).
type InvalidError = apierr.Invalid

// NotFoundError reports that no resource has the requested id.
type NotFoundError = apierr.NotFound

func invalidf(format string, a ...any) error { return apierr.Invalidf(format, a...) }

// notFound turns a repository miss into a NotFoundError and passes any other
// error through untouched — a dropped connection must not be reported as a
// missing row.
func notFound(err error, id int64) error {
	if errors.Is(err, database.ErrNotFound) {
		return apierr.NotFoundf("Resource dengan ID %d tidak ditemukan.", id)
	}
	return err
}

// isUUID reports whether s is a canonical 8-4-4-4-12 hex UUID. The Java code
// called UUID.fromString and answered 400 on a malformed value; this keeps that
// behaviour without a UUID dependency, since the value is only ever handed
// straight to a Postgres uuid column.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch i {
		case 8, 13, 18, 23:
			if s[i] != '-' {
				return false
			}
		default:
			if !isHex(s[i]) {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
