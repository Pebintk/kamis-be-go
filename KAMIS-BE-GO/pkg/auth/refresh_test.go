package auth

import (
	"strings"
	"testing"
)

// TestNewRefreshTokenIsOpaqueAndHashed covers the two properties the store
// depends on: tokens are unguessable, and what is stored cannot be presented.
func TestNewRefreshToken(t *testing.T) {
	token, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}

	// 32 bytes base64url with no padding.
	if len(token) != 43 {
		t.Errorf("token length = %d, want 43", len(token))
	}
	if strings.ContainsAny(token, "+/=") {
		t.Errorf("token %q is not base64url without padding", token)
	}
	if hash == token {
		t.Error("stored hash equals the token; a leaked database would be usable")
	}
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64 hex characters", len(hash))
	}

	// The hash must be reproducible from the token, and only from that token.
	if HashRefreshToken(token) != hash {
		t.Error("HashRefreshToken does not reproduce the stored hash")
	}
	other, _, _ := NewRefreshToken()
	if HashRefreshToken(other) == hash {
		t.Error("two different tokens hashed the same")
	}

	second, _, _ := NewRefreshToken()
	if second == token {
		t.Error("two calls produced the same token")
	}
}

func TestNewTokenID(t *testing.T) {
	first, err := NewTokenID()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 {
		t.Errorf("jti length = %d, want 32 hex characters", len(first))
	}

	second, _ := NewTokenID()
	if second == first {
		t.Error("two calls produced the same jti")
	}
}
