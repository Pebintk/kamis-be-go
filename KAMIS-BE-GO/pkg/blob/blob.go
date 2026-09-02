// Package blob stores and serves the image files the asset and purchase
// services accept on upload — the Go stand-in for the legacy Java
// FileStorageService, which wrote them to a directory on the container's disk.
//
// Two backends implement Store: GCS for deployment, and a local directory so a
// service can be run and tested without Google credentials. Pick one with
// FromEnv.
package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrNotFound reports that no object is stored under the given key. Handlers
// map it to 404.
var ErrNotFound = errors.New("object not found")

// Object is a stored file being read back.
type Object struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

// Store is the storage backend an image-holding service talks to.
type Store interface {
	// Put writes r under key, replacing anything already there.
	Put(ctx context.Context, key, contentType string, r io.Reader) error
	// Get opens the object at key, or returns ErrNotFound. The caller closes
	// the returned Body.
	Get(ctx context.Context, key string) (*Object, error)
	// Delete removes the object at key. Deleting a key that is not there is not
	// an error, so a retry of a half-finished cleanup succeeds.
	Delete(ctx context.Context, key string) error
}

// ValidKey reports whether key is safe to use with either backend.
//
// Keys reach the disk backend as filenames and GCS as a path segment, so this
// is the single choke point that keeps a caller-supplied name from escaping the
// storage directory. The services build keys from a plate number, which is
// user-supplied, so the check is not theoretical.
func ValidKey(key string) bool {
	if key == "" || len(key) > 200 {
		return false
	}
	if strings.ContainsAny(key, `/\`) || strings.Contains(key, "..") {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_'
		if !ok {
			return false
		}
	}
	return true
}

// checkKey is the guard every backend method runs first.
func checkKey(key string) error {
	if !ValidKey(key) {
		return fmt.Errorf("invalid storage key %q", key)
	}
	return nil
}

// FromEnv builds the backend the environment asks for: GCS when GCS_BUCKET is
// set, otherwise a local directory at FILE_UPLOAD_DIR (default "asset-images",
// the same default the Java service used).
//
// Deployments set GCS_BUCKET. Leaving it unset is what makes `make run` work
// without Google credentials.
func FromEnv(ctx context.Context) (Store, error) {
	if bucket := os.Getenv("GCS_BUCKET"); bucket != "" {
		return NewGCS(ctx, bucket)
	}
	dir := os.Getenv("FILE_UPLOAD_DIR")
	if dir == "" {
		dir = "asset-images"
	}
	return NewDisk(dir)
}
