// Package model holds the profile service's persistence entities.
package model

import (
	"strings"
	"time"
)

// EndUser is the account/login entity.
//
// The legacy Java service modeled roles with JPA JOINED inheritance — an
// EndUser table plus empty Admin/Operasional/Finance/Direksi child tables keyed
// by a `user_type` discriminator. That accidental complexity collapses here to a
// single table with a `user_type` column. The legacy UPPERCASE discriminator
// values are preserved so existing data flattens over directly (see ROLES).
type EndUser struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	UserType  string    `gorm:"column:user_type;not null" json:"userType"` // UPPERCASE discriminator
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (EndUser) TableName() string { return "end_user" }

// Authority returns the PascalCase role used as the JWT "role" claim and by the
// route guards — matching the legacy getClass().getSimpleName() behaviour.
func (u EndUser) Authority() string { return AuthorityFromDiscriminator(u.UserType) }

// APIRole returns the lowercase role used in request/response bodies.
func (u EndUser) APIRole() string { return apiRoleFromDiscriminator(u.UserType) }

// Role captures the three casings the system uses for a single role:
//   - Discriminator: stored in user_type, UPPERCASE   ("ADMIN")
//   - Authority:     JWT claim + route guards, Pascal  ("Admin")
//   - API:           request/response bodies, lowercase ("admin")
type Role struct {
	Discriminator string
	Authority     string
	API           string
}

// ROLES is the canonical mapping across all three casings.
var ROLES = []Role{
	{Discriminator: "ADMIN", Authority: "Admin", API: "admin"},
	{Discriminator: "OPERASIONAL", Authority: "Operasional", API: "operasional"},
	{Discriminator: "FINANCE", Authority: "Finance", API: "finance"},
	{Discriminator: "DIREKSI", Authority: "Direksi", API: "direksi"},
}

// AuthorityFromDiscriminator maps a stored user_type to the JWT role claim.
func AuthorityFromDiscriminator(userType string) string {
	for _, r := range ROLES {
		if strings.EqualFold(r.Discriminator, userType) {
			return r.Authority
		}
	}
	return ""
}

func apiRoleFromDiscriminator(userType string) string {
	for _, r := range ROLES {
		if strings.EqualFold(r.Discriminator, userType) {
			return r.API
		}
	}
	return ""
}

// DiscriminatorFromAPIRole maps a lowercase API role (e.g. from an add-user
// request) to the stored user_type. ok is false for an unknown role.
func DiscriminatorFromAPIRole(apiRole string) (discriminator string, ok bool) {
	for _, r := range ROLES {
		if strings.EqualFold(r.API, apiRole) {
			return r.Discriminator, true
		}
	}
	return "", false
}
