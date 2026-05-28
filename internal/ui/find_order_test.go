package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/widget"
)

func TestNormalizedSearchQueriesUPS(t *testing.T) {
	queries := normalizedSearchQueries("1Z999AA10123456784")
	if len(queries) != 1 || queries[0] != "1z999aa10123456784" {
		t.Fatalf("UPS query = %#v, want single lowercase full tracking", queries)
	}
}

func TestNormalizedSearchQueriesUSPS(t *testing.T) {
	prefix := "42038106"
	tracking := "9400111899223344556677"
	raw := prefix + tracking
	queries := normalizedSearchQueries(raw)
	foundSuffix := false
	for _, q := range queries {
		if q == strings.ToLower(tracking) {
			foundSuffix = true
		}
		if q == strings.ToLower(raw) {
			t.Fatalf("long USPS barcode should not include full raw query: %#v", queries)
		}
	}
	if !foundSuffix {
		t.Fatalf("expected 8-char-stripped USPS tracking %q in %#v", tracking, queries)
	}
}

func TestNormalizedSearchQueriesFedEx34(t *testing.T) {
	raw := "9240251452585145236258749512003264"
	queries := normalizedSearchQueries(raw)
	want := "749512003264"
	found := false
	for _, q := range queries {
		if q == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected FedEx last-12 query %q in %#v", want, queries)
	}
}

func TestNormalizedSearchQueriesFedEx32(t *testing.T) {
	raw := "32971514560102447849175802862014"
	queries := normalizedSearchQueries(raw)
	want := "784917580286"
	found := false
	for _, q := range queries {
		if q == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected FedEx ASTRA query %q in %#v", want, queries)
	}
}

func TestNormalizedSearchQueriesManualTracking(t *testing.T) {
	raw := "9400111899223344556677"
	queries := normalizedSearchQueries(raw)
	if len(queries) != 1 || queries[0] != raw {
		t.Fatalf("manual tracking query = %#v, want only full value", queries)
	}
}

func TestTrackingMatches(t *testing.T) {
	cases := []struct {
		stored string
		query  string
		want   bool
	}{
		{"986578788855", "986578788855", true},
		{"986578788855", "749512003264", false},
		{"749512003264", "9240251452585145236258749512003264", true},
		{"9400111899223344556677", "9400111899223344556677", true},
	}
	for _, tc := range cases {
		if got := trackingMatches(tc.stored, tc.query); got != tc.want {
			t.Fatalf("trackingMatches(%q, %q) = %v, want %v", tc.stored, tc.query, got, tc.want)
		}
	}
}

func TestLooksLikeBarcodeScan(t *testing.T) {
	if !looksLikeBarcodeScan("1Z999AA10123456784") {
		t.Fatal("UPS scan should look like barcode")
	}
	if looksLikeBarcodeScan("Jane Smith") {
		t.Fatal("customer name should not look like barcode")
	}
	if !looksLikeBarcodeScan("9240251452585145236258749512003264") {
		t.Fatal("FedEx scan should look like barcode")
	}
}

func TestTakeScanInputPrefersBuffer(t *testing.T) {
	tab := &FindOrderTab{
		trackingBuf: "1Z999AA10123456784",
		searchEntry: widget.NewEntry(),
	}
	tab.searchEntry.SetText("stale-label-text")

	got := tab.takeScanInput()
	if got != "1Z999AA10123456784" {
		t.Fatalf("takeScanInput() = %q, want buffer value", got)
	}
	if tab.searchEntry.Text != "" {
		t.Fatalf("search entry = %q, want cleared", tab.searchEntry.Text)
	}
	if tab.trackingBuf != "" {
		t.Fatalf("trackingBuf = %q, want cleared", tab.trackingBuf)
	}
}

func TestTakeScanInputFallsBackToEntry(t *testing.T) {
	tab := &FindOrderTab{searchEntry: widget.NewEntry()}
	tab.searchEntry.SetText("9400111899223344556677")

	got := tab.takeScanInput()
	if got != "9400111899223344556677" {
		t.Fatalf("takeScanInput() = %q, want entry fallback", got)
	}
}
