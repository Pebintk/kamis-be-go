// Package service holds business logic (the Spring @Service / restservice layer).
package service

import (
	"context"
	"errors"

	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/template/internal/model"
	"github.com/pebintk/kamis-be-go/services/template/internal/repository"
)

// ErrNotFound is a transport-agnostic error the handler maps to HTTP 404.
var ErrNotFound = errors.New("resource not found")

type ResourceService struct {
	repo *repository.ResourceRepository
}

func NewResourceService(repo *repository.ResourceRepository) *ResourceService {
	return &ResourceService{repo: repo}
}

func (s *ResourceService) List(ctx context.Context) ([]model.Resource, error) {
	return s.repo.FindAll(ctx)
}

func (s *ResourceService) Get(ctx context.Context, id uint) (*model.Resource, error) {
	res, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return nil, ErrNotFound
	}
	return res, err
}

func (s *ResourceService) Create(ctx context.Context, name string, qty int) (*model.Resource, error) {
	res := &model.Resource{Name: name, Quantity: qty}
	if err := s.repo.Create(ctx, res); err != nil {
		return nil, err
	}
	return res, nil
}
