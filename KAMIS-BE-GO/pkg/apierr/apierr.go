// Package apierr carries the two error kinds a service distinguishes for the
// caller, so every service maps errors onto HTTP the same way: 400 for a caller
// mistake, 404 for a thing that does not exist, 500 for anything else.
//
// The Java controllers had no such rule — each method's catch blocks chose their
// own statuses and they disagreed with each other. See MIGRATION.md, "Error
// statuses are uniform".
package apierr

import (
	"errors"
	"fmt"
	"net/http"
)

// Invalid is a caller mistake — the Java IllegalArgumentException. Its message
// is Indonesian text the frontend renders verbatim, so it reaches the client
// as-is.
type Invalid struct{ Message string }

func (e *Invalid) Error() string { return e.Message }

// Invalidf builds an Invalid with a formatted message.
func Invalidf(format string, a ...any) error {
	return &Invalid{Message: fmt.Sprintf(format, a...)}
}

// NotFound reports that the requested record does not exist. One condition, one
// type, one message, one status — Java raised IllegalArgumentException for this
// with different text at every call site.
type NotFound struct{ Message string }

func (e *NotFound) Error() string { return e.Message }

// NotFoundf builds a NotFound with a formatted message.
func NotFoundf(format string, a ...any) error {
	return &NotFound{Message: fmt.Sprintf(format, a...)}
}

// Status maps an error onto its HTTP status.
func Status(err error) int {
	var invalid *Invalid
	if errors.As(err, &invalid) {
		return http.StatusBadRequest
	}
	var missing *NotFound
	if errors.As(err, &missing) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}
