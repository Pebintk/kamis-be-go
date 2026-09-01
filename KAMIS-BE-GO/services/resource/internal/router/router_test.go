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

// newTestEngine builds the real route table with a nil handler: these tests
// exercise routing and the auth rules in front of the handlers only.
func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return New(testVerifier(t), nil, nil)
}

// TestRoutesRegister pins the route table, and guards against a gin conflict
// between the static segments (update, addToDb, add-supplier, update-supplier)
// and the :idResource wildcard that sits beside them in the PUT tree.
func TestRoutesRegister(t *testing.T) {
	engine := newTestEngine(t)

	want := map[string]bool{
		"GET /api/resource/viewall":                          true,
		"GET /api/resource/viewall/paginated":                true,
		"GET /api/resource/find/:idResource":                 true,
		"GET /api/resource/find-by-supplier/:idSupplier":     true,
		"GET /api/resource/find-by-stock/:stock":             true,
		"POST /api/resource/add":                             true,
		"PUT /api/resource/update/:idResource":               true,
		"PUT /api/resource/addToDb/:idResource/:stockUpdate": true,
		"PUT /api/resource/add-supplier":                     true,
		"PUT /api/resource/update-supplier":                  true,
		"PUT /api/resource/:idResource/add-stock":            true,
		"PUT /api/resource/:idResource/deduct-stock":         true,
	}
	for _, route := range engine.Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	for missing := range want {
		t.Errorf("route not registered: %s", missing)
	}
}

// TestTokenRequirement asserts that every /api route needs a token. Unlike
// profile, this service has no public API routes at all.
func TestTokenRequirement(t *testing.T) {
	engine := newTestEngine(t)

	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/resource/viewall"},
		{http.MethodGet, "/api/resource/viewall/paginated"},
		{http.MethodGet, "/api/resource/find/1"},
		{http.MethodGet, "/api/resource/find-by-supplier/abc"},
		{http.MethodGet, "/api/resource/find-by-stock/5"},
		{http.MethodPost, "/api/resource/add"},
		{http.MethodPut, "/api/resource/update/1"},
		{http.MethodPut, "/api/resource/addToDb/1/5"},
		{http.MethodPut, "/api/resource/add-supplier"},
		{http.MethodPut, "/api/resource/update-supplier"},
		{http.MethodPut, "/api/resource/1/add-stock"},
		{http.MethodPut, "/api/resource/1/deduct-stock"},
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
