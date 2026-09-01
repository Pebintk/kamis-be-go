package service

import (
	"errors"
	"fmt"

	"github.com/karina/kamis-be-go/pkg/database"
)

// InvalidError is a caller mistake — the Java IllegalArgumentException raised
// for a bad value. Handlers answer 400.
type InvalidError struct{ Message string }

func (e *InvalidError) Error() string { return e.Message }

func invalidf(format string, a ...any) error {
	return &InvalidError{Message: fmt.Sprintf(format, a...)}
}

// NotFoundError reports that no resource has the requested id. Handlers answer
// 404.
//
// Java raised IllegalArgumentException for this too, with a different message at
// each call site ("Resource not found", "Resource tidak ditermukan", "Resource
// dengan ID x tidak ditemukan."), and each controller method mapped it to
// whichever status its own catch blocks happened to use. It is one condition, so
// it now has one type, one message and one status.
type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

// notFound turns a repository miss into a NotFoundError and passes any other
// error through untouched — a dropped connection must not be reported as a
// missing row.
func notFound(err error, id int64) error {
	if errors.Is(err, database.ErrNotFound) {
		return &NotFoundError{Message: fmt.Sprintf("Resource dengan ID %d tidak ditemukan.", id)}
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
