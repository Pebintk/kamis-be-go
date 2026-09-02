// Package dto holds the request and response bodies of the asset API. The JSON
// field names are the contract the Vue frontend and the purchase service parse,
// so they match the Java dto classes exactly.
package dto

import (
	"github.com/karina/kamis-be-go/pkg/jsontime"
	"github.com/karina/kamis-be-go/pkg/page"
)

// PageOf is the shared Spring-shaped page, re-exported so call sites can take a
// parameter named `page` without shadowing the package.
type PageOf[T any] = page.Of[T]

// NewPage assembles the Spring-shaped page metadata around one page of content.
func NewPage[T any](content []T, number, size int, total int64) PageOf[T] {
	return page.New(content, number, size, total)
}

// Timestamp renders dates the way the un-annotated Java Date fields did (UTC);
// none of the asset DTOs carry @JsonFormat.
type Timestamp = jsontime.Time

// AddAssetRequest is the multipart form of POST /api/asset/addAsset.
//
// This endpoint is service-to-service, not browser-facing: the frontend creates
// assets through the purchase service, which forwards them here as
// multipart/form-data with the photo as a file part. The photo arrives
// separately under the form key "foto".
type AddAssetRequest struct {
	PlatNomor        string `form:"platNomor" binding:"required"`
	AssetName        string `form:"assetName" binding:"required"`
	AssetDescription string `form:"assetDescription" binding:"required"`
	AssetType        string `form:"assetType" binding:"required"`
	AssetPrice       *int   `form:"assetPrice" binding:"required"`
	Status           string `form:"status" binding:"required"`
	// TanggalPerolehan is a yyyy-MM-dd date. Java parsed it with an unguarded
	// SimpleDateFormat, so omitting it raised a NullPointerException and a 500;
	// it is required here.
	TanggalPerolehan string `form:"tanggalPerolehan" binding:"required"`
	SupplierID       string `form:"supplierId" binding:"required"`
}

// UpdateAssetRequest is the JSON body of PUT /api/asset/{platNomor}.
//
// The legacy DTO also had a MultipartFile photo field, but the controller bound
// it with @RequestBody — a JSON body can never carry a file, so that path was
// unreachable and the frontend sends plain JSON. Photos are set at creation.
type UpdateAssetRequest struct {
	Nama      string `json:"nama" binding:"required"`
	JenisAset string `json:"jenisAset" binding:"required"`
	Status    string `json:"status" binding:"required"`
	Deskripsi string `json:"deskripsi" binding:"required"`
}

// AssetListResponse is the row shape for the list and paginated endpoints.
type AssetListResponse struct {
	PlatNomor        string     `json:"platNomor"`
	TipeAset         string     `json:"tipeAset"`
	Nama             string     `json:"nama"`
	Status           string     `json:"status"`
	NilaiPerolehan   int        `json:"nilaiPerolehan"`
	TanggalPerolehan Timestamp  `json:"tanggalPerolehan"`
	SupplierID       *string    `json:"supplierId"`
	LastMaintenance  *Timestamp `json:"lastMaintenance"`
}

// AssetResponse is the detail representation.
type AssetResponse struct {
	PlatNomor        string    `json:"platNomor"`
	Nama             string    `json:"nama"`
	JenisAset        string    `json:"jenisAset"`
	Status           string    `json:"status"`
	TanggalPerolehan Timestamp `json:"tanggalPerolehan"`
	NilaiPerolehan   int       `json:"nilaiPerolehan"`
	Deskripsi        string    `json:"deskripsi"`
	// AssetMaintenance is always null. The Java DTO declares it and nothing ever
	// sets it; it is kept so the JSON shape is unchanged.
	AssetMaintenance *string `json:"assetMaintenance"`
	FotoContentType  *string `json:"fotoContentType"`
	FotoURL          *string `json:"fotoUrl"`
	SupplierID       *string `json:"supplierId"`
}

// MaintenanceResponse is one row of an asset's service history.
type MaintenanceResponse struct {
	ID                        int64      `json:"id"`
	TanggalMulaiMaintenance   Timestamp  `json:"tanggalMulaiMaintenance"`
	TanggalSelesaiMaintenance *Timestamp `json:"tanggalSelesaiMaintenance"`
	DeskripsiPekerjaan        string     `json:"deskripsiPekerjaan"`
	Biaya                     float64    `json:"biaya"`
	Status                    string     `json:"status"`
	PlatNomor                 string     `json:"platNomor"`
	NamaAset                  string     `json:"namaAset"`
}

// MaintenanceRequest is the JSON body of POST /api/maintenance/.
type MaintenanceRequest struct {
	PlatNomor          string `json:"platNomor" binding:"required"`
	DeskripsiPekerjaan string `json:"deskripsiPekerjaan" binding:"required"`
	// Biaya is @Positive in the legacy DTO, enforced in the service so the
	// message matches the other validation failures.
	Biaya                     *float64   `json:"biaya" binding:"required"`
	TanggalMulaiMaintenance   *Timestamp `json:"tanggalMulaiMaintenance" binding:"required"`
	TanggalSelesaiMaintenance *Timestamp `json:"tanggalSelesaiMaintenance"`
}

// AddLapkeuRequest is the body the asset service sends to finance when a
// maintenance record is created, so the cost lands in the ledger.
type AddLapkeuRequest struct {
	ID           string    `json:"id"`
	ActivityType int       `json:"activityType"`
	Pemasukan    int64     `json:"pemasukan"`
	Pengeluaran  int64     `json:"pengeluaran"`
	Description  string    `json:"description"`
	PaymentDate  Timestamp `json:"paymentDate"`
}

// AssetAvailabilityRequest is the body of POST /api/asset/reservations/check-availability.
// ExcludeProjectId is optional: when editing a project's asset list, that
// project's own bookings must not count against it.
type AssetAvailabilityRequest struct {
	PlatNomors       []string   `json:"platNomors" binding:"required"`
	StartDate        *Timestamp `json:"startDate" binding:"required"`
	EndDate          *Timestamp `json:"endDate" binding:"required"`
	ExcludeProjectID string     `json:"excludeProjectId"`
}

// AssetReservationRequest is the body of POST /api/asset/reservations/reserve.
type AssetReservationRequest struct {
	PlatNomors []string   `json:"platNomors" binding:"required"`
	ProjectID  string     `json:"projectId" binding:"required"`
	StartDate  *Timestamp `json:"startDate" binding:"required"`
	EndDate    *Timestamp `json:"endDate" binding:"required"`
}

// AssetReservationResponse is one booking. AssetName and AssetType are carried
// for the caller's convenience, as they were in Java.
type AssetReservationResponse struct {
	ID                string    `json:"id"`
	PlatNomor         string    `json:"platNomor"`
	ProjectID         string    `json:"projectId"`
	StartDate         Timestamp `json:"startDate"`
	EndDate           Timestamp `json:"endDate"`
	ReservationStatus string    `json:"reservationStatus"`
	AssetName         string    `json:"assetName"`
	AssetType         string    `json:"assetType"`
}
