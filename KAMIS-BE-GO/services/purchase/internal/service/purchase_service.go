// Package service holds the purchase business logic (the Spring @Service /
// restservice layer).
package service

import (
	"context"
	"log/slog"
	"sync"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/auth"
	"github.com/pebintk/kamis-be-go/pkg/blob"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/pkg/jsontime"
	"github.com/pebintk/kamis-be-go/services/purchase/internal/dto"
	"github.com/pebintk/kamis-be-go/services/purchase/internal/model"
	"github.com/pebintk/kamis-be-go/services/purchase/internal/repository"
)

// supplierFetchConcurrency bounds the parallel supplier-name lookups behind a
// list response, the same ceiling the profile service uses for its project
// lookups.
const supplierFetchConcurrency = 8

// Deps are the collaborators the purchase flows reach out to. Bundled so the
// constructor does not grow a parameter per downstream service.
type Deps struct {
	// Profile resolves supplier names and is told which purchases a supplier has.
	Profile *httpx.Client
	// Resource validates catalogue items and is credited stock on completion.
	Resource *httpx.Client
	// Asset registers a staged asset for real once its purchase completes.
	Asset *httpx.Client
	// Finance receives the spend when a payment is confirmed.
	Finance *httpx.Client
	// Photos holds the staged assets' images.
	Photos blob.Store
}

type PurchaseService struct {
	repo *repository.PurchaseRepository
	Deps
}

func NewPurchaseService(repo *repository.PurchaseRepository, deps Deps) *PurchaseService {
	return &PurchaseService{repo: repo, Deps: deps}
}

// typeName is the string the DTOs carry for a purchase's type.
func typeName(purchaseType bool) string {
	if purchaseType == model.TypeResource {
		return "Resource"
	}
	return "Aset"
}

// username is the token subject, which is what the audit log records. The
// legacy service read it from the JWT the same way.
func username(ctx context.Context) string {
	if claims, ok := auth.FromContext(ctx); ok {
		return claims.Subject
	}
	return ""
}

// ---- cross-service calls ----

// supplierName resolves one supplier's display name from the profile service.
//
// A failure yields an empty name rather than failing the caller: the purchase
// list is the main procurement screen, and a profile outage should not blank it
// out. Java rethrew here, so the whole list came back as a 400 — the same
// swallow-and-continue choice MIGRATION.md records for the profile service's own
// cross-service reads.
func (s *PurchaseService) supplierName(ctx context.Context, supplierID string) string {
	name, err := httpx.GetData[string](ctx, s.Profile, "/supplier/name/"+supplierID)
	if err != nil {
		slog.WarnContext(ctx, "could not resolve supplier name",
			"supplier", supplierID, "error", err)
		return ""
	}
	return name
}

// supplierNames resolves the distinct suppliers of a set of purchases, in
// parallel and bounded.
//
// Java resolved the name inside the per-row DTO mapper, so a list of 50
// purchases made 50 sequential HTTP calls to profile — even when they shared a
// handful of suppliers. Deduplicating first usually collapses that to a few.
func (s *PurchaseService) supplierNames(ctx context.Context, purchases []model.Purchase) map[string]string {
	unique := make(map[string]struct{}, len(purchases))
	for _, p := range purchases {
		unique[p.PurchaseSupplier] = struct{}{}
	}

	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		sem   = make(chan struct{}, supplierFetchConcurrency)
		names = make(map[string]string, len(unique))
	)
	for id := range unique {
		sem <- struct{}{} // acquire before starting, so the ceiling actually holds
		wg.Add(1)
		go func(id string) {
			defer func() { <-sem; wg.Done() }()
			name := s.supplierName(ctx, id)
			mu.Lock()
			names[id] = name
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return names
}

// attachToSupplier tells the profile service that a supplier now has this
// purchase against it.
func (s *PurchaseService) attachToSupplier(ctx context.Context, purchaseID, supplierID string) error {
	return s.Profile.Put(ctx, "/supplier/add-purchase", dto.AddPurchaseIDRequest{
		PurchaseID: purchaseID,
		SupplierID: supplierID,
	})
}

// catalogueItem looks one resource up in the resource service's catalogue.
func (s *PurchaseService) catalogueItem(ctx context.Context, resourceID int64) (*dto.ResourceResponse, error) {
	item, err := httpx.GetData[*dto.ResourceResponse](ctx, s.Resource,
		"/resource/find/"+itoa(resourceID))
	if err != nil {
		return nil, err
	}
	return item, nil
}

// ---- reads ----

// ListPurchases returns every matching purchase.
func (s *PurchaseService) ListPurchases(ctx context.Context, f repository.Filter) ([]dto.PurchaseListResponse, error) {
	purchases, err := s.repo.FindFiltered(ctx, f)
	if err != nil {
		return nil, err
	}
	return s.rows(ctx, purchases), nil
}

// ListPurchasesPaginated returns one page of matching purchases.
func (s *PurchaseService) ListPurchasesPaginated(ctx context.Context, f repository.Filter, number, size int) (dto.PageOf[dto.PurchaseListResponse], error) {
	var empty dto.PageOf[dto.PurchaseListResponse]

	purchases, total, err := s.repo.FindFilteredPaginated(ctx, f, number, size)
	if err != nil {
		return empty, err
	}
	return dto.NewPage(s.rows(ctx, purchases), number, size, total), nil
}

// rows builds the list representation, resolving supplier names once per
// distinct supplier rather than once per row.
func (s *PurchaseService) rows(ctx context.Context, purchases []model.Purchase) []dto.PurchaseListResponse {
	names := s.supplierNames(ctx, purchases)

	out := make([]dto.PurchaseListResponse, 0, len(purchases))
	for _, p := range purchases {
		row := dto.PurchaseListResponse{
			PurchaseID:             p.ID,
			PurchaseSubmissionDate: jsontime.Jakarta(p.PurchaseSubmissionDate),
			PurchaseUpdateDate:     jsontime.Jakarta(p.PurchaseUpdateDate),
			PurchaseSupplier:       names[p.PurchaseSupplier],
			PurchaseType:           typeName(p.PurchaseType),
			PurchaseStatus:         p.PurchaseStatus,
			PurchasePrice:          p.PurchasePrice,
		}
		if p.PurchasePaymentDate != nil {
			paid := jsontime.Jakarta(*p.PurchasePaymentDate)
			row.PurchasePaymentDate = &paid
		}
		out = append(out, row)
	}
	return out
}

// GetPurchase returns one purchase in full, with its line items or staged asset
// and its audit trail.
func (s *PurchaseService) GetPurchase(ctx context.Context, purchaseID string) (*dto.PurchaseResponse, error) {
	purchase, err := s.repo.FindByID(ctx, purchaseID)
	if err != nil {
		return nil, notFound(err, purchaseID)
	}
	details, err := s.details(ctx, []model.Purchase{*purchase})
	if err != nil {
		return nil, err
	}
	return &details[0], nil
}

// ListBySupplier returns every purchase made from one supplier, in full.
func (s *PurchaseService) ListBySupplier(ctx context.Context, supplierID string) ([]dto.PurchaseResponse, error) {
	if !isUUID(supplierID) {
		return nil, apierr.Invalidf("Format Supplier ID tidak valid")
	}
	purchases, err := s.repo.FindBySupplier(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	return s.details(ctx, purchases)
}

// details builds the full representation for a set of purchases, loading line
// items, logs and staged assets in one query each rather than per purchase.
func (s *PurchaseService) details(ctx context.Context, purchases []model.Purchase) ([]dto.PurchaseResponse, error) {
	ids := make([]string, 0, len(purchases))
	var assetIDs []int64
	for _, p := range purchases {
		ids = append(ids, p.ID)
		if p.PurchaseAsset != nil {
			assetIDs = append(assetIDs, *p.PurchaseAsset)
		}
	}

	lines, err := s.repo.FindLinesFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	logs, err := s.repo.FindLogsFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	assets, err := s.repo.FindAssetTempsByIDs(ctx, assetIDs)
	if err != nil {
		return nil, err
	}

	out := make([]dto.PurchaseResponse, 0, len(purchases))
	for _, p := range purchases {
		item := dto.PurchaseResponse{
			PurchaseID:             p.ID,
			PurchaseSubmissionDate: jsontime.Jakarta(p.PurchaseSubmissionDate),
			PurchaseUpdateDate:     jsontime.Jakarta(p.PurchaseUpdateDate),
			PurchaseSupplier:       p.PurchaseSupplier,
			PurchaseType:           typeName(p.PurchaseType),
			PurchaseStatus:         p.PurchaseStatus,
			PurchasePrice:          p.PurchasePrice,
			PurchaseNote:           p.PurchaseNote,
			PurchaseLogs:           logResponses(logs[p.ID]),
		}
		if p.PurchasePaymentDate != nil {
			paid := jsontime.Jakarta(*p.PurchasePaymentDate)
			item.PurchasePaymentDate = &paid
		}

		// Exactly one of these is populated; the other stays null, as in Java.
		if p.PurchaseType == model.TypeResource {
			item.PurchaseResource = lineResponses(lines[p.ID])
		} else if p.PurchaseAsset != nil {
			if asset, ok := assets[*p.PurchaseAsset]; ok {
				item.PurchaseAsset = assetTempResponse(asset)
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func lineResponses(lines []model.ResourceTemp) []dto.ResourceLineResponse {
	out := make([]dto.ResourceLineResponse, 0, len(lines))
	for _, l := range lines {
		out = append(out, dto.ResourceLineResponse{
			ResourceID:    l.ResourceID,
			ResourceName:  l.ResourceName,
			ResourceTotal: l.ResourceTotal,
			ResourcePrice: l.ResourcePrice,
		})
	}
	return out
}

func logResponses(logs []model.LogPurchase) []dto.LogResponse {
	out := make([]dto.LogResponse, 0, len(logs))
	for _, l := range logs {
		out = append(out, dto.LogResponse{
			ID:         l.ID,
			User:       l.Username,
			Action:     l.Action,
			ActionDate: jsontime.Jakarta(l.ActionDate),
		})
	}
	return out
}

func assetTempResponse(a model.AssetTemp) *dto.AssetTempResponse {
	out := &dto.AssetTempResponse{
		ID:               a.ID,
		AssetNameString:  a.AssetName,
		AssetDescription: a.AssetDescription,
		AssetType:        a.AssetType,
		AssetPrice:       a.AssetPrice,
	}
	if a.FotoKey != "" {
		contentType := a.FotoContentType
		url := "/api/purchase/asset/" + itoa(a.ID) + "/foto"
		out.FotoContentType = &contentType
		out.FotoURL = &url
	}
	return out
}

// ---- writes ----

// AddPurchase records a procurement request and attaches it to its supplier.
func (s *PurchaseService) AddPurchase(ctx context.Context, req dto.AddPurchaseRequest) (*dto.PurchaseResponse, error) {
	if !isUUID(req.PurchaseSupplier) {
		return nil, apierr.Invalidf("Format Supplier ID tidak valid")
	}

	purchase := model.Purchase{
		PurchaseSupplier: req.PurchaseSupplier,
		PurchaseType:     req.PurchaseType,
		PurchaseStatus:   model.StatusDiajukan,
		PurchaseNote:     req.PurchaseNote,
	}

	var lines []model.ResourceTemp
	if req.PurchaseType == model.TypeResource {
		if req.PurchaseAsset != nil {
			return nil, apierr.Invalidf("Anda memilih tipe pembelian resource, pastikan tidak menginput data aset.")
		}
		var err error
		lines, purchase.PurchasePrice, err = s.buildLines(ctx, req.PurchaseResource)
		if err != nil {
			return nil, err
		}
	} else {
		if req.PurchaseAsset == nil {
			return nil, apierr.Invalidf("Anda memilih tipe pembelian aset, pastikan menginput data aset.")
		}
		if len(req.PurchaseResource) > 0 {
			return nil, apierr.Invalidf("Anda memilih tipe pembelian aset, pastikan tidak menginput data resource.")
		}
		asset, err := s.repo.FindAssetTempByID(ctx, *req.PurchaseAsset)
		if err != nil {
			return nil, assetNotFound(err)
		}
		purchase.PurchaseAsset = &asset.ID
		purchase.PurchasePrice = asset.AssetPrice
	}

	err := s.repo.CreateWithGeneratedID(ctx, &purchase, lines, username(ctx),
		func(id string) string { return "Menambahkan Pembelian " + id })
	if err != nil {
		return nil, err
	}

	// The supplier link is a courtesy index kept by the profile service; the
	// purchase itself is already committed, so a failure is logged rather than
	// unwound. Java let the exception escape and answered 400 for a purchase it
	// had in fact created.
	if err := s.attachToSupplier(ctx, purchase.ID, purchase.PurchaseSupplier); err != nil {
		slog.WarnContext(ctx, "could not attach purchase to its supplier",
			"purchase", purchase.ID, "supplier", purchase.PurchaseSupplier, "error", err)
	}

	return s.GetPurchase(ctx, purchase.ID)
}

// buildLines validates the requested line items against the resource catalogue
// and returns them with the total price.
func (s *PurchaseService) buildLines(ctx context.Context, requested []dto.ResourceLineRequest) ([]model.ResourceTemp, int, error) {
	if len(requested) == 0 {
		return nil, 0, apierr.Invalidf("Anda memilih tipe pembelian resource, pastikan menginput data resource setidaknya satu.")
	}

	seen := make(map[int64]struct{}, len(requested))
	lines := make([]model.ResourceTemp, 0, len(requested))
	total := 0

	for _, line := range requested {
		if _, duplicate := seen[*line.ResourceID]; duplicate {
			return nil, 0, apierr.Invalidf("Tidak boleh terdapat lebih dari satu resource yang sama!")
		}
		seen[*line.ResourceID] = struct{}{}

		item, err := s.catalogueItem(ctx, *line.ResourceID)
		if err != nil || item == nil {
			return nil, 0, apierr.Invalidf("Resource Tidak Terdaftar pada Sistem.")
		}
		// The submitted name must match the catalogue, so a stale client cannot
		// record a purchase against the wrong item.
		if item.ResourceName != line.ResourceName {
			return nil, 0, apierr.Invalidf("Nama Resource Tidak Sesuai dengan Id pada Sistem.")
		}
		if *line.ResourceTotal <= 0 {
			return nil, 0, apierr.Invalidf("Jumlah barang harus lebih dari 0")
		}
		if *line.ResourcePrice < 0 {
			return nil, 0, apierr.Invalidf("Harga barang tidak boleh kurang dari 0")
		}

		lines = append(lines, model.ResourceTemp{
			ResourceID:    *line.ResourceID,
			ResourceName:  line.ResourceName,
			ResourceTotal: *line.ResourceTotal,
			ResourcePrice: *line.ResourcePrice,
		})
		total += *line.ResourcePrice * *line.ResourceTotal
	}
	return lines, total, nil
}

// UpdatePurchase edits a purchase that has not yet been decided.
func (s *PurchaseService) UpdatePurchase(ctx context.Context, purchaseID string, req dto.UpdatePurchaseRequest) (*dto.PurchaseResponse, error) {
	if !isUUID(req.PurchaseSupplier) {
		return nil, apierr.Invalidf("Format Supplier ID tidak valid")
	}

	purchase, err := s.repo.FindByID(ctx, purchaseID)
	if err != nil {
		return nil, notFound(err, purchaseID)
	}
	if model.Terminal(purchase.PurchaseStatus) {
		return nil, apierr.Invalidf("Status Pembelian sudah tidak bisa diperbarui.")
	}

	purchase.PurchaseSupplier = req.PurchaseSupplier
	purchase.PurchaseNote = req.PurchaseNote

	// Line items are only meaningful for a resource purchase; an asset
	// purchase's price is fixed by the staged asset.
	if purchase.PurchaseType == model.TypeResource {
		lines, total, buildErr := s.buildLines(ctx, req.PurchaseResource)
		if buildErr != nil {
			return nil, buildErr
		}
		purchase.PurchasePrice = total
		if err := s.repo.ReplaceLines(ctx, purchase.ID, lines); err != nil {
			return nil, err
		}
	} else if len(req.PurchaseResource) > 0 {
		return nil, apierr.Invalidf("Anda memilih tipe pembelian aset, pastikan tidak menginput data resource.")
	}

	if err := s.repo.Save(ctx, purchase); err != nil {
		return nil, err
	}
	if err := s.repo.AppendLog(ctx, &model.LogPurchase{
		PurchaseID: purchase.ID,
		Username:   username(ctx),
		Action:     "Memperbarui Pembelian " + purchase.ID,
	}); err != nil {
		return nil, err
	}

	return s.GetPurchase(ctx, purchase.ID)
}
