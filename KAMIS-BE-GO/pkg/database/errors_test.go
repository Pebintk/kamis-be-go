package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestTranslate(t *testing.T) {
	// want is the domain error expected; nil means "return the input unchanged".
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"record not found", gorm.ErrRecordNotFound, ErrNotFound},
		{"wrapped record not found", fmt.Errorf("finding user: %w", gorm.ErrRecordNotFound), ErrNotFound},
		{"unique violation", &pgconn.PgError{Code: "23505"}, ErrDuplicate},
		{"wrapped unique violation", fmt.Errorf("saving: %w", &pgconn.PgError{Code: "23505"}), ErrDuplicate},
		// A foreign-key violation is not one of our domain errors, so it must
		// come back untouched rather than be mistaken for a duplicate.
		{"foreign-key violation passes through", &pgconn.PgError{Code: "23503"}, nil},
		{"unrelated error passes through", errors.New("connection refused"), nil},
	}

	for _, tc := range cases {
		got := Translate(tc.in)
		if tc.want != nil {
			if !errors.Is(got, tc.want) {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			}
			continue
		}
		if !errors.Is(got, tc.in) {
			t.Errorf("%s: got %v, want the input error unchanged", tc.name, got)
		}
	}

	if Translate(nil) != nil {
		t.Error("Translate(nil) must be nil")
	}
}

// The translated errors must stay distinguishable from each other.
func TestDomainErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrNotFound, ErrDuplicate) || errors.Is(ErrDuplicate, ErrNotFound) {
		t.Error("ErrNotFound and ErrDuplicate must not match each other")
	}
}
