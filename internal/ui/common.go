package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"pickcart/internal/domain"
)

const initialBatchLoadLimit = 10
const pickNavButtonWidth float32 = 132
const pickNavButtonHeight float32 = 40
const idealGridCellWidth float32 = 72
const idealGridCellHeight float32 = 60
const minGridCellWidth float32 = 24
const minGridCellHeight float32 = 26
const headerCenterMinHeight float32 = 120

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

func quantityLEDColor(quantity int) [3]byte {
	fill := quantityFillColor(quantity)
	return [3]byte{fill.R, fill.G, fill.B}
}

func makeGridCell(location, centerText, footerText string, fill color.NRGBA, scale float32) fyne.CanvasObject {
	scale = clampFloat32(scale, 0.35, 1)

	bg := canvas.NewRectangle(fill)
	bg.SetMinSize(fyne.NewSize(minGridCellWidth, minGridCellHeight))
	bg.StrokeColor = gridCellBorderColor(fill)
	bg.StrokeWidth = clampFloat32(0.45*scale, 0.18, 0.45)

	accentColor := gridCellAccentColor(fill)

	locationLabel := canvas.NewText(location, accentColor)
	locationLabel.Alignment = fyne.TextAlignCenter
	locationLabel.TextSize = 22 * scale
	locationLabel.TextStyle = fyne.TextStyle{Bold: true}

	centerLabel := widget.NewLabel(centerText)
	centerLabel.Alignment = fyne.TextAlignCenter
	centerLabel.Wrapping = fyne.TextWrapWord
	centerLabel.Importance = widget.HighImportance

	footerLabel := canvas.NewText(footerText, accentColor)
	footerLabel.Alignment = fyne.TextAlignCenter
	footerLabel.TextSize = 20 * scale
	footerLabel.TextStyle = fyne.TextStyle{Bold: true}

	content := container.NewBorder(locationLabel, footerLabel, nil, nil, centerLabel)
	return container.NewStack(bg, content)
}

func gridCellAccentColor(fill color.NRGBA) color.Color {
	brightness := (0.299*float64(fill.R) + 0.587*float64(fill.G) + 0.114*float64(fill.B)) / 255
	if brightness >= 0.65 {
		return color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff}
	}
	return color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff}
}

func gridCellBorderColor(fill color.NRGBA) color.Color {
	brightness := (0.299*float64(fill.R) + 0.587*float64(fill.G) + 0.114*float64(fill.B)) / 255
	if brightness >= 0.65 {
		return color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0x88}
	}
	return color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0x66}
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
	centerContent := container.NewVBox(centerLines...)
	centerScroll := container.NewVScroll(centerContent)
	centerScroll.SetMinSize(fyne.NewSize(1, headerCenterMinHeight))
	return wrapWithMargin(container.NewPadded(container.NewBorder(nil, nil, leftPane, rightPane, centerScroll)), 14, 10)
}

func wrapWithMargin(content fyne.CanvasObject, horizontal float32, vertical float32) fyne.CanvasObject {
	left := canvas.NewRectangle(color.Transparent)
	left.SetMinSize(fyne.NewSize(horizontal, 1))
	right := canvas.NewRectangle(color.Transparent)
	right.SetMinSize(fyne.NewSize(horizontal, 1))
	top := canvas.NewRectangle(color.Transparent)
	top.SetMinSize(fyne.NewSize(1, vertical))
	bottom := canvas.NewRectangle(color.Transparent)
	bottom.SetMinSize(fyne.NewSize(1, vertical))
	return container.NewBorder(top, bottom, left, right, content)
}

func headerScaleForWidth(width float32) float32 {
	if width <= 0 {
		return 0.75
	}
	return clampFloat32(width/1400, 0.5, 1)
}

func gridScaleForWidth(width float32, maxColumns int) float32 {
	if maxColumns <= 0 {
		return 0.6
	}
	if width <= 0 {
		return 0.6
	}
	usableWidth := width - 64
	if usableWidth <= 0 {
		return 0.25
	}
	return clampFloat32((usableWidth/float32(maxColumns))/idealGridCellWidth, 0.25, 1)
}

func clampFloat32(value float32, min float32, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func maxColumnsForRows(rows [][]string) int {
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	return maxCols
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
	text := canvas.NewText(value, headerTextColor())
	text.Alignment = fyne.TextAlignCenter
	text.TextSize = size
	text.TextStyle = fyne.TextStyle{Bold: bold}
	return text
}

func headerTextColor() color.Color {
	return theme.Color(theme.ColorNameForeground)
}

func newWrappedHeaderLabel(value string, sizeName fyne.ThemeSizeName, bold bool) *widget.Label {
	label := widget.NewLabel(value)
	label.Alignment = fyne.TextAlignCenter
	label.Wrapping = fyne.TextWrapWord
	label.TextStyle = fyne.TextStyle{Bold: bold}
	label.Importance = widget.HighImportance
	label.SizeName = sizeName
	return label
}

func newHeaderTitleText(value string, size float32) *canvas.Text {
	text := canvas.NewText(value, theme.Color(theme.ColorNamePrimary))
	text.Alignment = fyne.TextAlignCenter
	text.TextSize = size
	text.TextStyle = fyne.TextStyle{Bold: true}
	return text
}

func setHeaderText(text *canvas.Text, value string) {
	if text == nil {
		return
	}
	text.Text = value
	text.Color = headerTextColor()
	text.Refresh()
}

func setHeaderLabelText(label *widget.Label, value string) {
	if label == nil {
		return
	}
	label.SetText(value)
}

func setHeaderTitleText(text *canvas.Text, value string) {
	if text == nil {
		return
	}
	text.Text = value
	text.Color = theme.Color(theme.ColorNamePrimary)
	text.Refresh()
}

func applyHeaderLabelScale(scale float32, primary *widget.Label, secondary ...*widget.Label) {
	if primary != nil {
		switch {
		case scale >= 0.9:
			primary.SizeName = theme.SizeNameHeadingText
		case scale >= 0.7:
			primary.SizeName = theme.SizeNameSubHeadingText
		default:
			primary.SizeName = theme.SizeNameText
		}
		primary.Refresh()
	}
	for _, label := range secondary {
		if label == nil {
			continue
		}
		if scale >= 0.75 {
			label.SizeName = theme.SizeNameText
		} else {
			label.SizeName = theme.SizeNameCaptionText
		}
		label.Refresh()
	}
}

func applyHeaderTitleScale(scale float32, titles ...*canvas.Text) {
	size := clampFloat32(56*scale, 24, 56)
	for _, title := range titles {
		if title == nil {
			continue
		}
		title.TextSize = size
		title.Color = theme.Color(theme.ColorNamePrimary)
		title.Refresh()
	}
}

func compactShipmentDisplayID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.LastIndex(value, "/"); idx >= 0 && idx+1 < len(value) {
		suffix := strings.TrimSpace(value[idx+1:])
		if suffix != "" {
			return suffix
		}
	}
	return value
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
