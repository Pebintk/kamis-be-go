package router

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pebintk/kamis-be-go/pkg/auth"
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

// signedToken mints a token carrying one role, as profile would issue it.
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
		{http.MethodPost, "/api/asset/maintenance"},
		{http.MethodGet, "/api/asset/maintenance/all"},
		{http.MethodGet, "/api/asset/maintenance/in-progress"},
		{http.MethodPatch, "/api/asset/maintenance/7/complete"},
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

// TestEverythingIsUnderAsset is the point of the prefix standardization: one
// path prefix means one auth rule, so no route can quietly fall outside it the
// way /api/maintenance/** did in the legacy WebSecurityConfig.
func TestEverythingIsUnderAsset(t *testing.T) {
	for _, route := range newTestEngine(t).Routes() {
		// /health and /metrics are the deliberate public routes: liveness and the
		// Prometheus scrape endpoint. Both sit outside the auth prefix on purpose
		// (Prometheus scrapes /metrics unauthenticated from inside the cluster),
		// so they are exempt from the "everything under /api/asset/" rule.
		if route.Path == "/health" || route.Path == "/metrics" {
			continue
		}
		if !strings.HasPrefix(route.Path, "/api/asset/") {
			t.Errorf("%s %s is outside the /api/asset/ prefix", route.Method, route.Path)
		}
	}
}

// TestMaintenanceWritesAreRestricted pins the guard the legacy config never
// applied: booking and completing a job are writes, so Finance and Direksi are
// refused, matching every other write in this service and what the frontend's
// canEditAsset already assumed.
func TestMaintenanceWritesAreRestricted(t *testing.T) {
	engine := newTestEngine(t)

	writes := []struct{ method, path string }{
		{http.MethodPost, "/api/asset/maintenance"},
		{http.MethodPatch, "/api/asset/maintenance/7/complete"},
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
}

// TestMaintenanceReadsStayOpen is the other half: the operational dashboard
// reads the in-progress list, and that route is reachable by every role.
func TestMaintenanceReadsStayOpen(t *testing.T) {
	engine := newTestEngine(t)

	for _, role := range []string{"Admin", "Direksi", "Finance", "Operasional"} {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/asset/maintenance/in-progress", nil)
		req.Header.Set("Authorization", "Bearer "+signedToken(t, role))
		func() {
			// The handler is nil in these tests, so reaching it panics — which is
			// itself the proof the role guard let the request through.
			defer func() { _ = recover() }()
			engine.ServeHTTP(res, req)
		}()

		if res.Code == http.StatusForbidden {
			t.Errorf("GET in-progress as %s: got 403, want the read to be allowed", role)
		}
	}
}
