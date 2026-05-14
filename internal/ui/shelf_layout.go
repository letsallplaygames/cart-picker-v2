package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
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
	quantityLabel    *widget.Label
	nameLabel        *widget.Label
	skuLabel         *widget.Label
	locationLabel    *widget.Label
	prevBtn          *widget.Button
	nextBtn          *widget.Button
	boxNameLabel     *widget.Label
	boxDimsLabel     *widget.Label
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
		quantityLabel:    widget.NewLabel(""),
		nameLabel:        widget.NewLabel("No items to pick"),
		skuLabel:         widget.NewLabel(""),
		locationLabel:    widget.NewLabel(""),
		boxNameLabel:     widget.NewLabel("No Boxes"),
		boxDimsLabel:     widget.NewLabel(""),
	}

	t.quantityLabel.Alignment = fyne.TextAlignCenter
	t.nameLabel.Alignment = fyne.TextAlignCenter
	t.skuLabel.Alignment = fyne.TextAlignCenter
	t.locationLabel.Alignment = fyne.TextAlignCenter
	t.boxNameLabel.Alignment = fyne.TextAlignCenter
	t.boxDimsLabel.Alignment = fyne.TextAlignCenter

	t.prevBtn = widget.NewButton("Previous", t.ShowPrevious)
	t.nextBtn = widget.NewButton("Next", t.ShowNext)
	t.prevBoxBtn = widget.NewButton("Previous Box", func() {
		if t.currentBoxIdx > 0 {
			t.currentBoxIdx--
			t.updateDisplay()
		}
	})
	t.nextBoxBtn = widget.NewButton("Next Box", func() {
		if t.currentBoxIdx < len(t.boxes)-1 {
			t.currentBoxIdx++
			t.updateDisplay()
		}
	})

	var top fyne.CanvasObject
	if enableBoxCycling {
		top = container.NewVBox(
			container.NewHBox(t.prevBoxBtn, t.boxNameLabel, t.boxDimsLabel, t.nextBoxBtn),
		)
	} else {
		top = container.NewVBox(
			container.NewHBox(t.prevBtn, t.quantityLabel, t.nextBtn),
			t.nameLabel,
			t.skuLabel,
			t.locationLabel,
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

func (t *ShelfLayoutTab) updateDisplay() {
	if t == nil {
		return
	}

	if t.enableBoxCycling {
		if len(t.boxes) == 0 {
			t.boxNameLabel.SetText("No Boxes")
			t.boxDimsLabel.SetText("")
			t.prevBoxBtn.Disable()
			t.nextBoxBtn.Disable()
		} else {
			if t.currentBoxIdx < 0 || t.currentBoxIdx >= len(t.boxes) {
				t.currentBoxIdx = 0
			}
			box := t.boxes[t.currentBoxIdx]
			t.boxNameLabel.SetText(box.Name)
			t.boxDimsLabel.SetText(fmt.Sprintf("%.3fft³ • %.3f×%.3f×%.3f in", box.Volume, box.Length, box.Width, box.Height))
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
		t.quantityLabel.SetText("")
		t.nameLabel.SetText("No items to pick")
		t.skuLabel.SetText("")
		t.locationLabel.SetText("")
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

	t.quantityLabel.SetText(fmt.Sprintf("Qty %d", current.Quantity))
	t.nameLabel.SetText(current.Name)
	t.skuLabel.SetText(current.SKU)
	t.locationLabel.SetText(fallbackLocation(location))
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
						footerText = fmt.Sprintf("Qty %d", qty)
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
