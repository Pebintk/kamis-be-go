package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"sync"
	"testing"
)

// testKeyPair returns one RSA pair for the whole test binary, base64-encoded in
// the same formats JWT_SECRET_KEY and JWT_PUBLIC_KEY carry.
var testKeyPair = func() func(*testing.T) (priv, pub string) {
	once := sync.OnceValues(func() (string, string) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		privDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			panic(err)
		}
		pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		if err != nil {
			panic(err)
		}
		return base64.StdEncoding.EncodeToString(privDER),
			base64.StdEncoding.EncodeToString(pubDER)
	})
	return func(t *testing.T) (string, string) {
		t.Helper()
		return once()
	}
}()
