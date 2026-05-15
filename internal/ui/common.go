package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"pickcart/internal/domain"
)

const initialBatchLoadLimit = 10
const pickNavButtonWidth float32 = 170
const pickNavButtonHeight float32 = 52

func cartLocations(profile domain.CartProfile) []string {
	locations := []string{}
	rowCount := len(profile.RowConfigs)
	for rowIdx := rowCount - 1; rowIdx >= 0; rowIdx-- {
		rowLetter := string(rune('A' + (rowCount - 1 - rowIdx)))
		for col := 1; col <= profile.RowConfigs[rowIdx].Cols; col++ {
			locations = append(locations, fmt.Sprintf("%s%d", rowLetter, col))
		}
	}
	return locations
}

func locationRows(profile domain.CartProfile) [][]string {
	rows := make([][]string, 0, len(profile.RowConfigs))
	rowCount := len(profile.RowConfigs)
	for rowIdx := 0; rowIdx < rowCount; rowIdx++ {
		rowLetter := string(rune('A' + (rowCount - 1 - rowIdx)))
		cols := profile.RowConfigs[rowIdx].Cols
		locations := make([]string, 0, cols)
		for col := 1; col <= cols; col++ {
			locations = append(locations, fmt.Sprintf("%s%d", rowLetter, col))
		}
		rows = append(rows, locations)
	}
	return rows
}

func quantityFillColor(quantity int) color.NRGBA {
	switch {
	case quantity > 3:
		return color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
	case quantity > 2:
		return color.NRGBA{R: 0x80, G: 0x00, B: 0x80, A: 0xff}
	case quantity > 1:
		return color.NRGBA{R: 0xff, G: 0xa5, B: 0x00, A: 0xff}
	default:
		return color.NRGBA{R: 0x90, G: 0xee, B: 0x90, A: 0xff}
	}
}

func makeGridCell(location, centerText, footerText string, fill color.NRGBA) fyne.CanvasObject {
	bg := canvas.NewRectangle(fill)
	bg.SetMinSize(fyne.NewSize(92, 82))

	accentColor := gridCellAccentColor(fill)

	locationLabel := canvas.NewText(location, accentColor)
	locationLabel.Alignment = fyne.TextAlignCenter
	locationLabel.TextSize = 22
	locationLabel.TextStyle = fyne.TextStyle{Bold: true}

	centerLabel := widget.NewLabel(centerText)
	centerLabel.Alignment = fyne.TextAlignCenter
	centerLabel.Wrapping = fyne.TextWrapWord
	centerLabel.Importance = widget.HighImportance

	footerLabel := canvas.NewText(footerText, accentColor)
	footerLabel.Alignment = fyne.TextAlignCenter
	footerLabel.TextSize = 20
	footerLabel.TextStyle = fyne.TextStyle{Bold: true}

	content := container.NewBorder(locationLabel, footerLabel, nil, nil, centerLabel)
	return container.NewStack(bg, container.NewPadded(content))
}

func gridCellAccentColor(fill color.NRGBA) color.Color {
	brightness := (0.299*float64(fill.R) + 0.587*float64(fill.G) + 0.114*float64(fill.B)) / 255
	if brightness >= 0.65 {
		return color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff}
	}
	return color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff}
}

func makeUnifiedHeader(leftButton *widget.Button, leftValue fyne.CanvasObject, rightButton *widget.Button, rightValue fyne.CanvasObject, centerLines ...fyne.CanvasObject) fyne.CanvasObject {
	leftPane := container.NewVBox(
		container.NewCenter(container.NewGridWrap(fyne.NewSize(pickNavButtonWidth, pickNavButtonHeight), leftButton)),
		container.NewCenter(valueOrSpacer(leftValue)),
	)
	rightPane := container.NewVBox(
		container.NewCenter(container.NewGridWrap(fyne.NewSize(pickNavButtonWidth, pickNavButtonHeight), rightButton)),
		container.NewCenter(valueOrSpacer(rightValue)),
	)
	centerPane := container.NewCenter(container.NewVBox(centerLines...))
	return container.NewPadded(container.NewBorder(nil, nil, leftPane, rightPane, centerPane))
}

func valueOrSpacer(obj fyne.CanvasObject) fyne.CanvasObject {
	if obj != nil {
		return obj
	}
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(1, 48))
	return spacer
}

func newHeaderText(value string, size float32, bold bool) *canvas.Text {
	text := canvas.NewText(value, color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff})
	text.Alignment = fyne.TextAlignCenter
	text.TextSize = size
	text.TextStyle = fyne.TextStyle{Bold: bold}
	return text
}

func setHeaderText(text *canvas.Text, value string) {
	if text == nil {
		return
	}
	text.Text = value
	text.Refresh()
}

func fallbackLocation(location string) string {
	location = strings.TrimSpace(location)
	if location == "" {
		return "UNKN"
	}
	return location
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
