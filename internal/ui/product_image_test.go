package ui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"pickcart/internal/domain"
)

// A 120x120 lossy WebP produced by cwebp. Go has no WebP encoder, so the
// format needs a fixture to be covered here.
const webpFixture = "UklGRsAAAABXRUJQVlA4ILQAAACwCQCdASp4AHgAPpFAnEolo6KhqJ8IALASCWVu3V0upX+M2z4ca/HL5/2fsGlrSO+/uTo6JjzcQhlgTmTKhJKL/rAlxMWySUTKhkh6YdE8xZaqu3F8AAD+y9eJIk6WyKvLDTGdMZ/eXvzun/6pGnlt9fLHPLkH+XL4a88Dr94KrE9+eoTmcF3ETwjRWaqGE5lcjC+jAuqjqcwRmKwIHhMNotD5/80hMDrmBbEVVLmaIzFAAAA="

func testProductImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 120, 120))
	for y := range 120 {
		for x := range 120 {
			img.Set(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 2), B: 0x80, A: 0xff})
		}
	}
	return img
}

// Odoo stores product photos in whatever format was uploaded, so every format
// the app registers a decoder for must survive the round trip.
func TestDecodeProductImageFormats(t *testing.T) {
	src := testProductImage()
	encoders := map[string]func(*bytes.Buffer) error{
		"png":  func(b *bytes.Buffer) error { return png.Encode(b, src) },
		"jpeg": func(b *bytes.Buffer) error { return jpeg.Encode(b, src, nil) },
		"gif":  func(b *bytes.Buffer) error { return gif.Encode(b, src, nil) },
		"bmp":  func(b *bytes.Buffer) error { return bmp.Encode(b, src) },
		"tiff": func(b *bytes.Buffer) error { return tiff.Encode(b, src, nil) },
	}

	for name, encode := range encoders {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := encode(&buf); err != nil {
				t.Fatalf("encode %s: %v", name, err)
			}
			got := decodeProductImage(domain.PickItem{Name: "Rainbow", Image: buf.Bytes()})
			if got == nil {
				t.Fatalf("decodeProductImage(%s) = nil, want an image", name)
			}
			if size := got.Bounds().Size(); size.X != 120 || size.Y != 120 {
				t.Fatalf("decoded %s size = %v, want 120x120", name, size)
			}
		})
	}

	t.Run("webp", func(t *testing.T) {
		data, err := base64.StdEncoding.DecodeString(webpFixture)
		if err != nil {
			t.Fatal(err)
		}
		got := decodeProductImage(domain.PickItem{Name: "Rainbow", Image: data})
		if got == nil {
			t.Fatal("decodeProductImage(webp) = nil, want an image")
		}
		if size := got.Bounds().Size(); size.X != 120 || size.Y != 120 {
			t.Fatalf("decoded webp size = %v, want 120x120", size)
		}
	})
}

func TestDecodeProductImageRejectsUnusableData(t *testing.T) {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, testProductImage()); err != nil {
		t.Fatal(err)
	}

	cases := map[string][]byte{
		"empty":          nil,
		"unknown format": []byte("this is not an image at all"),
		"truncated":      pngBuf.Bytes()[:40],
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if got := decodeProductImage(domain.PickItem{Name: "Rainbow", Image: data}); got != nil {
				t.Fatalf("decodeProductImage(%s) = %v, want nil", name, got.Bounds())
			}
		})
	}
}
