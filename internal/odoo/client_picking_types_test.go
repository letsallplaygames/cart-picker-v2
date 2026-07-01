package odoo

import "testing"

func TestLenexaPickingTypeIDs(t *testing.T) {
	want := []int{lenexaPickingTypeID, lenexaKSPickingTypeID, lenexaPREPickingTypeID}
	ids := lenexaPickingTypeIDs()
	if len(ids) != len(want) {
		t.Fatalf("lenexaPickingTypeIDs() len = %d, want %d", len(ids), len(want))
	}
	for i, expected := range want {
		if ids[i] != expected {
			t.Fatalf("ids[%d] = %v, want %d", i, ids[i], expected)
		}
	}
}
