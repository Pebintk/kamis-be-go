package repository

import (
	"context"

	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/profile/internal/model"
	"gorm.io/gorm"
)

type ClientRepository struct{ db *gorm.DB }

func NewClientRepository(db *gorm.DB) *ClientRepository { return &ClientRepository{db: db} }

func (r *ClientRepository) FindByID(ctx context.Context, id string) (*model.Client, error) {
	var c model.Client
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &c, nil
}

func (r *ClientRepository) Create(ctx context.Context, c *model.Client) error {
	return database.Translate(r.db.WithContext(ctx).Create(c).Error)
}

func (r *ClientRepository) Save(ctx context.Context, c *model.Client) error {
	return database.Translate(r.db.WithContext(ctx).Save(c).Error)
}

// filter applies the optional name (case-insensitive contains) and type
// predicates, standing in for the legacy findByNameClientContainingIgnoreCase /
// findByTypeClient repository method combinations.
func (r *ClientRepository) filter(ctx context.Context, nameClient string, typeClient *bool) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&model.Client{})
	if nameClient != "" {
		q = q.Where("name_client ILIKE ?", "%"+nameClient+"%")
	}
	if typeClient != nil {
		q = q.Where("type_client = ?", *typeClient)
	}
	return q
}

// FindFiltered returns every matching client.
func (r *ClientRepository) FindFiltered(ctx context.Context, nameClient string, typeClient *bool) ([]model.Client, error) {
	var clients []model.Client
	err := r.filter(ctx, nameClient, typeClient).Order("name_client").Find(&clients).Error
	return clients, err
}

// FindFilteredPaginated returns one page of matching clients plus the total.
func (r *ClientRepository) FindFilteredPaginated(ctx context.Context, nameClient string, typeClient *bool, page, size int) ([]model.Client, int64, error) {
	q := r.filter(ctx, nameClient, typeClient)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var clients []model.Client
	if err := q.Order("name_client").Offset(page * size).Limit(size).Find(&clients).Error; err != nil {
		return nil, 0, err
	}
	return clients, total, nil
}
