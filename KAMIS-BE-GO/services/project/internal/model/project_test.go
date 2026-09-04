package model

import "testing"

func ptr64(v int64) *int64 { return &v }

// TestProfit covers the arithmetic behind the list's projectProfit column,
// including the two cases that are not subtraction.
func TestProfit(t *testing.T) {
	// A distribution: income less what the job cost.
	distribution := Project{ProjectTotalPemasukkan: ptr64(1000), ProjectTotalPengeluaran: ptr64(400)}
	if got := distribution.Profit(); got == nil || *got != 600 {
		t.Errorf("profit = %v, want 600", got)
	}

	// A sale has no outgoings, so its profit is its income.
	sale := Project{ProjectTotalPemasukkan: ptr64(1000)}
	if got := sale.Profit(); got == nil || *got != 1000 {
		t.Errorf("profit with no outgoings = %v, want 1000", got)
	}

	// Unknown income means unknown profit, not zero.
	if got := (Project{ProjectTotalPengeluaran: ptr64(400)}).Profit(); got != nil {
		t.Errorf("profit with no income = %v, want nil", *got)
	}

	// A loss is a real answer.
	loss := Project{ProjectTotalPemasukkan: ptr64(100), ProjectTotalPengeluaran: ptr64(400)}
	if got := loss.Profit(); got == nil || *got != -300 {
		t.Errorf("loss = %v, want -300", got)
	}
}

// TestActive pins which statuses still hold vehicles and stock.
func TestActive(t *testing.T) {
	for _, status := range []int{StatusDirencanakan, StatusDilaksanakan} {
		if !(Project{ProjectStatus: status}).Active() {
			t.Errorf("status %d should be active", status)
		}
	}
	for _, status := range []int{StatusSelesai, StatusBatal} {
		if (Project{ProjectStatus: status}).Active() {
			t.Errorf("status %d should not be active", status)
		}
	}
}

func TestUsageArithmetic(t *testing.T) {
	asset := ProjectAssetUsage{AssetUseCost: 300, AssetFuelCost: 150}
	if got := asset.Cost(); got != 450 {
		t.Errorf("asset cost = %d, want 450", got)
	}

	resource := ProjectResourceUsage{SellPrice: 25000, QuantityUsed: 4}
	if got := resource.Revenue(); got != 100000 {
		t.Errorf("resource revenue = %d, want 100000", got)
	}
}
