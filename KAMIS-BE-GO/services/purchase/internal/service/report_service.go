package service

import (
	"context"
	"strings"
	"time"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/reporting"
	"github.com/karina/kamis-be-go/services/purchase/internal/dto"
	"github.com/karina/kamis-be-go/services/purchase/internal/model"
	"github.com/karina/kamis-be-go/services/purchase/internal/repository"
)

// Reporting ranges and granularities are the shared pkg/reporting names,
// re-exported so this service's callers and tests do not all import it.
const (
	RangeThisMonth   = reporting.RangeThisMonth
	RangeThisQuarter = reporting.RangeThisQuarter
	RangeThisYear    = reporting.RangeThisYear
	RangeLastYear    = reporting.RangeLastYear

	PeriodWeekly    = reporting.PeriodWeekly
	PeriodMonthly   = reporting.PeriodMonthly
	PeriodQuarterly = reporting.PeriodQuarterly
	PeriodYearly    = reporting.PeriodYearly
)

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
		periodType = reporting.DefaultPeriod(rangeName)
	}
	periodType = strings.ToUpper(periodType)

	win, err := reporting.RangeWindow(rangeName, time.Now())
	if err != nil {
		return nil, apierr.Invalidf("%s", err)
	}
	if err := reporting.CheckPeriod(rangeName, periodType); err != nil {
		return nil, apierr.Invalidf("%s", err)
	}

	counts, err := s.aggregate(ctx, periodType, win, scope)
	if err != nil {
		return nil, err
	}
	periods, err := reporting.Periods(periodType, win)
	if err != nil {
		return nil, apierr.Invalidf("Invalid period type: %s", periodType)
	}

	out := make([]dto.ActivityLineResponse, 0, len(periods))
	for _, p := range periods {
		out = append(out, dto.ActivityLineResponse{Period: p, Count: counts[p]})
	}
	return out, nil
}

// aggregate runs the grouped count for one granularity.
func (s *PurchaseService) aggregate(ctx context.Context, periodType string, win reporting.Window, scope repository.StatusScope) (map[string]int64, error) {
	counts := map[string]int64{}

	switch periodType {
	case PeriodWeekly:
		rows, err := s.repo.CountByDay(ctx, win.Start, win.End, scope)
		if err != nil {
			return nil, err
		}
		// Days are folded into the week labels of the window's opening month, so
		// a week straddling a month boundary still reports under that month.
		for _, row := range rows {
			counts[reporting.WeekLabel(row.Day, win.Start)] += row.Count
		}
		return counts, nil

	case PeriodMonthly:
		rows, err := s.repo.CountByMonth(ctx, win.Start, win.End, scope)
		if err != nil {
			return nil, err
		}
		return tally(rows), nil

	case PeriodQuarterly:
		rows, err := s.repo.CountByQuarter(ctx, win.Start, win.End, scope)
		if err != nil {
			return nil, err
		}
		return tally(rows), nil

	case PeriodYearly:
		// No range currently permits YEARLY, so this arm is unreachable through
		// the API. It is implemented because leaving a hole would turn a future
		// range into a silent empty chart.
		rows, err := s.repo.CountByYear(ctx, win.Start, win.End, scope)
		if err != nil {
			return nil, err
		}
		return tally(rows), nil

	default:
		return nil, apierr.Invalidf("Invalid period type: %s", periodType)
	}
}

func tally(rows []repository.PeriodCount) map[string]int64 {
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Period] = row.Count
	}
	return out
}

// ---- range list and summary ----

// ListByRange returns the purchases submitted in a named range, in the same row
// shape as /viewall.
func (s *PurchaseService) ListByRange(ctx context.Context, rangeName string) ([]dto.PurchaseListResponse, error) {
	win, err := reporting.RangeWindow(rangeName, time.Now())
	if err != nil {
		return nil, apierr.Invalidf("%s", err)
	}
	return s.ListPurchases(ctx, repository.Filter{StartDate: &win.Start, EndDate: &win.End})
}

// SummaryByRange returns how many purchases a period holds and how that compares
// with the previous one.
func (s *PurchaseService) SummaryByRange(ctx context.Context, rangeName string) (*dto.PurchaseSummaryResponse, error) {
	current, previous, err := reporting.SummaryWindows(rangeName, time.Now())
	if err != nil {
		return nil, apierr.Invalidf("Range tidak dikenali: %s. Gunakan THIS_YEAR, THIS_QUARTER, THIS_MONTH, atau LAST_YEAR.", rangeName)
	}

	currentCount, err := s.repo.CountBetween(ctx, current.Start, current.End)
	if err != nil {
		return nil, err
	}
	previousCount, err := s.repo.CountBetween(ctx, previous.Start, previous.End)
	if err != nil {
		return nil, err
	}

	return &dto.PurchaseSummaryResponse{
		TotalPurchase:    int(currentCount),
		PercentageChange: reporting.PercentageChange(currentCount, previousCount),
	}, nil
}
