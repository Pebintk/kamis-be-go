package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pebintk/kamis-be-go/pkg/apierr"
	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/services/purchase/internal/dto"
	"github.com/pebintk/kamis-be-go/services/purchase/internal/model"
)

const sampleUUID = "3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"

func ptrI64(v int64) *int64 { return &v }
func ptrInt(v int) *int     { return &v }

func isInvalid(err error) bool {
	var invalid *apierr.Invalid
	return errors.As(err, &invalid)
}

func TestTypeName(t *testing.T) {
	if got := typeName(model.TypeResource); got != "Resource" {
		t.Errorf("typeName(TypeResource) = %q, want Resource", got)
	}
	if got := typeName(model.TypeAset); got != "Aset" {
		t.Errorf("typeName(TypeAset) = %q, want Aset", got)
	}
}

func TestIsUUID(t *testing.T) {
	for _, s := range []string{sampleUUID, strings.ToUpper(sampleUUID)} {
		if !isUUID(s) {
			t.Errorf("isUUID(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "nope", strings.ReplaceAll(sampleUUID, "-", "")} {
		if isUUID(s) {
			t.Errorf("isUUID(%q) = true, want false", s)
		}
	}
}

// catalogue stands in for the resource service. Keys are resource ids.
func catalogue(t *testing.T, items map[int64]string) *httpx.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/resource/find/")
		name, ok := items[parseID(id)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"data":   dto.ResourceResponse{ID: parseID(id), ResourceName: name},
		})
	}))
	t.Cleanup(srv.Close)
	return httpx.NewClient(srv.URL, httpx.DefaultTimeout)
}

func parseID(s string) int64 {
	var v int64
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0
		}
		v = v*10 + int64(s[i]-'0')
	}
	return v
}

// TestBuildLines covers the validation a resource purchase goes through against
// the live catalogue before anything is written.
func TestBuildLines(t *testing.T) {
	svc := NewPurchaseService(nil, Deps{Resource: catalogue(t, map[int64]string{
		1: "Semen",
		2: "Besi",
	})})
	ctx := context.Background()

	line := func(id int64, name string, total, price int) dto.ResourceLineRequest {
		return dto.ResourceLineRequest{
			ResourceID: ptrI64(id), ResourceName: name,
			ResourceTotal: ptrInt(total), ResourcePrice: ptrInt(price),
		}
	}

	t.Run("prices the whole order", func(t *testing.T) {
		lines, total, err := svc.buildLines(ctx, []dto.ResourceLineRequest{
			line(1, "Semen", 3, 50000),
			line(2, "Besi", 2, 125000),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(lines) != 2 {
			t.Fatalf("got %d lines, want 2", len(lines))
		}
		if want := 3*50000 + 2*125000; total != want {
			t.Errorf("total = %d, want %d", total, want)
		}
	})

	failures := map[string][]dto.ResourceLineRequest{
		"empty order":         {},
		"duplicate resource":  {line(1, "Semen", 1, 1), line(1, "Semen", 2, 1)},
		"unknown resource":    {line(99, "Hantu", 1, 1)},
		"name does not match": {line(1, "Besi", 1, 1)},
		"zero quantity":       {line(1, "Semen", 0, 1)},
		"negative quantity":   {line(1, "Semen", -1, 1)},
		"negative price":      {line(1, "Semen", 1, -1)},
	}
	for name, requested := range failures {
		t.Run(name, func(t *testing.T) {
			if _, _, err := svc.buildLines(ctx, requested); !isInvalid(err) {
				t.Errorf("got %v, want an *apierr.Invalid", err)
			}
		})
	}
}

// TestAddPurchaseTypeMismatch pins the four cross-checks between purchaseType
// and the payload, all of which run before the database is touched. The service
// has a nil repository, so a call that reached it would panic.
func TestAddPurchaseTypeMismatch(t *testing.T) {
	svc := NewPurchaseService(nil, Deps{Resource: catalogue(t, map[int64]string{1: "Semen"})})
	ctx := context.Background()

	cases := map[string]dto.AddPurchaseRequest{
		"malformed supplier id": {
			PurchaseSupplier: "nope", PurchaseType: model.TypeResource,
		},
		"resource purchase carrying an asset": {
			PurchaseSupplier: sampleUUID, PurchaseType: model.TypeResource,
			PurchaseAsset: ptrI64(7),
		},
		"resource purchase with no lines": {
			PurchaseSupplier: sampleUUID, PurchaseType: model.TypeResource,
		},
		"asset purchase with no asset": {
			PurchaseSupplier: sampleUUID, PurchaseType: model.TypeAset,
		},
		"asset purchase carrying lines": {
			PurchaseSupplier: sampleUUID, PurchaseType: model.TypeAset,
			PurchaseAsset: ptrI64(7),
			PurchaseResource: []dto.ResourceLineRequest{{
				ResourceID: ptrI64(1), ResourceName: "Semen",
				ResourceTotal: ptrInt(1), ResourcePrice: ptrInt(1),
			}},
		},
	}

	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.AddPurchase(ctx, req); !isInvalid(err) {
				t.Errorf("got %v, want an *apierr.Invalid", err)
			}
		})
	}
}

// TestSupplierNameToleratesOutage is the deliberate divergence from Java, which
// rethrew and turned the whole purchase list into a 400 when profile was
// unreachable.
func TestSupplierNameToleratesOutage(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer down.Close()

	svc := NewPurchaseService(nil, Deps{Profile: httpx.NewClient(down.URL, httpx.DefaultTimeout)})
	if got := svc.supplierName(context.Background(), sampleUUID); got != "" {
		t.Errorf("supplierName on an outage = %q, want an empty name", got)
	}
}

// TestSupplierNamesDeduplicates is the point of resolving names per supplier
// rather than per row: many purchases share a supplier, and Java made one HTTP
// call for each row regardless.
func TestSupplierNamesDeduplicates(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "data": "PT Contoh"})
	}))
	defer srv.Close()

	svc := NewPurchaseService(nil, Deps{Profile: httpx.NewClient(srv.URL, httpx.DefaultTimeout)})
	purchases := make([]model.Purchase, 20)
	for i := range purchases {
		purchases[i].PurchaseSupplier = sampleUUID
	}

	names := svc.supplierNames(context.Background(), purchases)
	if calls != 1 {
		t.Errorf("made %d calls for 20 rows sharing one supplier, want 1", calls)
	}
	if names[sampleUUID] != "PT Contoh" {
		t.Errorf("name = %q, want PT Contoh", names[sampleUUID])
	}
}

func TestNotFound(t *testing.T) {
	var missing *apierr.NotFound
	err := notFound(database.ErrNotFound, "R-030926-001")
	if !errors.As(err, &missing) {
		t.Fatalf("got %v, want an *apierr.NotFound", err)
	}
	if !strings.Contains(missing.Error(), "R-030926-001") {
		t.Errorf("message %q does not name the purchase", missing.Error())
	}

	boom := errors.New("connection refused")
	if got := notFound(boom, "R-030926-001"); !errors.Is(got, boom) {
		t.Errorf("notFound rewrote a non-lookup error to %v", got)
	}
}
