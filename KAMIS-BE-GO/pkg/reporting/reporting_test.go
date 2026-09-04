package reporting

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// now is a fixed Thursday in the third quarter, so every range has a distinct
// answer and none coincides with a period boundary.
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
			win, err := RangeWindow(name, now)
			if err != nil {
				t.Fatal(err)
			}
			if ymd(win.Start) != want.start || ymd(win.End) != want.end {
				t.Errorf("window = %s..%s, want %s..%s", ymd(win.Start), ymd(win.End), want.start, want.end)
			}
			if win.End.Hour() != 23 || win.End.Minute() != 59 {
				t.Errorf("end = %v, want the last moment of the day", win.End)
			}
		})
	}

	if _, err := RangeWindow("LAST_DECADE", now); !errors.Is(err, ErrUnknownRange) {
		t.Errorf("unknown range = %v, want ErrUnknownRange", err)
	}
	// Range names are matched case-insensitively, as the legacy toUpperCase did.
	if _, err := RangeWindow("this_month", now); err != nil {
		t.Errorf("lowercase range rejected: %v", err)
	}
}

func TestDefaultPeriodAndCheckPeriod(t *testing.T) {
	if got := DefaultPeriod(RangeThisMonth); got != PeriodWeekly {
		t.Errorf("default for THIS_MONTH = %s, want WEEKLY", got)
	}
	for _, r := range []string{RangeThisQuarter, RangeThisYear, RangeLastYear} {
		if got := DefaultPeriod(r); got != PeriodMonthly {
			t.Errorf("default for %s = %s, want MONTHLY", r, got)
		}
	}

	// A month is only meaningful by week, a quarter only by month.
	valid := []struct{ rangeName, period string }{
		{RangeThisMonth, PeriodWeekly},
		{RangeThisQuarter, PeriodMonthly},
		{RangeThisYear, PeriodMonthly},
		{RangeThisYear, PeriodQuarterly},
		{RangeLastYear, PeriodQuarterly},
	}
	for _, tc := range valid {
		if err := CheckPeriod(tc.rangeName, tc.period); err != nil {
			t.Errorf("%s by %s refused: %v", tc.rangeName, tc.period, err)
		}
	}

	invalid := []struct{ rangeName, period string }{
		{RangeThisMonth, PeriodQuarterly},
		{RangeThisQuarter, PeriodWeekly},
		{RangeThisYear, PeriodWeekly},
		{RangeLastYear, PeriodYearly},
	}
	for _, tc := range invalid {
		if err := CheckPeriod(tc.rangeName, tc.period); err == nil {
			t.Errorf("%s by %s accepted, want refused", tc.rangeName, tc.period)
		}
	}

	if err := CheckPeriod("NOPE", PeriodMonthly); !errors.Is(err, ErrUnknownRange) {
		t.Errorf("unknown range = %v, want ErrUnknownRange", err)
	}
}

func TestMonthPeriods(t *testing.T) {
	win, _ := RangeWindow(RangeThisYear, now)
	want := []string{
		"2026-01", "2026-02", "2026-03", "2026-04",
		"2026-05", "2026-06", "2026-07", "2026-08",
	}
	if got := MonthPeriods(win); !slices.Equal(got, want) {
		t.Errorf("MonthPeriods = %v, want %v", got, want)
	}
}

func TestQuarterPeriodsCoverWholeYear(t *testing.T) {
	win, _ := RangeWindow(RangeLastYear, now)
	want := []string{"2025-Q1", "2025-Q2", "2025-Q3", "2025-Q4"}
	if got := QuarterPeriods(win); !slices.Equal(got, want) {
		t.Errorf("QuarterPeriods = %v, want %v", got, want)
	}
}

func TestYearPeriods(t *testing.T) {
	win, _ := RangeWindow(RangeLastYear, now)
	if got := YearPeriods(win); !slices.Equal(got, []string{"2025"}) {
		t.Errorf("YearPeriods = %v, want [2025]", got)
	}
}

// TestWeekPeriods covers the trickiest labels: weeks are numbered from the
// Monday of the month's first week, and a week is only a period of this month's
// chart if it actually touches the month.
func TestWeekPeriods(t *testing.T) {
	// August 2026 opens on a Saturday, so its first week starts in July.
	partial, _ := RangeWindow(RangeThisMonth, now)
	want := []string{"2026-08-W1", "2026-08-W2", "2026-08-W3", "2026-08-W4"}
	if got := WeekPeriods(partial); !slices.Equal(got, want) {
		t.Errorf("WeekPeriods to 20 Aug = %v, want %v", got, want)
	}

	full, _ := RangeWindow(RangeThisMonth, time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))
	if got := WeekPeriods(full); len(got) != 6 {
		t.Errorf("WeekPeriods for all of Aug 2026 = %v, want 6 weeks", got)
	}
}

// TestWeekLabelFoldsIntoTheOpeningMonth keeps a week that straddles a month
// boundary reporting under the month being charted, rather than splitting.
func TestWeekLabelFoldsIntoTheOpeningMonth(t *testing.T) {
	windowStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) // a Saturday

	july31 := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	if got := WeekLabel(july31, windowStart); got != "2026-08-W1" {
		t.Errorf("WeekLabel(31 Jul) = %q, want 2026-08-W1", got)
	}
	if got := WeekLabel(windowStart, windowStart); got != "2026-08-W1" {
		t.Errorf("WeekLabel(1 Aug) = %q, want 2026-08-W1", got)
	}
	if got := WeekLabel(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), windowStart); got != "2026-08-W2" {
		t.Errorf("WeekLabel(3 Aug) = %q, want 2026-08-W2", got)
	}
}

func TestPeriodsDispatch(t *testing.T) {
	win, _ := RangeWindow(RangeThisYear, now)
	for _, period := range []string{PeriodMonthly, PeriodQuarterly, PeriodYearly} {
		if got, err := Periods(period, win); err != nil || len(got) == 0 {
			t.Errorf("Periods(%s) = %v, %v", period, got, err)
		}
	}
	if _, err := Periods("FORTNIGHTLY", win); !errors.Is(err, ErrUnknownPeriod) {
		t.Errorf("unknown period = %v, want ErrUnknownPeriod", err)
	}
}

// TestSummaryWindows pins the comparison periods, including the asymmetry
// inherited from Java: a month is compared with the previous month, but a
// quarter or year with the same period one year earlier.
func TestSummaryWindows(t *testing.T) {
	cases := map[string]struct{ current, previous [2]string }{
		RangeThisMonth: {
			current:  [2]string{"2026-08-01", "2026-08-31"},
			previous: [2]string{"2026-07-01", "2026-07-31"},
		},
		RangeThisQuarter: {
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
			current, previous, err := SummaryWindows(name, now)
			if err != nil {
				t.Fatal(err)
			}
			if ymd(current.Start) != want.current[0] || ymd(current.End) != want.current[1] {
				t.Errorf("current = %s..%s, want %s..%s",
					ymd(current.Start), ymd(current.End), want.current[0], want.current[1])
			}
			if ymd(previous.Start) != want.previous[0] || ymd(previous.End) != want.previous[1] {
				t.Errorf("previous = %s..%s, want %s..%s",
					ymd(previous.Start), ymd(previous.End), want.previous[0], want.previous[1])
			}
		})
	}

	if _, _, err := SummaryWindows("NEXT_YEAR", now); !errors.Is(err, ErrUnknownRange) {
		t.Errorf("unknown range = %v, want ErrUnknownRange", err)
	}
}

// TestSummaryWindowsHandlesShortPreviousMonth covers 31 March against 28
// February.
func TestSummaryWindowsHandlesShortPreviousMonth(t *testing.T) {
	march := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	current, previous, err := SummaryWindows(RangeThisMonth, march)
	if err != nil {
		t.Fatal(err)
	}
	if ymd(current.End) != "2026-03-31" {
		t.Errorf("current end = %s, want 2026-03-31", ymd(current.End))
	}
	if ymd(previous.Start) != "2026-02-01" || ymd(previous.End) != "2026-02-28" {
		t.Errorf("previous = %s..%s, want all of February", ymd(previous.Start), ymd(previous.End))
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
		// With no baseline, any activity reads as +100% and none as flat.
		{5, 0, 100},
		{0, 0, 0},
		{0, 10, -100},
	}

	for _, tc := range cases {
		if got := PercentageChange(tc.current, tc.previous); got != tc.want {
			t.Errorf("PercentageChange(%d, %d) = %v, want %v", tc.current, tc.previous, got, tc.want)
		}
	}
}
