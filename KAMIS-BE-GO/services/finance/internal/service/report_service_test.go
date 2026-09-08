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
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/services/finance/internal/dto"
)

func isInvalid(err error) bool {
	var invalid *apierr.Invalid
	return errors.As(err, &invalid)
}

// activityService stands in for project or purchase, answering each chart path
// with a fixed series.
func activityService(t *testing.T, byPath map[string][]dto.ActivityLine, fail bool) *httpx.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		points, ok := byPath[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "data": points})
	}))
	t.Cleanup(srv.Close)
	return httpx.NewClient(srv.URL, httpx.DefaultTimeout)
}

// TestCombinedActivityChart covers the merge: three independent streams keyed
// into one row per period, ordered by label.
func TestCombinedActivityChart(t *testing.T) {
	purchase := activityService(t, map[string][]dto.ActivityLine{
		"/purchase/chart/purchase-activity": {{Period: "2026-01", Count: 5}, {Period: "2026-03", Count: 2}},
	}, false)
	project := activityService(t, map[string][]dto.ActivityLine{
		"/project/chart/penjualan-activity":  {{Period: "2026-01", Count: 7}},
		"/project/chart/distribusi-activity": {{Period: "2026-02", Count: 3}},
	}, false)

	svc := NewLapkeuService(nil, Deps{Purchase: purchase, Project: project})
	rows, err := svc.CombinedActivityChart(context.Background(), "THIS_YEAR", "MONTHLY", "ALL")
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 3 {
		t.Fatalf("got %d periods, want 3", len(rows))
	}
	// Ordered by label, which for every format this service uses is also
	// chronological.
	for i, want := range []string{"2026-01", "2026-02", "2026-03"} {
		if rows[i].Period != want {
			t.Errorf("row %d period = %q, want %q", i, rows[i].Period, want)
		}
	}

	// A period one stream reported and the others did not still appears, with
	// the missing streams at zero.
	if rows[0].PembelianCount != 5 || rows[0].PenjualanCount != 7 || rows[0].DistribusiCount != 0 {
		t.Errorf("2026-01 = %+v, want 5 / 7 / 0", rows[0])
	}
	if rows[1].DistribusiCount != 3 || rows[1].PembelianCount != 0 {
		t.Errorf("2026-02 = %+v, want only a distribution count", rows[1])
	}
	if rows[2].PembelianCount != 2 {
		t.Errorf("2026-03 = %+v, want a purchase count of 2", rows[2])
	}
}

// TestCombinedActivityChartFailsLoudly is the point of not defaulting a missing
// stream to zero: a chart of zeroes reads as "no activity", not "no data".
func TestCombinedActivityChartFailsLoudly(t *testing.T) {
	working := activityService(t, map[string][]dto.ActivityLine{
		"/project/chart/penjualan-activity":  {{Period: "2026-01", Count: 7}},
		"/project/chart/distribusi-activity": {},
	}, false)
	down := activityService(t, nil, true)

	svc := NewLapkeuService(nil, Deps{Purchase: down, Project: working})
	if _, err := svc.CombinedActivityChart(context.Background(), "THIS_YEAR", "MONTHLY", "ALL"); !isInvalid(err) {
		t.Fatalf("got %v, want an *apierr.Invalid when a stream is unavailable", err)
	}
}

// TestCombinedActivityChartForwardsParameters keeps the range, granularity and
// status reaching the services that actually do the counting.
func TestCombinedActivityChartForwardsParameters(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.RawQuery)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "data": []dto.ActivityLine{}})
	}))
	defer srv.Close()

	client := httpx.NewClient(srv.URL, httpx.DefaultTimeout)
	svc := NewLapkeuService(nil, Deps{Purchase: client, Project: client})
	if _, err := svc.CombinedActivityChart(context.Background(), "THIS_MONTH", "WEEKLY", "DONE"); err != nil {
		t.Fatal(err)
	}

	if len(seen) != 3 {
		t.Fatalf("made %d calls, want 3", len(seen))
	}
	for _, query := range seen {
		for _, want := range []string{"range=THIS_MONTH", "periodType=WEEKLY", "status=DONE"} {
			if !strings.Contains(query, want) {
				t.Errorf("query %q is missing %q", query, want)
			}
		}
	}
}

// TestCombinedActivityChartOmitsBlankPeriodType lets the downstream service
// pick its own default, as the legacy queryParamIfPresent did.
func TestCombinedActivityChartOmitsBlankPeriodType(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "data": []dto.ActivityLine{}})
	}))
	defer srv.Close()

	client := httpx.NewClient(srv.URL, httpx.DefaultTimeout)
	svc := NewLapkeuService(nil, Deps{Purchase: client, Project: client})
	if _, err := svc.CombinedActivityChart(context.Background(), "THIS_YEAR", "  ", "ALL"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(seen, "periodType") {
		t.Errorf("query %q sent a blank periodType instead of omitting it", seen)
	}
}
