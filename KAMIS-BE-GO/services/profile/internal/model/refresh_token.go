package model

import "time"

// RefreshToken is a long-lived credential that buys new access tokens.
//
// Only the SHA-256 hash of the token is stored, so a leaked database yields
// nothing presentable. Rows are kept after revocation rather than deleted: a
// revoked row is what lets a replayed token be recognised as theft.
type RefreshToken struct {
	// TokenHash is the primary key, so a token is looked up by exactly the value
	// presented and nothing else is indexed by it.
	TokenHash string `gorm:"primaryKey"`

	Username string `gorm:"not null;index"`
	// Role is captured at issue so a refresh does not need to re-read the user,
	// and so a role change takes effect on the next login rather than silently
	// on the next refresh.
	Role string `gorm:"not null"`

	ExpiresAt time.Time `gorm:"not null;index"`
	RevokedAt *time.Time
	CreatedAt time.Time `gorm:"autoCreateTime;not null"`
}

// Usable reports whether this token can still be exchanged.
func (r RefreshToken) Usable(now time.Time) bool {
	return r.RevokedAt == nil && now.Before(r.ExpiresAt)
}
