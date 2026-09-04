// Package model holds the GORM entities for the project service.
package model

import "time"

// Project statuses, stored as the integers the frontend sends and renders.
const (
	StatusDirencanakan = 0
	StatusDilaksanakan = 1
	StatusSelesai      = 2
	StatusBatal        = 3
)

// Payment statuses.
const (
	PaymentBelumLunas   = 0
	PaymentTelahLunas   = 1
	PaymentDikembalikan = 2
)

// Project kinds. The Java column is a bare boolean with the mapping in a
// comment; these name it at the call sites.
const (
	TypePenjualan  = false // a sale, drawing on the resource catalogue
	TypePengiriman = true  // a distribution, drawing on vehicles
)

// Project is one job for a client — either a sale (Penjualan) or a distribution
// (Pengiriman), told apart by ProjectType.
//
// Java modelled the two as JPA JOINED inheritance: a `project` table plus
// `Penjualan` and `distribusi` child tables keyed by a `jenis_proyek`
// discriminator. That discriminator was redundant — `tipe_proyek` already says
// which kind a row is — and every list query had to LEFT JOIN the child table
// back in. The Go port collapses it to one table with a nullable block of
// distribution-only columns, the same way profile's role inheritance was
// collapsed (see MIGRATION.md).
type Project struct {
	ID          string `gorm:"primaryKey"`
	ProjectType bool   `gorm:"not null;index"`

	ProjectStatus        int  `gorm:"not null;index"`
	ProjectPaymentStatus *int `gorm:"index"`

	ProjectName            string `gorm:"not null"`
	ProjectDescription     string
	ProjectClientID        string `gorm:"not null;index"`
	ProjectClientName      string `gorm:"not null"`
	ProjectDeliveryAddress string `gorm:"not null"`

	CreatedDate            time.Time  `gorm:"autoCreateTime"`
	ProjectStartDate       *time.Time `gorm:"index"`
	ProjectEndDate         *time.Time `gorm:"index"`
	ProjectPaymentDate     *time.Time
	ProjectTotalPemasukkan *int64

	// ---- distribution only ----
	// Null on a sale. Kept on the same row rather than in a child table: the
	// only thing the join bought was schema purity, and every list query paid
	// for it.
	ProjectPickupAddress    string
	ProjectPHLCount         *int
	ProjectPHLPay           *int64
	ProjectTotalPengeluaran *int64
}

// Profit is income minus outgoings, or nil when either side is unknown — the
// Java calculateProjectProfit.
func (p Project) Profit() *int64 {
	if p.ProjectTotalPemasukkan == nil {
		return nil
	}
	out := int64(0)
	if p.ProjectTotalPengeluaran != nil {
		out = *p.ProjectTotalPengeluaran
	}
	profit := *p.ProjectTotalPemasukkan - out
	return &profit
}

// Active reports whether a project still holds its resources and vehicles.
func (p Project) Active() bool {
	return p.ProjectStatus != StatusSelesai && p.ProjectStatus != StatusBatal
}

// ProjectAssetUsage is one vehicle booked to a distribution, with what it cost
// on this job.
type ProjectAssetUsage struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProjectID string `gorm:"not null;index"`

	PlatNomor     string `gorm:"not null"`
	TipeAset      string `gorm:"not null"`
	AssetUseCost  int    `gorm:"not null"`
	AssetFuelCost int    `gorm:"not null"`
}

// Cost is what this vehicle contributed to the project's outgoings.
func (u ProjectAssetUsage) Cost() int64 { return int64(u.AssetUseCost) + int64(u.AssetFuelCost) }

// ProjectResourceUsage is one catalogue item consumed by a sale, priced at what
// it was worth when the sale was recorded.
type ProjectResourceUsage struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProjectID string `gorm:"not null;index"`

	ResourceID   string `gorm:"not null"`
	SellPrice    int    `gorm:"not null"`
	QuantityUsed int    `gorm:"not null"`
}

// Revenue is what this line contributed to the project's income.
func (u ProjectResourceUsage) Revenue() int64 { return int64(u.SellPrice) * int64(u.QuantityUsed) }

// LogProject is one entry in a project's audit trail.
type LogProject struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProjectID string `gorm:"index"`

	Username   string    `gorm:"not null"`
	Action     string    `gorm:"size:1000;not null"`
	ActionDate time.Time `gorm:"autoCreateTime;not null"`
}
