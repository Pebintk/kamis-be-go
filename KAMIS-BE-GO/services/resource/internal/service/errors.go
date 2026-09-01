package service

import (
	"errors"

	"github.com/karina/kamis-be-go/pkg/database"
)

// notFound turns a repository miss into an InvalidError carrying the message the
// legacy service used at that call site, and passes any other error through
// untouched. Java raised IllegalArgumentException for a missing resource
// everywhere, with a different message each time.
func notFound(err error, format string, a ...any) error {
	if errors.Is(err, database.ErrNotFound) {
		return invalidf(format, a...)
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
