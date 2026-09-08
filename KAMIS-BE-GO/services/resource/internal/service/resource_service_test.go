package service

import (
	"context"
	"errors"
	"testing"

	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/services/resource/internal/dto"
)

func ptr(v int) *int { return &v }

const sampleUUID = "3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"

func TestIsUUID(t *testing.T) {
	valid := []string{
		sampleUUID,
		"3F1B2C4D-5E6F-4A7B-8C9D-0E1F2A3B4C5D",
	}
	for _, s := range valid {
		if !isUUID(s) {
			t.Errorf("isUUID(%q) = false, want true", s)
		}
	}

	invalid := []string{
		"",
		"not-a-uuid",
		"3f1b2c4d5e6f4a7b8c9d0e1f2a3b4c5d",                  // no dashes
		"3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5",               // too short
		"3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5dd",             // too long
		"3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5g",              // non-hex
		"3f1b2c4d_5e6f-4a7b-8c9d-0e1f2a3b4c5d",              // wrong separator
		"'; DROP TABLE resources; --                      ", // 36 chars, not hex
	}
	for _, s := range invalid {
		if isUUID(s) {
			t.Errorf("isUUID(%q) = true, want false", s)
		}
	}
}

// TestStockArithmetic covers the two functions the repository applies to the
// locked row.
func TestStockArithmetic(t *testing.T) {
	if got, err := addition(5)(10); got != 15 || err != nil {
		t.Errorf("addition(5)(10) = %d, %v; want 15, nil", got, err)
	}
	// addToDb passes the amount straight through with no sign check, as Java did.
	if got, err := addition(-3)(10); got != 7 || err != nil {
		t.Errorf("addition(-3)(10) = %d, %v; want 7, nil", got, err)
	}
	if got, err := deduction(4)(10); got != 6 || err != nil {
		t.Errorf("deduction(4)(10) = %d, %v; want 6, nil", got, err)
	}
	// Deducting exactly the stock on hand is allowed; one more is not.
	if _, err := deduction(10)(10); err != nil {
		t.Errorf("deduction(10)(10) = %v, want nil", err)
	}

	_, err := deduction(11)(10)
	if !isInvalid(err) {
		t.Fatalf("deduction past zero = %v, want an InvalidError", err)
	}
	const want = "Stock tidak mencukupi. Tersedia: 10, permintaan: 11"
	if err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
}

// TestValidationPrecedesRepository pins the inputs each entry point rejects
// before it touches the database. The service is built on a nil repository, so
// any call that did reach it would panic rather than pass quietly.
func TestValidationPrecedesRepository(t *testing.T) {
	svc := NewResourceService(nil)
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"add with negative price", func() error {
			_, err := svc.AddResource(ctx, dto.AddResourceRequest{
				ResourceName: "Semen", ResourceStock: ptr(1), ResourcePrice: ptr(-1),
				ResourceSupplierID: sampleUUID,
			})
			return err
		}},
		{"add with negative stock", func() error {
			_, err := svc.AddResource(ctx, dto.AddResourceRequest{
				ResourceName: "Semen", ResourceStock: ptr(-1), ResourcePrice: ptr(1),
				ResourceSupplierID: sampleUUID,
			})
			return err
		}},
		{"add with malformed supplier id", func() error {
			_, err := svc.AddResource(ctx, dto.AddResourceRequest{
				ResourceName: "Semen", ResourceStock: ptr(1), ResourcePrice: ptr(1),
				ResourceSupplierID: "not-a-uuid",
			})
			return err
		}},
		{"update with negative price", func() error {
			_, err := svc.UpdateResource(ctx, 1, dto.UpdateResourceRequest{
				ResourceDescription: "x", ResourcePrice: ptr(-1), ResourceStock: ptr(0),
			})
			return err
		}},
		{"update with negative stock", func() error {
			_, err := svc.UpdateResource(ctx, 1, dto.UpdateResourceRequest{
				ResourceDescription: "x", ResourcePrice: ptr(0), ResourceStock: ptr(-1),
			})
			return err
		}},
		{"add zero stock", func() error {
			_, err := svc.AddStock(ctx, 1, 0)
			return err
		}},
		{"deduct negative stock", func() error {
			_, err := svc.DeductStock(ctx, 1, -1)
			return err
		}},
		{"low-stock report with a negative level", func() error {
			_, err := svc.ListByStockAtMost(ctx, -1)
			return err
		}},
		{"list by malformed supplier id", func() error {
			_, err := svc.ListBySupplier(ctx, "nope")
			return err
		}},
		{"link with malformed supplier id", func() error {
			return svc.AddSupplier(ctx, "nope", []int64{1})
		}},
		{"relink with malformed supplier id", func() error {
			return svc.UpdateSupplier(ctx, "nope", []int64{1})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !isInvalid(err) {
				t.Errorf("got %v, want an InvalidError", err)
			}
		})
	}
}

// TestAddResourceAcceptsZeroes guards the pointer fields on the request DTO: a
// free item and an out-of-stock item are both legitimate, so zero must not be
// treated as missing. It reaches the (nil) repository, which is the proof that
// validation let it through.
func TestAddResourceAcceptsZeroes(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("validation rejected a zero stock/price instead of reaching the repository")
		}
	}()

	svc := NewResourceService(nil)
	_, _ = svc.AddResource(context.Background(), dto.AddResourceRequest{
		ResourceName: "Sampel", ResourceStock: ptr(0), ResourcePrice: ptr(0),
		ResourceSupplierID: sampleUUID,
	})
}

// TestNotFound covers both halves of the lookup-error mapping: a repository miss
// becomes a NotFoundError (404), and anything else passes through as-is so a
// dropped connection is not reported to the browser as a missing row.
func TestNotFound(t *testing.T) {
	err := notFound(database.ErrNotFound, 42)

	var missing *NotFoundError
	if !errors.As(err, &missing) {
		t.Fatalf("notFound(ErrNotFound) = %v, want a *NotFoundError", err)
	}
	const want = "Resource dengan ID 42 tidak ditemukan."
	if missing.Error() != want {
		t.Errorf("message = %q, want %q", missing.Error(), want)
	}
	if isInvalid(err) {
		t.Error("a missing resource must not also read as a caller mistake (400)")
	}

	boom := errors.New("connection refused")
	if got := notFound(boom, 42); !errors.Is(got, boom) {
		t.Errorf("notFound rewrote a non-lookup error to %v", got)
	}
}

func isInvalid(err error) bool {
	var invalid *InvalidError
	return errors.As(err, &invalid)
}
