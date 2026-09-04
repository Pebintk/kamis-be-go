// Package repository is the data-access layer (the Spring Data JPA repositories).
package repository

import (
	"context"
	"time"

	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/services/finance/internal/model"
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
