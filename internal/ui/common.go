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

	locationLabel := widget.NewLabel(location)
	locationLabel.Alignment = fyne.TextAlignCenter
	locationLabel.Importance = widget.MediumImportance

	centerLabel := widget.NewLabel(centerText)
	centerLabel.Alignment = fyne.TextAlignCenter
	centerLabel.Wrapping = fyne.TextWrapWord
	centerLabel.Importance = widget.HighImportance

	footerLabel := widget.NewLabel(footerText)
	footerLabel.Alignment = fyne.TextAlignCenter
	footerLabel.Importance = widget.MediumImportance

	content := container.NewBorder(locationLabel, footerLabel, nil, nil, centerLabel)
	return container.NewStack(bg, container.NewPadded(content))
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
