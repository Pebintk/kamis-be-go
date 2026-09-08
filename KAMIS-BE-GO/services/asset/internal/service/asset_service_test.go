package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/blob"
	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/asset/internal/dto"
)

const sampleUUID = "3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"

func ptr(v int) *int { return &v }

// TestPhotoKey covers the two things the Java filename scheme got wrong: real
// plate numbers contain spaces, and stripping them can collide.
func TestPhotoKey(t *testing.T) {
	key := photoKey("B 1234 XYZ", "image/jpeg")

	if !blob.ValidKey(key) {
		t.Errorf("photoKey produced %q, which blob.ValidKey rejects", key)
	}
	if !strings.HasPrefix(key, "B1234XYZ-") {
		t.Errorf("key = %q, want it to start with the sanitized plate", key)
	}
	if !strings.HasSuffix(key, ".jpg") {
		t.Errorf("key = %q, want a .jpg extension so the disk backend can recover the type", key)
	}

	// Plates that sanitize to the same characters must not share an object.
	if a, b := photoKey("B 1234", "image/png"), photoKey("B-1234", "image/png"); a == b {
		t.Errorf("distinct plates collided on one key: %q", a)
	}
	// The key is stable for a given plate, so a re-upload replaces in place.
	if a, b := photoKey("B 1234", "image/png"), photoKey("B 1234", "image/png"); a != b {
		t.Errorf("photoKey is not deterministic: %q vs %q", a, b)
	}
	// A plate with nothing safe in it still yields a usable key.
	if k := photoKey("!!!", "image/webp"); !blob.ValidKey(k) {
		t.Errorf("photoKey(%q) = %q, which blob.ValidKey rejects", "!!!", k)
	}
}

func TestIsUUID(t *testing.T) {
	for _, s := range []string{sampleUUID, strings.ToUpper(sampleUUID)} {
		if !isUUID(s) {
			t.Errorf("isUUID(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "not-a-uuid", strings.ReplaceAll(sampleUUID, "-", ""), sampleUUID + "a"} {
		if isUUID(s) {
			t.Errorf("isUUID(%q) = true, want false", s)
		}
	}
}

// TestNotFound covers both halves: a repository miss becomes the service's 404,
// and any other error passes through so a dropped connection is not reported as
// a missing asset.
func TestNotFound(t *testing.T) {
	err := notFound(database.ErrNotFound, "B1234XYZ")

	var missing *apierr.NotFound
	if !errors.As(err, &missing) {
		t.Fatalf("notFound(ErrNotFound) = %v, want an *apierr.NotFound", err)
	}
	const want = "Asset dengan plat nomor B1234XYZ tidak ditemukan"
	if missing.Error() != want {
		t.Errorf("message = %q, want %q", missing.Error(), want)
	}

	boom := errors.New("connection refused")
	if got := notFound(boom, "B1234XYZ"); !errors.Is(got, boom) {
		t.Errorf("notFound rewrote a non-lookup error to %v", got)
	}
}

// TestValidationPrecedesRepository pins what each entry point rejects before
// touching the database. The service is built on a nil repository, so a call
// that did reach it would panic rather than pass quietly.
func TestValidationPrecedesRepository(t *testing.T) {
	svc := NewAssetService(nil, nil)
	ctx := context.Background()

	valid := dto.AddAssetRequest{
		PlatNomor: "B1234XYZ", AssetName: "Truk", AssetDescription: "Truk besar",
		AssetType: "Truk", AssetPrice: ptr(1000), Status: "Tersedia",
		TanggalPerolehan: "2026-01-15", SupplierID: sampleUUID,
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"add with a malformed supplier id", func() error {
			bad := valid
			bad.SupplierID = "nope"
			_, err := svc.AddAsset(ctx, bad, nil)
			return err
		}},
		{"add with a malformed date", func() error {
			bad := valid
			bad.TanggalPerolehan = "15-01-2026"
			_, err := svc.AddAsset(ctx, bad, nil)
			return err
		}},
		{"add with a negative price", func() error {
			bad := valid
			bad.AssetPrice = ptr(-1)
			_, err := svc.AddAsset(ctx, bad, nil)
			return err
		}},
		{"list by a malformed supplier id", func() error {
			_, err := svc.ListBySupplier(ctx, "nope")
			return err
		}},
		{"set a malformed supplier id", func() error {
			_, err := svc.SetSupplier(ctx, "B1234XYZ", "nope")
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			var invalid *apierr.Invalid
			if !errors.As(err, &invalid) {
				t.Errorf("got %v, want an *apierr.Invalid", err)
			}
		})
	}
}

// TestAddAssetAcceptsZeroPrice guards the pointer field on the request DTO: a
// zero price is legitimate (an asset transferred at no cost), so it must not be
// read as missing. Reaching the nil repository is the proof it got through.
func TestAddAssetAcceptsZeroPrice(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("validation rejected a zero price instead of reaching the repository")
		}
	}()

	svc := NewAssetService(nil, nil)
	_, _ = svc.AddAsset(context.Background(), dto.AddAssetRequest{
		PlatNomor: "B1234XYZ", AssetName: "Truk", AssetDescription: "Hibah",
		AssetType: "Truk", AssetPrice: ptr(0), Status: "Tersedia",
		TanggalPerolehan: "2026-01-15", SupplierID: sampleUUID,
	}, nil)
}
