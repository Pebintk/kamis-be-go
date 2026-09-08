package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/asset/internal/model"
)

func day(d int) time.Time { return time.Date(2026, 6, d, 9, 0, 0, 0, time.UTC) }

func booking(status string, start, end int) model.AssetReservation {
	return model.AssetReservation{
		ProjectID:         "PRJ-1",
		StartDate:         day(start),
		EndDate:           day(end),
		ReservationStatus: status,
	}
}

// TestCheckReservationConflict covers the two rules the legacy service enforced
// before letting a vehicle into the workshop.
func TestCheckReservationConflict(t *testing.T) {
	planned := booking(model.ReservationDirencanakan, 10, 20)
	running := booking(model.ReservationDilaksanakan, 10, 20)

	cases := []struct {
		name         string
		reservations []model.AssetReservation
		start        time.Time
		wantErr      bool
		wantContains string
	}{
		{"no reservations at all", nil, day(15), false, ""},
		{"start inside a planned booking", []model.AssetReservation{planned}, day(15), true, "sudah direservasi"},
		{"start on the first day of a booking", []model.AssetReservation{planned}, day(10), true, "sudah direservasi"},
		{"start on the last day of a booking", []model.AssetReservation{planned}, day(20), true, "sudah direservasi"},
		{"start inside a running booking", []model.AssetReservation{running}, day(15), true, "sudah direservasi"},
		{"well clear after a booking", []model.AssetReservation{planned}, day(25), false, ""},
		{"well clear before a booking", []model.AssetReservation{planned}, day(5), false, ""},

		// The one-day buffer applies only to bookings that have not started.
		{"one day before a planned booking is allowed", []model.AssetReservation{planned}, day(9), false, ""},
		{"hours before a planned booking is refused", []model.AssetReservation{planned}, day(9).Add(6 * time.Hour), true, "jarak minimal 1 hari"},
		{"hours before a running booking is allowed", []model.AssetReservation{running}, day(9).Add(6 * time.Hour), false, ""},

		// Finished and cancelled bookings do not block anything.
		{"cancelled booking ignored", []model.AssetReservation{booking(model.ReservationBatal, 10, 20)}, day(15), false, ""},
		{"completed booking ignored", []model.AssetReservation{booking(model.ReservationSelesai, 10, 20)}, day(15), false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkReservationConflict(tc.reservations, tc.start, "B1234XYZ")

			if !tc.wantErr {
				if err != nil {
					t.Fatalf("got %v, want no conflict", err)
				}
				return
			}

			var invalid *apierr.Invalid
			if !errors.As(err, &invalid) {
				t.Fatalf("got %v, want an *apierr.Invalid", err)
			}
			if !strings.Contains(err.Error(), tc.wantContains) {
				t.Errorf("message %q does not mention %q", err.Error(), tc.wantContains)
			}
		})
	}
}

// TestConflictMessageFormatsDates keeps the Indonesian message readable — the
// frontend shows it verbatim, so a raw Go time would leak into the UI.
func TestConflictMessageFormatsDates(t *testing.T) {
	err := checkReservationConflict(
		[]model.AssetReservation{booking(model.ReservationDirencanakan, 10, 20)}, day(15), "B1234XYZ")
	if err == nil {
		t.Fatal("expected a conflict")
	}

	for _, want := range []string{"B1234XYZ", "PRJ-1", "10/06/2026", "20/06/2026"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q is missing %q", err.Error(), want)
		}
	}
}

// TestMaintenanceNotFound keeps a lookup miss and a real failure apart.
func TestMaintenanceNotFound(t *testing.T) {
	var missing *apierr.NotFound
	if err := maintenanceNotFound(database.ErrNotFound, 7); !errors.As(err, &missing) {
		t.Fatalf("got %v, want an *apierr.NotFound", err)
	} else if !strings.Contains(missing.Error(), "ID 7") {
		t.Errorf("message %q does not name the id", missing.Error())
	}

	boom := errors.New("connection refused")
	if got := maintenanceNotFound(boom, 7); !errors.Is(got, boom) {
		t.Errorf("maintenanceNotFound rewrote a non-lookup error to %v", got)
	}
}
