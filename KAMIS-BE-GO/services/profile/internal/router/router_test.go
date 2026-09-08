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

// signedToken mints a token carrying one role.
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

// newTestEngine builds the real route table. The handlers are nil: these tests
// only exercise routing and the auth rules in front of the handlers, and a
// request that reached a handler would panic loudly rather than pass silently.
func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return New(testVerifier(t), nil, Handlers{})
}

// TestRoutesRegister guards against a gin routing conflict between the static
// segments and the :id wildcards under /api/client.
func TestRoutesRegister(t *testing.T) {
	engine := newTestEngine(t)

	want := map[string]string{
		"POST /api/auth/login":                 "",
		"POST /api/auth/refresh":               "",
		"POST /api/auth/logout":                "",
		"POST /api/profile/add":                "",
		"GET /api/profile/all":                 "",
		"GET /api/profile/all/paginated":       "",
		"PUT /api/profile/:id":                 "",
		"GET /api/client/all":                  "",
		"POST /api/client/add":                 "",
		"GET /api/client/all/paginated":        "",
		"GET /api/client/:id":                  "",
		"PUT /api/client/update/:id":           "",
		"POST /api/supplier/add":               "",
		"PUT /api/supplier/update":             "",
		"PUT /api/supplier/add-purchase":       "",
		"GET /api/supplier/all":                "",
		"GET /api/supplier/all/paginated":      "",
		"GET /api/supplier/getall":             "",
		"GET /api/supplier/name/:supplierId":   "",
		"GET /api/supplier/detail/:supplierId": "",
	}
	for _, route := range engine.Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	for missing := range want {
		t.Errorf("route not registered: %s", missing)
	}
}

// TestTokenRequirement pins which routes are reachable without a token. Only
// login, refresh and logout are — the last two authenticate by the refresh
// token in their body — and everything else must answer 401. The
// three /api/client routes below were world-accessible in the legacy
// WebSecurityConfig (no /api/client/** catch-all) and are explicitly guarded
// here, so this test is what keeps them from regressing.
func TestTokenRequirement(t *testing.T) {
	engine := newTestEngine(t)

	cases := []struct {
		method, path string
		public       bool
	}{
		{http.MethodPost, "/api/auth/login", true},
		{http.MethodPost, "/api/auth/refresh", true},
		{http.MethodPost, "/api/auth/logout", true},
		{http.MethodPost, "/api/profile/add", false},
		{http.MethodGet, "/api/client/all/paginated", false},
		{http.MethodGet, "/api/client/abc-123", false},
		{http.MethodPut, "/api/client/update/abc-123", false},
		{http.MethodGet, "/api/client/all", false},
		{http.MethodPost, "/api/client/add", false},
		{http.MethodPost, "/api/supplier/add", false},
		{http.MethodPut, "/api/supplier/update", false},
		{http.MethodPut, "/api/supplier/add-purchase", false},
		{http.MethodGet, "/api/supplier/all", false},
		{http.MethodGet, "/api/supplier/getall", false},
		{http.MethodGet, "/api/supplier/detail/abc-123", false},
		{http.MethodGet, "/api/supplier/name/abc-123", false},
	}

	for _, tc := range cases {
		res := httptest.NewRecorder()
		func() {
			// A public route reaches its (nil) handler and panics; that panic is
			// itself the proof no auth middleware rejected the request first.
			defer func() { _ = recover() }()
			engine.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		}()

		unauthorized := res.Code == http.StatusUnauthorized
		if tc.public && unauthorized {
			t.Errorf("%s %s: got 401, want public", tc.method, tc.path)
		}
		if !tc.public && !unauthorized {
			t.Errorf("%s %s: got %d, want 401 without a token", tc.method, tc.path, res.Code)
		}
	}
}

// TestAccountCreationIsAdminOnly is the regression guard for the registration
// hole. The legacy config left POST /profile/add public, so anyone could mint an
// account with any role — including Admin — without credentials. The frontend
// always treated it as an admin screen: /account/add is roles: ["Admin"].
func TestAccountCreationIsAdminOnly(t *testing.T) {
	engine := newTestEngine(t)

	for _, role := range []string{"Direksi", "Finance", "Operasional"} {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/profile/add", nil)
		req.Header.Set("Authorization", "Bearer "+signedToken(t, role))
		engine.ServeHTTP(res, req)

		if res.Code != http.StatusForbidden {
			t.Errorf("POST /profile/add as %s: got %d, want 403", role, res.Code)
		}
	}

	// Admin reaches the (nil) handler and panics, which is the proof the guard
	// let it through.
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/profile/add", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken(t, "Admin"))
	func() {
		defer func() { _ = recover() }()
		engine.ServeHTTP(res, req)
	}()
	if res.Code == http.StatusForbidden {
		t.Error("POST /profile/add as Admin: got 403, want allowed")
	}
}
