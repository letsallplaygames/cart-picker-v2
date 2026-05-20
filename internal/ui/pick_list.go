package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"pickcart/internal/domain"
)

const (
	pickListColumnProduct  = 0
	pickListColumnQuantity = 1
	pickListColumnLocation = 2
)

type PickListTab struct {
	items         []domain.PickItem
	table         *widget.Table
	status        *widget.Label
	shipmentCount int
	root          fyne.CanvasObject
}

func NewPickListTab() *PickListTab {
	t := &PickListTab{
		status: widget.NewLabel("Total items: 0, Shipments: 0"),
	}
	t.status.Importance = widget.HighImportance

	t.table = widget.NewTable(
		func() (int, int) {
			return len(t.items) + 1, 3
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextWrapOff
			return label
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			label.TextStyle = fyne.TextStyle{}
			label.Alignment = fyne.TextAlignLeading

			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				switch id.Col {
				case pickListColumnProduct:
					label.SetText("Product")
					label.Alignment = fyne.TextAlignCenter
				case pickListColumnQuantity:
					label.SetText("Qty")
					label.Alignment = fyne.TextAlignCenter
				case pickListColumnLocation:
					label.SetText("Location")
					label.Alignment = fyne.TextAlignCenter
				default:
					label.SetText("")
				}
				label.Refresh()
				return
			}

			itemIdx := id.Row - 1
			if itemIdx < 0 || itemIdx >= len(t.items) {
				label.SetText("")
				label.Refresh()
				return
			}

			item := t.items[itemIdx]
			switch id.Col {
			case pickListColumnProduct:
				label.SetText(item.Name)
				label.Alignment = fyne.TextAlignLeading
			case pickListColumnQuantity:
				label.SetText(fmt.Sprintf("%d", item.Quantity))
				label.Alignment = fyne.TextAlignCenter
			case pickListColumnLocation:
				label.SetText(fallbackLocation(item.Location))
				label.Alignment = fyne.TextAlignLeading
			default:
				label.SetText("")
			}
			label.Refresh()
		},
	)
	t.table.SetColumnWidth(pickListColumnProduct, 420)
	t.table.SetColumnWidth(pickListColumnQuantity, 90)
	t.table.SetColumnWidth(pickListColumnLocation, 150)

	t.root = container.NewPadded(container.NewBorder(t.status, nil, nil, nil, t.table))
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
	t.shipmentCount = countUniqueShipments(items)
	t.status.SetText(fmt.Sprintf("Total items: %d, Shipments: %d", totalPickQuantity(items), t.shipmentCount))
	t.table.Refresh()
}

func totalPickQuantity(items []domain.PickItem) int {
	total := 0
	for _, item := range items {
		total += item.Quantity
	}
	return total
}

func countUniqueShipments(items []domain.PickItem) int {
	seen := map[string]struct{}{}
	for _, item := range items {
		for _, shipment := range item.Shipments {
			if shipment.ShipmentID == "" {
				continue
			}
			seen[shipment.ShipmentID] = struct{}{}
		}
	}
	return len(seen)
}
