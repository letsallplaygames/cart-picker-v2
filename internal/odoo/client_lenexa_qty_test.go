package odoo

import "testing"

func TestIsLenexaWarehouseLocation(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"lenexa stock", "LNX/STOCK/A/018/3", true},
		{"lowercase", "lnx/stock/a/018", true},
		{"other warehouse", "CON/STOCK/A/018/3", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLenexaWarehouseLocation(tc.input); got != tc.want {
				t.Fatalf("isLenexaWarehouseLocation(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestLenexaQtyAvailable(t *testing.T) {
	qtyMap := map[string]float64{
		"SKU1": 12,
		"SKU2": 0,
	}

	if got := lenexaQtyAvailable("SKU1", qtyMap); got != 12 {
		t.Fatalf("SKU1 qty = %v, want 12", got)
	}
	if got := lenexaQtyAvailable("SKU2", qtyMap); got != 0 {
		t.Fatalf("SKU2 qty = %v, want 0", got)
	}
	if got := lenexaQtyAvailable("SKU3", qtyMap); got != 0 {
		t.Fatalf("missing SKU qty = %v, want 0", got)
	}
	if got := lenexaQtyAvailable("SKU1", nil); got != -1 {
		t.Fatalf("nil map qty = %v, want -1", got)
	}
}
