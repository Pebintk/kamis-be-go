package service

import (
	"context"
	"strings"
	"time"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/reporting"
	"github.com/karina/kamis-be-go/services/project/internal/dto"
	"github.com/karina/kamis-be-go/services/project/internal/model"
	"github.com/karina/kamis-be-go/services/project/internal/repository"
)

// statusScopeFor turns the chart's filter name into a status predicate.
//
// "ALL" carries the cancelled status but *excludes* it, so it means "every
// project that was not cancelled" — the same misleading-but-deliberate naming
// the purchase chart uses.
func statusScopeFor(filter string) repository.StatusScope {
	switch strings.ToUpper(filter) {
	case "CANCELLED":
		return repository.StatusScope{Statuses: []int{model.StatusBatal}}
	case "DONE":
		return repository.StatusScope{Statuses: []int{model.StatusSelesai}}
	default:
		// Java fell through to ALL for any unrecognised value rather than
		// rejecting it, and the frontend only ever sends the three.
		return repository.StatusScope{Statuses: []int{model.StatusBatal}, Exclude: true}
	}
}

// ActivityLine returns one count per period for one kind of project, with empty
// periods filled in so the chart has no gaps.
func (s *ProjectService) ActivityLine(ctx context.Context, periodType, rangeName, statusFilter string, projectType bool) ([]dto.ActivityLineResponse, error) {
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

	counts, err := s.aggregate(ctx, periodType, win, projectType, statusScopeFor(statusFilter))
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

func (s *ProjectService) aggregate(ctx context.Context, periodType string, win reporting.Window, projectType bool, scope repository.StatusScope) (map[string]int64, error) {
	counts := map[string]int64{}

	switch periodType {
	case reporting.PeriodWeekly:
		rows, err := s.repo.CountByDay(ctx, win.Start, win.End, projectType, scope)
		if err != nil {
			return nil, err
		}
		// Days fold into the week labels of the window's opening month.
		for _, row := range rows {
			counts[reporting.WeekLabel(row.Day, win.Start)] += row.Count
		}
		return counts, nil

	case reporting.PeriodMonthly:
		rows, err := s.repo.CountByMonth(ctx, win.Start, win.End, projectType, scope)
		if err != nil {
			return nil, err
		}
		return tally(rows), nil

	case reporting.PeriodQuarterly:
		rows, err := s.repo.CountByQuarter(ctx, win.Start, win.End, projectType, scope)
		if err != nil {
			return nil, err
		}
		return tally(rows), nil

	case reporting.PeriodYearly:
		// Unreachable through the API: no range permits YEARLY. Implemented so a
		// future range does not silently chart nothing.
		rows, err := s.repo.CountByYear(ctx, win.Start, win.End, projectType, scope)
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

// ListByRange returns the projects recorded in a named range.
func (s *ProjectService) ListByRange(ctx context.Context, rangeName string) ([]dto.ProjectListResponse, error) {
	win, err := reporting.RangeWindow(rangeName, time.Now())
	if err != nil {
		return nil, apierr.Invalidf("%s", err)
	}
	return s.ListProjects(ctx, repository.Filter{StartDate: &win.Start, EndDate: &win.End})
}

// SummaryByRange counts each kind of project in a period and how each moved
// against the comparison period. Every status counts, cancelled included.
func (s *ProjectService) SummaryByRange(ctx context.Context, rangeName string) (*dto.SellDistributionSummaryResponse, error) {
	current, previous, err := reporting.SummaryWindows(rangeName, time.Now())
	if err != nil {
		return nil, apierr.Invalidf("Range tidak dikenali: %s. Gunakan THIS_YEAR, THIS_QUARTER, THIS_MONTH, atau LAST_YEAR.", rangeName)
	}

	counts := map[bool][2]int64{}
	for _, kind := range []bool{model.TypePenjualan, model.TypePengiriman} {
		now, err := s.repo.CountByType(ctx, current.Start, current.End, kind)
		if err != nil {
			return nil, err
		}
		before, err := s.repo.CountByType(ctx, previous.Start, previous.End, kind)
		if err != nil {
			return nil, err
		}
		counts[kind] = [2]int64{now, before}
	}

	sell, distribution := counts[model.TypePenjualan], counts[model.TypePengiriman]
	return &dto.SellDistributionSummaryResponse{
		TotalSell:                    sell[0],
		PercentageSellChange:         reporting.PercentageChange(sell[0], sell[1]),
		TotalDistribution:            distribution[0],
		PercentageDistributionChange: reporting.PercentageChange(distribution[0], distribution[1]),
	}, nil
}
