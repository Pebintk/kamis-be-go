package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// refreshTokenBytes is the entropy behind a refresh token. 32 bytes is the same
// order as the RSA key's practical strength and far past guessing.
const refreshTokenBytes = 32

// NewRefreshToken mints an opaque refresh token and the hash to store for it.
//
// The token is a bearer credential with a long life, so it is stored the way a
// password is: only the SHA-256 hash reaches the database. A leaked database
// therefore yields nothing that can be presented. SHA-256 rather than bcrypt is
// right here because the input is 256 bits of entropy we generated, not a
// human-chosen secret — there is nothing to brute-force, and the lookup happens
// on every refresh.
func NewRefreshToken() (token, hash string, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken is how a presented token is looked up.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewTokenID mints the jti carried by an access token.
func NewTokenID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
