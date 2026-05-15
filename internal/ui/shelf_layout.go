package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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
	nameText         *canvas.Text
	skuText          *canvas.Text
	locationText     *canvas.Text
	dimensionsText   *canvas.Text
	qtyAvailText     *canvas.Text
	prevBtn          *widget.Button
	nextBtn          *widget.Button
	boxNameText      *canvas.Text
	boxDimsText      *canvas.Text
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
		nameText:         newHeaderText("No items to pick", 52, true),
		skuText:          newHeaderText("", 18, false),
		locationText:     newHeaderText("", 46, true),
		dimensionsText:   newHeaderText("", 18, false),
		qtyAvailText:     newHeaderText("", 18, false),
		boxNameText:      newHeaderText("No Boxes", 52, true),
		boxDimsText:      newHeaderText("", 18, false),
	}

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
			t.nameText,
			t.skuText,
			t.dimensionsText,
			t.qtyAvailText,
		)
	}

	t.root = container.NewBorder(top, nil, nil, nil, container.NewVScroll(t.gridHolder))
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

	if t.enableBoxCycling {
		setHeaderText(t.quantityText, "")
		setHeaderText(t.locationText, "")
		if len(t.boxes) == 0 {
			setHeaderText(t.boxNameText, "No Boxes")
			setHeaderText(t.boxDimsText, "")
			t.prevBoxBtn.Disable()
			t.nextBoxBtn.Disable()
		} else {
			if t.currentBoxIdx < 0 || t.currentBoxIdx >= len(t.boxes) {
				t.currentBoxIdx = 0
			}
			box := t.boxes[t.currentBoxIdx]
			setHeaderText(t.boxNameText, box.Name)
			setHeaderText(t.boxDimsText, fmt.Sprintf("%.3fft³ • %.3f×%.3f×%.3f in", box.Volume, box.Length, box.Width, box.Height))
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
		setHeaderText(t.nameText, "No items to pick")
		setHeaderText(t.skuText, "")
		setHeaderText(t.locationText, "")
		setHeaderText(t.dimensionsText, "")
		setHeaderText(t.qtyAvailText, "")
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
	setHeaderText(t.nameText, current.Name)
	setHeaderText(t.skuText, current.SKU)
	setHeaderText(t.locationText, fallbackLocation(location))
	setHeaderText(t.dimensionsText, formatDimensions(current))
	setHeaderText(t.qtyAvailText, formatQtyAvailable(current.QtyAvailable))
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
					centerText = cell.ExternalID
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
			cellObjects = append(cellObjects, makeGridCell(location, centerText, footerText, fill))
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

	locations := make([]string, 0, len(item.Shipments))
	for _, shipment := range item.Shipments {
		if location := strings.TrimSpace(shipment.Location); location != "" {
			locations = append(locations, location)
		}
	}
	if len(locations) == 0 && strings.TrimSpace(item.Location) != "" {
		locations = append(locations, item.Location)
	}
	if len(locations) == 0 {
		t.led.ClearLEDs()
		return
	}

	t.led.ClearLEDs()
	t.led.HighlightLocations(locations, [3]byte{0, 255, 0})
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
