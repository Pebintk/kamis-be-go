package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/profile/internal/dto"
	"github.com/karina/kamis-be-go/services/profile/internal/model"
	"github.com/karina/kamis-be-go/services/profile/internal/repository"
	"gorm.io/gorm"
)

// ErrSupplierNotFound is returned for an unknown supplier id.
var ErrSupplierNotFound = errors.New("supplier tidak ditemukan")

// ErrMissingToken mirrors the legacy check: the supplier flows call three other
// services and refuse to run without a token to forward.
var ErrMissingToken = errors.New("Token tidak ditemukan di header Authorization.")

var digitsOnly = regexp.MustCompile(`^\d+$`)

type SupplierService struct {
	repo     *repository.SupplierRepository
	resource *httpx.Client
	asset    *httpx.Client
	purchase *httpx.Client
}

func NewSupplierService(repo *repository.SupplierRepository, resource, asset, purchase *httpx.Client) *SupplierService {
	return &SupplierService{repo: repo, resource: resource, asset: asset, purchase: purchase}
}

func toSupplierResponse(s *model.Supplier) dto.SupplierResponse {
	return dto.SupplierResponse{
		ID:              s.ID,
		NameSupplier:    s.NameSupplier,
		NoTelpSupplier:  s.NoTelpSupplier,
		EmailSupplier:   s.EmailSupplier,
		CompanySupplier: s.CompanySupplier,
		AddressSupplier: s.AddressSupplier,
		ResourceIDs:     orEmpty(s.ResourceIDs),
		AssetIDs:        orEmpty(s.AssetIDs),
		PurchaseIDs:     orEmpty(s.PurchaseIDs),
		CreatedDate:     dto.UTCTime(s.CreatedDate),
		UpdatedDate:     dto.UTCTime(s.UpdatedDate),
	}
}

func toSupplierListResponse(s *model.Supplier) dto.SupplierListResponse {
	return dto.SupplierListResponse{
		ID:              s.ID,
		NameSupplier:    s.NameSupplier,
		CompanySupplier: s.CompanySupplier,
		TotalPurchases:  len(s.PurchaseIDs),
	}
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// ---- cross-service calls ----

// fetchAllResources lists the resource catalogue, used to validate resourceIds.
func (s *SupplierService) fetchAllResources(ctx context.Context) []dto.ResourceResponse {
	return fetchList[dto.ResourceResponse](ctx, s.resource, "/resource/viewall", "resource")
}

func (s *SupplierService) fetchAssets(ctx context.Context, supplierID string) []dto.Asset {
	return fetchList[dto.Asset](ctx, s.asset, "/asset/by-supplier/"+supplierID, "asset")
}

func (s *SupplierService) fetchPurchases(ctx context.Context, supplierID string) []dto.PurchaseResponse {
	return fetchList[dto.PurchaseResponse](ctx, s.purchase, "/purchase/supplier/"+supplierID, "purchase")
}

func (s *SupplierService) fetchResources(ctx context.Context, supplierID string) []dto.ResourceResponse {
	return fetchList[dto.ResourceResponse](ctx, s.resource, "/resource/find-by-supplier/"+supplierID, "resource")
}

// fetchList performs a downstream GET, logging and swallowing failures the way
// the Java service did — an unreachable dependency yields an empty list rather
// than a failed request.
func fetchList[T any](ctx context.Context, client *httpx.Client, path, what string) []T {
	items, err := httpx.GetData[[]T](ctx, client, path)
	if err != nil {
		if !errors.Is(err, httpx.ErrNotFound) {
			log.Printf("fetching %s from %s: %v", what, path, err)
		}
		return []T{}
	}
	if items == nil {
		return []T{}
	}
	return items
}

// linkResources tells the resource service which resources belong to this
// supplier. add uses /resource/add-supplier, update uses /resource/update-supplier.
func (s *SupplierService) linkResources(ctx context.Context, path, supplierID string, resourceIDs []int64) error {
	return s.resource.Put(ctx, path, dto.AddSupplierIDRequest{
		SupplierID: supplierID,
		ResourceID: orEmpty(resourceIDs),
	})
}

// validateResourceIDs rejects ids that are not in the resource catalogue.
func (s *SupplierService) validateResourceIDs(ctx context.Context, resourceIDs []int64) error {
	if len(resourceIDs) == 0 {
		return nil
	}
	valid := make(map[int64]bool)
	for _, r := range s.fetchAllResources(ctx) {
		valid[r.ID] = true
	}
	for _, id := range resourceIDs {
		if !valid[id] {
			return fmt.Errorf("Resource ID tidak valid: %d", id)
		}
	}
	return nil
}

// ---- commands ----

func (s *SupplierService) AddSupplier(ctx context.Context, req dto.AddSupplierRequest) (dto.SupplierResponse, error) {
	if httpx.TokenFromContext(ctx) == "" {
		return dto.SupplierResponse{}, ErrMissingToken
	}
	if !digitsOnly.MatchString(req.NoTelpSupplier) {
		return dto.SupplierResponse{}, errors.New("Nomor telepon hanya boleh terdiri dari angka.")
	}

	for _, check := range []struct {
		exists func(context.Context, string) (bool, error)
		value  string
		msg    string
	}{
		{s.repo.ExistsByName, req.NameSupplier, "Nama supplier sudah digunakan."},
		{s.repo.ExistsByNoTelp, req.NoTelpSupplier, "Nomor telepon sudah digunakan oleh supplier lain."},
		{s.repo.ExistsByEmail, req.EmailSupplier, "Email sudah digunakan oleh supplier lain."},
		{s.repo.ExistsByCompany, req.CompanySupplier, "Nama perusahaan sudah digunakan oleh supplier lain."},
	} {
		taken, err := check.exists(ctx, check.value)
		if err != nil {
			return dto.SupplierResponse{}, err
		}
		if taken {
			return dto.SupplierResponse{}, errors.New(check.msg)
		}
	}

	resourceIDs := orEmpty(req.ResourceIDs)
	if err := s.validateResourceIDs(ctx, resourceIDs); err != nil {
		return dto.SupplierResponse{}, err
	}

	supplier := &model.Supplier{
		NameSupplier:    req.NameSupplier,
		NoTelpSupplier:  req.NoTelpSupplier,
		EmailSupplier:   req.EmailSupplier,
		CompanySupplier: req.CompanySupplier,
		AddressSupplier: req.AddressSupplier,
		ResourceIDs:     resourceIDs,
		AssetIDs:        []string{},
		PurchaseIDs:     []string{},
	}
	if err := s.repo.Create(ctx, supplier); err != nil {
		return dto.SupplierResponse{}, err
	}

	if len(resourceIDs) > 0 {
		if err := s.linkResources(ctx, "/resource/add-supplier", supplier.ID, resourceIDs); err != nil {
			return dto.SupplierResponse{}, err
		}
	}
	return toSupplierResponse(supplier), nil
}

// UpdateSupplier updates everything except the company name, which the legacy
// service deliberately leaves alone. The supplier id comes from the body.
func (s *SupplierService) UpdateSupplier(ctx context.Context, req dto.UpdateSupplierRequest) (dto.SupplierResponse, error) {
	if httpx.TokenFromContext(ctx) == "" {
		return dto.SupplierResponse{}, ErrMissingToken
	}

	supplier, err := s.repo.FindByID(ctx, req.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.SupplierResponse{}, ErrSupplierNotFound
	}
	if err != nil {
		return dto.SupplierResponse{}, err
	}

	if !digitsOnly.MatchString(req.NoTelpSupplier) {
		return dto.SupplierResponse{}, errors.New("Nomor telepon hanya boleh terdiri dari angka.")
	}

	for _, check := range []struct {
		exists func(context.Context, string, string) (bool, error)
		value  string
		msg    string
	}{
		{s.repo.ExistsByNoTelpExcluding, req.NoTelpSupplier, "Nomor telepon sudah digunakan."},
		{s.repo.ExistsByEmailExcluding, req.EmailSupplier, "Email sudah digunakan."},
		{s.repo.ExistsByNameExcluding, req.NameSupplier, "Nama supplier sudah digunakan."},
	} {
		taken, err := check.exists(ctx, check.value, req.ID)
		if err != nil {
			return dto.SupplierResponse{}, err
		}
		if taken {
			return dto.SupplierResponse{}, errors.New(check.msg)
		}
	}

	resourceIDs := orEmpty(req.ResourceIDs)
	if err := s.validateResourceIDs(ctx, resourceIDs); err != nil {
		return dto.SupplierResponse{}, err
	}

	supplier.AddressSupplier = req.AddressSupplier
	supplier.NoTelpSupplier = req.NoTelpSupplier
	supplier.EmailSupplier = req.EmailSupplier
	supplier.NameSupplier = req.NameSupplier
	supplier.ResourceIDs = resourceIDs

	if err := s.repo.Save(ctx, supplier); err != nil {
		return dto.SupplierResponse{}, err
	}

	// Re-sync the resource service even when the list is empty: that is how a
	// resource gets detached from this supplier.
	if err := s.linkResources(ctx, "/resource/update-supplier", supplier.ID, resourceIDs); err != nil {
		return dto.SupplierResponse{}, err
	}
	return toSupplierResponse(supplier), nil
}

// AddPurchaseID links a purchase to a supplier; called by the purchase service.
func (s *SupplierService) AddPurchaseID(ctx context.Context, supplierID, purchaseID string) error {
	if _, err := s.repo.FindByID(ctx, supplierID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSupplierNotFound
		}
		return err
	}
	return s.repo.AddPurchaseID(ctx, supplierID, purchaseID)
}

// ---- queries ----

func (s *SupplierService) FilterSuppliers(ctx context.Context, nameSupplier, companySupplier string) ([]dto.SupplierListResponse, error) {
	suppliers, err := s.repo.FindFiltered(ctx, nameSupplier, companySupplier)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SupplierListResponse, 0, len(suppliers))
	for i := range suppliers {
		out = append(out, toSupplierListResponse(&suppliers[i]))
	}
	return out, nil
}

func (s *SupplierService) GetAllSuppliers(ctx context.Context) ([]dto.SupplierResponse, error) {
	suppliers, err := s.repo.FindFiltered(ctx, "", "")
	if err != nil {
		return nil, err
	}
	out := make([]dto.SupplierResponse, 0, len(suppliers))
	for i := range suppliers {
		out = append(out, toSupplierResponse(&suppliers[i]))
	}
	return out, nil
}

func (s *SupplierService) GetAllSupplierPaginated(ctx context.Context, nameSupplier, companySupplier string, page, size int) (dto.PageOf[dto.SupplierListResponse], error) {
	suppliers, total, err := s.repo.FindFilteredPaginated(ctx, nameSupplier, companySupplier, page, size)
	if err != nil {
		return dto.PageOf[dto.SupplierListResponse]{}, err
	}
	content := make([]dto.SupplierListResponse, 0, len(suppliers))
	for i := range suppliers {
		content = append(content, toSupplierListResponse(&suppliers[i]))
	}
	return dto.NewPage(content, page, size, total), nil
}

// GetSupplierName returns the supplier's *company* name — the legacy method is
// named getSupplierName but returns companySupplier, and other services depend
// on that, so it is kept as-is.
func (s *SupplierService) GetSupplierName(ctx context.Context, supplierID string) (string, error) {
	supplier, err := s.repo.FindByID(ctx, supplierID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrSupplierNotFound
	}
	if err != nil {
		return "", err
	}
	return supplier.CompanySupplier, nil
}

// GetSupplierDetail aggregates the supplier with its assets, purchases and
// resources from the three owning services.
func (s *SupplierService) GetSupplierDetail(ctx context.Context, supplierID string) (dto.DetailSupplier, error) {
	if httpx.TokenFromContext(ctx) == "" {
		return dto.DetailSupplier{}, ErrMissingToken
	}

	supplier, err := s.repo.FindByID(ctx, supplierID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.DetailSupplier{}, ErrSupplierNotFound
	}
	if err != nil {
		return dto.DetailSupplier{}, err
	}

	purchases := s.fetchPurchases(ctx, supplierID)
	for i := range purchases {
		purchases[i].ActivityName = activityName(purchases[i])
	}

	return dto.DetailSupplier{
		SupplierName:    supplier.NameSupplier,
		SupplierPhone:   supplier.NoTelpSupplier,
		SupplierEmail:   supplier.EmailSupplier,
		SupplierCompany: supplier.CompanySupplier,
		SupplierAddress: supplier.AddressSupplier,
		Assets:          s.fetchAssets(ctx, supplierID),
		Purchases:       purchases,
		Resources:       s.fetchResources(ctx, supplierID),
	}, nil
}

// activityName builds the human-readable label the detail view shows for a
// purchase, e.g. "Pembelian Aset seharga Rp1.500.000".
func activityName(p dto.PurchaseResponse) string {
	price := 0
	if p.PurchasePrice != nil {
		price = *p.PurchasePrice
	}
	return "Pembelian " + p.PurchaseType + " seharga Rp" + formatRupiah(price)
}

// formatRupiah groups digits in threes with "." separators, matching
// NumberFormat.getNumberInstance(new Locale("id","ID")) in the Java service.
func formatRupiah(amount int) string {
	digits := strconv.Itoa(amount)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}

	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return sign + b.String()
}
