package services

import "testing"

func TestRentDueFor(t *testing.T) {
	cases := []struct {
		name   string
		cycle  string
		anchor string
		month  string
		want   float64
	}{
		{"monthly always charged", "月度", "2026-01", "2026-05", 1000},
		{"monthly without anchor", "月度", "", "2026-05", 1000},
		{"quarterly first month", "季度", "2026-01", "2026-01", 3000},
		{"quarterly second month", "季度", "2026-01", "2026-02", 0},
		{"quarterly third month", "季度", "2026-01", "2026-03", 0},
		{"quarterly next cycle", "季度", "2026-01", "2026-04", 3000},
		{"quarterly across year", "季度", "2025-11", "2026-02", 3000},
		{"anchor from full check-in date", "季度", "2026-02-15", "2026-05", 3000},
		{"half-year off cycle", "半年", "2026-03", "2026-08", 0},
		{"half-year on cycle", "半年", "2026-03", "2026-09", 6000},
		{"yearly anniversary", "年度", "2025-07", "2026-07", 12000},
		{"before move-in", "季度", "2026-06", "2026-03", 0},
		{"unknown anchor falls back to charging", "季度", "", "2026-03", 3000},
	}
	for _, tc := range cases {
		anchor := tc.anchor
		if len(anchor) > 7 {
			anchor = anchor[:7]
		}
		if got := RentDueFor(1000, tc.cycle, anchor, tc.month); got != tc.want {
			t.Errorf("%s: RentDueFor(1000, %s, %q, %s) = %v, want %v", tc.name, tc.cycle, tc.anchor, tc.month, got, tc.want)
		}
	}
}

func TestCollectionMonthFor(t *testing.T) {
	cases := []struct {
		cycle, anchor, month, want string
	}{
		{"月度", "2026-01", "2026-05", "2026-05"},
		{"季度", "2026-01", "2026-01", "2026-01"},
		{"季度", "2026-01", "2026-02", "2026-04"},
		{"季度", "2026-01", "2026-03", "2026-04"},
		{"季度", "2026-01", "2026-04", "2026-04"},
		{"季度", "2025-11", "2025-12", "2026-02"},
		{"半年", "2026-03", "2026-04", "2026-09"},
		{"半年", "2026-03", "2026-09", "2026-09"},
		{"年度", "2025-07", "2025-08", "2026-07"},
		{"季度", "2026-06", "2026-03", "2026-03"}, // before move-in: collected in its own month
		{"季度", "", "2026-03", "2026-03"},
	}
	for _, tc := range cases {
		if got := CollectionMonthFor(tc.cycle, tc.anchor, tc.month); got != tc.want {
			t.Errorf("CollectionMonthFor(%s, %q, %s) = %s, want %s", tc.cycle, tc.anchor, tc.month, got, tc.want)
		}
	}
}
