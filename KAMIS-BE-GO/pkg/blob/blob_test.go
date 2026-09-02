package blob

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidKey is the guard against a caller-supplied name escaping the storage
// directory or the bucket path. Keys are built from plate numbers, which the
// user types.
func TestValidKey(t *testing.T) {
	for _, key := range []string{"B1234XYZ.jpg", "a.png", "asset-1_foto.webp"} {
		if !ValidKey(key) {
			t.Errorf("ValidKey(%q) = false, want true", key)
		}
	}

	rejected := []string{
		"",
		"../../etc/passwd",
		"..",
		"a/b.jpg",
		`a\b.jpg`,
		"foo/../../bar.jpg",
		"with space.jpg",
		"semi;colon.jpg",
		"null\x00byte.jpg",
		strings.Repeat("a", 201),
	}
	for _, key := range rejected {
		if ValidKey(key) {
			t.Errorf("ValidKey(%q) = true, want false", key)
		}
	}
}

func TestDiskRoundTrip(t *testing.T) {
	store, err := NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := store.Put(ctx, "photo.png", "image/png", strings.NewReader("first")); err != nil {
		t.Fatal(err)
	}

	obj, err := store.Get(ctx, "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(obj.Body)
	_ = obj.Body.Close()

	if string(body) != "first" {
		t.Errorf("body = %q, want %q", body, "first")
	}
	if obj.ContentType != "image/png" {
		t.Errorf("content type = %q, want image/png", obj.ContentType)
	}
	if obj.Size != 5 {
		t.Errorf("size = %d, want 5", obj.Size)
	}

	// A second Put replaces rather than appends.
	if err := store.Put(ctx, "photo.png", "image/png", strings.NewReader("second")); err != nil {
		t.Fatal(err)
	}
	obj, _ = store.Get(ctx, "photo.png")
	body, _ = io.ReadAll(obj.Body)
	_ = obj.Body.Close()
	if string(body) != "second" {
		t.Errorf("after replace, body = %q, want %q", body, "second")
	}

	if err := store.Delete(ctx, "photo.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "photo.png"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
	// Deleting again is not an error, so a retried cleanup succeeds.
	if err := store.Delete(ctx, "photo.png"); err != nil {
		t.Errorf("second Delete = %v, want nil", err)
	}
}

// TestDiskPutLeavesNoTempFiles checks the write-then-rename path cleans up, so
// the storage directory does not fill with .upload-* debris.
func TestDiskPutLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDisk(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "a.jpg", "image/jpeg", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != "a.jpg" {
		t.Errorf("directory contains %v, want just [a.jpg]", names)
	}
}

// TestDiskRejectsTraversal proves the key check is wired into every method, not
// just exported as a helper.
func TestDiskRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewDisk(dir)
	ctx := context.Background()
	const escape = "../escaped.txt"

	if err := store.Put(ctx, escape, "text/plain", strings.NewReader("pwned")); err == nil {
		t.Error("Put accepted a traversal key")
	}
	if _, err := store.Get(ctx, escape); err == nil {
		t.Error("Get accepted a traversal key")
	}
	if err := store.Delete(ctx, escape); err == nil {
		t.Error("Delete accepted a traversal key")
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "escaped.txt")); err == nil {
		t.Fatal("a file was written outside the storage directory")
	}
}

// newTestGCS builds a GCS store pointed at a stub server, bypassing
// NewGCS's credential lookup — which would need real Google credentials.
func newTestGCS(t *testing.T, h http.HandlerFunc) *GCS {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &GCS{bucket: "kamis-assets", baseURL: srv.URL, http: srv.Client()}
}

func TestGCSPut(t *testing.T) {
	var gotPath, gotQuery, gotType, gotBody string
	store := newTestGCS(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		gotType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	})

	if err := store.Put(context.Background(), "B1234XYZ.jpg", "image/jpeg", strings.NewReader("bytes")); err != nil {
		t.Fatal(err)
	}
	if want := "/upload/storage/v1/b/kamis-assets/o"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if !strings.Contains(gotQuery, "uploadType=media") || !strings.Contains(gotQuery, "name=B1234XYZ.jpg") {
		t.Errorf("query = %q, want uploadType=media and the object name", gotQuery)
	}
	if gotType != "image/jpeg" || gotBody != "bytes" {
		t.Errorf("sent %q / %q, want image/jpeg / bytes", gotType, gotBody)
	}
}

func TestGCSGet(t *testing.T) {
	store := newTestGCS(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") != "media" {
			t.Errorf("missing alt=media; query was %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("pngdata"))
	})

	obj, err := store.Get(context.Background(), "a.png")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = obj.Body.Close() }()

	body, _ := io.ReadAll(obj.Body)
	if string(body) != "pngdata" || obj.ContentType != "image/png" {
		t.Errorf("got %q / %q, want pngdata / image/png", body, obj.ContentType)
	}
}

// TestGCSStatusMapping pins the two statuses callers branch on: a missing object
// is ErrNotFound, and everything else is a real error rather than a silent
// empty image.
func TestGCSStatusMapping(t *testing.T) {
	missing := newTestGCS(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := missing.Get(context.Background(), "gone.jpg"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get on 404 = %v, want ErrNotFound", err)
	}
	// Delete treats an already-absent object as success.
	if err := missing.Delete(context.Background(), "gone.jpg"); err != nil {
		t.Errorf("Delete on 404 = %v, want nil", err)
	}

	broken := newTestGCS(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if _, err := broken.Get(context.Background(), "a.jpg"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("Get on 500 = %v, want a non-ErrNotFound error", err)
	}
	if err := broken.Delete(context.Background(), "a.jpg"); err == nil {
		t.Error("Delete on 500 returned nil")
	}
	if err := broken.Put(context.Background(), "a.jpg", "image/jpeg", strings.NewReader("x")); err == nil {
		t.Error("Put on 500 returned nil")
	}
}

// TestContentTypeFor keeps non-images out of the store. The legacy service
// accepted any upload, so an HTML file served back from /asset/{id}/foto with
// its own content type would have run as script on the frontend's origin.
func TestContentTypeFor(t *testing.T) {
	accepted := map[string]string{
		"image/jpeg":                "image/jpeg",
		"image/jpg":                 "image/jpeg",
		"IMAGE/PNG":                 "image/png",
		"image/webp; charset=utf-8": "image/webp",
		" image/gif ":               "image/gif",
	}
	for in, want := range accepted {
		got, ok := ContentTypeFor(in)
		if !ok || got != want {
			t.Errorf("ContentTypeFor(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}

	for _, in := range []string{"", "text/html", "application/pdf", "image/svg+xml", "application/octet-stream"} {
		if got, ok := ContentTypeFor(in); ok {
			t.Errorf("ContentTypeFor(%q) = %q, true; want rejected", in, got)
		}
	}
}
