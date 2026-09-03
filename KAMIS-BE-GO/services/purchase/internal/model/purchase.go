// Package model holds the GORM entities for the purchase service.
package model

import (
	"time"

	"gorm.io/gorm"
)

// Purchase statuses. A purchase advances Diajukan → Disetujui → Diproses →
// Selesai; Dibatalkan and Ditolak are terminal branches off that path. The
// legacy service repeats these literals across every transition and comparison.
const (
	StatusDiajukan   = "Diajukan"
	StatusDisetujui  = "Disetujui"
	StatusDiproses   = "Diproses"
	StatusSelesai    = "Selesai"
	StatusDibatalkan = "Dibatalkan"
	StatusDitolak    = "Ditolak"
)

// Terminal reports whether a status can still be advanced.
func Terminal(status string) bool {
	switch status {
	case StatusSelesai, StatusDibatalkan, StatusDitolak:
		return true
	}
	return false
}

// Purchase type. The Java column is a bare boolean with the mapping in a
// comment; these name it at the call sites.
const (
	TypeAset     = false
	TypeResource = true
)

// Purchase is one procurement request. Its ID is a human-readable code the
// service generates: {A|R}-{ddMMyy}-{NNN}, e.g. R-030926-001.
//
// The Java entity declared quoted, space-separated columns ("Supplier
// Pembelian", "Tanggal Pengajuan"); those are gone in favour of GORM's
// defaults, per the naming decision in MIGRATION.md.
type Purchase struct {
	ID               string `gorm:"primaryKey"`
	PurchaseSupplier string `gorm:"type:uuid;not null;index"`

	// PurchaseType is false for an asset purchase, true for resources — see
	// TypeAset / TypeResource.
	PurchaseType   bool   `gorm:"not null"`
	PurchaseStatus string `gorm:"not null;index"`
	PurchasePrice  int    `gorm:"not null"`

	// PurchaseNote is NOT NULL in the legacy schema while its request DTO left
	// it optional, so an omitted note failed at insert with a 500. A Go string
	// defaults to empty, which the constraint accepts.
	PurchaseNote string `gorm:"not null"`

	PurchaseSubmissionDate time.Time `gorm:"autoCreateTime;not null;index"`
	PurchaseUpdateDate     time.Time `gorm:"autoUpdateTime;not null"`

	// PurchaseAsset points at the AssetTemp being bought, for an asset purchase.
	PurchaseAsset       *int64
	PurchasePaymentDate *time.Time
}

// ResourceTemp is one line item of a resource purchase: a snapshot of the
// resource's name and price at the time of ordering, held here rather than read
// back from the resource service so a later price change does not rewrite
// history.
//
// The Java entity was soft-deleted through Hibernate's @SQLDelete /
// @SQLRestriction pair; gorm.DeletedAt does the same thing and filters every
// query automatically.
type ResourceTemp struct {
	ID         string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	PurchaseID string `gorm:"index"`

	ResourceID    int64  `gorm:"not null"`
	ResourceName  string `gorm:"not null"`
	ResourceTotal int    `gorm:"not null"`
	ResourcePrice int    `gorm:"not null"`

	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Subtotal is what this line contributes to the purchase price.
func (r ResourceTemp) Subtotal() int { return r.ResourcePrice * r.ResourceTotal }

// AssetTemp is a proposed asset, staged here while its purchase is in flight.
// Once the purchase completes, the asset service is asked to register it for
// real and this row stays as the record of what was ordered.
type AssetTemp struct {
	ID               int64  `gorm:"primaryKey"`
	AssetName        string `gorm:"not null"`
	AssetDescription string `gorm:"not null"`
	AssetType        string `gorm:"not null"`
	AssetPrice       int    `gorm:"not null"`

	// FotoKey is the object key in the blob store; Java called it fotoFilename,
	// when the images lived on the container's own disk.
	FotoKey         string
	FotoContentType string
}

// LogPurchase is one entry in a purchase's audit trail.
type LogPurchase struct {
	ID         string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	PurchaseID string `gorm:"index"`

	Username   string    `gorm:"not null"`
	Action     string    `gorm:"size:1000;not null"`
	ActionDate time.Time `gorm:"autoCreateTime;not null"`
}
