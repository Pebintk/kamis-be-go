package model

import "time"

// Supplier mirrors the legacy Supplier entity. See the note on Client about the
// quoted, space-containing column names.
//
// The three ID lists are JPA @ElementCollection tables in the Java model, each a
// plain (supplier_id, <x>_id) pair with no primary key. GORM has no direct
// equivalent, so they are modelled as the three structs below and loaded/saved
// explicitly by SupplierRepository — the join tables keep their legacy names and
// columns, so the same rows are read either way.
type Supplier struct {
	ID              string    `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	NameSupplier    string    `gorm:"column:Nama;not null;uniqueIndex"`
	NoTelpSupplier  string    `gorm:"column:Nomor Telepon;not null;uniqueIndex"`
	EmailSupplier   string    `gorm:"column:Email;not null;uniqueIndex"`
	CompanySupplier string    `gorm:"column:Perusahaan"`
	AddressSupplier string    `gorm:"column:Alamat;not null"`
	CreatedDate     time.Time `gorm:"column:Created Date;autoCreateTime;not null"`
	UpdatedDate     time.Time `gorm:"column:Updated Date;autoUpdateTime;not null"`

	// Populated by the repository from the collection tables; not columns.
	AssetIDs    []string `gorm:"-"`
	ResourceIDs []int64  `gorm:"-"`
	PurchaseIDs []string `gorm:"-"`
}

func (Supplier) TableName() string { return "Supplier" }

// SupplierAsset is one row of the supplier_assets @ElementCollection table.
type SupplierAsset struct {
	SupplierID string `gorm:"column:supplier_id;type:uuid;index"`
	AssetID    string `gorm:"column:asset_id"`
}

func (SupplierAsset) TableName() string { return "supplier_assets" }

// SupplierResource is one row of the supplier_resources @ElementCollection table.
type SupplierResource struct {
	SupplierID string `gorm:"column:supplier_id;type:uuid;index"`
	ResourceID int64  `gorm:"column:resource_id"`
}

func (SupplierResource) TableName() string { return "supplier_resources" }

// SupplierPurchase is one row of the supplier_purchases @ElementCollection table.
type SupplierPurchase struct {
	SupplierID string `gorm:"column:supplier_id;type:uuid;index"`
	PurchaseID string `gorm:"column:purchase_id"`
}

func (SupplierPurchase) TableName() string { return "supplier_purchases" }
