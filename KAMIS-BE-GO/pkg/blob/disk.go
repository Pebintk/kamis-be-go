package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
)

// Disk stores objects as files in one directory. It exists so a service can run
// locally without Google credentials, and it is what the legacy Java
// FileStorageServiceImpl did in production.
type Disk struct{ dir string }

// NewDisk creates the directory if it is missing and returns a store over it.
func NewDisk(dir string) (*Disk, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create storage directory %s: %w", dir, err)
	}
	return &Disk{dir: dir}, nil
}

// path is only ever called with a key that passed checkKey, so it cannot escape
// d.dir.
func (d *Disk) path(key string) string { return filepath.Join(d.dir, key) }

func (d *Disk) Put(_ context.Context, key, _ string, r io.Reader) error {
	if err := checkKey(key); err != nil {
		return err
	}

	// Write to a temp file in the same directory and rename, so a reader never
	// observes a half-written image and a failed upload leaves the previous one
	// intact.
	tmp, err := os.CreateTemp(d.dir, ".upload-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name()) // no-op once the rename below succeeds
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		return fmt.Errorf("write object: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), d.path(key)); err != nil {
		return fmt.Errorf("store object: %w", err)
	}
	return nil
}

func (d *Disk) Get(_ context.Context, key string) (*Object, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}

	f, err := os.Open(d.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open object: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat object: %w", err)
	}

	// The disk backend stores no metadata, so the type comes from the
	// extension. Keys are generated with one, and an unknown extension falls
	// back to the generic type rather than an empty header.
	contentType := mime.TypeByExtension(filepath.Ext(key))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &Object{Body: f, ContentType: contentType, Size: info.Size()}, nil
}

func (d *Disk) Delete(_ context.Context, key string) error {
	if err := checkKey(key); err != nil {
		return err
	}
	if err := os.Remove(d.path(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}
