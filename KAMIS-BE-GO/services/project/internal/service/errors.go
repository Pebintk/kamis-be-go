package service

import (
	"errors"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/database"
)

// notFound turns a repository miss into the service's 404, and passes any other
// error through untouched so a dropped connection is not reported as a missing
// project.
func notFound(err error, projectID string) error {
	if errors.Is(err, database.ErrNotFound) {
		return apierr.NotFoundf("Proyek dengan ID %s tidak ditemukan", projectID)
	}
	return err
}
