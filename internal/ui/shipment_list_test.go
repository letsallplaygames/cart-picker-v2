package ui

import "testing"

func TestBatchMatchesSearch(t *testing.T) {
	cases := []struct {
		name  string
		batch string
		query string
		want  bool
	}{
		{"empty query", "BATCH/KST/00684", "", true},
		{"exact match", "BATCH/KST/00684", "KST", true},
		{"case insensitive", "BATCH/KST/00684", "kst", true},
		{"partial match", "BATCH/LNX/00123", "00123", true},
		{"no match", "BATCH/KST/00684", "LNX", false},
		{"whitespace query", "BATCH/KST/00684", "  ", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := batchMatchesSearch(tc.batch, tc.query); got != tc.want {
				t.Fatalf("batchMatchesSearch(%q, %q) = %v, want %v", tc.batch, tc.query, got, tc.want)
			}
		})
	}
}
