package model

import "time"

// Client is a customer of PT Karina.
//
// Table and column names come from GORM's default naming strategy
// (clients, name_client, no_telp_client, …). The Java entity spelled them as
// quoted identifiers with spaces — "Nomor Telepon", "Created Date" — which only
// mattered while the Go service had to read Hibernate's existing tables. It no
// longer does, so the awkward names are gone.
type Client struct {
	ID            string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	NameClient    string `gorm:"uniqueIndex;not null"`
	NoTelpClient  string `gorm:"uniqueIndex;not null"`
	EmailClient   string `gorm:"uniqueIndex;not null"`
	TypeClient    bool   `gorm:"not null"` // false = perorangan, true = perusahaan
	CompanyClient string
	AddressClient string `gorm:"not null"`

	// GORM manages these two by name; no tags needed.
	CreatedAt time.Time
	UpdatedAt time.Time
}
