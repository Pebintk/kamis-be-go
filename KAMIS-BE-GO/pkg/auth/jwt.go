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
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidCredentials is returned by login flows; kept here so the message is
// uniform and never reveals whether the username or the password was wrong.
var ErrInvalidCredentials = errors.New("invalid username or password")

// Claims carries the legacy shape plus the multi-role set.
//
// `role` is the primary role and keeps its original meaning, so anything that
// already reads it — the Vue router's guards, the dashboard redirect — is
// unaffected. `roles` is every role the account holds, primary included, and is
// what the route guards check. A token minted before `roles` existed still
// authorises correctly: see HasAnyRole.
type Claims struct {
	Role  string   `json:"role"`
	Roles []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

// Authorities is every role this token carries.
//
// It falls back to the single `role` claim when `roles` is absent, which is what
// keeps a token issued before multi-role existed working until it expires.
func (c Claims) Authorities() []string {
	if len(c.Roles) > 0 {
		return c.Roles
	}
	if c.Role == "" {
		return nil
	}
	return []string{c.Role}
}

// HasAnyRole reports whether this token holds at least one of the allowed
// roles — the Go form of Spring's hasAnyAuthority(...).
func (c Claims) HasAnyRole(allowed ...string) bool {
	for _, held := range c.Authorities() {
		if slices.Contains(allowed, held) {
			return true
		}
	}
	return false
}

// ---- Verifier: held by every service, validates with the RSA public keys ----

// Verifier holds every public key a token may have been signed with.
//
// More than one, because rotating a keypair otherwise needs every service
// redeployed at the same instant as profile: the moment profile signs with a new
// key, any service still holding only the old one rejects every request. With a
// list, a rotation is three ordered steps that never overlap —
//
//  1. add the new public key to every service and redeploy them; they now accept
//     tokens signed with either key, and profile still signs with the old one
//  2. redeploy profile signing with the new key; tokens already issued keep
//     verifying against the old one, which is still in the list
//  3. once the longest-lived token has expired, drop the old key
//
// This is the cheap half of a JWKS endpoint. It does not give profile a way to
// publish keys, so services still learn them from configuration, but it removes
// the simultaneity requirement, which is the part that makes rotation risky.
type Verifier struct{ publicKeys []*rsa.PublicKey }

// NewVerifier decodes one base64 X509 public key.
func NewVerifier(base64PublicKey string) (*Verifier, error) {
	return NewVerifierFromKeys(base64PublicKey)
}

// NewVerifierFromKeys decodes one or more base64 X509 public keys. A token is
// accepted if any of them verifies it.
func NewVerifierFromKeys(base64PublicKeys ...string) (*Verifier, error) {
	keys := make([]*rsa.PublicKey, 0, len(base64PublicKeys))
	for _, encoded := range base64PublicKeys {
		encoded = strings.TrimSpace(encoded)
		if encoded == "" {
			continue
		}
		key, err := parsePublicKey(encoded)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, errors.New("no public key configured")
	}
	return &Verifier{publicKeys: keys}, nil
}

func parsePublicKey(base64PublicKey string) (*rsa.PublicKey, error) {
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
	return rsaPub, nil
}

// Keys is how many public keys this verifier accepts. More than one means a
// rotation is in progress.
func (v *Verifier) Keys() int { return len(v.publicKeys) }

// Parse verifies signature and expiry against each configured key in turn, and
// pins the algorithm to RS256.
//
// The error returned is the *first* key's, not the last: with one key that is
// the only error there is, and with several the first is the current key, whose
// failure is the one worth reporting.
func (v *Verifier) Parse(token string) (*Claims, error) {
	var firstErr error
	for _, key := range v.publicKeys {
		claims := &Claims{}
		_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
			return key, nil
		}, jwt.WithValidMethods([]string{"RS256"}))
		if err == nil {
			return claims, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
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
	return i.GenerateMulti(username, []string{role})
}

// GenerateMulti mints an access token for an account holding several roles. The
// first is the primary one, carried in the legacy `role` claim.
func (i *Issuer) GenerateMulti(username string, roles []string) (string, error) {
	if len(roles) == 0 {
		return "", errors.New("cannot mint a token with no roles")
	}
	now := time.Now()
	id, err := NewTokenID()
	if err != nil {
		return "", err
	}
	claims := Claims{
		Role:  roles[0],
		Roles: roles,
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
