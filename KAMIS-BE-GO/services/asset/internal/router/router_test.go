package router

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
)

func testVerifier(t *testing.T) *auth.Verifier {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	v, err := auth.NewVerifier(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return New(testVerifier(t), nil, nil, nil, nil)
}

// TestRoutesRegister pins the route table and guards against a gin conflict
// between the static segments (all, addAsset, by-supplier, viewall) and the
// :platNomor wildcard that sits beside them.
func TestRoutesRegister(t *testing.T) {
	engine := newTestEngine(t)

	want := map[string]bool{
		"GET /api/asset/all":                     true,
		"GET /api/asset/viewall/paginated":       true,
		"GET /api/asset/by-supplier/:supplierId": true,
		"GET /api/asset/:platNomor":              true,
		"GET /api/asset/:platNomor/foto":         true,
		"GET /api/asset/:platNomor/maintenance":  true,
		"POST /api/asset/addAsset":               true,
		"PUT /api/asset/:platNomor":              true,
		"PUT /api/asset/:platNomor/supplier":     true,
		"DELETE /api/asset/:platNomor":           true,
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
		{http.MethodGet, "/api/asset/all"},
		{http.MethodGet, "/api/asset/viewall/paginated"},
		{http.MethodGet, "/api/asset/by-supplier/abc"},
		{http.MethodGet, "/api/asset/B1234XYZ"},
		{http.MethodGet, "/api/asset/B1234XYZ/foto"},
		{http.MethodGet, "/api/asset/B1234XYZ/maintenance"},
		{http.MethodPost, "/api/asset/addAsset"},
		{http.MethodPut, "/api/asset/B1234XYZ"},
		{http.MethodPut, "/api/asset/B1234XYZ/supplier"},
		{http.MethodDelete, "/api/asset/B1234XYZ"},
		{http.MethodPost, "/api/maintenance/"},
		{http.MethodGet, "/api/maintenance/all"},
		{http.MethodGet, "/api/maintenance/maintenance-in-progress"},
		{http.MethodGet, "/api/maintenance/B1234XYZ"},
		{http.MethodPatch, "/api/maintenance/7/complete"},
		{http.MethodPost, "/api/asset/reservations/check-availability"},
		{http.MethodPost, "/api/asset/reservations/reserve"},
		{http.MethodPut, "/api/asset/reservations/project/PRJ-1/status"},
		{http.MethodPut, "/api/asset/reservations/abc/status"},
		{http.MethodGet, "/api/asset/reservations/asset/B1234XYZ"},
		{http.MethodGet, "/api/asset/reservations/project/PRJ-1"},
	}

	for _, tc := range cases {
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401 without a token", tc.method, tc.path, res.Code)
		}
	}
}

// TestHealthIsPublic keeps the liveness probe reachable — Java's
// securityMatcher("/api/**") left everything outside /api unsecured.
func TestHealthIsPublic(t *testing.T) {
	res := httptest.NewRecorder()
	newTestEngine(t).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/health", nil))
	if res.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", res.Code)
	}
}
