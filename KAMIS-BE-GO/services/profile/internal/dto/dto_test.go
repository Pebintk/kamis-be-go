package dto

import (
	"encoding/json"
	"testing"
	"time"
)

// TestTimestampZones covers the split in the legacy DTOs: the Client fields
// carry @JsonFormat(timezone="Asia/Jakarta"), the Supplier fields do not and so
// serialize in UTC.
func TestTimestampZones(t *testing.T) {
	instant := time.Date(2026, 3, 1, 3, 30, 0, 0, time.UTC)

	if got := marshal(t, JakartaTime(instant)); got != `"2026-03-01T10:30:00.000+07:00"` {
		t.Errorf("JakartaTime = %s", got)
	}
	if got := marshal(t, UTCTime(instant)); got != `"2026-03-01T03:30:00.000+00:00"` {
		t.Errorf("UTCTime = %s", got)
	}
	if got := marshal(t, Timestamp{}); got != "null" {
		t.Errorf("zero Timestamp = %s, want null", got)
	}
}

// TestTimestampUnmarshal covers the shapes the other KAMIS services emit.
func TestTimestampUnmarshal(t *testing.T) {
	want := time.Date(2026, 3, 1, 3, 30, 0, 0, time.UTC)

	for _, raw := range []string{
		`"2026-03-01T03:30:00.000+00:00"`,
		`"2026-03-01T03:30:00Z"`,
		`1772335800000`,
	} {
		var ts Timestamp
		if err := json.Unmarshal([]byte(raw), &ts); err != nil {
			t.Fatalf("unmarshal %s: %v", raw, err)
		}
		if !ts.UTC().Equal(want) {
			t.Errorf("unmarshal %s = %v, want %v", raw, ts.UTC(), want)
		}
	}

	var ts Timestamp
	if err := json.Unmarshal([]byte("null"), &ts); err != nil || !ts.IsZero() {
		t.Errorf("null should decode to the zero time, got %v (%v)", ts.Time, err)
	}
}

func TestCalculateProfit(t *testing.T) {
	ptr := func(v int64) *int64 { return &v }

	p := ProjectResponse{ProjectTotalPemasukkan: ptr(1000), ProjectTotalPengeluaran: ptr(400)}
	p.CalculateProfit()
	if p.Profit == nil || *p.Profit != 600 {
		t.Errorf("profit = %v, want 600", p.Profit)
	}

	// Either total missing leaves profit null, as the Java version does.
	for _, missing := range []ProjectResponse{
		{ProjectTotalPemasukkan: ptr(1000)},
		{ProjectTotalPengeluaran: ptr(400)},
		{},
	} {
		missing.CalculateProfit()
		if missing.Profit != nil {
			t.Errorf("profit = %v, want nil", *missing.Profit)
		}
	}
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
