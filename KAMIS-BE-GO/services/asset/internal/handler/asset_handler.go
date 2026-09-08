// Package handler is the transport layer (Spring @RestController).
//
// Errors go through httpx.RespondError, so every endpoint answers 400 for a
// caller mistake, 404 for a missing asset and 500 for anything else. The legacy
// AssetController answered 404 for almost everything, including validation
// failures and database errors.
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/blob"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/services/asset/internal/dto"
	"github.com/pebintk/kamis-be-go/services/asset/internal/service"
)

// maxUploadBytes matches the legacy spring.servlet.multipart.max-file-size.
const maxUploadBytes = 10 << 20

type AssetHandler struct{ svc *service.AssetService }

func NewAssetHandler(svc *service.AssetService) *AssetHandler { return &AssetHandler{svc: svc} }

// All handles GET /api/asset/all.
func (h *AssetHandler) All(c *gin.Context) {
	assets, err := h.svc.ListAssets(c.Request.Context())
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "List Asset berhasil ditemukan", assets)
}

// Paginated handles GET /api/asset/viewall/paginated.
func (h *AssetHandler) Paginated(c *gin.Context) {
	number, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	result, err := h.svc.ListAssetsPaginated(c.Request.Context(),
		c.Query("nama"), c.Query("jenisAset"), c.Query("status"), number, size)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", result)
}

// Detail handles GET /api/asset/{platNomor}.
func (h *AssetHandler) Detail(c *gin.Context) {
	asset, err := h.svc.GetAsset(c.Request.Context(), c.Param("platNomor"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Detail Asset berhasil ditemukan", asset)
}

// Delete handles DELETE /api/asset/{platNomor}. The row is soft-deleted, as in
// Java; the photo is removed for real.
func (h *AssetHandler) Delete(c *gin.Context) {
	if err := h.svc.DeleteAsset(c.Request.Context(), c.Param("platNomor")); err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Asset berhasil dihapus", nil)
}

// Update handles PUT /api/asset/{platNomor}.
func (h *AssetHandler) Update(c *gin.Context) {
	var req dto.UpdateAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	asset, err := h.svc.UpdateAsset(c.Request.Context(), c.Param("platNomor"), req)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Detail Asset berhasil diperbarui", asset)
}

// Add handles POST /api/asset/addAsset, a multipart/form-data request from the
// purchase service carrying the asset fields and, optionally, a photo.
func (h *AssetHandler) Add(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	var req dto.AddAssetRequest
	if err := c.ShouldBind(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	photo, err := uploadedPhoto(c)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	if photo != nil {
		defer func() { _ = photo.close() }()
	}

	var upload *service.UploadedPhoto
	if photo != nil {
		upload = &photo.UploadedPhoto
	}

	asset, err := h.svc.AddAsset(c.Request.Context(), req, upload)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", asset)
}

// Photo handles GET /api/asset/{platNomor}/foto, streaming the image straight
// from the blob store to the response rather than buffering it.
func (h *AssetHandler) Photo(c *gin.Context) {
	object, err := h.svc.GetPhoto(c.Request.Context(), c.Param("platNomor"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	defer func() { _ = object.Body.Close() }()

	// Content-Disposition mirrors the legacy response, which served the image as
	// an attachment.
	c.Header("Content-Disposition", `attachment; filename="`+c.Param("platNomor")+`"`)
	c.DataFromReader(http.StatusOK, object.Size, object.ContentType, object.Body, nil)
}

// Maintenance handles GET /api/asset/{platNomor}/maintenance.
func (h *AssetHandler) Maintenance(c *gin.Context) {
	records, err := h.svc.ListMaintenance(c.Request.Context(), c.Param("platNomor"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Riwayat maintenance aset berhasil ditemukan", records)
}

// BySupplier handles GET /api/asset/by-supplier/{supplierId}.
func (h *AssetHandler) BySupplier(c *gin.Context) {
	assets, err := h.svc.ListBySupplier(c.Request.Context(), c.Param("supplierId"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Daftar aset dari supplier berhasil ditemukan", assets)
}

// SetSupplier handles PUT /api/asset/{platNomor}/supplier. The supplier id
// arrives as a query parameter, not a body, as it did in Java.
func (h *AssetHandler) SetSupplier(c *gin.Context) {
	asset, err := h.svc.SetSupplier(c.Request.Context(), c.Param("platNomor"), c.Query("supplierId"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Supplier berhasil ditambahkan ke aset", asset)
}

// openPhoto is an uploaded file plus the handle to close when the request ends.
type openPhoto struct {
	service.UploadedPhoto
	close func() error
}

// uploadedPhoto reads the optional "foto" part. A missing part is not an error —
// assets can be registered without a photo — but a part that is not an image is
// rejected, unlike Java, which stored whatever arrived and later served it back
// with its own content type.
func uploadedPhoto(c *gin.Context) (*openPhoto, error) {
	header, err := c.FormFile("foto")
	if err != nil || header == nil {
		return nil, nil
	}
	if header.Size > maxUploadBytes {
		return nil, apierr.Invalidf("Ukuran foto melebihi batas 10MB")
	}

	contentType, ok := blob.ContentTypeFor(header.Header.Get("Content-Type"))
	if !ok {
		return nil, apierr.Invalidf("Format foto tidak didukung: gunakan JPEG, PNG, WebP, atau GIF")
	}

	file, err := header.Open()
	if err != nil {
		return nil, apierr.Invalidf("Gagal membaca foto: %v", err)
	}
	return &openPhoto{
		UploadedPhoto: service.UploadedPhoto{ContentType: contentType, Body: file},
		close:         file.Close,
	}, nil
}
