package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"testing"
	"time"
)

// genKeysB64 produces a base64 PKCS8 private key and base64 X509 public key,
// matching the formats stored in JWT_SECRET_KEY / JWT_PUBLIC_KEY.
func genKeysB64(t *testing.T) (privB64, pubB64 string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(privDER), base64.StdEncoding.EncodeToString(pubDER)
}

func TestIssueThenVerify(t *testing.T) {
	privB64, pubB64 := genKeysB64(t)

	issuer, err := NewIssuer(privB64, time.Hour)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	verifier, err := NewVerifier(pubB64)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	token, err := issuer.Generate("alice", "Operasional")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	claims, err := verifier.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.Subject != "alice" {
		t.Errorf("subject = %q, want alice", claims.Subject)
	}
	if claims.Role != "Operasional" {
		t.Errorf("role = %q, want Operasional", claims.Role)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	privB64, pubB64 := genKeysB64(t)
	issuer, _ := NewIssuer(privB64, -time.Minute) // already expired
	verifier, _ := NewVerifier(pubB64)

	token, err := issuer.Generate("bob", "Admin")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	privB64, _ := genKeysB64(t)
	_, otherPubB64 := genKeysB64(t) // public key from a different pair

	issuer, _ := NewIssuer(privB64, time.Hour)
	verifier, _ := NewVerifier(otherPubB64)

	token, _ := issuer.Generate("eve", "Admin")
	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("expected token signed by a different key to be rejected")
	}
}

// TestGenerateCarriesJTI keeps every access token individually nameable, which
// is what lets one be identified in a log or an audit trail.
func TestGenerateCarriesJTI(t *testing.T) {
	privB64, pubB64 := genKeysB64(t)
	issuer, _ := NewIssuer(privB64, time.Hour)
	verifier, _ := NewVerifier(pubB64)

	first, err := issuer.Generate("tester", "Admin")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := verifier.Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	if claims.ID == "" {
		t.Fatal("access token carries no jti")
	}

	second, _ := issuer.Generate("tester", "Admin")
	secondClaims, _ := verifier.Parse(second)
	if secondClaims.ID == claims.ID {
		t.Error("two tokens share a jti")
	}
}

// TestHasAnyRole covers the guard's decision, including the fallback that keeps
// a token minted before `roles` existed working until it expires.
func TestHasAnyRole(t *testing.T) {
	multi := Claims{Role: "Finance", Roles: []string{"Finance", "Operasional"}}
	if !multi.HasAnyRole("Operasional") {
		t.Error("a secondary role does not authorise")
	}
	if !multi.HasAnyRole("Finance") {
		t.Error("the primary role does not authorise")
	}
	if !multi.HasAnyRole("Admin", "Operasional") {
		t.Error("holding one of several allowed roles does not authorise")
	}
	if multi.HasAnyRole("Admin") {
		t.Error("a role the account does not hold authorises")
	}

	// A token issued before multi-role: only `role` is set, and it must still
	// authorise on that role alone.
	legacy := Claims{Role: "Admin"}
	if !legacy.HasAnyRole("Admin") {
		t.Error("a single-role token no longer authorises")
	}
	if legacy.HasAnyRole("Finance") {
		t.Error("a single-role token authorises on a role it does not hold")
	}

	var empty Claims
	if empty.HasAnyRole("Admin") {
		t.Error("a token with no role at all authorises")
	}
	if got := empty.Authorities(); got != nil {
		t.Errorf("Authorities on an empty token = %v, want nil", got)
	}
}

// TestGenerateMulti pins the wire shape: `role` stays the primary one so the
// frontend's existing reads are unaffected, and `roles` carries the whole set.
func TestGenerateMulti(t *testing.T) {
	privB64, pubB64 := genKeysB64(t)
	issuer, _ := NewIssuer(privB64, time.Hour)
	verifier, _ := NewVerifier(pubB64)

	token, err := issuer.GenerateMulti("tester", []string{"Finance", "Operasional"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := verifier.Parse(token)
	if err != nil {
		t.Fatal(err)
	}

	if claims.Role != "Finance" {
		t.Errorf("role claim = %q, want the primary role Finance", claims.Role)
	}
	if len(claims.Roles) != 2 || claims.Roles[1] != "Operasional" {
		t.Errorf("roles claim = %v, want both roles in order", claims.Roles)
	}

	// Generate stays the single-role form and must still produce a usable token.
	single, err := issuer.Generate("tester", "Admin")
	if err != nil {
		t.Fatal(err)
	}
	singleClaims, _ := verifier.Parse(single)
	if singleClaims.Role != "Admin" || !singleClaims.HasAnyRole("Admin") {
		t.Errorf("single-role token = %+v", singleClaims)
	}

	if _, err := issuer.GenerateMulti("tester", nil); err == nil {
		t.Error("minting a token with no roles returned nil error")
	}
}
