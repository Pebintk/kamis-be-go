package router

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
)

// testKeys generates one RSA pair for the whole test binary: the verifier gets
// the public half, signedToken signs with the private half, so role guards can
// be exercised with a real token rather than only the no-token case.
var testKeys = sync.OnceValues(func() (string, string) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		panic(err)
	}
	priv, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv)
})

func testVerifier(t *testing.T) *auth.Verifier {
	t.Helper()
	pub, _ := testKeys()
	v, err := auth.NewVerifier(pub)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func signedToken(t *testing.T, role string) string {
	t.Helper()
	_, priv := testKeys()
	issuer, err := auth.NewIssuer(priv, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.Generate("tester", role)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return New(testVerifier(t), nil, nil, nil)
}

// TestRoutesRegister pins the route table and guards against a gin conflict
// between the static segments and the :purchaseId wildcard beside them.
func TestRoutesRegister(t *testing.T) {
	engine := newTestEngine(t)

	want := map[string]bool{
		"POST /api/purchase/add":                 true,
		"PUT /api/purchase/update/:purchaseId":   true,
		"GET /api/purchase/viewall":              true,
		"GET /api/purchase/viewall/paginated":    true,
		"GET /api/purchase/detail/:purchaseId":   true,
		"GET /api/purchase/supplier/:supplierId": true,
	}
	for _, route := range engine.Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	for missing := range want {
		t.Errorf("route not registered: %s", missing)
	}
}

// TestTokenRequirement asserts every /api route needs a token.
func TestTokenRequirement(t *testing.T) {
	engine := newTestEngine(t)

	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/purchase/add"},
		{http.MethodPut, "/api/purchase/update/R-030926-001"},
		{http.MethodGet, "/api/purchase/viewall"},
		{http.MethodGet, "/api/purchase/viewall/paginated"},
		{http.MethodGet, "/api/purchase/detail/R-030926-001"},
		{http.MethodGet, "/api/purchase/supplier/abc"},
		{http.MethodPost, "/api/purchase/addAsset"},
		{http.MethodGet, "/api/purchase/asset/7"},
		{http.MethodGet, "/api/purchase/asset/7/foto"},
		{http.MethodPut, "/api/purchase/updatestatus/next/R-030926-001"},
		{http.MethodPut, "/api/purchase/updatestatus/cancel/R-030926-001"},
		{http.MethodPut, "/api/purchase/updatestatus/pembayaran/R-030926-001"},
		{http.MethodGet, "/api/purchase/chart/purchase-activity"},
		{http.MethodGet, "/api/purchase/range"},
		{http.MethodGet, "/api/purchase/summary"},
	}

	for _, tc := range cases {
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401 without a token", tc.method, tc.path, res.Code)
		}
	}
}

// TestWritesAreRestricted pins the legacy rule: creating and editing a purchase
// is Operasional/Admin, while every read is open to all four roles.
func TestWritesAreRestricted(t *testing.T) {
	engine := newTestEngine(t)

	writes := []struct{ method, path string }{
		{http.MethodPost, "/api/purchase/add"},
		{http.MethodPut, "/api/purchase/update/R-030926-001"},
		{http.MethodPost, "/api/purchase/addAsset"},
	}
	for _, tc := range writes {
		for _, role := range []string{"Finance", "Direksi"} {
			res := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+signedToken(t, role))
			engine.ServeHTTP(res, req)

			if res.Code != http.StatusForbidden {
				t.Errorf("%s %s as %s: got %d, want 403", tc.method, tc.path, role, res.Code)
			}
		}
	}

	for _, role := range []string{"Admin", "Direksi", "Finance", "Operasional"} {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/purchase/viewall", nil)
		req.Header.Set("Authorization", "Bearer "+signedToken(t, role))
		func() {
			// The handler is nil here, so reaching it panics — which is itself
			// the proof the role guard let the request through.
			defer func() { _ = recover() }()
			engine.ServeHTTP(res, req)
		}()
		if res.Code == http.StatusForbidden {
			t.Errorf("GET viewall as %s: got 403, want the read to be allowed", role)
		}
	}
}

// TestHealthIsPublic keeps the liveness probe reachable.
func TestHealthIsPublic(t *testing.T) {
	res := httptest.NewRecorder()
	newTestEngine(t).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/health", nil))
	if res.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", res.Code)
	}
}
