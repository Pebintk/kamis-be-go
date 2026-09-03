// Package repository is the data-access layer (the Spring Data JPA repositories).
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/services/purchase/internal/model"
	"gorm.io/gorm"
)

type PurchaseRepository struct{ db *gorm.DB }

func NewPurchaseRepository(db *gorm.DB) *PurchaseRepository { return &PurchaseRepository{db: db} }

// Filter carries the optional list predicates. Java applied all of these in
// memory after loading the whole table; they are SQL here.
type Filter struct {
	StartNominal *int
	EndNominal   *int
	StartDate    *time.Time
	EndDate      *time.Time
	Type         string // "aset", "resource", or "" / "all" for both
	IDSearch     string
	Status       string

	// HighNominal sorts by price when set (true = descending); NewDate sorts by
	// submission date descending. Price wins if both are given, matching the
	// legacy comparator.
	HighNominal *bool
	NewDate     *bool
}

func (r *PurchaseRepository) query(ctx context.Context, f Filter) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&model.Purchase{})

	if f.StartNominal != nil {
		q = q.Where("purchase_price >= ?", *f.StartNominal)
	}
	if f.EndNominal != nil {
		q = q.Where("purchase_price <= ?", *f.EndNominal)
	}
	if f.StartDate != nil {
		q = q.Where("purchase_submission_date >= ?", *f.StartDate)
	}
	if f.EndDate != nil {
		// The legacy code pushed endDate to 23:59:59.999 so the whole day counts;
		// a half-open bound to the next midnight is the same thing without the
		// millisecond fudge.
		q = q.Where("purchase_submission_date < ?", f.EndDate.AddDate(0, 0, 1))
	}
	switch f.Type {
	case "aset":
		q = q.Where("purchase_type = ?", model.TypeAset)
	case "resource":
		q = q.Where("purchase_type = ?", model.TypeResource)
	}
	if f.IDSearch != "" {
		q = q.Where("id ILIKE ?", "%"+f.IDSearch+"%")
	}
	if f.Status != "" {
		q = q.Where("purchase_status = ?", f.Status)
	}
	return q
}

// order reproduces the legacy comparator: price if highNominal was given at
// all, else date descending if newDate is true, else date ascending.
func order(f Filter) string {
	switch {
	case f.HighNominal != nil && *f.HighNominal:
		return "purchase_price DESC"
	case f.HighNominal != nil:
		return "purchase_price ASC"
	case f.NewDate != nil && *f.NewDate:
		return "purchase_submission_date DESC"
	default:
		return "purchase_submission_date ASC"
	}
}

// FindFiltered returns every matching purchase.
func (r *PurchaseRepository) FindFiltered(ctx context.Context, f Filter) ([]model.Purchase, error) {
	var out []model.Purchase
	err := r.query(ctx, f).Order(order(f)).Find(&out).Error
	return out, err
}

// FindFilteredPaginated returns one page of matching purchases plus the total.
func (r *PurchaseRepository) FindFilteredPaginated(ctx context.Context, f Filter, number, size int) ([]model.Purchase, int64, error) {
	q := r.query(ctx, f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var out []model.Purchase
	if err := q.Order(order(f)).Offset(number * size).Limit(size).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *PurchaseRepository) FindByID(ctx context.Context, id string) (*model.Purchase, error) {
	var p model.Purchase
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &p, nil
}

func (r *PurchaseRepository) FindBySupplier(ctx context.Context, supplierID string) ([]model.Purchase, error) {
	var out []model.Purchase
	err := r.db.WithContext(ctx).Where("purchase_supplier = ?", supplierID).
		Order("purchase_submission_date DESC").Find(&out).Error
	return out, err
}

func (r *PurchaseRepository) Save(ctx context.Context, p *model.Purchase) error {
	return database.Translate(r.db.WithContext(ctx).Save(p).Error)
}

// ---- id generation ----

// maxIDAttempts bounds the retry below. A collision needs two creates in the
// same millisecond window; more than a handful of retries means something else
// is wrong.
const maxIDAttempts = 5

// idPrefix is the {A|R}-{ddMMyy}- part of a purchase id.
func idPrefix(purchaseType bool, now time.Time) string {
	kind := "A-"
	if purchaseType == model.TypeResource {
		kind = "R-"
	}
	return kind + now.Format("020106") + "-"
}

// CreateWithGeneratedID inserts a purchase under the next id for today,
// together with its line items and first log entry, in one transaction.
//
// The id is {A|R}-{ddMMyy}-{NNN}, where NNN counts *all* purchases submitted
// today regardless of type — so an asset and a resource purchase on the same day
// share one sequence. Java derived NNN by loading every purchase ever recorded
// and counting today's in memory, which was both a full table scan per create
// and a lost-update race: two concurrent creates read the same count, built the
// same id, and `save()` on an existing primary key is an update, so one purchase
// silently overwrote the other. Here the count is a SQL COUNT and a colliding
// insert is retried with the next number.
// The first log entry's text names the id, which does not exist until the
// insert succeeds, so the caller supplies actionFor to build it.
func (r *PurchaseRepository) CreateWithGeneratedID(
	ctx context.Context,
	p *model.Purchase,
	lines []model.ResourceTemp,
	logUsername string,
	actionFor func(purchaseID string) string,
) error {
	now := time.Now()
	prefix := idPrefix(p.PurchaseType, now)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var lastErr error
	for attempt := 0; attempt < maxIDAttempts; attempt++ {
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var todayCount int64
			if err := tx.Model(&model.Purchase{}).
				Where("purchase_submission_date >= ? AND purchase_submission_date < ?",
					dayStart, dayStart.AddDate(0, 0, 1)).
				Count(&todayCount).Error; err != nil {
				return err
			}

			p.ID = fmt.Sprintf("%s%03d", prefix, todayCount+1+int64(attempt))
			if err := tx.Create(p).Error; err != nil {
				return database.Translate(err)
			}

			for i := range lines {
				lines[i].PurchaseID = p.ID
			}
			if len(lines) > 0 {
				if err := tx.Create(&lines).Error; err != nil {
					return err
				}
			}

			return tx.Create(&model.LogPurchase{
				PurchaseID: p.ID,
				Username:   logUsername,
				Action:     actionFor(p.ID),
			}).Error
		})
		if err == nil {
			return nil
		}
		lastErr = err
		if !errors.Is(err, database.ErrDuplicate) {
			return err
		}
	}
	return fmt.Errorf("could not allocate a purchase id after %d attempts: %w", maxIDAttempts, lastErr)
}

// ---- line items, logs, staged assets ----

// FindLinesFor loads the resource line items of several purchases at once,
// keyed by purchase id. Java reached them through the @OneToMany on every row.
func (r *PurchaseRepository) FindLinesFor(ctx context.Context, purchaseIDs []string) (map[string][]model.ResourceTemp, error) {
	out := make(map[string][]model.ResourceTemp, len(purchaseIDs))
	if len(purchaseIDs) == 0 {
		return out, nil
	}

	var rows []model.ResourceTemp
	if err := r.db.WithContext(ctx).Where("purchase_id IN ?", purchaseIDs).
		Order("resource_name").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PurchaseID] = append(out[row.PurchaseID], row)
	}
	return out, nil
}

// FindLogsFor loads the audit trails of several purchases at once, oldest first.
func (r *PurchaseRepository) FindLogsFor(ctx context.Context, purchaseIDs []string) (map[string][]model.LogPurchase, error) {
	out := make(map[string][]model.LogPurchase, len(purchaseIDs))
	if len(purchaseIDs) == 0 {
		return out, nil
	}

	var rows []model.LogPurchase
	if err := r.db.WithContext(ctx).Where("purchase_id IN ?", purchaseIDs).
		Order("action_date").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PurchaseID] = append(out[row.PurchaseID], row)
	}
	return out, nil
}

// ReplaceLines swaps a purchase's line items for a new set, soft-deleting the
// old ones so a cancelled edit is still auditable.
func (r *PurchaseRepository) ReplaceLines(ctx context.Context, purchaseID string, lines []model.ResourceTemp) error {
	return database.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("purchase_id = ?", purchaseID).Delete(&model.ResourceTemp{}).Error; err != nil {
			return err
		}
		if len(lines) == 0 {
			return nil
		}
		for i := range lines {
			lines[i].PurchaseID = purchaseID
		}
		return tx.Create(&lines).Error
	}))
}

func (r *PurchaseRepository) AppendLog(ctx context.Context, entry *model.LogPurchase) error {
	return database.Translate(r.db.WithContext(ctx).Create(entry).Error)
}

func (r *PurchaseRepository) FindAssetTempByID(ctx context.Context, id int64) (*model.AssetTemp, error) {
	var a model.AssetTemp
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&a).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &a, nil
}

// FindAssetTempsByIDs loads several staged assets at once, keyed by id.
func (r *PurchaseRepository) FindAssetTempsByIDs(ctx context.Context, ids []int64) (map[int64]model.AssetTemp, error) {
	out := make(map[int64]model.AssetTemp, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	var rows []model.AssetTemp
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row
	}
	return out, nil
}

func (r *PurchaseRepository) CreateAssetTemp(ctx context.Context, a *model.AssetTemp) error {
	return database.Translate(r.db.WithContext(ctx).Create(a).Error)
}

func (r *PurchaseRepository) FindAllAssetTemps(ctx context.Context) ([]model.AssetTemp, error) {
	var out []model.AssetTemp
	err := r.db.WithContext(ctx).Order("id").Find(&out).Error
	return out, err
}

// ---- reporting aggregations ----

// PeriodCount is one row of a grouped count.
type PeriodCount struct {
	Period string
	Count  int64
}

// DayCount is one row of a per-day grouped count, kept as a date so the caller
// can bucket it into weeks.
type DayCount struct {
	Day   time.Time
	Count int64
}

// StatusScope narrows an aggregation to, or away from, a set of statuses.
type StatusScope struct {
	Statuses []string
	// Exclude counts everything *but* Statuses. The chart's "ALL" filter uses
	// this to mean "all active purchases", excluding rejected and cancelled.
	Exclude bool
}

func (r *PurchaseRepository) window(ctx context.Context, start, end time.Time, scope StatusScope) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&model.Purchase{}).
		Where("purchase_submission_date >= ? AND purchase_submission_date <= ?", start, end)
	if len(scope.Statuses) == 0 {
		return q
	}
	if scope.Exclude {
		return q.Where("purchase_status NOT IN ?", scope.Statuses)
	}
	return q.Where("purchase_status IN ?", scope.Statuses)
}

// countByExpr groups by a SQL expression over the submission date.
func (r *PurchaseRepository) countByExpr(ctx context.Context, expr string, start, end time.Time, scope StatusScope) ([]PeriodCount, error) {
	var out []PeriodCount
	err := r.window(ctx, start, end, scope).
		Select(expr + " AS period, COUNT(*) AS count").
		Group(expr).Order("period").Scan(&out).Error
	return out, err
}

// CountByMonth groups as yyyy-MM.
func (r *PurchaseRepository) CountByMonth(ctx context.Context, start, end time.Time, scope StatusScope) ([]PeriodCount, error) {
	return r.countByExpr(ctx, "to_char(purchase_submission_date, 'YYYY-MM')", start, end, scope)
}

// CountByQuarter groups as yyyy-Qn.
func (r *PurchaseRepository) CountByQuarter(ctx context.Context, start, end time.Time, scope StatusScope) ([]PeriodCount, error) {
	return r.countByExpr(ctx,
		"to_char(purchase_submission_date, 'YYYY') || '-Q' || to_char(purchase_submission_date, 'Q')",
		start, end, scope)
}

// CountByYear groups as yyyy.
func (r *PurchaseRepository) CountByYear(ctx context.Context, start, end time.Time, scope StatusScope) ([]PeriodCount, error) {
	return r.countByExpr(ctx, "to_char(purchase_submission_date, 'YYYY')", start, end, scope)
}

// CountByDay groups by calendar day, which the service then buckets into weeks.
//
// The legacy query grouped by the raw timestamp column, so every purchase landed
// in its own group; the in-memory re-aggregation into weeks happened to hide it.
// date_trunc groups by the day actually meant.
func (r *PurchaseRepository) CountByDay(ctx context.Context, start, end time.Time, scope StatusScope) ([]DayCount, error) {
	var out []DayCount
	err := r.window(ctx, start, end, scope).
		Select("date_trunc('day', purchase_submission_date) AS day, COUNT(*) AS count").
		Group("date_trunc('day', purchase_submission_date)").Order("day").Scan(&out).Error
	return out, err
}

// CountBetween counts every purchase submitted in a window, whatever its status.
func (r *PurchaseRepository) CountBetween(ctx context.Context, start, end time.Time) (int64, error) {
	var count int64
	err := r.window(ctx, start, end, StatusScope{}).Count(&count).Error
	return count, err
}
