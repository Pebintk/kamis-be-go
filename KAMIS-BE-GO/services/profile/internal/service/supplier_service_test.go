package service

import (
	"testing"

	"github.com/pebintk/kamis-be-go/services/profile/internal/dto"
)

// TestFormatRupiah pins the Indonesian grouping that
// NumberFormat.getNumberInstance(new Locale("id","ID")) produces in the Java
// service, since the string ends up in user-visible activity labels.
func TestFormatRupiah(t *testing.T) {
	cases := []struct{ in, want any }{
		{0, "0"},
		{7, "7"},
		{999, "999"},
		{1000, "1.000"},
		{1500000, "1.500.000"},
		{123456789, "123.456.789"},
		{-2500, "-2.500"},
	}
	for _, tc := range cases {
		if got := formatRupiah(tc.in.(int)); got != tc.want {
			t.Errorf("formatRupiah(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestActivityName(t *testing.T) {
	price := 1500000
	got := activityName(dto.PurchaseResponse{PurchaseType: "Aset", PurchasePrice: &price})
	if want := "Pembelian Aset seharga Rp1.500.000"; got != want {
		t.Errorf("activityName = %q, want %q", got, want)
	}

	// A null price must not panic; the Java code would have thrown here.
	if got := activityName(dto.PurchaseResponse{PurchaseType: "Aset"}); got != "Pembelian Aset seharga Rp0" {
		t.Errorf("activityName with nil price = %q", got)
	}
}
