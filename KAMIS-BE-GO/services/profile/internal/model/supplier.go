package model

import "time"

// Supplier is a vendor PT Karina buys assets and resources from. See the note on
// Client about naming.
//
// The three ID lists are JPA @ElementCollection tables in the Java model, each a
// plain (supplier_id, <x>_id) pair with no primary key. GORM has no direct
// equivalent, so they are modelled as the three structs below and loaded/saved
// explicitly by SupplierRepository.
type Supplier struct {
	ID              string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	NameSupplier    string `gorm:"uniqueIndex;not null"`
	NoTelpSupplier  string `gorm:"uniqueIndex;not null"`
	EmailSupplier   string `gorm:"uniqueIndex;not null"`
	CompanySupplier string
	AddressSupplier string `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time

	// Populated by the repository from the collection tables; not columns.
	AssetIDs    []string `gorm:"-"`
	ResourceIDs []int64  `gorm:"-"`
	PurchaseIDs []string `gorm:"-"`
}

// SupplierAsset is one row of the supplier_assets collection table.
type SupplierAsset struct {
	SupplierID string `gorm:"type:uuid;index"`
	AssetID    string
}

// SupplierResource is one row of the supplier_resources collection table.
type SupplierResource struct {
	SupplierID string `gorm:"type:uuid;index"`
	ResourceID int64
}

// SupplierPurchase is one row of the supplier_purchases collection table.
type SupplierPurchase struct {
	SupplierID string `gorm:"type:uuid;index"`
	PurchaseID string
}
