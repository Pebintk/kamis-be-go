// Package service holds the resource business logic (the Spring @Service /
// restservice layer).
package service

import (
	"context"

	"github.com/karina/kamis-be-go/services/resource/internal/dto"
	"github.com/karina/kamis-be-go/services/resource/internal/model"
	"github.com/karina/kamis-be-go/services/resource/internal/repository"
)

type ResourceService struct {
	repo *repository.ResourceRepository
}

func NewResourceService(repo *repository.ResourceRepository) *ResourceService {
	return &ResourceService{repo: repo}
}

func toResponse(r model.Resource) dto.ResourceResponse {
	return dto.ResourceResponse{
		ID:                  r.ID,
		ResourceName:        r.ResourceName,
		ResourceDescription: r.ResourceDescription,
		ResourceStock:       r.ResourceStock,
		ResourcePrice:       r.ResourcePrice,
	}
}

func toResponses(resources []model.Resource) []dto.ResourceResponse {
	out := make([]dto.ResourceResponse, 0, len(resources))
	for _, r := range resources {
		out = append(out, toResponse(r))
	}
	return out
}

// AddResource creates a catalogue entry, attached to the supplier that sells it.
func (s *ResourceService) AddResource(ctx context.Context, req dto.AddResourceRequest) (*dto.ResourceResponse, error) {
	if *req.ResourcePrice < 0 {
		return nil, invalidf("Harga barang tidak boleh kurang dari 0")
	}
	if *req.ResourceStock < 0 {
		return nil, invalidf("Stok barang tidak boleh kurang dari 0")
	}
	if !isUUID(req.ResourceSupplierID) {
		return nil, invalidf("Supplier ID tidak valid: %s", req.ResourceSupplierID)
	}

	exists, err := s.repo.ExistsByName(ctx, req.ResourceName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, invalidf("Nama barang sudah ada di database")
	}

	resource := model.Resource{
		ResourceName:        req.ResourceName,
		ResourceDescription: req.ResourceDescription,
		ResourceStock:       *req.ResourceStock,
		ResourcePrice:       *req.ResourcePrice,
	}
	if err := s.repo.CreateWithSupplier(ctx, &resource, req.ResourceSupplierID); err != nil {
		return nil, err
	}

	response := toResponse(resource)
	return &response, nil
}

// ListResources returns the whole catalogue.
func (s *ResourceService) ListResources(ctx context.Context) ([]dto.ResourceResponse, error) {
	resources, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return toResponses(resources), nil
}

// ListResourcesPaginated returns one page, optionally filtered by name.
func (s *ResourceService) ListResourcesPaginated(ctx context.Context, nameFilter string, number, size int) (dto.PageOf[dto.ResourceResponse], error) {
	resources, total, err := s.repo.FindPaginated(ctx, nameFilter, number, size)
	if err != nil {
		return dto.PageOf[dto.ResourceResponse]{}, err
	}
	return dto.NewPage(toResponses(resources), number, size, total), nil
}

// GetResource returns one resource by id.
func (s *ResourceService) GetResource(ctx context.Context, id int64) (*dto.ResourceResponse, error) {
	resource, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, notFound(err, id)
	}
	response := toResponse(*resource)
	return &response, nil
}

// UpdateResource edits the description, price and stock of a resource. The name
// is not editable — there is no name field in the legacy update DTO either.
func (s *ResourceService) UpdateResource(ctx context.Context, id int64, req dto.UpdateResourceRequest) (*dto.ResourceResponse, error) {
	if *req.ResourcePrice < 0 {
		return nil, invalidf("Harga barang tidak boleh kurang dari 0")
	}
	// The legacy DTO carries @Min(0) on the stock too, but that controller never
	// inspected the BindingResult, so a negative stock went straight into the
	// database. Enforced here.
	if *req.ResourceStock < 0 {
		return nil, invalidf("Stok barang tidak boleh kurang dari 0")
	}

	resource, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, notFound(err, id)
	}

	resource.ResourceDescription = req.ResourceDescription
	resource.ResourcePrice = *req.ResourcePrice
	resource.ResourceStock = *req.ResourceStock
	if err := s.repo.Save(ctx, resource); err != nil {
		return nil, err
	}

	response := toResponse(*resource)
	return &response, nil
}

// AddStock increases a resource's stock by quantity, under a row lock.
func (s *ResourceService) AddStock(ctx context.Context, id int64, quantity int) (*dto.ResourceResponse, error) {
	if quantity <= 0 {
		return nil, invalidf("Jumlah penambahan stok harus positif")
	}
	return s.adjustStock(ctx, id, addition(quantity))
}

// DeductStock decreases a resource's stock by quantity, refusing to go negative.
func (s *ResourceService) DeductStock(ctx context.Context, id int64, quantity int) (*dto.ResourceResponse, error) {
	if quantity <= 0 {
		return nil, invalidf("Jumlah pengurangan stok harus positif")
	}
	return s.adjustStock(ctx, id, deduction(quantity))
}

// AddStockToDB is the /addToDb/{id}/{stock} endpoint the purchase service calls
// when a purchase is confirmed. Unlike AddStock it takes the amount from the
// path and does not require it to be positive, matching the legacy contract.
func (s *ResourceService) AddStockToDB(ctx context.Context, id int64, stock int) (*dto.ResourceResponse, error) {
	return s.adjustStock(ctx, id, addition(stock))
}

// addition and deduction are the stock-change functions handed to the
// repository, which calls them with the locked row's current stock and writes
// back what they return. They are the whole of the stock arithmetic, kept out of
// the transaction so they can be tested on their own.
func addition(delta int) func(current int) (int, error) {
	return func(current int) (int, error) { return current + delta, nil }
}

func deduction(quantity int) func(current int) (int, error) {
	return func(current int) (int, error) {
		next := current - quantity
		if next < 0 {
			return 0, invalidf("Stock tidak mencukupi. Tersedia: %d, permintaan: %d", current, quantity)
		}
		return next, nil
	}
}

// adjustStock applies a stock change under the repository's row lock.
func (s *ResourceService) adjustStock(ctx context.Context, id int64, apply func(int) (int, error)) (*dto.ResourceResponse, error) {
	resource, err := s.repo.UpdateStock(ctx, id, apply)
	if err != nil {
		return nil, notFound(err, id)
	}
	response := toResponse(*resource)
	return &response, nil
}

// ListBySupplier returns the resources a supplier sells.
func (s *ResourceService) ListBySupplier(ctx context.Context, supplierID string) ([]dto.ResourceResponse, error) {
	if !isUUID(supplierID) {
		return nil, invalidf("Supplier ID tidak valid: %s", supplierID)
	}
	resources, err := s.repo.FindBySupplierID(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	return toResponses(resources), nil
}

// ListByStockAtMost returns every resource at or below a stock level. The
// purchase and dashboard views use it as a low-stock report.
func (s *ResourceService) ListByStockAtMost(ctx context.Context, stock int) ([]dto.ResourceResponse, error) {
	if stock < 0 {
		return nil, invalidf("Stock tidak boleh kurang dari 0")
	}
	resources, err := s.repo.FindByStockAtMost(ctx, stock)
	if err != nil {
		return nil, err
	}
	return toResponses(resources), nil
}

// AddSupplier attaches a supplier to each of the given resources, leaving any
// other links alone. The profile service calls this when a supplier is created.
func (s *ResourceService) AddSupplier(ctx context.Context, supplierID string, resourceIDs []int64) error {
	if !isUUID(supplierID) {
		return invalidf("Supplier ID tidak valid: %s", supplierID)
	}
	for _, id := range resourceIDs {
		if err := s.linkExisting(ctx, id, supplierID); err != nil {
			return err
		}
	}
	return nil
}

// UpdateSupplier makes resourceIDs the complete set of resources a supplier
// sells: links that are missing are added, and links to resources no longer in
// the list are removed. The profile service calls this when a supplier is
// edited, including with an empty list to detach everything.
func (s *ResourceService) UpdateSupplier(ctx context.Context, supplierID string, resourceIDs []int64) error {
	if !isUUID(supplierID) {
		return invalidf("Supplier ID tidak valid: %s", supplierID)
	}

	current, err := s.repo.FindBySupplierID(ctx, supplierID)
	if err != nil {
		return err
	}
	linked := make(map[int64]bool, len(current))
	for _, r := range current {
		linked[r.ID] = true
	}

	wanted := make(map[int64]bool, len(resourceIDs))
	for _, id := range resourceIDs {
		wanted[id] = true
	}

	for id := range linked {
		if !wanted[id] {
			if err := s.repo.UnlinkSupplier(ctx, id, supplierID); err != nil {
				return err
			}
		}
	}
	for _, id := range resourceIDs {
		if linked[id] {
			continue
		}
		if err := s.linkExisting(ctx, id, supplierID); err != nil {
			return err
		}
	}
	return nil
}

// linkExisting attaches a supplier to a resource, rejecting an unknown id.
func (s *ResourceService) linkExisting(ctx context.Context, resourceID int64, supplierID string) error {
	if _, err := s.repo.FindByID(ctx, resourceID); err != nil {
		return notFound(err, resourceID)
	}
	return s.repo.LinkSupplier(ctx, resourceID, supplierID)
}
