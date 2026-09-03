package service

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/services/purchase/internal/model"
)

// now is a fixed Thursday in the third quarter, so every range has a distinct
// answer and none of them coincides with a period boundary.
var now = time.Date(2026, 8, 20, 15, 4, 5, 0, time.UTC)

func ymd(t time.Time) string { return t.Format("2006-01-02") }

func TestRangeWindow(t *testing.T) {
	cases := map[string]struct{ start, end string }{
		// The THIS_* ranges stop today rather than at the period's end, so a
		// chart of the current month does not trail off into the future.
		RangeThisMonth:   {"2026-08-01", "2026-08-20"},
		RangeThisQuarter: {"2026-07-01", "2026-08-20"},
		RangeThisYear:    {"2026-01-01", "2026-08-20"},
		RangeLastYear:    {"2025-01-01", "2025-12-31"},
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			win, err := rangeWindow(name, now)
			if err != nil {
				t.Fatal(err)
			}
			if ymd(win.start) != want.start || ymd(win.end) != want.end {
				t.Errorf("window = %s..%s, want %s..%s", ymd(win.start), ymd(win.end), want.start, want.end)
			}
			// The end must include everything submitted on that final day.
			if win.end.Hour() != 23 || win.end.Minute() != 59 {
				t.Errorf("end = %v, want the last moment of the day", win.end)
			}
		})
	}

	if _, err := rangeWindow("LAST_DECADE", now); !isInvalid(err) {
		t.Errorf("unknown range = %v, want an *apierr.Invalid", err)
	}
	// Range names are matched case-insensitively, as the legacy toUpperCase did.
	if _, err := rangeWindow("this_month", now); err != nil {
		t.Errorf("lowercase range rejected: %v", err)
	}
}

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

func TestDefaultPeriodAndAllowedPeriods(t *testing.T) {
	if got := defaultPeriod(RangeThisMonth); got != PeriodWeekly {
		t.Errorf("default for THIS_MONTH = %s, want WEEKLY", got)
	}
	for _, r := range []string{RangeThisQuarter, RangeThisYear, RangeLastYear} {
		if got := defaultPeriod(r); got != PeriodMonthly {
			t.Errorf("default for %s = %s, want MONTHLY", r, got)
		}
	}

	// A month is only meaningful by week, a quarter only by month.
	if !slices.Equal(allowedPeriods[RangeThisMonth], []string{PeriodWeekly}) {
		t.Errorf("THIS_MONTH allows %v", allowedPeriods[RangeThisMonth])
	}
	if !slices.Equal(allowedPeriods[RangeThisQuarter], []string{PeriodMonthly}) {
		t.Errorf("THIS_QUARTER allows %v", allowedPeriods[RangeThisQuarter])
	}
	for _, r := range []string{RangeThisYear, RangeLastYear} {
		if !slices.Equal(allowedPeriods[r], []string{PeriodMonthly, PeriodQuarterly}) {
			t.Errorf("%s allows %v", r, allowedPeriods[r])
		}
	}
}

func TestMonthPeriods(t *testing.T) {
	win, _ := rangeWindow(RangeThisYear, now)
	got := monthPeriods(win)

	want := []string{
		"2026-01", "2026-02", "2026-03", "2026-04",
		"2026-05", "2026-06", "2026-07", "2026-08",
	}
	if !slices.Equal(got, want) {
		t.Errorf("monthPeriods = %v, want %v", got, want)
	}
}

func TestQuarterPeriodsCoverWholeYear(t *testing.T) {
	win, _ := rangeWindow(RangeLastYear, now)
	got := quarterPeriods(win)

	want := []string{"2025-Q1", "2025-Q2", "2025-Q3", "2025-Q4"}
	if !slices.Equal(got, want) {
		t.Errorf("quarterPeriods = %v, want %v", got, want)
	}
}

func TestYearPeriods(t *testing.T) {
	win, _ := rangeWindow(RangeLastYear, now)
	if got := yearPeriods(win); !slices.Equal(got, []string{"2025"}) {
		t.Errorf("yearPeriods = %v, want [2025]", got)
	}
}

// TestWeekPeriods covers the trickiest labels: weeks are numbered from the
// Monday of the month's first week, and a week is only a period of this month's
// chart if it actually touches the month.
func TestWeekPeriods(t *testing.T) {
	// August 2026 opens on a Saturday, so its first week starts in July.
	partial, _ := rangeWindow(RangeThisMonth, now)
	got := weekPeriods(partial)
	want := []string{"2026-08-W1", "2026-08-W2", "2026-08-W3", "2026-08-W4"}
	if !slices.Equal(got, want) {
		t.Errorf("weekPeriods to 20 Aug = %v, want %v", got, want)
	}

	// A whole month reaches its final week.
	full, _ := rangeWindow(RangeThisMonth, time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	if got := weekPeriods(full); len(got) != 6 {
		t.Errorf("weekPeriods for all of Aug 2026 = %v, want 6 weeks", got)
	}
}

// TestWeekLabelFoldsIntoTheOpeningMonth keeps a week that straddles a month
// boundary reporting under the month being charted, rather than splitting.
func TestWeekLabelFoldsIntoTheOpeningMonth(t *testing.T) {
	windowStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) // a Saturday

	// 31 July falls in the same ISO week as 1 August.
	july31 := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	if got := weekLabel(july31, windowStart); got != "2026-08-W1" {
		t.Errorf("weekLabel(31 Jul) = %q, want 2026-08-W1", got)
	}
	if got := weekLabel(windowStart, windowStart); got != "2026-08-W1" {
		t.Errorf("weekLabel(1 Aug) = %q, want 2026-08-W1", got)
	}
	// The following Monday opens week 2.
	if got := weekLabel(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), windowStart); got != "2026-08-W2" {
		t.Errorf("weekLabel(3 Aug) = %q, want 2026-08-W2", got)
	}
}

// TestSummaryWindows pins the comparison periods, including the asymmetry
// inherited from Java: a month is compared with the previous month, but a
// quarter or year with the same period one year earlier.
func TestSummaryWindows(t *testing.T) {
	cases := map[string]struct {
		current, previous [2]string
	}{
		RangeThisMonth: {
			// August against July — the previous *month*.
			current:  [2]string{"2026-08-01", "2026-08-31"},
			previous: [2]string{"2026-07-01", "2026-07-31"},
		},
		RangeThisQuarter: {
			// Q3 2026 against Q3 2025 — the same quarter a year earlier.
			current:  [2]string{"2026-07-01", "2026-09-30"},
			previous: [2]string{"2025-07-01", "2025-09-30"},
		},
		RangeThisYear: {
			current:  [2]string{"2026-01-01", "2026-12-31"},
			previous: [2]string{"2025-01-01", "2025-12-31"},
		},
		RangeLastYear: {
			current:  [2]string{"2025-01-01", "2025-12-31"},
			previous: [2]string{"2024-01-01", "2024-12-31"},
		},
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			current, previous, err := summaryWindows(name, now)
			if err != nil {
				t.Fatal(err)
			}
			if ymd(current.start) != want.current[0] || ymd(current.end) != want.current[1] {
				t.Errorf("current = %s..%s, want %s..%s",
					ymd(current.start), ymd(current.end), want.current[0], want.current[1])
			}
			if ymd(previous.start) != want.previous[0] || ymd(previous.end) != want.previous[1] {
				t.Errorf("previous = %s..%s, want %s..%s",
					ymd(previous.start), ymd(previous.end), want.previous[0], want.previous[1])
			}
		})
	}

	if _, _, err := summaryWindows("NEXT_YEAR", now); !isInvalid(err) {
		t.Errorf("unknown range = %v, want an *apierr.Invalid", err)
	}
}

// TestSummaryWindowsHandlesShortPreviousMonth covers the month comparison where
// the previous month is shorter — 31 March against 28 February.
func TestSummaryWindowsHandlesShortPreviousMonth(t *testing.T) {
	march := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	current, previous, err := summaryWindows(RangeThisMonth, march)
	if err != nil {
		t.Fatal(err)
	}
	if ymd(current.end) != "2026-03-31" {
		t.Errorf("current end = %s, want 2026-03-31", ymd(current.end))
	}
	if ymd(previous.start) != "2026-02-01" || ymd(previous.end) != "2026-02-28" {
		t.Errorf("previous = %s..%s, want all of February",
			ymd(previous.start), ymd(previous.end))
	}
}

func TestPercentageChange(t *testing.T) {
	cases := []struct {
		current, previous int64
		want              float64
	}{
		{120, 100, 20},
		{80, 100, -20},
		{100, 100, 0},
		// With no baseline, any activity reads as +100% and none as flat —
		// dividing by zero is the alternative.
		{5, 0, 100},
		{0, 0, 0},
		{0, 10, -100},
	}

	for _, tc := range cases {
		if got := percentageChange(tc.current, tc.previous); got != tc.want {
			t.Errorf("percentageChange(%d, %d) = %v, want %v",
				tc.current, tc.previous, got, tc.want)
		}
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
