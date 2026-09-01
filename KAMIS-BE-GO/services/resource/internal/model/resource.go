// Package model holds the GORM entities for the resource service.
package model

// Resource is one item in the inventory catalogue — the Java
// gpl.karina.resource.model.Resource.
//
// The Java entity declared its columns as quoted identifiers with spaces
// (@Column(name = "Nama Barang") and friends). Those are gone: this takes GORM's
// defaults (table `resources`, columns `resource_name`, `resource_stock`, …).
// The field names still match the DTO field names the frontend expects.
type Resource struct {
	ID                  int64  `gorm:"primaryKey"`
	ResourceName        string `gorm:"uniqueIndex;not null"`
	ResourceDescription string `gorm:"not null"`
	ResourceStock       int    `gorm:"not null"`
	ResourcePrice       int    `gorm:"not null"`
}

// ResourceSupplier links a resource to a supplier that sells it. In Java this
// was an @ElementCollection List<UUID> on Resource, backed by the
// resource_supplier_ids table.
//
// Unlike the equivalent tables in the profile service, this one has a composite
// primary key. The link is a set — "supplier X sells resource Y" is either true
// or not — and the key lets the repository attach a link with a single
// ON CONFLICT DO NOTHING insert instead of the read-then-append the Java service
// did, which raced with itself.
type ResourceSupplier struct {
	ResourceID int64  `gorm:"primaryKey"`
	SupplierID string `gorm:"type:uuid;primaryKey"`
}
