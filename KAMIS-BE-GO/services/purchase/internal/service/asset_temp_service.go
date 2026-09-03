package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/blob"
	"github.com/karina/kamis-be-go/services/purchase/internal/dto"
	"github.com/karina/kamis-be-go/services/purchase/internal/model"
)

// UploadedPhoto is one incoming image. ContentType has already passed
// blob.ContentTypeFor, so it is a type we are willing to serve back.
type UploadedPhoto struct {
	ContentType string
	Body        io.Reader
}

// Photo is a staged asset's image on its way to the HTTP response.
type Photo = blob.Object

// stagedPhotoKey names a staged asset's image.
//
// The key is random rather than derived from the asset, because the asset's id
// does not exist until it is inserted and its name is not unique. Java used
// "asset_" + a random UUID for the same reason; this keeps the shape and adds
// the extension so the disk backend can recover the content type.
func stagedPhotoKey(contentType string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "asset_" + hex.EncodeToString(buf) + blob.ExtensionFor(contentType), nil
}

// AddAssetTemp stages an asset for a purchase, optionally with a photo. photo
// may be nil.
func (s *PurchaseService) AddAssetTemp(ctx context.Context, req dto.AddAssetTempRequest, photo *UploadedPhoto) (*dto.AssetTempResponse, error) {
	if *req.AssetPrice < 0 {
		return nil, apierr.Invalidf("Harga Aset tidak boleh kurang dari 0")
	}

	staged := model.AssetTemp{
		AssetName:        req.AssetName,
		AssetDescription: req.AssetDescription,
		AssetType:        req.AssetType,
		AssetPrice:       *req.AssetPrice,
	}

	if photo != nil {
		key, err := stagedPhotoKey(photo.ContentType)
		if err != nil {
			return nil, err
		}
		if err := s.Photos.Put(ctx, key, photo.ContentType, photo.Body); err != nil {
			return nil, apierr.Invalidf("Gagal mengupload foto: %v", err)
		}
		staged.FotoKey = key
		staged.FotoContentType = photo.ContentType
	}

	if err := s.repo.CreateAssetTemp(ctx, &staged); err != nil {
		// The photo is already stored; drop it so a failed insert leaves nothing
		// orphaned in the bucket.
		if staged.FotoKey != "" {
			if delErr := s.Photos.Delete(ctx, staged.FotoKey); delErr != nil {
				slog.WarnContext(ctx, "could not delete orphaned staged photo",
					"key", staged.FotoKey, "error", delErr)
			}
		}
		return nil, err
	}
	return assetTempResponse(staged), nil
}

// GetAssetTemp returns one staged asset.
//
// Java answered 200 with a null body for an id that does not exist, because its
// mapper turned a missing row into null. That is exactly what the uniform status
// rule exists to prevent, so this is a 404.
func (s *PurchaseService) GetAssetTemp(ctx context.Context, id int64) (*dto.AssetTempResponse, error) {
	staged, err := s.repo.FindAssetTempByID(ctx, id)
	if err != nil {
		return nil, assetNotFound(err)
	}
	return assetTempResponse(*staged), nil
}

// ListAssetTemps returns every staged asset.
func (s *PurchaseService) ListAssetTemps(ctx context.Context) ([]dto.AssetTempResponse, error) {
	staged, err := s.repo.FindAllAssetTemps(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.AssetTempResponse, 0, len(staged))
	for _, a := range staged {
		out = append(out, *assetTempResponse(a))
	}
	return out, nil
}

// GetAssetPhoto opens a staged asset's photo. The caller closes the returned
// Body.
func (s *PurchaseService) GetAssetPhoto(ctx context.Context, id int64) (*Photo, error) {
	staged, err := s.repo.FindAssetTempByID(ctx, id)
	if err != nil {
		return nil, assetNotFound(err)
	}
	if staged.FotoKey == "" {
		return nil, apierr.NotFoundf("Aset %d tidak memiliki foto", id)
	}

	object, err := s.Photos.Get(ctx, staged.FotoKey)
	if err != nil {
		// A key recorded on the row with nothing behind it is a broken
		// reference, not a missing asset.
		return nil, apierr.NotFoundf("Foto untuk aset %d tidak ditemukan", id)
	}
	return object, nil
}
