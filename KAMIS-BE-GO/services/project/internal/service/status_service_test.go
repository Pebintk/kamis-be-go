package service

import (
	"strings"
	"testing"
	"time"

	"github.com/pebintk/kamis-be-go/services/project/internal/model"
)

// TestCheckStatusTransition pins the lifecycle: planned, then carried out, then
// finished; cancellable up to the point it finishes; frozen after.
func TestCheckStatusTransition(t *testing.T) {
	allowed := []struct{ from, to int }{
		{model.StatusDirencanakan, model.StatusDilaksanakan},
		{model.StatusDirencanakan, model.StatusBatal},
		{model.StatusDilaksanakan, model.StatusSelesai},
		{model.StatusDilaksanakan, model.StatusBatal},
	}
	for _, tc := range allowed {
		if err := checkStatusTransition(tc.from, tc.to); err != nil {
			t.Errorf("%d -> %d refused: %v", tc.from, tc.to, err)
		}
	}

	refused := map[string]struct {
		from, to int
		because  string
	}{
		"cannot skip straight to finished": {model.StatusDirencanakan, model.StatusSelesai, "langsung"},
		"cannot go back to planned":        {model.StatusDilaksanakan, model.StatusDirencanakan, "dikembalikan"},
		"finished is frozen":               {model.StatusSelesai, model.StatusBatal, "tidak dapat diubah"},
		"cancelled is frozen":              {model.StatusBatal, model.StatusDilaksanakan, "tidak dapat diubah"},
	}
	for name, tc := range refused {
		t.Run(name, func(t *testing.T) {
			err := checkStatusTransition(tc.from, tc.to)
			if !isInvalid(err) {
				t.Fatalf("%d -> %d: got %v, want an *apierr.Invalid", tc.from, tc.to, err)
			}
			if !strings.Contains(err.Error(), tc.because) {
				t.Errorf("message %q does not explain why", err.Error())
			}
		})
	}
}

// TestCheckPaymentTransition covers paying once, and refunding only a cancelled
// project that was actually paid.
func TestCheckPaymentTransition(t *testing.T) {
	unpaid := func(status int) model.Project {
		belum := model.PaymentBelumLunas
		return model.Project{ProjectStatus: status, ProjectPaymentStatus: &belum}
	}
	paid := func(status int) model.Project {
		lunas := model.PaymentTelahLunas
		return model.Project{ProjectStatus: status, ProjectPaymentStatus: &lunas}
	}

	if err := checkPaymentTransition(unpaid(model.StatusDilaksanakan), model.PaymentTelahLunas); err != nil {
		t.Errorf("paying an unpaid project refused: %v", err)
	}
	if err := checkPaymentTransition(paid(model.StatusBatal), model.PaymentDikembalikan); err != nil {
		t.Errorf("refunding a cancelled, paid project refused: %v", err)
	}

	refused := map[string]struct {
		project model.Project
		next    int
		because string
	}{
		"paying twice":                   {paid(model.StatusDilaksanakan), model.PaymentTelahLunas, "sudah dibayar"},
		"refunding an unpaid project":    {unpaid(model.StatusBatal), model.PaymentDikembalikan, "sudah dibayar"},
		"refunding a project still open": {paid(model.StatusDilaksanakan), model.PaymentDikembalikan, "dibatalkan"},
	}
	for name, tc := range refused {
		t.Run(name, func(t *testing.T) {
			err := checkPaymentTransition(tc.project, tc.next)
			if !isInvalid(err) {
				t.Fatalf("got %v, want an *apierr.Invalid", err)
			}
			if !strings.Contains(err.Error(), tc.because) {
				t.Errorf("message %q does not explain why", err.Error())
			}
		})
	}

	// A project with no payment status recorded at all behaves as unpaid.
	if err := checkPaymentTransition(model.Project{ProjectStatus: model.StatusDilaksanakan}, model.PaymentTelahLunas); err != nil {
		t.Errorf("paying a project with no payment status refused: %v", err)
	}
}

// TestAtNoon covers the timezone guard Java added deliberately: these are
// calendar dates, and midnight or 23:59 shifts a day when it crosses a zone.
func TestAtNoon(t *testing.T) {
	for _, at := range []time.Time{
		time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 10, 23, 59, 59, 0, time.UTC),
		time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
	} {
		got := atNoon(at)
		if got.Hour() != 12 || got.Minute() != 0 || got.Second() != 0 || got.Nanosecond() != 0 {
			t.Errorf("atNoon(%v) = %v, want midday", at, got)
		}
		if got.Year() != at.Year() || got.Month() != at.Month() || got.Day() != at.Day() {
			t.Errorf("atNoon(%v) moved the date to %v", at, got)
		}
	}
}

// TestStatusTextCoversEveryStatus keeps a status from being logged as
// "Mengubah Status menjadi " with nothing after it, which is what an
// unrecognised value produced in Java.
func TestStatusTextCoversEveryStatus(t *testing.T) {
	for _, status := range []int{
		model.StatusDirencanakan, model.StatusDilaksanakan,
		model.StatusSelesai, model.StatusBatal,
	} {
		if statusText[status] == "" {
			t.Errorf("status %d has no name", status)
		}
	}
	if _, known := statusText[99]; known {
		t.Error("an unknown status must not be nameable")
	}
}
