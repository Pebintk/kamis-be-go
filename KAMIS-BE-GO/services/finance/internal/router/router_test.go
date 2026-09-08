package router

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pebintk/kamis-be-go/pkg/auth"
)

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
	return New(testVerifier(t), nil, nil)
}

func TestRoutesRegister(t *testing.T) {
	want := map[string]bool{
		"GET /api/lapkeu/all":     true,
		"GET /api/lapkeu/summary": true,
		"GET /api/lapkeu/page":    true,
		"POST /api/lapkeu/add":    true,
		"DELETE /api/lapkeu/:id":  true,
	}
	for _, route := range newTestEngine(t).Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	for missing := range want {
		t.Errorf("route not registered: %s", missing)
	}
}

// TestLedgerRequiresAToken is the regression guard for the worst hole in the
// legacy stack: WebSecurityConfig declared `/api/lapkeu/** permitAll()`, so the
// entire financial ledger could be read, written and deleted with no
// credentials at all. Every route here must answer 401 without a token.
func TestLedgerRequiresAToken(t *testing.T) {
	engine := newTestEngine(t)

	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/lapkeu/all"},
		{http.MethodGet, "/api/lapkeu/summary"},
		{http.MethodGet, "/api/lapkeu/page"},
		{http.MethodPost, "/api/lapkeu/add"},
		{http.MethodDelete, "/api/lapkeu/P001260903"},
		{http.MethodGet, "/api/lapkeu/chart-pengeluaran"},
		{http.MethodGet, "/api/lapkeu/chart-pemasukan-pengeluaran"},
		{http.MethodGet, "/api/lapkeu/chart-total-pemasukan-pengeluaran"},
		{http.MethodGet, "/api/finance-report/summary"},
		{http.MethodGet, "/api/operational-report/activity-chart"},
	}

	for _, tc := range cases {
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401 without a token", tc.method, tc.path, res.Code)
		}
	}
}

// TestLedgerRoles pins who may do what. Reads match the two dashboards that
// consume this data; deletes are narrower still, since only a Finance refund
// removes an entry.
func TestLedgerRoles(t *testing.T) {
	engine := newTestEngine(t)

	cases := []struct {
		method, path string
		allowed      []string
	}{
		{http.MethodGet, "/api/lapkeu/all", []string{"Admin", "Finance", "Direksi"}},
		{http.MethodGet, "/api/lapkeu/summary", []string{"Admin", "Finance", "Direksi"}},
		{http.MethodGet, "/api/lapkeu/page", []string{"Admin", "Finance", "Direksi"}},
		// Writes arrive from another service forwarding the token of whoever
		// triggered the flow, and Operasional completes purchases and projects.
		{http.MethodPost, "/api/lapkeu/add", []string{"Admin", "Finance", "Direksi", "Operasional"}},
		{http.MethodDelete, "/api/lapkeu/P001260903", []string{"Finance", "Admin"}},
		{http.MethodGet, "/api/lapkeu/chart-pengeluaran", []string{"Admin", "Finance", "Direksi"}},
		{http.MethodGet, "/api/finance-report/summary", []string{"Admin", "Finance", "Direksi"}},
		// The operational dashboard reads this one, so Operasional is included.
		{http.MethodGet, "/api/operational-report/activity-chart",
			[]string{"Admin", "Finance", "Direksi", "Operasional"}},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			for _, role := range []string{"Admin", "Direksi", "Finance", "Operasional"} {
				res := httptest.NewRecorder()
				req := httptest.NewRequest(tc.method, tc.path, nil)
				req.Header.Set("Authorization", "Bearer "+signedToken(t, role))
				func() {
					// An allowed role reaches the nil handler and panics, which
					// is the proof the guard let it through.
					defer func() { _ = recover() }()
					engine.ServeHTTP(res, req)
				}()

				permitted := slices.Contains(tc.allowed, role)
				if permitted && res.Code == http.StatusForbidden {
					t.Errorf("as %s: got 403, want allowed", role)
				}
				if !permitted && res.Code != http.StatusForbidden {
					t.Errorf("as %s: got %d, want 403", role, res.Code)
				}
			}
		})
	}
}

// TestOperasionalCannotReadTheBooks states the read rule as its own case,
// because it is the one role deliberately excluded.
func TestOperasionalCannotReadTheBooks(t *testing.T) {
	engine := newTestEngine(t)

	for _, path := range []string{"/api/lapkeu/all", "/api/lapkeu/summary", "/api/lapkeu/page"} {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+signedToken(t, "Operasional"))
		engine.ServeHTTP(res, req)

		if res.Code != http.StatusForbidden {
			t.Errorf("GET %s as Operasional: got %d, want 403", path, res.Code)
		}
	}
}

func TestHealthIsPublic(t *testing.T) {
	res := httptest.NewRecorder()
	newTestEngine(t).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/health", nil))
	if res.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", res.Code)
	}
}
