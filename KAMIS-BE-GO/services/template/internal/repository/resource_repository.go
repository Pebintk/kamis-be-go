// Package repository is the data-access layer (Spring Data JPA repositories).
package repository

import (
	"context"

	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/template/internal/model"
	"gorm.io/gorm"
)

type ResourceRepository struct{ db *gorm.DB }

func NewResourceRepository(db *gorm.DB) *ResourceRepository {
	return &ResourceRepository{db: db}
}

func (r *ResourceRepository) FindAll(ctx context.Context) ([]model.Resource, error) {
	var out []model.Resource
	err := r.db.WithContext(ctx).Order("id").Find(&out).Error
	return out, err
}

func (r *ResourceRepository) FindByID(ctx context.Context, id uint) (*model.Resource, error) {
	var res model.Resource
	if err := r.db.WithContext(ctx).First(&res, id).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &res, nil
}

func (r *ResourceRepository) Create(ctx context.Context, res *model.Resource) error {
	return database.Translate(r.db.WithContext(ctx).Create(res).Error)
}
