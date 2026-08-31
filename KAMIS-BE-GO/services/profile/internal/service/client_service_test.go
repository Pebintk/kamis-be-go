package service

import "testing"

func TestWithinProfit(t *testing.T) {
	ptr := func(v int64) *int64 { return &v }

	cases := []struct {
		name     string
		profit   int64
		min, max *int64
		want     bool
	}{
		{"no bounds", 500, nil, nil, true},
		{"above min", 500, ptr(100), nil, true},
		{"below min", 50, ptr(100), nil, false},
		{"at min", 100, ptr(100), nil, true},
		{"below max", 500, nil, ptr(1000), true},
		{"above max", 5000, nil, ptr(1000), false},
		{"at max", 1000, nil, ptr(1000), true},
		{"inside range", 500, ptr(100), ptr(1000), true},
		{"outside range", 5000, ptr(100), ptr(1000), false},
		{"negative profit under min", -100, ptr(0), nil, false},
	}
	for _, tc := range cases {
		if got := withinProfit(tc.profit, tc.min, tc.max); got != tc.want {
			t.Errorf("%s: withinProfit(%d) = %v, want %v", tc.name, tc.profit, got, tc.want)
		}
	}
}
