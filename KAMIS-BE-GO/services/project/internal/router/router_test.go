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
	"github.com/karina/kamis-be-go/pkg/auth"
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

// TestRoutesRegister pins the route table and guards against a gin conflict
// between the static `all` segment and the :id wildcard beside it.
func TestRoutesRegister(t *testing.T) {
	want := map[string]bool{
		"POST /api/project/add":          true,
		"GET /api/project/all":           true,
		"GET /api/project/all/paginated": true,
		"GET /api/project/:id":           true,
	}
	for _, route := range newTestEngine(t).Routes() {
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
		{http.MethodPost, "/api/project/add"},
		{http.MethodGet, "/api/project/all"},
		{http.MethodGet, "/api/project/all/paginated"},
		{http.MethodGet, "/api/project/D001260903"},
		{http.MethodPut, "/api/project/update/D001260903"},
		{http.MethodPut, "/api/project/update-status/D001260903"},
		{http.MethodPut, "/api/project/update-payment/D001260903"},
	}
	for _, tc := range cases {
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401 without a token", tc.method, tc.path, res.Code)
		}
	}
}

// TestCreateIsOperasionalOnly pins the narrowest write rule in the whole port:
// unlike every other service, creating a project excludes Admin.
func TestCreateIsOperasionalOnly(t *testing.T) {
	engine := newTestEngine(t)

	for _, role := range []string{"Admin", "Direksi", "Finance"} {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/project/add", nil)
		req.Header.Set("Authorization", "Bearer "+signedToken(t, role))
		engine.ServeHTTP(res, req)

		if res.Code != http.StatusForbidden {
			t.Errorf("POST /add as %s: got %d, want 403", role, res.Code)
		}
	}

	// Operasional gets through to the (nil) handler, which panics — itself the
	// proof the guard allowed it.
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/project/add", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken(t, "Operasional"))
	func() {
		defer func() { _ = recover() }()
		engine.ServeHTTP(res, req)
	}()
	if res.Code == http.StatusForbidden {
		t.Error("POST /add as Operasional: got 403, want the write to be allowed")
	}
}

// TestReadsAreOpenToEveryRole covers the /api/project/** catch-all.
func TestReadsAreOpenToEveryRole(t *testing.T) {
	engine := newTestEngine(t)

	for _, role := range []string{"Admin", "Direksi", "Finance", "Operasional"} {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/project/all", nil)
		req.Header.Set("Authorization", "Bearer "+signedToken(t, role))
		func() {
			defer func() { _ = recover() }()
			engine.ServeHTTP(res, req)
		}()
		if res.Code == http.StatusForbidden {
			t.Errorf("GET /all as %s: got 403, want the read to be allowed", role)
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

// TestWriteRolesAreDistinct pins the three different write rules this service
// has, which no other service in the port splits this finely.
func TestWriteRolesAreDistinct(t *testing.T) {
	engine := newTestEngine(t)

	cases := []struct {
		path    string
		allowed []string
	}{
		{"/api/project/update/D001260903", []string{"Operasional", "Direksi"}},
		{"/api/project/update-status/D001260903", []string{"Operasional"}},
		{"/api/project/update-payment/D001260903", []string{"Finance"}},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			for _, role := range []string{"Admin", "Direksi", "Finance", "Operasional"} {
				res := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPut, tc.path, nil)
				req.Header.Set("Authorization", "Bearer "+signedToken(t, role))
				func() {
					// An allowed role reaches the nil handler and panics, which
					// is the proof the guard let it through.
					defer func() { _ = recover() }()
					engine.ServeHTTP(res, req)
				}()

				permitted := slices.Contains(tc.allowed, role)
				if permitted && res.Code == http.StatusForbidden {
					t.Errorf("%s as %s: got 403, want allowed", tc.path, role)
				}
				if !permitted && res.Code != http.StatusForbidden {
					t.Errorf("%s as %s: got %d, want 403", tc.path, role, res.Code)
				}
			}
		})
	}
}
