package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"pickcart/internal/domain"
)

type PickListTab struct {
	items  []domain.PickItem
	list   *widget.List
	status *widget.Label
	root   fyne.CanvasObject
}

func NewPickListTab() *PickListTab {
	t := &PickListTab{status: widget.NewLabel("No items")}
	t.list = widget.NewList(
		func() int { return len(t.items) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			item := t.items[id]
			obj.(*widget.Label).SetText(fmt.Sprintf("%s • qty %d • %s", item.Name, item.Quantity, fallbackLocation(item.Location)))
		},
	)
	t.root = container.NewBorder(t.status, nil, nil, nil, t.list)
	return t
}

func (t *PickListTab) Object() fyne.CanvasObject {
	if t == nil {
		return widget.NewLabel("")
	}
	return t.root
}

func (t *PickListTab) UpdateItems(items []domain.PickItem) {
	t.items = append([]domain.PickItem(nil), items...)
	t.status.SetText(fmt.Sprintf("Aggregated items: %d", len(t.items)))
	t.list.Refresh()
}
