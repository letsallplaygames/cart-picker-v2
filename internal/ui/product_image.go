package ui

import (
	"bytes"
	"image"
	_ "image/gif"  // Odoo keeps whatever format was uploaded, so register the
	_ "image/jpeg" // formats Go can decode rather than relying on Fyne, which
	_ "image/png"  // only registers JPEG and PNG.
	"log/slog"
	"net/http"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"pickcart/internal/domain"
)

// decodeProductImage decodes an Odoo product photo for display, reporting an
// undecodable image against the product it belongs to. Decoding here rather
// than letting Fyne read the raw bytes keeps an unsupported format from
// surfacing as an unattributed "Failed to load image" render error.
func decodeProductImage(item domain.PickItem) image.Image {
	if len(item.Image) == 0 {
		return nil
	}
	decoded, _, err := image.Decode(bytes.NewReader(item.Image))
	if err != nil {
		slog.Warn("skipping undecodable product image",
			"name", item.Name,
			"sku", item.SKU,
			"content_type", http.DetectContentType(item.Image),
			"bytes", len(item.Image),
			"error", err,
		)
		return nil
	}
	return decoded
}
