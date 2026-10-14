package ui

import "testing"

func TestParseCartNumber(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "valid", input: "3", want: 3},
		{name: "trimmed", input: " 12 ", want: 12},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "text", input: "cart", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseCartNumber(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseCartNumber(%q) = %d, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCartNumber(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseCartNumber(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestProfileNameFromDisplay(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "display name", input: "Small Cart", want: "small_cart"},
		{name: "internal name", input: "medium_cart", want: "medium_cart"},
		{name: "empty", input: " ", wantErr: true},
		{name: "unknown", input: "tiny", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := profileNameFromDisplay(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("profileNameFromDisplay(%q) = %q, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("profileNameFromDisplay(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("profileNameFromDisplay(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolveLaunchSettings(t *testing.T) {
	t.Parallel()

	got := resolveLaunchSettings(nil, LaunchSettings{})
	if got.CartNumber != 1 || got.ProfileName != "standard" {
		t.Fatalf("resolveLaunchSettings(nil, empty) = %+v, want cart 1 standard", got)
	}

	got = resolveLaunchSettings(nil, LaunchSettings{CartNumber: 4, ProfileName: "small_cart"})
	if got.CartNumber != 4 || got.ProfileName != "small_cart" {
		t.Fatalf("resolveLaunchSettings(nil, flags) = %+v, want cart 4 small_cart", got)
	}

	got = resolveLaunchSettings(nil, LaunchSettings{CartNumber: 2, ProfileName: "missing"})
	if got.ProfileName != "standard" {
		t.Fatalf("resolveLaunchSettings(nil, unknown profile) = %q, want standard", got.ProfileName)
	}
}
