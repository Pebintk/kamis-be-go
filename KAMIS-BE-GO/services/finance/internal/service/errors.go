package service

import (
	"errors"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/database"
)

// notFound turns a repository miss into the service's 404, and passes any other
// error through untouched.
func notFound(err error, id string) error {
	if errors.Is(err, database.ErrNotFound) {
		return apierr.NotFoundf("Laporan keuangan dengan ID %s tidak ditemukan", id)
	}
	return err
}
