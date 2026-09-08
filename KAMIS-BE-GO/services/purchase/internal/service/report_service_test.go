package service

import (
	"errors"
	"slices"
	"testing"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/services/purchase/internal/model"
)

func TestStatusScopeFor(t *testing.T) {
	cancelled, err := statusScopeFor("CANCELLED")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Exclude {
		t.Error("CANCELLED must count the cancelled statuses, not exclude them")
	}
	if !slices.Equal(cancelled.Statuses, []string{model.StatusDitolak, model.StatusDibatalkan}) {
		t.Errorf("CANCELLED statuses = %v", cancelled.Statuses)
	}

	done, _ := statusScopeFor("done")
	if done.Exclude || !slices.Equal(done.Statuses, []string{model.StatusSelesai}) {
		t.Errorf("DONE scope = %+v", done)
	}

	// "ALL" carries the same statuses as CANCELLED but excludes them, so it
	// means every *active* purchase. Misleading name, deliberate behaviour.
	all, _ := statusScopeFor("ALL")
	if !all.Exclude {
		t.Error("ALL must exclude its statuses; it means every active purchase")
	}
	if !slices.Equal(all.Statuses, cancelled.Statuses) {
		t.Errorf("ALL statuses = %v, want the same pair CANCELLED uses", all.Statuses)
	}

	if _, err := statusScopeFor("PENDING"); !isInvalid(err) {
		t.Errorf("unknown filter = %v, want an *apierr.Invalid", err)
	}
}

// TestActivityLineRejectsMismatchedPeriod keeps an unsupported granularity from
// silently charting the wrong thing. The service has a nil repository, so a call
// that got as far as querying would panic.
func TestActivityLineRejectsMismatchedPeriod(t *testing.T) {
	svc := NewPurchaseService(nil, Deps{})

	cases := map[string][2]string{
		"month by quarter": {PeriodQuarterly, RangeThisMonth},
		"quarter by week":  {PeriodWeekly, RangeThisQuarter},
		"year by week":     {PeriodWeekly, RangeThisYear},
		"unknown period":   {"FORTNIGHTLY", RangeThisYear},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.ActivityLine(t.Context(), tc[0], tc[1], "ALL")
			var invalid *apierr.Invalid
			if !errors.As(err, &invalid) {
				t.Errorf("got %v, want an *apierr.Invalid", err)
			}
		})
	}
}
