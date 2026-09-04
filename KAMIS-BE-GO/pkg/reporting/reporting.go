// Package reporting holds the calendar arithmetic behind the KAMIS activity
// charts and period summaries: which window a named range covers, which
// granularities that range can be charted at, and the ordered list of period
// labels to plot.
//
// It carries no I/O and no domain knowledge — a caller supplies the counts. The
// purchase and project services chart different things over identical periods,
// and finance.report will make a third.
package reporting

import (
	"fmt"
	"strings"
	"time"
)

// Ranges, as the frontend sends them.
const (
	RangeThisMonth   = "THIS_MONTH"
	RangeThisQuarter = "THIS_QUARTER"
	RangeThisYear    = "THIS_YEAR"
	RangeLastYear    = "LAST_YEAR"
)

// Granularities a range can be charted at.
const (
	PeriodWeekly    = "WEEKLY"
	PeriodMonthly   = "MONTHLY"
	PeriodQuarterly = "QUARTERLY"
	PeriodYearly    = "YEARLY"
)

// ErrUnknownRange and ErrUnknownPeriod are returned as plain errors; callers
// wrap them in whatever their API's error type is.
var (
	ErrUnknownRange  = fmt.Errorf("range tidak valid. Gunakan LAST_YEAR, THIS_YEAR, THIS_QUARTER, atau THIS_MONTH")
	ErrUnknownPeriod = fmt.Errorf("period type tidak valid")
)

// Window is a closed date range, inclusive of both ends.
type Window struct{ Start, End time.Time }

// QuarterStart is the first day of the quarter containing t.
func QuarterStart(t time.Time) time.Time {
	month := time.Month((int(t.Month())-1)/3*3 + 1)
	return time.Date(t.Year(), month, 1, 0, 0, 0, 0, t.Location())
}

// EndOfDay pushes t to the last moment of its day, so a closed range includes
// everything recorded on the final date.
func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
}

// RangeWindow resolves a range name against today.
//
// The three THIS_* ranges end today rather than at the period's end, so a chart
// of the current month stops at the present instead of trailing into the future.
func RangeWindow(name string, now time.Time) (Window, error) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch strings.ToUpper(name) {
	case RangeThisMonth:
		return Window{time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location()), EndOfDay(day)}, nil
	case RangeThisQuarter:
		return Window{QuarterStart(day), EndOfDay(day)}, nil
	case RangeThisYear:
		return Window{time.Date(day.Year(), time.January, 1, 0, 0, 0, 0, day.Location()), EndOfDay(day)}, nil
	case RangeLastYear:
		start := time.Date(day.Year()-1, time.January, 1, 0, 0, 0, 0, day.Location())
		end := time.Date(day.Year()-1, time.December, 31, 0, 0, 0, 0, day.Location())
		return Window{start, EndOfDay(end)}, nil
	default:
		return Window{}, ErrUnknownRange
	}
}

// DefaultPeriod is the granularity a range charts at when none is given.
func DefaultPeriod(rangeName string) string {
	if strings.EqualFold(rangeName, RangeThisMonth) {
		return PeriodWeekly
	}
	return PeriodMonthly
}

// AllowedPeriods is which granularities each range accepts. A month is only
// meaningful by week, a quarter only by month.
var AllowedPeriods = map[string][]string{
	RangeThisMonth:   {PeriodWeekly},
	RangeThisQuarter: {PeriodMonthly},
	RangeThisYear:    {PeriodMonthly, PeriodQuarterly},
	RangeLastYear:    {PeriodMonthly, PeriodQuarterly},
}

// CheckPeriod reports whether a granularity is meaningful for a range, and says
// which are if it is not.
func CheckPeriod(rangeName, periodType string) error {
	allowed, ok := AllowedPeriods[strings.ToUpper(rangeName)]
	if !ok {
		return ErrUnknownRange
	}
	for _, item := range allowed {
		if strings.EqualFold(item, periodType) {
			return nil
		}
	}
	return fmt.Errorf("%s hanya mendukung periodType = %s",
		strings.ToUpper(rangeName), strings.Join(allowed, " atau "))
}

// ---- period labels ----

// MondayOf is the Monday of t's ISO week.
func MondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // Sunday is 0 in Go, 6 in ISO
	return t.AddDate(0, 0, -offset)
}

// WeekLabel names the week containing day, numbered from the first week of the
// window's opening month: yyyy-MM-Wn.
//
// Weeks are numbered against that month rather than the ISO year, so a week
// straddling a month boundary still reports under the month being charted.
func WeekLabel(day, windowStart time.Time) string {
	firstOfMonth := time.Date(windowStart.Year(), windowStart.Month(), 1, 0, 0, 0, 0, windowStart.Location())
	weeks := int(MondayOf(day).Sub(MondayOf(firstOfMonth)).Hours() / (24 * 7))
	return fmt.Sprintf("%04d-%02d-W%d", windowStart.Year(), int(windowStart.Month()), weeks+1)
}

// WeekPeriods lists the week labels of a window, keeping only weeks that touch
// its opening month.
func WeekPeriods(w Window) []string {
	month, year := w.Start.Month(), w.Start.Year()

	var periods []string
	seen := map[string]bool{}
	for pointer := MondayOf(w.Start); !pointer.After(w.End); pointer = pointer.AddDate(0, 0, 7) {
		for offset := range 7 {
			day := pointer.AddDate(0, 0, offset)
			if day.Month() != month || day.Year() != year {
				continue
			}
			if label := WeekLabel(pointer, w.Start); !seen[label] {
				seen[label] = true
				periods = append(periods, label)
			}
			break
		}
	}
	return periods
}

// MonthPeriods lists yyyy-MM across a window.
func MonthPeriods(w Window) []string {
	var periods []string
	pointer := time.Date(w.Start.Year(), w.Start.Month(), 1, 0, 0, 0, 0, w.Start.Location())
	last := time.Date(w.End.Year(), w.End.Month(), 1, 0, 0, 0, 0, w.End.Location())

	for ; !pointer.After(last); pointer = pointer.AddDate(0, 1, 0) {
		periods = append(periods, pointer.Format("2006-01"))
	}
	return periods
}

// QuarterPeriods lists yyyy-Qn across a window.
func QuarterPeriods(w Window) []string {
	var periods []string
	seen := map[string]bool{}
	pointer := time.Date(w.Start.Year(), w.Start.Month(), 1, 0, 0, 0, 0, w.Start.Location())
	last := time.Date(w.End.Year(), w.End.Month(), 1, 0, 0, 0, 0, w.End.Location())

	for ; !pointer.After(last); pointer = pointer.AddDate(0, 1, 0) {
		label := fmt.Sprintf("%d-Q%d", pointer.Year(), (int(pointer.Month())-1)/3+1)
		if !seen[label] {
			seen[label] = true
			periods = append(periods, label)
		}
	}
	return periods
}

// YearPeriods lists yyyy across a window.
func YearPeriods(w Window) []string {
	var periods []string
	for year := w.Start.Year(); year <= w.End.Year(); year++ {
		periods = append(periods, fmt.Sprint(year))
	}
	return periods
}

// Periods is the ordered list of labels for a granularity over a window.
func Periods(periodType string, w Window) ([]string, error) {
	switch strings.ToUpper(periodType) {
	case PeriodWeekly:
		return WeekPeriods(w), nil
	case PeriodMonthly:
		return MonthPeriods(w), nil
	case PeriodQuarterly:
		return QuarterPeriods(w), nil
	case PeriodYearly:
		return YearPeriods(w), nil
	default:
		return nil, ErrUnknownPeriod
	}
}

// ---- summaries ----

// SummaryWindows resolves the period being reported and the one it is compared
// against.
//
// The comparison is not uniform, and this is inherited from the Java services: a
// month is compared with the *previous month*, while a quarter or year is
// compared with the same period a year earlier. The current window also runs to
// the end of the period rather than to today, so a mid-month figure is the
// month's total so far.
func SummaryWindows(rangeName string, now time.Time) (current, previous Window, err error) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch strings.ToUpper(rangeName) {
	case RangeThisYear:
		start := time.Date(day.Year(), time.January, 1, 0, 0, 0, 0, day.Location())
		end := time.Date(day.Year(), time.December, 31, 0, 0, 0, 0, day.Location())
		return Window{start, EndOfDay(end)},
			Window{start.AddDate(-1, 0, 0), EndOfDay(end.AddDate(-1, 0, 0))}, nil

	case RangeThisQuarter:
		start := QuarterStart(day)
		end := start.AddDate(0, 3, -1)
		return Window{start, EndOfDay(end)},
			Window{start.AddDate(-1, 0, 0), EndOfDay(end.AddDate(-1, 0, 0))}, nil

	case RangeThisMonth:
		start := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
		end := start.AddDate(0, 1, -1)
		prevStart := start.AddDate(0, -1, 0)
		return Window{start, EndOfDay(end)},
			Window{prevStart, EndOfDay(prevStart.AddDate(0, 1, -1))}, nil

	case RangeLastYear:
		start := time.Date(day.Year()-1, time.January, 1, 0, 0, 0, 0, day.Location())
		end := time.Date(day.Year()-1, time.December, 31, 0, 0, 0, 0, day.Location())
		return Window{start, EndOfDay(end)},
			Window{start.AddDate(-1, 0, 0), EndOfDay(end.AddDate(-1, 0, 0))}, nil

	default:
		return Window{}, Window{}, ErrUnknownRange
	}
}

// PercentageChange is the movement between two counts. With no baseline, any
// activity at all reads as +100% and none as flat — dividing by zero is the
// alternative.
func PercentageChange(current, previous int64) float64 {
	switch {
	case previous > 0:
		return float64(current-previous) / float64(previous) * 100
	case current > 0:
		return 100
	default:
		return 0
	}
}
