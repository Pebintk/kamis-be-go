package model

import "time"

// Client is a customer of PT Karina.
//
// The column names below are quoted identifiers containing spaces and capitals
// ("Nomor Telepon", "Created Date", …). That is not a stylistic choice: the
// legacy JPA entity declares them via @Column(name = "..."), and because those
// names are not valid unquoted SQL identifiers Hibernate emits them quoted — so
// they exist in the live database exactly as spelled here, case included. GORM
// likewise quotes identifiers, so these tags read and write the same columns.
// Verify with `\d "Client"` in psql before cutting over.
type Client struct {
	ID            string    `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	NameClient    string    `gorm:"column:Nama;not null;uniqueIndex"`
	NoTelpClient  string    `gorm:"column:Nomor Telepon;not null;uniqueIndex"`
	EmailClient   string    `gorm:"column:Email;not null;uniqueIndex"`
	TypeClient    bool      `gorm:"column:Tipe;not null"` // false = perorangan, true = perusahaan
	CompanyClient string    `gorm:"column:Perusahaan"`
	AddressClient string    `gorm:"column:Alamat;not null"`
	CreatedDate   time.Time `gorm:"column:Created Date;autoCreateTime;not null"`
	UpdatedDate   time.Time `gorm:"column:Updated Date;autoUpdateTime;not null"`
}

func (Client) TableName() string { return "Client" }
