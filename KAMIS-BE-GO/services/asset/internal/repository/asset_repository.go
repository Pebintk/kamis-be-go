// Package repository is the data-access layer (the Spring Data JPA repositories).
package repository

import (
	"context"
	"time"

	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/services/asset/internal/model"
	"gorm.io/gorm"
)

type AssetRepository struct{ db *gorm.DB }

func NewAssetRepository(db *gorm.DB) *AssetRepository { return &AssetRepository{db: db} }

// Every query below relies on GORM's soft-delete support to exclude deleted
// rows, which is what the Java repository's ...AndIsDeletedFalse suffixes did by
// hand.

func (r *AssetRepository) FindAll(ctx context.Context) ([]model.Asset, error) {
	var out []model.Asset
	err := r.db.WithContext(ctx).Order("plat_nomor").Find(&out).Error
	return out, err
}

func (r *AssetRepository) FindByPlatNomor(ctx context.Context, platNomor string) (*model.Asset, error) {
	var asset model.Asset
	if err := r.db.WithContext(ctx).Where("plat_nomor = ?", platNomor).First(&asset).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &asset, nil
}

func (r *AssetRepository) ExistsByPlatNomor(ctx context.Context, platNomor string) (bool, error) {
	var count int64
	// Unscoped so a soft-deleted row still counts: the plate number is the
	// primary key, so re-inserting it would resurrect the old row rather than
	// create a new asset.
	err := r.db.WithContext(ctx).Unscoped().Model(&model.Asset{}).
		Where("plat_nomor = ?", platNomor).Count(&count).Error
	return count > 0, err
}

func (r *AssetRepository) Create(ctx context.Context, asset *model.Asset) error {
	return database.Translate(r.db.WithContext(ctx).Create(asset).Error)
}

func (r *AssetRepository) Save(ctx context.Context, asset *model.Asset) error {
	return database.Translate(r.db.WithContext(ctx).Save(asset).Error)
}

// SoftDelete marks the asset deleted, leaving the row in place.
func (r *AssetRepository) SoftDelete(ctx context.Context, platNomor string) error {
	return database.Translate(r.db.WithContext(ctx).
		Where("plat_nomor = ?", platNomor).Delete(&model.Asset{}).Error)
}

func (r *AssetRepository) FindBySupplier(ctx context.Context, supplierID string) ([]model.Asset, error) {
	var out []model.Asset
	err := r.db.WithContext(ctx).Where("id_supplier = ?", supplierID).
		Order("plat_nomor").Find(&out).Error
	return out, err
}

// FindPaginated returns one page of assets plus the total, with the three
// optional case-insensitive substring filters the Java repository spelled out as
// eight separate derived query methods.
func (r *AssetRepository) FindPaginated(ctx context.Context, nama, jenisAset, status string, number, size int) ([]model.Asset, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Asset{})
	for column, value := range map[string]string{
		"nama":       nama,
		"jenis_aset": jenisAset,
		"status":     status,
	} {
		if value != "" {
			q = q.Where(column+" ILIKE ?", "%"+value+"%")
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var out []model.Asset
	if err := q.Order("plat_nomor").Offset(number * size).Limit(size).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// LastMaintenanceDates returns, for each of the given plate numbers that has
// one, the start date of its most recent completed maintenance.
//
// The Java list mapper called a per-asset lookup while building each row, so a
// page of 10 assets issued 10 extra queries and the unpaginated list issued one
// per asset in the table. This answers the whole page at once.
func (r *AssetRepository) LastMaintenanceDates(ctx context.Context, platNomors []string) (map[string]time.Time, error) {
	out := make(map[string]time.Time, len(platNomors))
	if len(platNomors) == 0 {
		return out, nil
	}

	var rows []struct {
		AssetPlatNomor string
		Last           time.Time
	}
	err := r.db.WithContext(ctx).Model(&model.Maintenance{}).
		Select("asset_plat_nomor, MAX(tanggal_mulai_maintenance) AS last").
		Where("asset_plat_nomor IN ? AND status = ?", platNomors, StatusSelesai).
		Group("asset_plat_nomor").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		out[row.AssetPlatNomor] = row.Last
	}
	return out, nil
}

// StatusSelesai is the maintenance status that counts as finished. The legacy
// code repeats this literal in several queries.
const StatusSelesai = "Selesai"

// FindMaintenanceByAsset returns an asset's service history, newest first.
func (r *AssetRepository) FindMaintenanceByAsset(ctx context.Context, platNomor string) ([]model.Maintenance, error) {
	var out []model.Maintenance
	err := r.db.WithContext(ctx).
		Where("asset_plat_nomor = ?", platNomor).
		Order("tanggal_mulai_maintenance DESC").
		Find(&out).Error
	return out, err
}

// ---- maintenance ----

func (r *AssetRepository) FindMaintenanceByID(ctx context.Context, id int64) (*model.Maintenance, error) {
	var m model.Maintenance
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &m, nil
}

func (r *AssetRepository) CreateMaintenance(ctx context.Context, m *model.Maintenance) error {
	return database.Translate(r.db.WithContext(ctx).Create(m).Error)
}

func (r *AssetRepository) SaveMaintenance(ctx context.Context, m *model.Maintenance) error {
	return database.Translate(r.db.WithContext(ctx).Save(m).Error)
}

func (r *AssetRepository) FindAllMaintenance(ctx context.Context) ([]model.Maintenance, error) {
	var out []model.Maintenance
	err := r.db.WithContext(ctx).Order("tanggal_mulai_maintenance DESC").Find(&out).Error
	return out, err
}

func (r *AssetRepository) FindMaintenanceByStatus(ctx context.Context, status string) ([]model.Maintenance, error) {
	var out []model.Maintenance
	err := r.db.WithContext(ctx).Where("status = ?", status).
		Order("tanggal_mulai_maintenance DESC").Find(&out).Error
	return out, err
}

// FindAssetsByPlatNomors loads several assets at once, keyed by plate number.
// The maintenance DTO carries the asset's name, and the Java mapper reached it
// through the @ManyToOne association on every row.
func (r *AssetRepository) FindAssetsByPlatNomors(ctx context.Context, platNomors []string) (map[string]model.Asset, error) {
	out := make(map[string]model.Asset, len(platNomors))
	if len(platNomors) == 0 {
		return out, nil
	}

	var assets []model.Asset
	// Unscoped so maintenance history still names its asset after that asset has
	// been soft-deleted.
	if err := r.db.WithContext(ctx).Unscoped().
		Where("plat_nomor IN ?", platNomors).Find(&assets).Error; err != nil {
		return nil, err
	}
	for _, a := range assets {
		out[a.PlatNomor] = a
	}
	return out, nil
}

// ---- reservations ----

// FindReservationsByAsset returns every reservation held against one vehicle,
// whatever its status.
func (r *AssetRepository) FindReservationsByAsset(ctx context.Context, platNomor string) ([]model.AssetReservation, error) {
	var out []model.AssetReservation
	err := r.db.WithContext(ctx).Where("plat_nomor = ?", platNomor).
		Order("start_date").Find(&out).Error
	return out, err
}
