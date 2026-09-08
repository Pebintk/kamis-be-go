package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/pkg/jsontime"
	"github.com/pebintk/kamis-be-go/services/asset/internal/dto"
	"github.com/pebintk/kamis-be-go/services/asset/internal/model"
)

// TestValidStatus is the guard Java never had: the status arrived as a query
// parameter and was written straight to the column, so ?status=garbage
// corrupted a booking — and because availability keys off exact status values, a
// corrupted booking blocked its vehicle forever.
func TestValidStatus(t *testing.T) {
	for _, s := range []string{
		model.ReservationDirencanakan,
		model.ReservationDilaksanakan,
		model.ReservationSelesai,
		model.ReservationBatal,
	} {
		if !validStatus(s) {
			t.Errorf("validStatus(%q) = false, want true", s)
		}
	}

	for _, s := range []string{"", "garbage", "selesai", "SELESAI", "Direncanakan "} {
		if validStatus(s) {
			t.Errorf("validStatus(%q) = true, want false", s)
		}
	}
}

// TestUpdateStatusRejectsBadInput pins what is refused before the database is
// touched. The service is built on a nil repository, so a call that reached it
// would panic rather than pass quietly.
func TestUpdateStatusRejectsBadInput(t *testing.T) {
	svc := NewReservationService(nil)

	cases := map[string]struct{ id, status string }{
		"unknown status": {sampleUUID, "garbage"},
		"empty status":   {sampleUUID, ""},
		"malformed id":   {"not-a-uuid", model.ReservationSelesai},
		"both wrong":     {"nope", "garbage"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.UpdateStatus(t.Context(), tc.id, tc.status)
			var invalid *apierr.Invalid
			if !errors.As(err, &invalid) {
				t.Errorf("got %v, want an *apierr.Invalid", err)
			}
		})
	}
}

// TestUpdateProjectStatusRejectsBadStatus covers the bulk variant, which takes
// the same query parameter.
func TestUpdateProjectStatusRejectsBadStatus(t *testing.T) {
	_, err := NewReservationService(nil).UpdateProjectStatus(t.Context(), "PRJ-1", "garbage")
	var invalid *apierr.Invalid
	if !errors.As(err, &invalid) {
		t.Fatalf("got %v, want an *apierr.Invalid", err)
	}
	if !strings.Contains(err.Error(), "garbage") {
		t.Errorf("message %q does not name the rejected status", err.Error())
	}
}

// TestReservationNotFound keeps a lookup miss and a real failure apart.
func TestReservationNotFound(t *testing.T) {
	var missing *apierr.NotFound
	if err := reservationNotFound(database.ErrNotFound, sampleUUID); !errors.As(err, &missing) {
		t.Fatalf("got %v, want an *apierr.NotFound", err)
	}

	boom := errors.New("connection refused")
	if got := reservationNotFound(boom, sampleUUID); !errors.Is(got, boom) {
		t.Errorf("reservationNotFound rewrote a non-lookup error to %v", got)
	}
}

// TestReserveRejectsEmptyAndBackwardsWindows covers the two checks that run
// before any availability query.
func TestReserveRejectsEmptyAndBackwardsWindows(t *testing.T) {
	svc := NewReservationService(nil)
	early, late := ts(day(10)), ts(day(20))

	if _, err := svc.Reserve(t.Context(), reserveReq(nil, early, late)); !isInvalid(err) {
		t.Errorf("empty plate list: got %v, want an *apierr.Invalid", err)
	}
	if _, err := svc.Reserve(t.Context(), reserveReq([]string{"B1234XYZ"}, late, early)); !isInvalid(err) {
		t.Errorf("backwards window: got %v, want an *apierr.Invalid", err)
	}
}

func ts(t time.Time) *dto.Timestamp {
	stamp := jsontime.UTC(t)
	return &stamp
}

func reserveReq(plats []string, start, end *dto.Timestamp) dto.AssetReservationRequest {
	return dto.AssetReservationRequest{
		PlatNomors: plats, ProjectID: "PRJ-1", StartDate: start, EndDate: end,
	}
}

func isInvalid(err error) bool {
	var invalid *apierr.Invalid
	return errors.As(err, &invalid)
}
