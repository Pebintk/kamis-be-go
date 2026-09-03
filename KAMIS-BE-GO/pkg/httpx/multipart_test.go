package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPostFormSendsFieldsAndFile is the round-trip the purchase service depends
// on when it hands a completed asset purchase to the asset service.
func TestPostFormSendsFieldsAndFile(t *testing.T) {
	var (
		gotFields   = map[string]string{}
		gotFile     string
		gotFilename string
		gotType     string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("server could not parse the multipart body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for name := range r.MultipartForm.Value {
			gotFields[name] = r.FormValue(name)
		}
		if file, header, err := r.FormFile("foto"); err == nil {
			defer func() { _ = file.Close() }()
			body, _ := io.ReadAll(file)
			gotFile, gotFilename = string(body), header.Filename
			gotType = header.Header.Get("Content-Type")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, DefaultTimeout)
	err := client.PostForm(context.Background(), "/asset/addAsset",
		map[string]string{"platNomor": "B 1234 XYZ", "assetPrice": "5000"},
		&FilePart{Field: "foto", Filename: "asset_ab12.jpg", ContentType: "image/jpeg",
			Body: strings.NewReader("jpegbytes")})
	if err != nil {
		t.Fatal(err)
	}

	if gotFields["platNomor"] != "B 1234 XYZ" {
		t.Errorf("platNomor = %q, want %q", gotFields["platNomor"], "B 1234 XYZ")
	}
	if gotFields["assetPrice"] != "5000" {
		t.Errorf("assetPrice = %q, want 5000", gotFields["assetPrice"])
	}
	if gotFile != "jpegbytes" {
		t.Errorf("file body = %q, want jpegbytes", gotFile)
	}
	if gotFilename != "asset_ab12.jpg" || gotType != "image/jpeg" {
		t.Errorf("file header = %q / %q, want asset_ab12.jpg / image/jpeg", gotFilename, gotType)
	}
}

// TestPostFormWithoutFile covers staging an asset that has no photo.
func TestPostFormWithoutFile(t *testing.T) {
	var hadFile bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		if r.MultipartForm != nil && len(r.MultipartForm.File) > 0 {
			hadFile = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, DefaultTimeout)
	if err := client.PostForm(context.Background(), "/x", map[string]string{"a": "b"}, nil); err != nil {
		t.Fatal(err)
	}
	if hadFile {
		t.Error("a nil FilePart still produced a file part")
	}
}

// TestPostFormForwardsToken keeps the caller's bearer token on the request, so
// the downstream service authorizes the original user rather than rejecting an
// anonymous call.
func TestPostFormForwardsToken(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := ContextWithToken(context.Background(), "tok123")
	client := NewClient(srv.URL, DefaultTimeout)
	if err := client.PostForm(ctx, "/x", nil, nil); err != nil {
		t.Fatal(err)
	}
	if got != "Bearer tok123" {
		t.Errorf("Authorization = %q, want Bearer tok123", got)
	}
}

// TestPostFormReportsDownstreamFailure keeps a rejected handoff from reading as
// a success — the purchase service leaves the purchase in Diproses on this.
func TestPostFormReportsDownstreamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, DefaultTimeout)
	if err := client.PostForm(context.Background(), "/x", nil, nil); err == nil {
		t.Error("PostForm returned nil for a 400 from the downstream service")
	}
}

// TestPostFormWithNoBaseURL covers an unconfigured dependency: the call must
// fail rather than dial an empty host.
func TestPostFormWithNoBaseURL(t *testing.T) {
	if err := NewClient("", DefaultTimeout).PostForm(context.Background(), "/x", nil, nil); err == nil {
		t.Error("PostForm with no base URL returned nil")
	}
}
