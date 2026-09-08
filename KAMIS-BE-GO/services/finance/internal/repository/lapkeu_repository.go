// Package repository is the data-access layer (the Spring Data JPA repositories).
package repository

import (
	"context"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/finance/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LapkeuRepository struct{ db *gorm.DB }

func NewLapkeuRepository(db *gorm.DB) *LapkeuRepository { return &LapkeuRepository{db: db} }

// Filter narrows the ledger by payment date and activity type.
type Filter struct {
	StartDate    *time.Time
	EndDate      *time.Time
	ActivityType *int
}

func (r *LapkeuRepository) query(ctx context.Context, f Filter) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&model.Lapkeu{})

	// A row with no payment date is never excluded by a date filter, which is
	// what the legacy in-memory predicate did — it only compared when the date
	// was present.
	if f.StartDate != nil {
		q = q.Where("payment_date IS NULL OR payment_date >= ?", *f.StartDate)
	}
	if f.EndDate != nil {
		q = q.Where("payment_date IS NULL OR payment_date <= ?", *f.EndDate)
	}
	if f.ActivityType != nil {
		q = q.Where("activity_type = ?", *f.ActivityType)
	}
	return q
}

// FindFiltered returns the matching ledger lines, newest payment first.
//
// Java loaded the whole table and filtered it in memory, for both this and the
// summary below.
func (r *LapkeuRepository) FindFiltered(ctx context.Context, f Filter) ([]model.Lapkeu, error) {
	var out []model.Lapkeu
	err := r.query(ctx, f).Order("payment_date DESC NULLS LAST, id").Find(&out).Error
	return out, err
}

// Totals is the summary of a slice of the ledger, computed in SQL.
type Totals struct {
	Count       int64
	Pemasukan   int64
	Pengeluaran int64
}

// SummariseFiltered totals the matching ledger lines. COALESCE keeps an absent
// amount out of the sum rather than turning the whole total null.
func (r *LapkeuRepository) SummariseFiltered(ctx context.Context, f Filter) (Totals, error) {
	var out Totals
	err := r.query(ctx, f).Select(
		"COUNT(*) AS count, " +
			"COALESCE(SUM(COALESCE(pemasukan, 0)), 0) AS pemasukan, " +
			"COALESCE(SUM(COALESCE(pengeluaran, 0)), 0) AS pengeluaran").
		Scan(&out).Error
	return out, err
}

func (r *LapkeuRepository) FindByID(ctx context.Context, id string) (*model.Lapkeu, error) {
	var l model.Lapkeu
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&l).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &l, nil
}

// Upsert writes a ledger line, replacing any entry already under that id.
//
// The id is the id of whatever caused the entry, so this is what makes the
// calling services' retries idempotent: confirming the same payment twice
// updates one row rather than adding a second. Java relied on JPA's save()
// doing the same thing implicitly.
func (r *LapkeuRepository) Upsert(ctx context.Context, l *model.Lapkeu) error {
	return database.Translate(r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).Create(l).Error)
}

func (r *LapkeuRepository) Delete(ctx context.Context, id string) error {
	return database.Translate(r.db.WithContext(ctx).
		Where("id = ?", id).Delete(&model.Lapkeu{}).Error)
}

// ---- reporting aggregations ----

// ActivityTotal is one activity type's contribution over a window.
type ActivityTotal struct {
	ActivityType int
	Pemasukan    int64
	Pengeluaran  int64
	Count        int64
}

// PeriodTotal is one period's income and outgoings.
type PeriodTotal struct {
	Period      string
	Pemasukan   int64
	Pengeluaran int64
}

// DayTotal is one day's income and outgoings, kept as a date so the caller can
// bucket it into weeks.
type DayTotal struct {
	Day         time.Time
	Pemasukan   int64
	Pengeluaran int64
}

func (r *LapkeuRepository) between(ctx context.Context, start, end time.Time) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Lapkeu{}).
		Where("payment_date >= ? AND payment_date <= ?", start, end)
}

// TotalsByActivityType answers the whole financial summary in one grouped query.
//
// Java asked for each figure separately — income by type, expense by type, and a
// count for each of the four types, twice over for the comparison period. That
// is sixteen round trips for one dashboard panel; this is two.
func (r *LapkeuRepository) TotalsByActivityType(ctx context.Context, start, end time.Time) ([]ActivityTotal, error) {
	var out []ActivityTotal
	err := r.between(ctx, start, end).
		Select("activity_type, " +
			"COALESCE(SUM(COALESCE(pemasukan, 0)), 0) AS pemasukan, " +
			"COALESCE(SUM(COALESCE(pengeluaran, 0)), 0) AS pengeluaran, " +
			"COUNT(*) AS count").
		Group("activity_type").Order("activity_type").Scan(&out).Error
	return out, err
}

// ExpenseByActivityType is the expense chart's data: outgoings per activity
// type, excluding types that never spent anything.
func (r *LapkeuRepository) ExpenseByActivityType(ctx context.Context, start, end time.Time) ([]ActivityTotal, error) {
	var out []ActivityTotal
	err := r.between(ctx, start, end).
		Where("pengeluaran IS NOT NULL").
		Select("activity_type, 0 AS pemasukan, " +
			"COALESCE(SUM(pengeluaran), 0) AS pengeluaran, COUNT(*) AS count").
		Group("activity_type").Order("activity_type").Scan(&out).Error
	return out, err
}

func (r *LapkeuRepository) incomeExpenseBy(ctx context.Context, expr string, start, end time.Time) ([]PeriodTotal, error) {
	var out []PeriodTotal
	err := r.between(ctx, start, end).
		Select(expr + " AS period, " +
			"COALESCE(SUM(COALESCE(pemasukan, 0)), 0) AS pemasukan, " +
			"COALESCE(SUM(COALESCE(pengeluaran, 0)), 0) AS pengeluaran").
		Group(expr).Order("period").Scan(&out).Error
	return out, err
}

// IncomeExpenseByMonth groups as yyyy-MM.
func (r *LapkeuRepository) IncomeExpenseByMonth(ctx context.Context, start, end time.Time) ([]PeriodTotal, error) {
	return r.incomeExpenseBy(ctx, "to_char(payment_date, 'YYYY-MM')", start, end)
}

// IncomeExpenseByQuarter groups as yyyy-Qn.
func (r *LapkeuRepository) IncomeExpenseByQuarter(ctx context.Context, start, end time.Time) ([]PeriodTotal, error) {
	return r.incomeExpenseBy(ctx,
		"to_char(payment_date, 'YYYY') || '-Q' || to_char(payment_date, 'Q')", start, end)
}

// IncomeExpenseByYear groups as yyyy.
func (r *LapkeuRepository) IncomeExpenseByYear(ctx context.Context, start, end time.Time) ([]PeriodTotal, error) {
	return r.incomeExpenseBy(ctx, "to_char(payment_date, 'YYYY')", start, end)
}

// IncomeExpenseByDay groups by calendar day, which the service buckets into
// weeks. The legacy query grouped by the raw date column, which for a
// @Temporal(DATE) field happened to be a day already.
func (r *LapkeuRepository) IncomeExpenseByDay(ctx context.Context, start, end time.Time) ([]DayTotal, error) {
	var out []DayTotal
	err := r.between(ctx, start, end).
		Select("date_trunc('day', payment_date) AS day, " +
			"COALESCE(SUM(COALESCE(pemasukan, 0)), 0) AS pemasukan, " +
			"COALESCE(SUM(COALESCE(pengeluaran, 0)), 0) AS pengeluaran").
		Group("date_trunc('day', payment_date)").Order("day").Scan(&out).Error
	return out, err
}
