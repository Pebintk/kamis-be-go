package model

import "testing"

func ptr64(v int64) *int64 { return &v }

// TestIncomeExpense covers the absent-amount handling every total relies on: a
// sale has no outgoing, a purchase no income, and both must add as zero rather
// than poisoning the sum.
func TestIncomeExpense(t *testing.T) {
	sale := Lapkeu{Pemasukan: ptr64(1000)}
	if sale.Income() != 1000 || sale.Expense() != 0 {
		t.Errorf("sale = %d / %d, want 1000 / 0", sale.Income(), sale.Expense())
	}

	purchase := Lapkeu{Pengeluaran: ptr64(400)}
	if purchase.Income() != 0 || purchase.Expense() != 400 {
		t.Errorf("purchase = %d / %d, want 0 / 400", purchase.Income(), purchase.Expense())
	}

	empty := Lapkeu{}
	if empty.Income() != 0 || empty.Expense() != 0 {
		t.Errorf("empty = %d / %d, want 0 / 0", empty.Income(), empty.Expense())
	}
}

// TestActivityNameCoversPenjualan is the fix for the expense chart, which
// mapped 1, 2 and 3 and let 0 fall through to "UNKNOWN". A sale is recorded with
// an outgoing of zero rather than none, so it passed the chart's
// `pengeluaran IS NOT NULL` filter and appeared as an UNKNOWN slice worth
// nothing.
func TestActivityNameCoversPenjualan(t *testing.T) {
	want := map[int]string{
		ActivityPenjualan:   "Penjualan",
		ActivityDistribusi:  "Distribusi",
		ActivityPurchase:    "Pembelian",
		ActivityMaintenance: "Maintenance",
	}
	for code, name := range want {
		if ActivityName[code] != name {
			t.Errorf("activity %d = %q, want %q", code, ActivityName[code], name)
		}
	}
	if _, known := ActivityName[99]; known {
		t.Error("an unknown activity type must not be nameable")
	}
}
