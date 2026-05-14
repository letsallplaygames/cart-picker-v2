package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"pickcart/internal/domain"
)

type ShelfListTab struct {
	items        []domain.PickItem
	currentIdx   int
	quantityText *widget.Label
	nameText     *widget.Label
	locationText *widget.Label
	list         *widget.List
	prevBtn      *widget.Button
	nextBtn      *widget.Button
	root         fyne.CanvasObject
}

func NewShelfListTab() *ShelfListTab {
	t := &ShelfListTab{
		currentIdx:   -1,
		quantityText: widget.NewLabel(""),
		nameText:     widget.NewLabel("No items to pick"),
		locationText: widget.NewLabel(""),
	}
	t.prevBtn = widget.NewButton("Previous", t.ShowPrevious)
	t.nextBtn = widget.NewButton("Next", t.ShowNext)
	t.list = widget.NewList(
		func() int {
			if t.currentIdx < 0 || t.currentIdx >= len(t.items) {
				return 0
			}
			return len(t.items[t.currentIdx].Shipments)
		},
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			shipment := t.items[t.currentIdx].Shipments[id]
			obj.(*widget.Label).SetText(fmt.Sprintf("%s • %s • qty %d", fallbackLocation(shipment.Location), shipment.ExternalID, shipment.Quantity))
		},
	)
	t.root = container.NewBorder(container.NewVBox(t.quantityText, t.nameText, t.locationText, container.NewHBox(t.prevBtn, t.nextBtn)), nil, nil, nil, t.list)
	t.updateButtonStates()
	return t
}

func (t *ShelfListTab) Object() fyne.CanvasObject {
	if t == nil {
		return widget.NewLabel("")
	}
	return t.root
}

func (t *ShelfListTab) UpdateItems(items []domain.PickItem) {
	t.items = append([]domain.PickItem(nil), items...)
	if len(t.items) == 0 {
		t.currentIdx = -1
	} else if t.currentIdx < 0 || t.currentIdx >= len(t.items) {
		t.currentIdx = 0
	}
	t.updateDisplay()
}

func (t *ShelfListTab) ShowNext() {
	if t.currentIdx < len(t.items)-1 {
		t.currentIdx++
		t.updateDisplay()
	}
}

func (t *ShelfListTab) ShowPrevious() {
	if t.currentIdx > 0 {
		t.currentIdx--
		t.updateDisplay()
	}
}

func (t *ShelfListTab) updateDisplay() {
	if t.currentIdx < 0 || t.currentIdx >= len(t.items) {
		t.quantityText.SetText("")
		t.nameText.SetText("No items to pick")
		t.locationText.SetText("")
		t.list.Refresh()
		t.updateButtonStates()
		return
	}

	current := t.items[t.currentIdx]
	t.quantityText.SetText(fmt.Sprintf("Qty %d", current.Quantity))
	t.nameText.SetText(current.Name)
	t.locationText.SetText(fallbackLocation(current.Location))
	t.list.Refresh()
	t.updateButtonStates()
}

func (t *ShelfListTab) updateButtonStates() {
	if t.currentIdx <= 0 {
		t.prevBtn.Disable()
	} else {
		t.prevBtn.Enable()
	}
	if t.currentIdx < 0 || t.currentIdx >= len(t.items)-1 {
		t.nextBtn.Disable()
	} else {
		t.nextBtn.Enable()
	}
}
