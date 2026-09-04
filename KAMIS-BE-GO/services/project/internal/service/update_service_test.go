package service

import (
	"strings"
	"testing"
)

// TestChangeLog covers the audit entry an edit records: only what actually
// changed, and a readable fallback when nothing did.
func TestChangeLog(t *testing.T) {
	var empty changeLog
	if got := empty.String(); got != "Memperbarui Proyek" {
		t.Errorf("empty log = %q, want the bare heading", got)
	}

	var changes changeLog
	changes.text("deskripsi", "lama", "baru")
	changes.text("alamat pengiriman", "Jl. A", "Jl. A") // unchanged
	changes.text("alamat penjemputan", "Jl. B", "")     // omitted

	got := changes.String()
	if !strings.Contains(got, "Mengubah deskripsi menjadi: baru") {
		t.Errorf("log %q is missing the description change", got)
	}
	if strings.Contains(got, "alamat pengiriman") {
		t.Errorf("log %q recorded a field that did not change", got)
	}
	if strings.Contains(got, "alamat penjemputan") {
		t.Errorf("log %q recorded a field that was not submitted", got)
	}
	if lines := strings.Count(got, "\n"); lines != 1 {
		t.Errorf("log %q has %d change lines, want 1", got, lines)
	}
}

// TestOrExisting covers the omitted-field rule: what is not sent is left alone
// rather than cleared.
func TestOrExisting(t *testing.T) {
	stored, sent := 10, 20

	if got := orExisting(&sent, &stored); *got != 20 {
		t.Errorf("submitted value = %d, want 20", *got)
	}
	if got := orExisting(nil, &stored); *got != 10 {
		t.Errorf("omitted value = %d, want the stored 10", *got)
	}
	if got := orExisting[int](nil, nil); got != nil {
		t.Errorf("both absent = %v, want nil", *got)
	}
}
