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
// login and profile registration are; everything else must answer 401. The
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
		{http.MethodPost, "/api/profile/add", true},
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
