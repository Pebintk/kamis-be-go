package service

import (
	"errors"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/database"
)

// notFound turns a repository miss into the service's 404, and passes any other
// error through untouched so a dropped connection is not reported as a missing
// purchase.
func notFound(err error, purchaseID string) error {
	if errors.Is(err, database.ErrNotFound) {
		return apierr.NotFoundf("Pembelian dengan ID %s tidak ditemukan", purchaseID)
	}
	return err
}

// assetNotFound is notFound for staged assets. The legacy message names no id,
// and it is what the frontend shows, so it is kept verbatim.
func assetNotFound(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return &apierr.NotFound{Message: "Aset tidak ditemukan dalam database."}
	}
	return err
}

// isUUID reports whether s is a canonical 8-4-4-4-12 hex UUID, standing in for
// the Java UUID.fromString calls without a UUID dependency.
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
