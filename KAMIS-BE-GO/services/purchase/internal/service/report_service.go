package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/services/purchase/internal/dto"
	"github.com/karina/kamis-be-go/services/purchase/internal/model"
	"github.com/karina/kamis-be-go/services/purchase/internal/repository"
)

// Reporting ranges and period granularities, as the frontend sends them.
const (
	RangeThisMonth   = "THIS_MONTH"
	RangeThisQuarter = "THIS_QUARTER"
	RangeThisYear    = "THIS_YEAR"
	RangeLastYear    = "LAST_YEAR"

	PeriodWeekly    = "WEEKLY"
	PeriodMonthly   = "MONTHLY"
	PeriodQuarterly = "QUARTERLY"
	PeriodYearly    = "YEARLY"
)

// window is a closed date range, inclusive of both ends.
type window struct{ start, end time.Time }

// quarterStart is the first day of the quarter containing t.
func quarterStart(t time.Time) time.Time {
	month := time.Month((int(t.Month())-1)/3*3 + 1)
	return time.Date(t.Year(), month, 1, 0, 0, 0, 0, t.Location())
}

// endOfDay pushes t to the last moment of its day, so a closed range includes
// everything submitted on the final date.
func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
}

// rangeWindow resolves a range name against today. The three THIS_* ranges end
// today rather than at the period's end, so a chart of the current month stops
// at the present.
func rangeWindow(name string, now time.Time) (window, error) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch strings.ToUpper(name) {
	case RangeThisMonth:
		return window{time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location()), endOfDay(day)}, nil
	case RangeThisQuarter:
		return window{quarterStart(day), endOfDay(day)}, nil
	case RangeThisYear:
		return window{time.Date(day.Year(), time.January, 1, 0, 0, 0, 0, day.Location()), endOfDay(day)}, nil
	case RangeLastYear:
		start := time.Date(day.Year()-1, time.January, 1, 0, 0, 0, 0, day.Location())
		return window{start, endOfDay(time.Date(day.Year()-1, time.December, 31, 0, 0, 0, 0, day.Location()))}, nil
	default:
		return window{}, apierr.Invalidf(
			"Range tidak valid. Gunakan LAST_YEAR, THIS_YEAR, THIS_QUARTER, atau THIS_MONTH.")
	}
}

// defaultPeriod is the granularity a range charts at when none is given.
func defaultPeriod(rangeName string) string {
	if strings.EqualFold(rangeName, RangeThisMonth) {
		return PeriodWeekly
	}
	return PeriodMonthly
}

// allowedPeriods is which granularities each range accepts. A month is only
// meaningful by week, a quarter only by month.
var allowedPeriods = map[string][]string{
	RangeThisMonth:   {PeriodWeekly},
	RangeThisQuarter: {PeriodMonthly},
	RangeThisYear:    {PeriodMonthly, PeriodQuarterly},
	RangeLastYear:    {PeriodMonthly, PeriodQuarterly},
}

// statusScopeFor turns the chart's filter name into a status predicate.
//
// "ALL" is the odd one: it carries the same two statuses as "CANCELLED" but
// *excludes* them, so it means "every active purchase". The name is misleading
// and inherited; the behaviour is deliberate.
func statusScopeFor(filter string) (repository.StatusScope, error) {
	cancelled := []string{model.StatusDitolak, model.StatusDibatalkan}

	switch strings.ToUpper(filter) {
	case "CANCELLED":
		return repository.StatusScope{Statuses: cancelled}, nil
	case "DONE":
		return repository.StatusScope{Statuses: []string{model.StatusSelesai}}, nil
	case "ALL":
		return repository.StatusScope{Statuses: cancelled, Exclude: true}, nil
	default:
		return repository.StatusScope{}, apierr.Invalidf("Invalid status filter")
	}
}

// ActivityLine returns one count per period across the requested window, with
// empty periods filled in so the chart has no gaps.
func (s *PurchaseService) ActivityLine(ctx context.Context, periodType, rangeName, statusFilter string) ([]dto.ActivityLineResponse, error) {
	scope, err := statusScopeFor(statusFilter)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(periodType) == "" {
		periodType = defaultPeriod(rangeName)
	}
	periodType = strings.ToUpper(periodType)

	win, err := rangeWindow(rangeName, time.Now())
	if err != nil {
		return nil, err
	}

	allowed, ok := allowedPeriods[strings.ToUpper(rangeName)]
	if !ok {
		return nil, apierr.Invalidf(
			"Range tidak valid. Gunakan LAST_YEAR, THIS_YEAR, THIS_QUARTER, atau THIS_MONTH.")
	}
	if !contains(allowed, periodType) {
		return nil, apierr.Invalidf("%s hanya mendukung periodType = %s",
			strings.ToUpper(rangeName), strings.Join(allowed, " atau "))
	}

	counts, periods, err := s.aggregate(ctx, periodType, win, scope)
	if err != nil {
		return nil, err
	}

	out := make([]dto.ActivityLineResponse, 0, len(periods))
	for _, p := range periods {
		out = append(out, dto.ActivityLineResponse{Period: p, Count: counts[p]})
	}
	return out, nil
}

// aggregate runs the grouped count for one granularity and returns both the
// counts and the complete ordered list of periods in the window.
func (s *PurchaseService) aggregate(ctx context.Context, periodType string, win window, scope repository.StatusScope) (map[string]int64, []string, error) {
	counts := map[string]int64{}

	switch periodType {
	case PeriodWeekly:
		rows, err := s.repo.CountByDay(ctx, win.start, win.end, scope)
		if err != nil {
			return nil, nil, err
		}
		// Days are folded into the week labels of the window's opening month, so
		// a week straddling a month boundary still reports under that month.
		for _, row := range rows {
			counts[weekLabel(row.Day, win.start)] += row.Count
		}
		return counts, weekPeriods(win), nil

	case PeriodMonthly:
		rows, err := s.repo.CountByMonth(ctx, win.start, win.end, scope)
		if err != nil {
			return nil, nil, err
		}
		return tally(rows), monthPeriods(win), nil

	case PeriodQuarterly:
		rows, err := s.repo.CountByQuarter(ctx, win.start, win.end, scope)
		if err != nil {
			return nil, nil, err
		}
		return tally(rows), quarterPeriods(win), nil

	case PeriodYearly:
		// No range currently permits YEARLY, so this arm is unreachable through
		// the API. It is implemented because the legacy switch had it, and
		// leaving a hole would turn a future range into a silent empty chart.
		rows, err := s.repo.CountByYear(ctx, win.start, win.end, scope)
		if err != nil {
			return nil, nil, err
		}
		return tally(rows), yearPeriods(win), nil

	default:
		return nil, nil, apierr.Invalidf("Invalid period type: %s", periodType)
	}
}

func tally(rows []repository.PeriodCount) map[string]int64 {
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Period] = row.Count
	}
	return out
}

// ---- period labels ----

// mondayOf is the Monday of t's ISO week.
func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // Sunday is 0 in Go, 6 in ISO
	return t.AddDate(0, 0, -offset)
}

// weekLabel names the week containing day, numbered from the first week of the
// window's opening month: yyyy-MM-Wn.
func weekLabel(day, windowStart time.Time) string {
	firstOfMonth := time.Date(windowStart.Year(), windowStart.Month(), 1, 0, 0, 0, 0, windowStart.Location())
	weeks := int(mondayOf(day).Sub(mondayOf(firstOfMonth)).Hours() / (24 * 7))
	return fmt.Sprintf("%04d-%02d-W%d", windowStart.Year(), int(windowStart.Month()), weeks+1)
}

// weekPeriods lists the week labels of the window, keeping only weeks that touch
// the opening month — a week straddling into the next month is not a period of
// this month's chart.
func weekPeriods(win window) []string {
	month, year := win.start.Month(), win.start.Year()

	var periods []string
	seen := map[string]bool{}
	for pointer := mondayOf(win.start); !pointer.After(win.end); pointer = pointer.AddDate(0, 0, 7) {
		for offset := range 7 {
			day := pointer.AddDate(0, 0, offset)
			if day.Month() != month || day.Year() != year {
				continue
			}
			if label := weekLabel(pointer, win.start); !seen[label] {
				seen[label] = true
				periods = append(periods, label)
			}
			break
		}
	}
	return periods
}

func monthPeriods(win window) []string {
	var periods []string
	pointer := time.Date(win.start.Year(), win.start.Month(), 1, 0, 0, 0, 0, win.start.Location())
	last := time.Date(win.end.Year(), win.end.Month(), 1, 0, 0, 0, 0, win.end.Location())

	for ; !pointer.After(last); pointer = pointer.AddDate(0, 1, 0) {
		periods = append(periods, pointer.Format("2006-01"))
	}
	return periods
}

func quarterPeriods(win window) []string {
	var periods []string
	seen := map[string]bool{}
	pointer := time.Date(win.start.Year(), win.start.Month(), 1, 0, 0, 0, 0, win.start.Location())
	last := time.Date(win.end.Year(), win.end.Month(), 1, 0, 0, 0, 0, win.end.Location())

	for ; !pointer.After(last); pointer = pointer.AddDate(0, 1, 0) {
		label := fmt.Sprintf("%d-Q%d", pointer.Year(), (int(pointer.Month())-1)/3+1)
		if !seen[label] {
			seen[label] = true
			periods = append(periods, label)
		}
	}
	return periods
}

func yearPeriods(win window) []string {
	var periods []string
	for year := win.start.Year(); year <= win.end.Year(); year++ {
		periods = append(periods, fmt.Sprint(year))
	}
	return periods
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if strings.EqualFold(item, want) {
			return true
		}
	}
	return false
}

// ---- range list and summary ----

// ListByRange returns the purchases submitted in a named range, in the same row
// shape as /viewall.
func (s *PurchaseService) ListByRange(ctx context.Context, rangeName string) ([]dto.PurchaseListResponse, error) {
	win, err := rangeWindow(rangeName, time.Now())
	if err != nil {
		return nil, err
	}
	return s.ListPurchases(ctx, repository.Filter{StartDate: &win.start, EndDate: &win.end})
}

// summaryWindows resolves the period being reported and the one it is compared
// against.
//
// The comparison is not uniform, and this is inherited: a month is compared with
// the *previous month*, while a quarter or year is compared with the same period
// a year earlier. Note too that the current window runs to the end of the
// period, not to today, so a mid-month figure is the month's total so far.
func summaryWindows(rangeName string, now time.Time) (current, previous window, err error) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch strings.ToUpper(rangeName) {
	case RangeThisYear:
		start := time.Date(day.Year(), time.January, 1, 0, 0, 0, 0, day.Location())
		end := time.Date(day.Year(), time.December, 31, 0, 0, 0, 0, day.Location())
		return window{start, endOfDay(end)},
			window{start.AddDate(-1, 0, 0), endOfDay(end.AddDate(-1, 0, 0))}, nil

	case RangeThisQuarter:
		start := quarterStart(day)
		end := start.AddDate(0, 3, -1)
		return window{start, endOfDay(end)},
			window{start.AddDate(-1, 0, 0), endOfDay(end.AddDate(-1, 0, 0))}, nil

	case RangeThisMonth:
		start := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
		end := start.AddDate(0, 1, -1)
		prevStart := start.AddDate(0, -1, 0)
		return window{start, endOfDay(end)},
			window{prevStart, endOfDay(prevStart.AddDate(0, 1, -1))}, nil

	case RangeLastYear:
		start := time.Date(day.Year()-1, time.January, 1, 0, 0, 0, 0, day.Location())
		end := time.Date(day.Year()-1, time.December, 31, 0, 0, 0, 0, day.Location())
		return window{start, endOfDay(end)},
			window{start.AddDate(-1, 0, 0), endOfDay(end.AddDate(-1, 0, 0))}, nil

	default:
		return window{}, window{}, apierr.Invalidf(
			"Range tidak dikenali: %s. Gunakan THIS_YEAR, THIS_QUARTER, THIS_MONTH, atau LAST_YEAR.", rangeName)
	}
}

// SummaryByRange returns how many purchases a period holds and how that compares
// with the previous one.
func (s *PurchaseService) SummaryByRange(ctx context.Context, rangeName string) (*dto.PurchaseSummaryResponse, error) {
	current, previous, err := summaryWindows(rangeName, time.Now())
	if err != nil {
		return nil, err
	}

	currentCount, err := s.repo.CountBetween(ctx, current.start, current.end)
	if err != nil {
		return nil, err
	}
	previousCount, err := s.repo.CountBetween(ctx, previous.start, previous.end)
	if err != nil {
		return nil, err
	}

	return &dto.PurchaseSummaryResponse{
		TotalPurchase:    int(currentCount),
		PercentageChange: percentageChange(currentCount, previousCount),
	}, nil
}

// percentageChange is the movement between two counts. With no baseline, any
// activity at all reads as +100% and none as flat — dividing by zero is the
// alternative.
func percentageChange(current, previous int64) float64 {
	switch {
	case previous > 0:
		return float64(current-previous) / float64(previous) * 100
	case current > 0:
		return 100
	default:
		return 0
	}
}
