package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDataUnwrapsEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/resource/viewall" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"status":200,"message":"ok","data":[{"id":1},{"id":2}]}`))
	}))
	defer server.Close()

	type resource struct {
		ID int64 `json:"id"`
	}
	got, err := GetData[[]resource](context.Background(), NewClient(server.URL, 0), "/resource/viewall")
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("data = %+v", got)
	}
}

// The caller's bearer token must reach the downstream service, which is what
// makes the legacy inter-service calls authorize.
func TestForwardsBearerToken(t *testing.T) {
	var seen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		w.Write([]byte(`{"data":null}`))
	}))
	defer server.Close()

	ctx := ContextWithToken(context.Background(), "tok-123")
	if _, err := GetData[any](ctx, NewClient(server.URL, 0), "/x"); err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if seen != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want %q", seen, "Bearer tok-123")
	}

	// With no token on the context the header must be absent, not "Bearer ".
	if _, err := GetData[any](context.Background(), NewClient(server.URL, 0), "/x"); err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if seen != "" {
		t.Errorf("Authorization = %q, want empty", seen)
	}
}

// A 404 is distinguishable so callers can treat it as "no data" rather than a
// failure, matching the Java onStatus(...) handling.
func TestNotFoundIsTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := GetData[[]int](context.Background(), NewClient(server.URL, 0), "/missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestServerErrorIsReported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := GetData[[]int](context.Background(), NewClient(server.URL, 0), "/boom"); err == nil {
		t.Error("expected an error for a 500")
	}
}

// An unconfigured dependency must fail cleanly rather than build a bad URL.
func TestUnconfiguredBaseURL(t *testing.T) {
	if _, err := GetData[[]int](context.Background(), NewClient("", 0), "/x"); err == nil {
		t.Error("expected an error when no base URL is configured")
	}
}

func TestPutSendsBody(t *testing.T) {
	var method, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		body = string(buf)
	}))
	defer server.Close()

	err := NewClient(server.URL, 0).Put(context.Background(), "/resource/add-supplier",
		map[string]any{"supplierId": "abc"})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if method != http.MethodPut {
		t.Errorf("method = %s", method)
	}
	if body != `{"supplierId":"abc"}` {
		t.Errorf("body = %s", body)
	}
}
