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

	"pickcart/internal/boxing"
	"pickcart/internal/domain"
	"pickcart/internal/led"
)

type ShelfLayoutTab struct {
	profile          domain.CartProfile
	led              *led.Controller
	enableBoxCycling bool
	pickItems        []domain.PickItem
	currentIdx       int
	cells            []ShelfCell
	boxes            []domain.Box
	currentBoxIdx    int
	root             fyne.CanvasObject
	gridHolder       *fyne.Container
	quantityText     *canvas.Text
	nameLabel        *widget.Label
	detailsLabel     *widget.Label
	locationText     *canvas.Text
	prevBtn          *widget.Button
	nextBtn          *widget.Button
	boxNameText      *widget.Label
	boxDimsText      *widget.Label
	prevBoxBtn       *widget.Button
	nextBoxBtn       *widget.Button
}

func NewShelfLayoutTab(profile domain.CartProfile, ledController *led.Controller, enableBoxCycling bool) *ShelfLayoutTab {
	t := &ShelfLayoutTab{
		profile:          profile,
		led:              ledController,
		enableBoxCycling: enableBoxCycling,
		currentIdx:       -1,
		currentBoxIdx:    0,
		boxes:            append([]domain.Box(nil), boxing.Boxes...),
		gridHolder:       container.NewMax(widget.NewLabel("No shipments selected")),
		quantityText:     newHeaderText("", 46, true),
		nameLabel:        newWrappedHeaderLabel("No items to pick", theme.SizeNameHeadingText, true),
		detailsLabel:     widget.NewLabel(""),
		locationText:     newHeaderText("", 46, true),
		boxNameText:      newWrappedHeaderLabel("No Boxes", theme.SizeNameHeadingText, true),
		boxDimsText:      newWrappedHeaderLabel("", theme.SizeNameText, false),
	}
	t.detailsLabel.Alignment = fyne.TextAlignCenter
	t.detailsLabel.Wrapping = fyne.TextWrapOff
	t.detailsLabel.TextStyle = fyne.TextStyle{}
	t.detailsLabel.Importance = widget.HighImportance
	t.detailsLabel.SizeName = theme.SizeNameText

	t.prevBtn = widget.NewButton("← Previous", t.ShowPrevious)
	t.prevBtn.Importance = widget.HighImportance
	t.nextBtn = widget.NewButton("Next →", t.ShowNext)
	t.nextBtn.Importance = widget.HighImportance
	t.prevBoxBtn = widget.NewButton("← Previous Box", t.ShowPreviousBox)
	t.prevBoxBtn.Importance = widget.HighImportance
	t.nextBoxBtn = widget.NewButton("Next Box →", t.ShowNextBox)
	t.nextBoxBtn.Importance = widget.HighImportance

	var top fyne.CanvasObject
	if enableBoxCycling {
		top = makeUnifiedHeader(
			t.prevBoxBtn,
			t.quantityText,
			t.nextBoxBtn,
			t.locationText,
			t.boxNameText,
			t.boxDimsText,
		)
	} else {
		top = makeUnifiedHeader(
			t.prevBtn,
			t.quantityText,
			t.nextBtn,
			t.locationText,
			t.nameLabel,
			t.detailsLabel,
		)
	}

	t.root = container.NewBorder(top, nil, nil, nil, wrapWithMargin(container.NewVScroll(t.gridHolder), 14, 10))
	t.updateDisplay()
	return t
}

func (t *ShelfLayoutTab) Object() fyne.CanvasObject {
	if t == nil {
		return widget.NewLabel("")
	}
	return t.root
}

func (t *ShelfLayoutTab) UpdateShipments(cells []ShelfCell) {
	t.cells = append([]ShelfCell(nil), cells...)
	t.updateDisplay()
}

func (t *ShelfLayoutTab) UpdatePickItems(items []domain.PickItem) {
	t.pickItems = append([]domain.PickItem(nil), items...)
	if len(t.pickItems) == 0 {
		t.currentIdx = -1
	} else if t.currentIdx < 0 || t.currentIdx >= len(t.pickItems) {
		t.currentIdx = 0
	}
	t.updateDisplay()
}

func (t *ShelfLayoutTab) ShowNext() {
	if t.currentIdx >= 0 && t.currentIdx < len(t.pickItems)-1 {
		t.currentIdx++
		t.updateDisplay()
	}
}

func (t *ShelfLayoutTab) ShowPrevious() {
	if t.currentIdx > 0 {
		t.currentIdx--
		t.updateDisplay()
	}
}

func (t *ShelfLayoutTab) ShowNextBox() {
	if t.currentBoxIdx < len(t.boxes)-1 {
		t.currentBoxIdx++
		t.updateDisplay()
	}
}

func (t *ShelfLayoutTab) ShowPreviousBox() {
	if t.currentBoxIdx > 0 {
		t.currentBoxIdx--
		t.updateDisplay()
	}
}

func (t *ShelfLayoutTab) updateDisplay() {
	if t == nil {
		return
	}

	t.applyResponsiveHeaderScale()

	if t.enableBoxCycling {
		setHeaderText(t.quantityText, "")
		setHeaderText(t.locationText, "")
		if len(t.boxes) == 0 {
			setHeaderLabelText(t.boxNameText, "No Boxes")
			setHeaderLabelText(t.boxDimsText, "")
			t.prevBoxBtn.Disable()
			t.nextBoxBtn.Disable()
		} else {
			if t.currentBoxIdx < 0 || t.currentBoxIdx >= len(t.boxes) {
				t.currentBoxIdx = 0
			}
			box := t.boxes[t.currentBoxIdx]
			setHeaderLabelText(t.boxNameText, box.Name)
			setHeaderLabelText(t.boxDimsText, fmt.Sprintf("%.3fft³ • %.3f×%.3f×%.3f in", box.Volume, box.Length, box.Width, box.Height))
			if t.currentBoxIdx <= 0 {
				t.prevBoxBtn.Disable()
			} else {
				t.prevBoxBtn.Enable()
			}
			if t.currentBoxIdx >= len(t.boxes)-1 {
				t.nextBoxBtn.Disable()
			} else {
				t.nextBoxBtn.Enable()
			}
		}
		t.rebuildGrid()
		return
	}

	if len(t.pickItems) == 0 || t.currentIdx < 0 || t.currentIdx >= len(t.pickItems) {
		setHeaderText(t.quantityText, "")
		setHeaderLabelText(t.nameLabel, "No items to pick")
		setHeaderLabelText(t.detailsLabel, "")
		setHeaderText(t.locationText, "")
		t.prevBtn.Disable()
		t.nextBtn.Disable()
		t.rebuildGrid()
		t.updateLEDs(nil)
		return
	}

	current := t.pickItems[t.currentIdx]
	location := current.Location
	if location == "" && len(current.Shipments) > 0 {
		location = current.Shipments[0].Location
	}

	setHeaderText(t.quantityText, fmt.Sprintf("Qty %d", current.Quantity))
	setHeaderLabelText(t.nameLabel, current.Name)
	setHeaderLabelText(t.detailsLabel, formatPickDebugLine(current))
	setHeaderText(t.locationText, fallbackLocation(location))
	if t.currentIdx <= 0 {
		t.prevBtn.Disable()
	} else {
		t.prevBtn.Enable()
	}
	if t.currentIdx >= len(t.pickItems)-1 {
		t.nextBtn.Disable()
	} else {
		t.nextBtn.Enable()
	}

	t.rebuildGrid()
	t.updateLEDs(&current)
}

func (t *ShelfLayoutTab) rebuildGrid() {
	rows := locationRows(t.profile)
	gridScale := gridScaleForWidth(t.root.Size().Width, maxColumnsForRows(rows))
	cellByLocation := make(map[string]ShelfCell, len(t.cells))
	for _, cell := range t.cells {
		cellByLocation[cell.Location] = cell
	}

	qtyByExternal := map[string]int{}
	currentLocation := ""
	selectedBox := ""
	if t.enableBoxCycling {
		if len(t.boxes) > 0 && t.currentBoxIdx >= 0 && t.currentBoxIdx < len(t.boxes) {
			selectedBox = t.boxes[t.currentBoxIdx].Name
		}
	} else if t.currentIdx >= 0 && t.currentIdx < len(t.pickItems) {
		current := t.pickItems[t.currentIdx]
		currentLocation = current.Location
		for _, shipment := range current.Shipments {
			qtyByExternal[shipment.ExternalID] = shipment.Quantity
		}
	}

	rowObjects := make([]fyne.CanvasObject, 0, len(rows))
	for _, row := range rows {
		cellObjects := make([]fyne.CanvasObject, 0, len(row))
		for _, location := range row {
			cell, ok := cellByLocation[location]
			fill := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
			centerText := ""
			footerText := ""
			if ok {
				if t.enableBoxCycling {
					centerText = cell.BoxName
					if selectedBox != "" && cell.BoxName == selectedBox {
						fill = color.NRGBA{R: 0x90, G: 0xee, B: 0x90, A: 0xff}
					}
				} else {
					centerText = compactShipmentDisplayID(cell.ExternalID)
					qty := qtyByExternal[cell.ExternalID]
					if qty > 0 {
						fill = quantityFillColor(qty)
						footerText = fmt.Sprintf("%d", qty)
					}
					if currentLocation != "" && location == currentLocation && qty == 0 {
						fill = color.NRGBA{R: 0xdd, G: 0xee, B: 0xff, A: 0xff}
					}
				}
			}
			cellObjects = append(cellObjects, makeGridCell(location, centerText, footerText, fill, gridScale))
		}
		rowObjects = append(rowObjects, container.NewGridWithColumns(len(cellObjects), cellObjects...))
	}

	t.gridHolder.Objects = []fyne.CanvasObject{container.NewVBox(rowObjects...)}
	t.gridHolder.Refresh()
}

func (t *ShelfLayoutTab) updateLEDs(item *domain.PickItem) {
	if t == nil || t.led == nil {
		return
	}
	if item == nil {
		t.led.ClearLEDs()
		return
	}

	cellByShipmentID := make(map[string]string, len(t.cells))
	cellByExternalID := make(map[string]string, len(t.cells))
	for _, cell := range t.cells {
		if location := strings.TrimSpace(cell.Location); location != "" {
			if shipmentID := strings.TrimSpace(cell.ShipmentID); shipmentID != "" {
				cellByShipmentID[shipmentID] = location
			}
			if externalID := strings.TrimSpace(cell.ExternalID); externalID != "" {
				cellByExternalID[externalID] = location
			}
		}
	}

	locationColors := make(map[string][3]byte, len(item.Shipments))
	for _, shipment := range item.Shipments {
		location := firstNonEmpty(
			cellByShipmentID[strings.TrimSpace(shipment.ShipmentID)],
			cellByExternalID[strings.TrimSpace(shipment.ExternalID)],
		)
		if location == "" {
			continue
		}
		locationColors[location] = quantityLEDColor(shipment.Quantity)
	}
	if len(locationColors) == 0 {
		t.led.ClearLEDs()
		return
	}

	t.led.ClearLEDs()
	t.led.HighlightLocationColors(locationColors)
}

func (t *ShelfLayoutTab) refreshResponsiveLayout() {
	if t == nil {
		return
	}
	t.applyResponsiveHeaderScale()
	t.rebuildGrid()
}

func (t *ShelfLayoutTab) applyResponsiveHeaderScale() {
	if t == nil {
		return
	}

	scale := headerScaleForWidth(t.root.Size().Width)
	t.quantityText.TextSize = 46 * scale
	t.locationText.TextSize = 46 * scale
	for _, text := range []*canvas.Text{t.quantityText, t.locationText} {
		if text != nil {
			text.Refresh()
		}
	}
	applyHeaderLabelScale(scale, t.nameLabel, t.detailsLabel)
	applyHeaderLabelScale(scale, t.boxNameText, t.boxDimsText)
}

func formatDimensions(item domain.PickItem) string {
	if item.Length <= 0 && item.Width <= 0 && item.Height <= 0 {
		return ""
	}
	return fmt.Sprintf("Dimensions %.2f × %.2f × %.2f", item.Length, item.Width, item.Height)
}

func formatQtyAvailable(qty float64) string {
	if qty < 0 {
		return ""
	}
	if qty == float64(int64(qty)) {
		return fmt.Sprintf("Available %d", int64(qty))
	}
	return fmt.Sprintf("Available %.2f", qty)
}

func formatPickDebugLine(item domain.PickItem) string {
	parts := make([]string, 0, 3)
	if sku := strings.TrimSpace(item.SKU); sku != "" {
		parts = append(parts, sku)
	}
	if dimensions := strings.TrimSpace(formatDimensions(item)); dimensions != "" {
		parts = append(parts, dimensions)
	}
	if available := strings.TrimSpace(formatQtyAvailable(item.QtyAvailable)); available != "" {
		parts = append(parts, available)
	}
	return strings.Join(parts, " • ")
}
