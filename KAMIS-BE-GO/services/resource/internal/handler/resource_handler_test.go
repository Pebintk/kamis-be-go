package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/services/resource/internal/service"
)

// TestStatusFor pins the mapping the frontend now branches on. The legacy
// controller answered a different status per endpoint for the same condition;
// these three rules replace all of that.
func TestStatusFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"caller mistake", &service.InvalidError{Message: "Stok barang tidak boleh kurang dari 0"}, http.StatusBadRequest},
		{"missing resource", &service.NotFoundError{Message: "Resource dengan ID 7 tidak ditemukan."}, http.StatusNotFound},
		{"unexpected failure", errors.New("connection refused"), http.StatusInternalServerError},
		{"wrapped caller mistake", wrap(&service.InvalidError{Message: "x"}), http.StatusBadRequest},
		{"wrapped missing resource", wrap(&service.NotFoundError{Message: "x"}), http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusFor(tc.err); got != tc.want {
				t.Errorf("statusFor(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// TestRespondErrorHidesInternals keeps an unexpected failure from reaching the
// browser verbatim — a connection error carries the database DSN.
func TestRespondErrorHidesInternals(t *testing.T) {
	body := respond(t, errors.New(`dial tcp: postgres://kamis:hunter2@db:5432 refused`))

	if body.Status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", body.Status)
	}
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
	if body := respond(t, &service.InvalidError{Message: invalid}); body.Message != invalid {
		t.Errorf("message = %q, want %q", body.Message, invalid)
	}

	const missing = "Resource dengan ID 7 tidak ditemukan."
	if body := respond(t, &service.NotFoundError{Message: missing}); body.Message != missing {
		t.Errorf("message = %q, want %q", body.Message, missing)
	}
}

type envelope struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func respond(t *testing.T, err error) envelope {
	t.Helper()
	gin.SetMode(gin.TestMode)

	res := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(res)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/resource/find/7", nil)
	respondError(c, err)

	var body envelope
	if decodeErr := json.Unmarshal(res.Body.Bytes(), &body); decodeErr != nil {
		t.Fatalf("decode response: %v (body %s)", decodeErr, res.Body)
	}
	if res.Code != body.Status {
		t.Errorf("HTTP status %d does not match the envelope status %d", res.Code, body.Status)
	}
	return body
}

func wrap(err error) error { return errors.Join(errors.New("while saving"), err) }
