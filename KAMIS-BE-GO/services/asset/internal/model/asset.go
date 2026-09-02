// Package model holds the GORM entities for the asset service.
package model

import (
	"time"

	"gorm.io/gorm"
)

// Asset is a company vehicle, keyed by its plate number. The Java entity
// declared quoted camelCase columns (@Column(name = "platNomor")); those are
// gone in favour of GORM's defaults, per the naming decision in MIGRATION.md.
type Asset struct {
	PlatNomor        string `gorm:"primaryKey"`
	Nama             string `gorm:"not null"`
	JenisAset        string `gorm:"not null"`
	Status           string `gorm:"not null"`
	TanggalPerolehan time.Time
	NilaiPerolehan   int     `gorm:"not null"`
	Deskripsi        string  `gorm:"not null"`
	IDSupplier       *string `gorm:"type:uuid;index"`

	// FotoKey is the object key in the blob store, and FotoContentType the type
	// to serve it back with. Java called the first one fotoFilename, when the
	// images lived on the container's own disk; neither is exposed in any DTO,
	// so the rename is invisible to callers.
	FotoKey         string
	FotoContentType string

	// DeletedAt replaces the Java isDeleted flag plus deletedAt column. GORM
	// filters soft-deleted rows out of every query automatically, which is what
	// the repository's hand-written `WHERE isDeleted = false` variants did — and
	// unlike the Java softDeleteById, it actually records when.
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Maintenance is one service record against an asset.
type Maintenance struct {
	ID                        int64     `gorm:"primaryKey"`
	TanggalMulaiMaintenance   time.Time `gorm:"not null"`
	TanggalSelesaiMaintenance *time.Time
	DeskripsiPekerjaan        string `gorm:"not null"`

	// Biaya is money held as a float because the Java column is a Float and the
	// frontend parses it as a JSON number. Worth revisiting as minor units if
	// this service ever does arithmetic on it.
	Biaya  float64 `gorm:"not null"`
	Status string  `gorm:"not null"`

	// AssetPlatNomor is the Java @ManyToOne join column. The association is not
	// modelled: every query here fetches maintenance rows for known plate
	// numbers, so an explicit column keeps the loading behaviour visible.
	AssetPlatNomor string `gorm:"not null;index"`
}

// Reservation statuses. The legacy code repeats these literals across queries
// and conditionals.
const (
	ReservationDirencanakan = "Direncanakan"
	ReservationDilaksanakan = "Dilaksanakan"
	ReservationSelesai      = "Selesai"
	ReservationBatal        = "Batal"
)

// Asset statuses that gate what can be scheduled against a vehicle.
const (
	AssetTersedia          = "Tersedia"
	AssetSedangMaintenance = "Sedang Maintenance"
	AssetDalamAktivitas    = "Dalam Aktivitas"
)

// AssetReservation books a vehicle to a project for a date range.
type AssetReservation struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	PlatNomor string    `gorm:"not null;index"`
	ProjectID string    `gorm:"not null;index"`
	StartDate time.Time `gorm:"not null"`
	EndDate   time.Time `gorm:"not null"`

	// ReservationStatus is one of the Reservation* constants above. The Java
	// column is named "status"; GORM derives reservation_status from the field,
	// which is clearer and, since the schema is ours, free to change.
	ReservationStatus string `gorm:"not null;index"`
}

// Active reports whether this reservation still blocks the vehicle.
func (r AssetReservation) Active() bool {
	return r.ReservationStatus != ReservationBatal && r.ReservationStatus != ReservationSelesai
}
