package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/pkg/jsontime"
	"github.com/karina/kamis-be-go/services/project/internal/dto"
	"github.com/karina/kamis-be-go/services/project/internal/model"
)

func ptrInt(v int) *int    { return &v }
func ptr64(v int64) *int64 { return &v }

func boolPtr(v bool) *bool { return &v }

func jakarta(t time.Time) dto.Timestamp { return jsontime.Jakarta(t) }

func isInvalid(err error) bool {
	var invalid *apierr.Invalid
	return errors.As(err, &invalid)
}

var (
	start = time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	end   = time.Date(2026, 9, 12, 17, 0, 0, 0, time.UTC)
)

// envelope writes the BaseResponseDTO wrapper every KAMIS service replies with.
func envelope(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "data": data})
}

// resourceService stands in for the resource catalogue. Keys are resource ids.
func resourceService(t *testing.T, items map[string]dto.ResourceDetail) *httpx.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/resource/find/")
		item, ok := items[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		envelope(w, item)
	}))
	t.Cleanup(srv.Close)
	return httpx.NewClient(srv.URL, httpx.DefaultTimeout)
}

// assetService stands in for the asset service: known vehicles, and an
// availability answer for each.
func assetService(t *testing.T, known map[string]bool) *httpx.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset/reservations/check-availability" {
			var req dto.AssetAvailabilityRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			out := map[string]bool{}
			for _, plat := range req.PlatNomors {
				out[plat] = known[plat]
			}
			envelope(w, out)
			return
		}
		plat := strings.TrimPrefix(r.URL.Path, "/asset/")
		if _, ok := known[plat]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		envelope(w, dto.AssetDetail{PlatNomor: plat, Status: "Tersedia"})
	}))
	t.Cleanup(srv.Close)
	return httpx.NewClient(srv.URL, httpx.DefaultTimeout)
}

// TestPlanSale covers pricing a sale from the catalogue and the checks that run
// before any stock is consumed.
func TestPlanSale(t *testing.T) {
	svc := NewProjectService(nil, Deps{Resource: resourceService(t, map[string]dto.ResourceDetail{
		"1": {ID: 1, ResourceName: "Semen", ResourceStock: 100, ResourcePrice: 50000},
		"2": {ID: 2, ResourceName: "Besi", ResourceStock: 3, ResourcePrice: 125000},
	})})
	ctx := context.Background()

	use := func(id string, qty int) dto.ResourceUsage {
		return dto.ResourceUsage{ResourceID: id, ResourceStockUsed: ptrInt(qty)}
	}

	t.Run("prices at the catalogue's price, not the caller's", func(t *testing.T) {
		got, err := svc.planSale(ctx, planInput{resources: []dto.ResourceUsage{use("1", 3), use("2", 2)}})
		if err != nil {
			t.Fatal(err)
		}
		if want := int64(3*50000 + 2*125000); *got.totalPemasukkan != want {
			t.Errorf("income = %d, want %d", *got.totalPemasukkan, want)
		}
		if len(got.resources) != 2 {
			t.Fatalf("got %d usage rows, want 2", len(got.resources))
		}
		if got.resources[0].SellPrice != 50000 {
			t.Errorf("snapshot price = %d, want the catalogue's 50000", got.resources[0].SellPrice)
		}
	})

	// Stock is checked up front, so a sale that cannot be filled never draws
	// down the items ahead of it.
	t.Run("refuses more than is in stock", func(t *testing.T) {
		_, err := svc.planSale(ctx, planInput{resources: []dto.ResourceUsage{use("1", 1), use("2", 99)}})
		if !isInvalid(err) {
			t.Fatalf("got %v, want an *apierr.Invalid", err)
		}
		if !strings.Contains(err.Error(), "Tersedia: 3") {
			t.Errorf("message %q does not say what is available", err.Error())
		}
	})

	failures := map[string][]dto.ResourceUsage{
		"unknown resource":  {use("99", 1)},
		"zero quantity":     {use("1", 0)},
		"negative quantity": {use("1", -1)},
		"missing quantity":  {{ResourceID: "1"}},
	}
	for name, resources := range failures {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.planSale(ctx, planInput{resources: resources}); !isInvalid(err) {
				t.Errorf("got %v, want an *apierr.Invalid", err)
			}
		})
	}
}

// TestPlanDistribution covers pricing a distribution and the availability check.
func TestPlanDistribution(t *testing.T) {
	svc := NewProjectService(nil, Deps{Asset: assetService(t, map[string]bool{
		"B1234XYZ": true,
		"B9999ZZZ": false, // exists, but already booked
	})})
	ctx := context.Background()

	use := func(plat string, useCost, fuel int) dto.AssetUsage {
		return dto.AssetUsage{PlatNomor: plat, TipeAset: "Truk",
			AssetUseCost: ptrInt(useCost), AssetFuelCost: ptrInt(fuel)}
	}

	t.Run("sums vehicle costs and day labour", func(t *testing.T) {
		got, err := svc.planDistribution(ctx, planInput{
			assets:   []dto.AssetUsage{use("B1234XYZ", 300000, 150000)},
			phlCount: ptrInt(4), phlPay: ptr64(120000),
			totalPemasukkan: ptr64(2000000),
			start:           start, end: end,
		})
		if err != nil {
			t.Fatal(err)
		}
		if want := int64(300000 + 150000 + 4*120000); *got.totalPengeluaran != want {
			t.Errorf("outgoings = %d, want %d", *got.totalPengeluaran, want)
		}
		// Unlike a sale, a distribution's income is what the caller stated.
		if *got.totalPemasukkan != 2000000 {
			t.Errorf("income = %d, want the submitted 2000000", *got.totalPemasukkan)
		}
	})

	// Java multiplied both PHL fields unguarded, so omitting them was a 500.
	t.Run("day labour is optional", func(t *testing.T) {
		got, err := svc.planDistribution(ctx, planInput{
			assets: []dto.AssetUsage{use("B1234XYZ", 100, 50)},
			start:  start, end: end,
		})
		if err != nil {
			t.Fatal(err)
		}
		if *got.totalPengeluaran != 150 {
			t.Errorf("outgoings = %d, want 150", *got.totalPengeluaran)
		}
	})

	t.Run("refuses a vehicle that is already booked", func(t *testing.T) {
		_, err := svc.planDistribution(ctx, planInput{
			assets: []dto.AssetUsage{use("B9999ZZZ", 100, 50)},
			start:  start, end: end,
		})
		if !isInvalid(err) {
			t.Fatalf("got %v, want an *apierr.Invalid", err)
		}
		if !strings.Contains(err.Error(), "tidak tersedia") {
			t.Errorf("message %q does not say the vehicle is unavailable", err.Error())
		}
	})

	t.Run("refuses an unknown vehicle", func(t *testing.T) {
		_, err := svc.planDistribution(ctx, planInput{
			assets: []dto.AssetUsage{use("NOPE", 100, 50)},
			start:  start, end: end,
		})
		if !isInvalid(err) {
			t.Errorf("got %v, want an *apierr.Invalid", err)
		}
	})

	t.Run("refuses a vehicle with no costs", func(t *testing.T) {
		_, err := svc.planDistribution(ctx, planInput{
			assets: []dto.AssetUsage{{PlatNomor: "B1234XYZ", TipeAset: "Truk"}},
			start:  start, end: end,
		})
		if !isInvalid(err) {
			t.Errorf("got %v, want an *apierr.Invalid", err)
		}
	})
}

// TestAddProjectRejectsBackwardsDates runs before any downstream call, so the
// nil clients are never reached.
func TestAddProjectRejectsBackwardsDates(t *testing.T) {
	svc := NewProjectService(nil, Deps{})
	after, before := jakarta(end), jakarta(start)

	_, err := svc.AddProject(context.Background(), dto.AddProjectRequest{
		ProjectType:      boolPtr(model.TypePenjualan),
		ProjectStartDate: &after, ProjectEndDate: &before,
	})
	if !isInvalid(err) {
		t.Fatalf("got %v, want an *apierr.Invalid", err)
	}
	if !strings.Contains(err.Error(), "sebelum tanggal mulai") {
		t.Errorf("message %q does not explain the date order", err.Error())
	}
}
