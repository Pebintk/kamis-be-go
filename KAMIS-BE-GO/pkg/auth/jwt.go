// Package auth provides JWT issuance/verification shared by every KAMIS service.
//
// It is framework-agnostic: the token logic here has no dependency on Gin, so it
// can be unit-tested and reused. Gin adapters live in middleware.go.
//
// Tokens are RS256 with the claim shape inherited from the Java services
// (subject = username, single "role" claim), so tokens minted by the legacy
// Spring `profile` service and by Go services are interchangeable during the
// migration. The env key formats are reused as-is:
//   - JWT_PUBLIC_KEY  : base64 X509 / SubjectPublicKeyInfo  (every service)
//   - JWT_SECRET_KEY  : base64 PKCS8 private key            (profile only)
package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidCredentials is returned by login flows; kept here so the message is
// uniform and never reveals whether the username or the password was wrong.
var ErrInvalidCredentials = errors.New("invalid username or password")

// Claims mirrors the legacy token exactly.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// ---- Verifier: held by every service, validates with the RSA public key ----

type Verifier struct{ publicKey *rsa.PublicKey }

// NewVerifier decodes the value of JWT_PUBLIC_KEY (base64 X509) unchanged.
func NewVerifier(base64PublicKey string) (*Verifier, error) {
	der, err := base64.StdEncoding.DecodeString(base64PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}
	return &Verifier{publicKey: rsaPub}, nil
}

// Parse verifies signature + expiry and pins the algorithm to RS256.
func (v *Verifier) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return v.publicKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// ---- Issuer: held ONLY by profile, signs with the RSA private key ----

type Issuer struct {
	privateKey *rsa.PrivateKey
	ttl        time.Duration
}

// NewIssuer decodes the value of JWT_SECRET_KEY (base64 PKCS8) unchanged.
func NewIssuer(base64PrivateKey string, ttl time.Duration) (*Issuer, error) {
	der, err := base64.StdEncoding.DecodeString(base64PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return &Issuer{privateKey: rsaKey, ttl: ttl}, nil
}

// Generate mints an access token. Beyond the legacy claim shape it carries a
// jti, so a specific token can be named in a log or an audit trail; the token
// itself stays self-contained, and no service calls back to profile to validate
// one.
func (i *Issuer) Generate(username, role string) (string, error) {
	now := time.Now()
	id, err := NewTokenID()
	if err != nil {
		return "", err
	}
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        id,
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(i.privateKey)
}

// TTL is how long the access tokens this issuer mints stay valid. Callers need
// it to tell a client when to refresh.
func (i *Issuer) TTL() time.Duration { return i.ttl }

// ---- request-context plumbing ----

type ctxKey struct{}

// FromContext returns the claims placed by the auth middleware, if any.
func FromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(*Claims)
	return c, ok
}
