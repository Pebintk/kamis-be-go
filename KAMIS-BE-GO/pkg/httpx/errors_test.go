package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/apierr"
)

// TestRespondErrorStatuses pins the mapping every service's handlers rely on and
// the frontend branches on.
func TestRespondErrorStatuses(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"caller mistake", &apierr.Invalid{Message: "Stok barang tidak boleh kurang dari 0"}, http.StatusBadRequest},
		{"missing record", &apierr.NotFound{Message: "Resource dengan ID 7 tidak ditemukan."}, http.StatusNotFound},
		{"unexpected failure", errors.New("connection refused"), http.StatusInternalServerError},
		{"wrapped caller mistake", wrap(apierr.Invalidf("bad")), http.StatusBadRequest},
		{"wrapped missing record", wrap(apierr.NotFoundf("gone")), http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := respond(t, tc.err).Status; got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestRespondErrorHidesInternals keeps an unexpected failure from reaching the
// browser verbatim — a connection error carries the database DSN.
func TestRespondErrorHidesInternals(t *testing.T) {
	body := respond(t, errors.New(`dial tcp: postgres://kamis:hunter2@db:5432 refused`))

	if strings.Contains(body.Message, "hunter2") || strings.Contains(body.Message, "postgres://") {
		t.Errorf("message leaked the underlying error: %q", body.Message)
	}
	if body.Message == "" {
		t.Error("message is empty; the frontend renders it in a toast")
	}
}

// TestRespondErrorKeepsUserFacingMessages is the other half: the Indonesian
// messages for 400 and 404 are what the frontend shows, so they must survive.
func TestRespondErrorKeepsUserFacingMessages(t *testing.T) {
	const invalid = "Stock tidak mencukupi. Tersedia: 3, permintaan: 5"
	if got := respond(t, &apierr.Invalid{Message: invalid}).Message; got != invalid {
		t.Errorf("message = %q, want %q", got, invalid)
	}

	const missing = "Resource dengan ID 7 tidak ditemukan."
	if got := respond(t, &apierr.NotFound{Message: missing}).Message; got != missing {
		t.Errorf("message = %q, want %q", got, missing)
	}
}

type body struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func respond(t *testing.T, err error) body {
	t.Helper()
	gin.SetMode(gin.TestMode)

	res := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(res)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/resource/find/7", nil)
	RespondError(c, err)

	var out body
	if decodeErr := json.Unmarshal(res.Body.Bytes(), &out); decodeErr != nil {
		t.Fatalf("decode response: %v (body %s)", decodeErr, res.Body)
	}
	if res.Code != out.Status {
		t.Errorf("HTTP status %d does not match the envelope status %d", res.Code, out.Status)
	}
	return out
}

func wrap(err error) error { return errors.Join(errors.New("while saving"), err) }
