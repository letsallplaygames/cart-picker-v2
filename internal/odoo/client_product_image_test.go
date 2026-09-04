package odoo

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestSelectImageField(t *testing.T) {
	binary := map[string]any{"type": "binary"}
	cases := []struct {
		name   string
		fields map[string]any
		want   string
	}{
		{"prefers smallest thumbnail", map[string]any{"image_1920": binary, "image_128": binary, "image_256": binary}, "image_256"},
		{"falls back to next small size", map[string]any{"image_1920": binary, "image_512": binary}, "image_512"},
		{"never uses full resolution", map[string]any{"image_1920": binary}, ""},
		{"ignores non binary fields", map[string]any{"image_256": map[string]any{"type": "char"}}, ""},
		{"no image fields", map[string]any{"name": map[string]any{"type": "char"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectImageField(tc.fields); got != tc.want {
				t.Fatalf("selectImageField() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestImageFromSelectedField(t *testing.T) {
	payload := []byte{0x89, 'P', 'N', 'G'}
	values := map[string]any{
		"image_256": base64.StdEncoding.EncodeToString(payload),
		"image_128": "not-base64!!",
		"image_512": "",
		"image_bad": false,
	}

	if got := imageFromSelectedField(values, "image_256"); !bytes.Equal(got, payload) {
		t.Fatalf("decoded image = %v, want %v", got, payload)
	}
	for _, field := range []string{"image_128", "image_512", "image_bad", "missing", ""} {
		if got := imageFromSelectedField(values, field); got != nil {
			t.Fatalf("imageFromSelectedField(%q) = %v, want nil", field, got)
		}
	}
}
