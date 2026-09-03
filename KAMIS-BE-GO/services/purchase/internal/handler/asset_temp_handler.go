package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/blob"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/purchase/internal/dto"
	"github.com/karina/kamis-be-go/services/purchase/internal/service"
)

// maxUploadBytes matches the legacy spring.servlet.multipart.max-file-size.
const maxUploadBytes = 10 << 20

type AssetTempHandler struct{ svc *service.PurchaseService }

func NewAssetTempHandler(svc *service.PurchaseService) *AssetTempHandler {
	return &AssetTempHandler{svc: svc}
}

// Add handles POST /api/purchase/addAsset, a multipart/form-data request
// staging an asset for a purchase.
func (h *AssetTempHandler) Add(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	var req dto.AddAssetTempRequest
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

	staged, err := h.svc.AddAssetTemp(c.Request.Context(), req, upload)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", staged)
}

// Detail handles GET /api/purchase/asset/{idAsset}.
func (h *AssetTempHandler) Detail(c *gin.Context) {
	id, ok := pathInt64(c, "idAsset")
	if !ok {
		return
	}
	staged, err := h.svc.GetAssetTemp(c.Request.Context(), id)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", staged)
}

// Photo handles GET /api/purchase/asset/{idAsset}/foto, streaming the image
// straight from the blob store to the response rather than buffering it.
func (h *AssetTempHandler) Photo(c *gin.Context) {
	id, ok := pathInt64(c, "idAsset")
	if !ok {
		return
	}
	object, err := h.svc.GetAssetPhoto(c.Request.Context(), id)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	defer func() { _ = object.Body.Close() }()

	c.Header("Content-Disposition", `attachment; filename="`+c.Param("idAsset")+`"`)
	c.DataFromReader(http.StatusOK, object.Size, object.ContentType, object.Body, nil)
}

// openPhoto is an uploaded file plus the handle to close when the request ends.
type openPhoto struct {
	service.UploadedPhoto
	close func() error
}

// uploadedPhoto reads the optional "foto" part. A missing part is not an error —
// an asset can be staged without a photo — but a part that is not an image is
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

// pathInt64 parses a numeric path segment, answering 400 on a bad value the way
// Spring did before the controller method ran.
func pathInt64(c *gin.Context, name string) (int64, bool) {
	raw := c.Param(name)
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		httpx.Respond(c, http.StatusBadRequest, name+" harus berupa angka: "+raw, nil)
		return 0, false
	}
	return value, true
}
