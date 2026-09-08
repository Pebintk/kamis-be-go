package repository

import (
	"testing"

	"github.com/pebintk/kamis-be-go/services/project/internal/model"
)

// TestIDPrefix pins the letter a project id opens with, which is how the
// frontend tells the two kinds apart in a list.
func TestIDPrefix(t *testing.T) {
	if got := idPrefix(model.TypePengiriman); got != "D" {
		t.Errorf("distribution prefix = %q, want D", got)
	}
	if got := idPrefix(model.TypePenjualan); got != "P" {
		t.Errorf("sale prefix = %q, want P", got)
	}
}
