package odoo

import "testing"

func TestLenexaPickingTypeIDs(t *testing.T) {
	ids := lenexaPickingTypeIDs()
	if len(ids) != 2 {
		t.Fatalf("lenexaPickingTypeIDs() len = %d, want 2", len(ids))
	}
	if ids[0] != lenexaPickingTypeID {
		t.Fatalf("ids[0] = %v, want %d", ids[0], lenexaPickingTypeID)
	}
	if ids[1] != lenexaKSPickingTypeID {
		t.Fatalf("ids[1] = %v, want %d", ids[1], lenexaKSPickingTypeID)
	}
}
