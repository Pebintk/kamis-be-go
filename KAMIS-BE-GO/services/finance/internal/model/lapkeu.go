// Package model holds the GORM entities for the finance service.
package model

import "time"

// Activity types. The entity's own comment and the response DTO's disagreed
// about which number meant what; these follow the entity and the callers, which
// send 0 for a sale and 1 for a distribution.
const (
	ActivityPenjualan   = 0
	ActivityDistribusi  = 1
	ActivityPurchase    = 2
	ActivityMaintenance = 3
)

// ActivityName is how the expense chart labels each type.
//
// The legacy chart mapped 1, 2 and 3 and let everything else fall through to
// "UNKNOWN" — including 0, Penjualan. A sale is recorded with an outgoing of
// zero rather than none, so it passes the chart's `pengeluaran IS NOT NULL`
// filter and showed up as an UNKNOWN slice worth nothing.
var ActivityName = map[int]string{
	ActivityPenjualan:   "Penjualan",
	ActivityDistribusi:  "Distribusi",
	ActivityPurchase:    "Pembelian",
	ActivityMaintenance: "Maintenance",
}

// Lapkeu is one line of the financial ledger. Its id is the id of whatever
// caused it — a project, a purchase, a maintenance job — so re-recording the
// same event replaces its entry rather than duplicating it.
type Lapkeu struct {
	ID           string `gorm:"primaryKey"`
	ActivityType int    `gorm:"not null;index"`

	// Either side may be absent: a sale has no outgoing, a purchase no income.
	Pemasukan   *int64
	Pengeluaran *int64

	Description string
	PaymentDate *time.Time `gorm:"index"`
}

// Income and Expense read the two amounts with absent treated as zero, which is
// how every total in this service adds them up.
func (l Lapkeu) Income() int64 {
	if l.Pemasukan == nil {
		return 0
	}
	return *l.Pemasukan
}

func (l Lapkeu) Expense() int64 {
	if l.Pengeluaran == nil {
		return 0
	}
	return *l.Pengeluaran
}
