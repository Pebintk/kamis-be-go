package model

import (
	"testing"
	"time"
)

// TestRefreshTokenUsable covers the three ways a token stops working. Each is
// checked separately because the service must not tell them apart to a caller,
// which makes it easy to conflate them here too.
func TestRefreshTokenUsable(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	revoked := now.Add(-time.Hour)

	live := RefreshToken{ExpiresAt: now.Add(time.Hour)}
	if !live.Usable(now) {
		t.Error("a live token is not usable")
	}

	expired := RefreshToken{ExpiresAt: now.Add(-time.Second)}
	if expired.Usable(now) {
		t.Error("an expired token is usable")
	}

	spent := RefreshToken{ExpiresAt: now.Add(time.Hour), RevokedAt: &revoked}
	if spent.Usable(now) {
		t.Error("a revoked token is usable")
	}

	// Revoked wins over an expiry still in the future — that combination is a
	// token someone logged out of, and it must stay dead.
	both := RefreshToken{ExpiresAt: now.Add(-time.Second), RevokedAt: &revoked}
	if both.Usable(now) {
		t.Error("an expired, revoked token is usable")
	}

	// Expiry is exclusive at the boundary: a token expiring exactly now is done.
	edge := RefreshToken{ExpiresAt: now}
	if edge.Usable(now) {
		t.Error("a token expiring exactly now is still usable")
	}
}
