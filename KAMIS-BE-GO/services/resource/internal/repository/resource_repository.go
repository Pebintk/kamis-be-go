// Package repository is the data-access layer (the Spring Data JPA repositories).
package repository

import (
	"context"

	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/resource/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ResourceRepository struct{ db *gorm.DB }

func NewResourceRepository(db *gorm.DB) *ResourceRepository {
	return &ResourceRepository{db: db}
}

// FindAll returns the whole catalogue, ordered by id. The Java findAll() had no
// order at all, so its row order was whatever Postgres happened to return.
func (r *ResourceRepository) FindAll(ctx context.Context) ([]model.Resource, error) {
	var out []model.Resource
	err := r.db.WithContext(ctx).Order("id").Find(&out).Error
	return out, err
}

func (r *ResourceRepository) FindByID(ctx context.Context, id int64) (*model.Resource, error) {
	var res model.Resource
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&res).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &res, nil
}

// ExistsByName stands in for findByResourceName(name) != null.
func (r *ResourceRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Resource{}).
		Where("resource_name = ?", name).Count(&count).Error
	return count > 0, err
}

// CreateWithSupplier inserts the resource and its first supplier link in one
// transaction. The Java service got that atomicity from @Transactional on the
// whole service class, which wrote the entity and its element collection
// together.
func (r *ResourceRepository) CreateWithSupplier(ctx context.Context, res *model.Resource, supplierID string) error {
	return database.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(res).Error; err != nil {
			return err
		}
		link := model.ResourceSupplier{ResourceID: res.ID, SupplierID: supplierID}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error
	}))
}

func (r *ResourceRepository) Save(ctx context.Context, res *model.Resource) error {
	return database.Translate(r.db.WithContext(ctx).Save(res).Error)
}

// FindPaginated returns one page of the catalogue plus the total, optionally
// filtered by a case-insensitive substring of the name
// (findByResourceNameContainingIgnoreCase).
func (r *ResourceRepository) FindPaginated(ctx context.Context, nameFilter string, number, size int) ([]model.Resource, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Resource{})
	if nameFilter != "" {
		q = q.Where("resource_name ILIKE ?", "%"+nameFilter+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var out []model.Resource
	if err := q.Order("id").Offset(number * size).Limit(size).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// FindByStockAtMost is findByResourceStockLessThanEqual.
func (r *ResourceRepository) FindByStockAtMost(ctx context.Context, stock int) ([]model.Resource, error) {
	var out []model.Resource
	err := r.db.WithContext(ctx).Where("resource_stock <= ?", stock).Order("id").Find(&out).Error
	return out, err
}

// FindBySupplierID returns the resources a supplier sells, joining through the
// link table the way the Java @Query joined the element collection.
func (r *ResourceRepository) FindBySupplierID(ctx context.Context, supplierID string) ([]model.Resource, error) {
	var out []model.Resource
	err := r.db.WithContext(ctx).
		Joins("JOIN resource_suppliers ON resource_suppliers.resource_id = resources.id").
		Where("resource_suppliers.supplier_id = ?", supplierID).
		Order("resources.id").
		Find(&out).Error
	return out, err
}

// LinkSupplier attaches a supplier to a resource. It is idempotent: the
// composite primary key plus ON CONFLICT DO NOTHING replaces the Java
// load-list/check-contains/append/save sequence.
func (r *ResourceRepository) LinkSupplier(ctx context.Context, resourceID int64, supplierID string) error {
	link := model.ResourceSupplier{ResourceID: resourceID, SupplierID: supplierID}
	return database.Translate(r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error)
}

// UnlinkSupplier detaches a supplier from a resource, and is a no-op if they
// were not linked.
func (r *ResourceRepository) UnlinkSupplier(ctx context.Context, resourceID int64, supplierID string) error {
	return database.Translate(r.db.WithContext(ctx).
		Where("resource_id = ? AND supplier_id = ?", resourceID, supplierID).
		Delete(&model.ResourceSupplier{}).Error)
}

// UpdateStock applies a stock change under a row lock, and is the port of the
// Java @Lock(PESSIMISTIC_WRITE) findByIdWithPessimisticLock: the read, the
// business check inside apply, and the write all happen in one transaction with
// the row held FOR UPDATE, so two concurrent deductions cannot both see the same
// starting stock.
//
// apply receives the current stock and returns the new one; the error it returns
// (insufficient stock, say) aborts the transaction and is passed through.
func (r *ResourceRepository) UpdateStock(ctx context.Context, id int64, apply func(current int) (int, error)) (*model.Resource, error) {
	var out model.Resource
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var res model.Resource
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&res).Error; err != nil {
			return database.Translate(err)
		}

		next, err := apply(res.ResourceStock)
		if err != nil {
			return err
		}

		res.ResourceStock = next
		if err := tx.Save(&res).Error; err != nil {
			return database.Translate(err)
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}
