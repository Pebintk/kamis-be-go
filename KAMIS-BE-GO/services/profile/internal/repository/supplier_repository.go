package repository

import (
	"context"

	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/profile/internal/model"
	"gorm.io/gorm"
)

type SupplierRepository struct{ db *gorm.DB }

func NewSupplierRepository(db *gorm.DB) *SupplierRepository { return &SupplierRepository{db: db} }

// FindByID returns the supplier with its three ID collections loaded.
func (r *SupplierRepository) FindByID(ctx context.Context, id string) (*model.Supplier, error) {
	var s model.Supplier
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&s).Error; err != nil {
		return nil, database.Translate(err)
	}
	if err := r.loadCollections(ctx, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Create inserts the supplier and its resource/purchase links.
func (r *SupplierRepository) Create(ctx context.Context, s *model.Supplier) error {
	return database.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(s).Error; err != nil {
			return err
		}
		return replaceCollections(tx, s)
	}))
}

// Save updates the supplier and replaces its collection rows.
func (r *SupplierRepository) Save(ctx context.Context, s *model.Supplier) error {
	return database.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(s).Error; err != nil {
			return err
		}
		return replaceCollections(tx, s)
	}))
}

// AddPurchaseID appends one purchase link, the narrow write the purchase service
// makes via PUT /api/supplier/add-purchase.
func (r *SupplierRepository) AddPurchaseID(ctx context.Context, supplierID, purchaseID string) error {
	return database.Translate(r.db.WithContext(ctx).
		Create(&model.SupplierPurchase{SupplierID: supplierID, PurchaseID: purchaseID}).Error)
}

// Uniqueness checks. The legacy repository spells these out as
// existsByNameSupplier / existsByNameSupplierAndIdNot and so on; the two helpers
// below back all of them, with the column names as constants so no caller can
// pass an arbitrary identifier into the query.
const (
	colName    = "name_supplier"
	colNoTelp  = "no_telp_supplier"
	colEmail   = "email_supplier"
	colCompany = "company_supplier"
)

func (r *SupplierRepository) exists(ctx context.Context, column, value string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Supplier{}).
		Where(column+" = ?", value).Count(&count).Error
	return count > 0, err
}

// existsExcluding is the existsBy...AndIdNot family: uniqueness checks that
// ignore the row currently being updated.
func (r *SupplierRepository) existsExcluding(ctx context.Context, column, value, excludeID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Supplier{}).
		Where(column+" = ?", value).
		Where("id <> ?", excludeID).Count(&count).Error
	return count > 0, err
}

func (r *SupplierRepository) ExistsByName(ctx context.Context, v string) (bool, error) {
	return r.exists(ctx, colName, v)
}

func (r *SupplierRepository) ExistsByNoTelp(ctx context.Context, v string) (bool, error) {
	return r.exists(ctx, colNoTelp, v)
}

func (r *SupplierRepository) ExistsByEmail(ctx context.Context, v string) (bool, error) {
	return r.exists(ctx, colEmail, v)
}

func (r *SupplierRepository) ExistsByCompany(ctx context.Context, v string) (bool, error) {
	return r.exists(ctx, colCompany, v)
}

func (r *SupplierRepository) ExistsByNameExcluding(ctx context.Context, v, id string) (bool, error) {
	return r.existsExcluding(ctx, colName, v, id)
}

func (r *SupplierRepository) ExistsByNoTelpExcluding(ctx context.Context, v, id string) (bool, error) {
	return r.existsExcluding(ctx, colNoTelp, v, id)
}

func (r *SupplierRepository) ExistsByEmailExcluding(ctx context.Context, v, id string) (bool, error) {
	return r.existsExcluding(ctx, colEmail, v, id)
}

func (r *SupplierRepository) filter(ctx context.Context, nameSupplier, companySupplier string) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&model.Supplier{})
	if nameSupplier != "" {
		q = q.Where("name_supplier ILIKE ?", "%"+nameSupplier+"%")
	}
	if companySupplier != "" {
		q = q.Where("company_supplier ILIKE ?", "%"+companySupplier+"%")
	}
	return q
}

// FindFiltered returns every matching supplier, collections loaded.
func (r *SupplierRepository) FindFiltered(ctx context.Context, nameSupplier, companySupplier string) ([]model.Supplier, error) {
	var suppliers []model.Supplier
	if err := r.filter(ctx, nameSupplier, companySupplier).Order("name_supplier").Find(&suppliers).Error; err != nil {
		return nil, err
	}
	return suppliers, r.loadCollectionsFor(ctx, suppliers)
}

// FindFilteredPaginated returns one page of matching suppliers plus the total.
func (r *SupplierRepository) FindFilteredPaginated(ctx context.Context, nameSupplier, companySupplier string, page, size int) ([]model.Supplier, int64, error) {
	q := r.filter(ctx, nameSupplier, companySupplier)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var suppliers []model.Supplier
	if err := q.Order("name_supplier").Offset(page * size).Limit(size).Find(&suppliers).Error; err != nil {
		return nil, 0, err
	}
	if err := r.loadCollectionsFor(ctx, suppliers); err != nil {
		return nil, 0, err
	}
	return suppliers, total, nil
}

func (r *SupplierRepository) loadCollections(ctx context.Context, s *model.Supplier) error {
	return r.loadCollectionsFor(ctx, []model.Supplier{*s})
}

// loadCollectionsFor fills the ID slices for a batch of suppliers in three
// queries, rather than three per supplier.
func (r *SupplierRepository) loadCollectionsFor(ctx context.Context, suppliers []model.Supplier) error {
	if len(suppliers) == 0 {
		return nil
	}
	ids := make([]string, 0, len(suppliers))
	byID := make(map[string]*model.Supplier, len(suppliers))
	for i := range suppliers {
		ids = append(ids, suppliers[i].ID)
		byID[suppliers[i].ID] = &suppliers[i]
		suppliers[i].AssetIDs = []string{}
		suppliers[i].ResourceIDs = []int64{}
		suppliers[i].PurchaseIDs = []string{}
	}

	db := r.db.WithContext(ctx)

	var assets []model.SupplierAsset
	if err := db.Where("supplier_id IN ?", ids).Find(&assets).Error; err != nil {
		return err
	}
	for _, a := range assets {
		if s := byID[a.SupplierID]; s != nil {
			s.AssetIDs = append(s.AssetIDs, a.AssetID)
		}
	}

	var resources []model.SupplierResource
	if err := db.Where("supplier_id IN ?", ids).Find(&resources).Error; err != nil {
		return err
	}
	for _, res := range resources {
		if s := byID[res.SupplierID]; s != nil {
			s.ResourceIDs = append(s.ResourceIDs, res.ResourceID)
		}
	}

	var purchases []model.SupplierPurchase
	if err := db.Where("supplier_id IN ?", ids).Find(&purchases).Error; err != nil {
		return err
	}
	for _, p := range purchases {
		if s := byID[p.SupplierID]; s != nil {
			s.PurchaseIDs = append(s.PurchaseIDs, p.PurchaseID)
		}
	}
	return nil
}

// replaceCollections rewrites the collection tables to match the in-memory
// slices, the way Hibernate re-syncs an @ElementCollection on flush.
func replaceCollections(tx *gorm.DB, s *model.Supplier) error {
	if err := tx.Where("supplier_id = ?", s.ID).Delete(&model.SupplierAsset{}).Error; err != nil {
		return err
	}
	if err := tx.Where("supplier_id = ?", s.ID).Delete(&model.SupplierResource{}).Error; err != nil {
		return err
	}
	if err := tx.Where("supplier_id = ?", s.ID).Delete(&model.SupplierPurchase{}).Error; err != nil {
		return err
	}

	// Batch each collection into a single INSERT rather than one per row.
	if len(s.AssetIDs) > 0 {
		rows := make([]model.SupplierAsset, 0, len(s.AssetIDs))
		for _, assetID := range s.AssetIDs {
			rows = append(rows, model.SupplierAsset{SupplierID: s.ID, AssetID: assetID})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	if len(s.ResourceIDs) > 0 {
		rows := make([]model.SupplierResource, 0, len(s.ResourceIDs))
		for _, resourceID := range s.ResourceIDs {
			rows = append(rows, model.SupplierResource{SupplierID: s.ID, ResourceID: resourceID})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	if len(s.PurchaseIDs) > 0 {
		rows := make([]model.SupplierPurchase, 0, len(s.PurchaseIDs))
		for _, purchaseID := range s.PurchaseIDs {
			rows = append(rows, model.SupplierPurchase{SupplierID: s.ID, PurchaseID: purchaseID})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	return nil
}
