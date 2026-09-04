package service

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/pkg/reporting"
	"github.com/karina/kamis-be-go/services/finance/internal/dto"
	"github.com/karina/kamis-be-go/services/finance/internal/model"
	"github.com/karina/kamis-be-go/services/finance/internal/repository"
)

// ExpenseChart breaks outgoings down by what caused them.
func (s *LapkeuService) ExpenseChart(ctx context.Context, rangeName string) ([]dto.ChartPengeluaranResponse, error) {
	// The legacy switch fell through to THIS_YEAR for an unrecognised range
	// rather than rejecting it, so an empty parameter still drew a chart.
	win, err := reporting.RangeWindow(rangeName, time.Now())
	if err != nil {
		win, _ = reporting.RangeWindow(reporting.RangeThisYear, time.Now())
	}

	rows, err := s.repo.ExpenseByActivityType(ctx, win.Start, win.End)
	if err != nil {
		return nil, err
	}

	out := make([]dto.ChartPengeluaranResponse, 0, len(rows))
	for _, row := range rows {
		name, known := model.ActivityName[row.ActivityType]
		if !known {
			name = "UNKNOWN"
		}
		out = append(out, dto.ChartPengeluaranResponse{
			ActivityType:     name,
			TotalPengeluaran: row.Pengeluaran,
		})
	}
	return out, nil
}

// IncomeExpenseChart plots income against outgoings across a window, with empty
// periods filled in so the line has no gaps.
func (s *LapkeuService) IncomeExpenseChart(ctx context.Context, periodType, rangeName string) ([]dto.IncomeExpenseResponse, error) {
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

	totals, err := s.aggregate(ctx, periodType, win)
	if err != nil {
		return nil, err
	}
	periods, err := reporting.Periods(periodType, win)
	if err != nil {
		return nil, apierr.Invalidf("Invalid period type: %s", periodType)
	}

	out := make([]dto.IncomeExpenseResponse, 0, len(periods))
	for _, p := range periods {
		row := totals[p]
		out = append(out, dto.IncomeExpenseResponse{
			Period:           p,
			TotalPemasukan:   row.Pemasukan,
			TotalPengeluaran: row.Pengeluaran,
		})
	}
	return out, nil
}

// IncomeExpenseTotals is the bar chart: the same periods, followed by a "Total"
// bar summing them. The trailing row is the legacy shape and the frontend
// renders it as its own bar.
func (s *LapkeuService) IncomeExpenseTotals(ctx context.Context, periodType, rangeName string) ([]dto.IncomeExpenseResponse, error) {
	rows, err := s.IncomeExpenseChart(ctx, periodType, rangeName)
	if err != nil {
		return nil, err
	}

	var income, expense int64
	for _, row := range rows {
		income += row.TotalPemasukan
		expense += row.TotalPengeluaran
	}
	return append(rows, dto.IncomeExpenseResponse{
		Period:           "Total",
		TotalPemasukan:   income,
		TotalPengeluaran: expense,
	}), nil
}

func (s *LapkeuService) aggregate(ctx context.Context, periodType string, win reporting.Window) (map[string]repository.PeriodTotal, error) {
	totals := map[string]repository.PeriodTotal{}

	if periodType == reporting.PeriodWeekly {
		rows, err := s.repo.IncomeExpenseByDay(ctx, win.Start, win.End)
		if err != nil {
			return nil, err
		}
		// Days fold into the week labels of the window's opening month.
		for _, row := range rows {
			label := reporting.WeekLabel(row.Day, win.Start)
			running := totals[label]
			running.Pemasukan += row.Pemasukan
			running.Pengeluaran += row.Pengeluaran
			totals[label] = running
		}
		return totals, nil
	}

	var (
		rows []repository.PeriodTotal
		err  error
	)
	switch periodType {
	case reporting.PeriodMonthly:
		rows, err = s.repo.IncomeExpenseByMonth(ctx, win.Start, win.End)
	case reporting.PeriodQuarterly:
		rows, err = s.repo.IncomeExpenseByQuarter(ctx, win.Start, win.End)
	case reporting.PeriodYearly:
		rows, err = s.repo.IncomeExpenseByYear(ctx, win.Start, win.End)
	default:
		return nil, apierr.Invalidf("Invalid period type: %s", periodType)
	}
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		totals[row.Period] = row
	}
	return totals, nil
}

// FinancialSummary is the finance dashboard's headline panel: what came in,
// what went out, and how both moved against the comparison period.
func (s *LapkeuService) FinancialSummary(ctx context.Context, rangeName string) (*dto.FinancialSummaryResponse, error) {
	current, previous, err := reporting.SummaryWindows(rangeName, time.Now())
	if err != nil {
		return nil, apierr.Invalidf("Range tidak dikenali: %s. Gunakan THIS_YEAR, THIS_QUARTER, THIS_MONTH, atau LAST_YEAR.", rangeName)
	}

	// Two grouped queries answer the whole panel. Java asked for each figure
	// separately — sixteen round trips across the two windows.
	now, err := s.totals(ctx, current)
	if err != nil {
		return nil, err
	}
	before, err := s.totals(ctx, previous)
	if err != nil {
		return nil, err
	}

	return &dto.FinancialSummaryResponse{
		TotalIncome:               now.income,
		TotalIncomeFromDistribusi: now.byType[model.ActivityDistribusi].Pemasukan,
		TotalIncomeFromPenjualan:  now.byType[model.ActivityPenjualan].Pemasukan,
		TotalPurchase:             now.byType[model.ActivityPurchase].Pengeluaran,
		TotalMaintenanceExpense:   now.byType[model.ActivityMaintenance].Pengeluaran,
		// Projects spend on distributions; a sale has no outgoings, but it is
		// summed in anyway so the figure means "what projects cost".
		TotalProjectExpense: now.byType[model.ActivityDistribusi].Pengeluaran +
			now.byType[model.ActivityPenjualan].Pengeluaran,
		TotalProfit:                 now.income - now.expense,
		TotalTransactions:           int(now.count),
		TransactionPercentageChange: reporting.PercentageChange(now.count, before.count),
		ProfitPercentageChange:      reporting.PercentageChange(now.income-now.expense, before.income-before.expense),
	}, nil
}

// windowTotals is one window's ledger, indexed by activity type.
type windowTotals struct {
	byType                 map[int]repository.ActivityTotal
	income, expense, count int64
}

func (s *LapkeuService) totals(ctx context.Context, win reporting.Window) (windowTotals, error) {
	out := windowTotals{byType: map[int]repository.ActivityTotal{}}

	rows, err := s.repo.TotalsByActivityType(ctx, win.Start, win.End)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		out.byType[row.ActivityType] = row
		out.income += row.Pemasukan
		out.expense += row.Pengeluaran
		out.count += row.Count
	}
	return out, nil
}

// CombinedActivityChart puts the three activity streams side by side, reading
// each from the service that owns it.
//
// The three calls are independent, so they run together; Java made them one
// after another.
func (s *LapkeuService) CombinedActivityChart(ctx context.Context, rangeName, periodType, status string) ([]dto.ActivityComparisonResponse, error) {
	type stream struct {
		client *httpx.Client
		path   string
		assign func(*dto.ActivityComparisonResponse, int64)
	}

	query := url.Values{}
	query.Set("range", rangeName)
	query.Set("status", status)
	if strings.TrimSpace(periodType) != "" {
		query.Set("periodType", periodType)
	}
	suffix := "?" + query.Encode()

	streams := []stream{
		{s.Purchase, "/purchase/chart/purchase-activity" + suffix,
			func(r *dto.ActivityComparisonResponse, v int64) { r.PembelianCount = v }},
		{s.Project, "/project/chart/penjualan-activity" + suffix,
			func(r *dto.ActivityComparisonResponse, v int64) { r.PenjualanCount = v }},
		{s.Project, "/project/chart/distribusi-activity" + suffix,
			func(r *dto.ActivityComparisonResponse, v int64) { r.DistribusiCount = v }},
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		byPeriod = map[string]*dto.ActivityComparisonResponse{}
		failures []error
	)
	for _, st := range streams {
		wg.Add(1)
		go func(st stream) {
			defer wg.Done()
			points, err := httpx.GetData[[]dto.ActivityLine](ctx, st.client, st.path)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures = append(failures, err)
				return
			}
			for _, point := range points {
				row, ok := byPeriod[point.Period]
				if !ok {
					row = &dto.ActivityComparisonResponse{Period: point.Period}
					byPeriod[point.Period] = row
				}
				st.assign(row, point.Count)
			}
		}(st)
	}
	wg.Wait()

	// A stream that could not be read would silently plot as zero, which reads
	// as "no activity" rather than "no data" — so the request fails instead.
	if len(failures) > 0 {
		return nil, apierr.Invalidf("Gagal mengambil data aktivitas: %v", failures[0])
	}

	periods := make([]string, 0, len(byPeriod))
	for period := range byPeriod {
		periods = append(periods, period)
	}
	// The legacy code used a TreeMap, so periods came back sorted by label —
	// which for every label format this service uses is also chronological.
	sort.Strings(periods)

	out := make([]dto.ActivityComparisonResponse, 0, len(periods))
	for _, period := range periods {
		out = append(out, *byPeriod[period])
	}
	return out, nil
}
