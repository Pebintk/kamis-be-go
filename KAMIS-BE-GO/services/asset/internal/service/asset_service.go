// Package service holds the asset business logic (the Spring @Service layer).
package service

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/blob"
	"github.com/pebintk/kamis-be-go/pkg/jsontime"
	"github.com/pebintk/kamis-be-go/services/asset/internal/dto"
	"github.com/pebintk/kamis-be-go/services/asset/internal/model"
	"github.com/pebintk/kamis-be-go/services/asset/internal/repository"
)

// acquisitionDateLayout is the format the legacy SimpleDateFormat parsed.
const acquisitionDateLayout = "2006-01-02"

type AssetService struct {
	repo   *repository.AssetRepository
	photos blob.Store
}

func NewAssetService(repo *repository.AssetRepository, photos blob.Store) *AssetService {
	return &AssetService{repo: repo, photos: photos}
}

// Photo is an asset's image on its way to the HTTP response.
type Photo = blob.Object

func (s *AssetService) detail(a model.Asset) dto.AssetResponse {
	out := dto.AssetResponse{
		PlatNomor:        a.PlatNomor,
		Nama:             a.Nama,
		JenisAset:        a.JenisAset,
		Status:           a.Status,
		TanggalPerolehan: jsontime.UTC(a.TanggalPerolehan),
		NilaiPerolehan:   a.NilaiPerolehan,
		Deskripsi:        a.Deskripsi,
		SupplierID:       a.IDSupplier,
	}
	if a.FotoKey != "" {
		url := "/api/asset/" + a.PlatNomor + "/foto"
		out.FotoURL = &url
		contentType := a.FotoContentType
		out.FotoContentType = &contentType
	}
	return out
}

// rows builds the list representation for a set of assets, resolving every
// asset's last completed maintenance in one query rather than one per row.
func (s *AssetService) rows(ctx context.Context, assets []model.Asset) ([]dto.AssetListResponse, error) {
	platNomors := make([]string, 0, len(assets))
	for _, a := range assets {
		platNomors = append(platNomors, a.PlatNomor)
	}
	lastMaintenance, err := s.repo.LastMaintenanceDates(ctx, platNomors)
	if err != nil {
		return nil, err
	}

	out := make([]dto.AssetListResponse, 0, len(assets))
	for _, a := range assets {
		row := dto.AssetListResponse{
			PlatNomor:        a.PlatNomor,
			TipeAset:         a.JenisAset,
			Nama:             a.Nama,
			Status:           a.Status,
			NilaiPerolehan:   a.NilaiPerolehan,
			TanggalPerolehan: jsontime.UTC(a.TanggalPerolehan),
			SupplierID:       a.IDSupplier,
		}
		if last, ok := lastMaintenance[a.PlatNomor]; ok {
			at := jsontime.UTC(last)
			row.LastMaintenance = &at
		}
		out = append(out, row)
	}
	return out, nil
}

// ListAssets returns every asset that has not been deleted.
func (s *AssetService) ListAssets(ctx context.Context) ([]dto.AssetListResponse, error) {
	assets, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return s.rows(ctx, assets)
}

// ListAssetsPaginated returns one page, with the optional name, type and status
// filters.
func (s *AssetService) ListAssetsPaginated(ctx context.Context, nama, jenisAset, status string, number, size int) (dto.PageOf[dto.AssetListResponse], error) {
	var empty dto.PageOf[dto.AssetListResponse]

	assets, total, err := s.repo.FindPaginated(ctx, nama, jenisAset, status, number, size)
	if err != nil {
		return empty, err
	}
	rows, err := s.rows(ctx, assets)
	if err != nil {
		return empty, err
	}
	return dto.NewPage(rows, number, size, total), nil
}

// GetAsset returns one asset by plate number.
func (s *AssetService) GetAsset(ctx context.Context, platNomor string) (*dto.AssetResponse, error) {
	asset, err := s.repo.FindByPlatNomor(ctx, platNomor)
	if err != nil {
		return nil, notFound(err, platNomor)
	}
	out := s.detail(*asset)
	return &out, nil
}

// ListBySupplier returns the assets bought from one supplier.
func (s *AssetService) ListBySupplier(ctx context.Context, supplierID string) ([]dto.AssetResponse, error) {
	if !isUUID(supplierID) {
		return nil, apierr.Invalidf("Format Supplier ID tidak valid")
	}
	assets, err := s.repo.FindBySupplier(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.AssetResponse, 0, len(assets))
	for _, a := range assets {
		out = append(out, s.detail(a))
	}
	return out, nil
}

// AddAsset registers an asset, optionally with a photo. photo may be nil.
//
// This is the endpoint the purchase service calls when a purchase of an asset is
// confirmed; the frontend does not call it directly.
func (s *AssetService) AddAsset(ctx context.Context, req dto.AddAssetRequest, photo *UploadedPhoto) (*dto.AssetResponse, error) {
	if !isUUID(req.SupplierID) {
		return nil, apierr.Invalidf("Format Supplier ID tidak valid")
	}
	acquired, err := time.Parse(acquisitionDateLayout, req.TanggalPerolehan)
	if err != nil {
		return nil, apierr.Invalidf("Format tanggal tidak valid, gunakan yyyy-MM-dd")
	}
	if *req.AssetPrice < 0 {
		return nil, apierr.Invalidf("Harga Aset tidak boleh kurang dari 0")
	}

	// Java called save() on an entity whose primary key is the plate number, so
	// posting an existing plate silently overwrote that asset instead of
	// failing. A soft-deleted row counts too — reusing its key would resurrect
	// it with new values.
	exists, err := s.repo.ExistsByPlatNomor(ctx, req.PlatNomor)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apierr.Invalidf("Asset dengan plat nomor %s sudah terdaftar", req.PlatNomor)
	}

	supplierID := req.SupplierID
	asset := model.Asset{
		PlatNomor:        req.PlatNomor,
		Nama:             req.AssetName,
		JenisAset:        req.AssetType,
		Status:           req.Status,
		TanggalPerolehan: acquired,
		NilaiPerolehan:   *req.AssetPrice,
		Deskripsi:        req.AssetDescription,
		IDSupplier:       &supplierID,
	}

	if photo != nil {
		key := photoKey(req.PlatNomor, photo.ContentType)
		if err := s.photos.Put(ctx, key, photo.ContentType, photo.Body); err != nil {
			return nil, apierr.Invalidf("Gagal mengupload foto: %v", err)
		}
		asset.FotoKey = key
		asset.FotoContentType = photo.ContentType
	}

	if err := s.repo.Create(ctx, &asset); err != nil {
		// The photo is already stored; drop it so a failed create leaves nothing
		// behind in the bucket.
		s.discardPhoto(ctx, asset.FotoKey)
		return nil, err
	}

	out := s.detail(asset)
	return &out, nil
}

// UploadedPhoto is one incoming image. ContentType has already passed
// blob.ContentTypeFor, so it is a type we are willing to serve back.
type UploadedPhoto struct {
	ContentType string
	Body        io.Reader
}

// UpdateAsset edits the editable fields. The plate number, acquisition date and
// value are fixed at creation, as they were in Java (@Column(updatable = false)).
func (s *AssetService) UpdateAsset(ctx context.Context, platNomor string, req dto.UpdateAssetRequest) (*dto.AssetResponse, error) {
	asset, err := s.repo.FindByPlatNomor(ctx, platNomor)
	if err != nil {
		return nil, notFound(err, platNomor)
	}

	asset.Nama = req.Nama
	asset.JenisAset = req.JenisAset
	asset.Status = req.Status
	asset.Deskripsi = req.Deskripsi

	if err := s.repo.Save(ctx, asset); err != nil {
		return nil, err
	}
	out := s.detail(*asset)
	return &out, nil
}

// SetSupplier attaches a supplier to an asset. The purchase service calls this
// once it knows which supplier a bought asset came from.
func (s *AssetService) SetSupplier(ctx context.Context, platNomor, supplierID string) (*dto.AssetResponse, error) {
	if !isUUID(supplierID) {
		return nil, apierr.Invalidf("Format Supplier ID tidak valid")
	}

	asset, err := s.repo.FindByPlatNomor(ctx, platNomor)
	if err != nil {
		return nil, notFound(err, platNomor)
	}

	asset.IDSupplier = &supplierID
	if err := s.repo.Save(ctx, asset); err != nil {
		return nil, err
	}
	out := s.detail(*asset)
	return &out, nil
}

// DeleteAsset soft-deletes an asset and drops its photo.
func (s *AssetService) DeleteAsset(ctx context.Context, platNomor string) error {
	asset, err := s.repo.FindByPlatNomor(ctx, platNomor)
	if err != nil {
		return notFound(err, platNomor)
	}
	if err := s.repo.SoftDelete(ctx, platNomor); err != nil {
		return err
	}
	// The row is gone either way; a photo left behind is waste, not corruption,
	// so a failure here is logged rather than failing the request.
	s.discardPhoto(ctx, asset.FotoKey)
	return nil
}

func (s *AssetService) discardPhoto(ctx context.Context, key string) {
	if key == "" {
		return
	}
	if err := s.photos.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "could not delete asset photo", "key", key, "error", err)
	}
}

// GetPhoto opens an asset's photo. The caller closes the returned Body.
func (s *AssetService) GetPhoto(ctx context.Context, platNomor string) (*Photo, error) {
	asset, err := s.repo.FindByPlatNomor(ctx, platNomor)
	if err != nil {
		return nil, notFound(err, platNomor)
	}
	if asset.FotoKey == "" {
		return nil, apierr.NotFoundf("Asset %s tidak memiliki foto", platNomor)
	}

	object, err := s.photos.Get(ctx, asset.FotoKey)
	if err != nil {
		// A key recorded on the row with nothing behind it is a broken
		// reference, not a missing asset.
		return nil, apierr.NotFoundf("Foto untuk asset %s tidak ditemukan", platNomor)
	}
	return object, nil
}

// ListMaintenance returns an asset's service history, newest first.
func (s *AssetService) ListMaintenance(ctx context.Context, platNomor string) ([]dto.MaintenanceResponse, error) {
	asset, err := s.repo.FindByPlatNomor(ctx, platNomor)
	if err != nil {
		return nil, notFound(err, platNomor)
	}

	records, err := s.repo.FindMaintenanceByAsset(ctx, platNomor)
	if err != nil {
		return nil, err
	}

	out := make([]dto.MaintenanceResponse, 0, len(records))
	for _, m := range records {
		row := dto.MaintenanceResponse{
			ID:                      m.ID,
			TanggalMulaiMaintenance: jsontime.UTC(m.TanggalMulaiMaintenance),
			DeskripsiPekerjaan:      m.DeskripsiPekerjaan,
			Biaya:                   m.Biaya,
			Status:                  m.Status,
			PlatNomor:               asset.PlatNomor,
			NamaAset:                asset.Nama,
		}
		if m.TanggalSelesaiMaintenance != nil {
			finished := jsontime.UTC(*m.TanggalSelesaiMaintenance)
			row.TanggalSelesaiMaintenance = &finished
		}
		out = append(out, row)
	}
	return out, nil
}
