package repository

import (
	"testing"
	"time"

	"github.com/karina/kamis-be-go/services/purchase/internal/model"
)

// TestIDPrefix pins the human-readable id format the frontend displays and
// searches by substring: {A|R}-{ddMMyy}-.
func TestIDPrefix(t *testing.T) {
	at := time.Date(2026, 9, 3, 14, 30, 0, 0, time.UTC)

	if got := idPrefix(model.TypeResource, at); got != "R-030926-" {
		t.Errorf("resource prefix = %q, want R-030926-", got)
	}
	if got := idPrefix(model.TypeAset, at); got != "A-030926-" {
		t.Errorf("asset prefix = %q, want A-030926-", got)
	}

	// Single-digit days and months stay zero-padded, so ids sort lexically
	// within a month.
	if got := idPrefix(model.TypeAset, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)); got != "A-050126-" {
		t.Errorf("padded prefix = %q, want A-050126-", got)
	}
}

// TestOrder pins the legacy comparator's precedence: price wins whenever
// highNominal is present at all, even alongside newDate; otherwise date, newest
// first only when newDate is true.
func TestOrder(t *testing.T) {
	yes, no := true, false

	cases := map[string]struct {
		filter Filter
		want   string
	}{
		"no sort given":       {Filter{}, "purchase_submission_date ASC"},
		"highest price first": {Filter{HighNominal: &yes}, "purchase_price DESC"},
		"lowest price first":  {Filter{HighNominal: &no}, "purchase_price ASC"},
		"newest first":        {Filter{NewDate: &yes}, "purchase_submission_date DESC"},
		"newDate false":       {Filter{NewDate: &no}, "purchase_submission_date ASC"},
		"price beats newDate": {Filter{HighNominal: &no, NewDate: &yes}, "purchase_price ASC"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := order(tc.filter); got != tc.want {
				t.Errorf("order = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSubtotal covers the line-item arithmetic behind a purchase's price.
func TestSubtotal(t *testing.T) {
	line := model.ResourceTemp{ResourceTotal: 3, ResourcePrice: 50000}
	if got := line.Subtotal(); got != 150000 {
		t.Errorf("Subtotal = %d, want 150000", got)
	}
}

// TestTerminal pins which statuses close a purchase to further edits.
func TestTerminal(t *testing.T) {
	for _, s := range []string{model.StatusSelesai, model.StatusDibatalkan, model.StatusDitolak} {
		if !model.Terminal(s) {
			t.Errorf("Terminal(%q) = false, want true", s)
		}
	}
	for _, s := range []string{model.StatusDiajukan, model.StatusDisetujui, model.StatusDiproses, ""} {
		if model.Terminal(s) {
			t.Errorf("Terminal(%q) = true, want false", s)
		}
	}
}
