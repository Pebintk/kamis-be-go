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
