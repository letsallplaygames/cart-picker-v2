package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"pickcart/internal/domain"
	"pickcart/internal/picker"
)

const (
	rootNodeID         = ""
	batchNodePrefix    = "batch:"
	shipmentNodePrefix = "shipment:"
)

type ShipmentListTab struct {
	picker          *picker.Picker
	onSelChanged    func()
	tree            *widget.Tree
	loadBtn         *widget.Button
	forceRefreshBtn *widget.Button
	batchCountEntry *widget.Entry
	statusLabel     *widget.Label
	root            fyne.CanvasObject
}

func NewShipmentListTab(p *picker.Picker, onSelChanged func()) *ShipmentListTab {
	t := &ShipmentListTab{
		picker:       p,
		onSelChanged: onSelChanged,
		statusLabel:  widget.NewLabel("No batches loaded"),
	}
	statusRow := container.NewHBox()
	statusRow.Add(t.statusLabel)

	t.tree = widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			if uid == rootNodeID {
				return t.batchNodeIDs()
			}
			if strings.HasPrefix(string(uid), batchNodePrefix) {
				batchID := strings.TrimPrefix(string(uid), batchNodePrefix)
				shipments := t.shipmentsForBatch(batchID)
				nodes := make([]widget.TreeNodeID, 0, len(shipments))
				for _, shipment := range shipments {
					if shipment == nil {
						continue
					}
					nodes = append(nodes, widget.TreeNodeID(shipmentNodePrefix+shipment.ID))
				}
				return nodes
			}
			return nil
		},
		func(uid widget.TreeNodeID) bool {
			return uid == rootNodeID || strings.HasPrefix(string(uid), batchNodePrefix)
		},
		func(branch bool) fyne.CanvasObject {
			check := widget.NewCheck("", nil)
			name := widget.NewLabel("")
			name.Wrapping = fyne.TextWrapOff
			details := widget.NewLabel("")
			details.Wrapping = fyne.TextWrapOff
			created := widget.NewLabel("")
			created.Wrapping = fyne.TextWrapOff
			return container.NewGridWithColumns(4,
				container.NewCenter(check),
				name,
				details,
				created,
			)
		},
		func(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
			row := obj.(*fyne.Container)
			checkWrap := row.Objects[0].(*fyne.Container)
			check := checkWrap.Objects[0].(*widget.Check)
			name := row.Objects[1].(*widget.Label)
			details := row.Objects[2].(*widget.Label)
			created := row.Objects[3].(*widget.Label)

			check.OnChanged = nil
			check.Partial = false
			created.SetText("")
			details.SetText("")

			switch {
			case uid == rootNodeID:
				check.Hide()
				name.SetText("Batches")
				details.SetText("")
				created.SetText("")
			case strings.HasPrefix(string(uid), batchNodePrefix):
				check.Show()
				batchID := strings.TrimPrefix(string(uid), batchNodePrefix)
				batch := t.batchByID(batchID)
				if batch == nil {
					check.Disable()
					name.SetText(batchID)
					return
				}
				selectedCount, totalCount := t.batchSelectionState(batch)
				check.SetChecked(totalCount > 0 && selectedCount == totalCount)
				check.Partial = selectedCount > 0 && selectedCount < totalCount
				check.Enable()
				check.OnChanged = func(checked bool) {
					t.onBatchChecked(batchID, checked)
				}

				name.SetText(batch.Name)
				if batch.ShipmentsLoaded {
					details.SetText(fmt.Sprintf("%d", len(batch.Shipments)))
				} else {
					details.SetText(fmt.Sprintf("%d", len(batch.ShipmentIDs)))
				}
				created.SetText(formatBatchCreatedCT(batch.CreatedAt))
			case strings.HasPrefix(string(uid), shipmentNodePrefix):
				check.Show()
				shipmentID := strings.TrimPrefix(string(uid), shipmentNodePrefix)
				shipment := t.shipmentByID(shipmentID)
				if shipment == nil {
					check.Disable()
					name.SetText(shipmentID)
					return
				}
				check.SetChecked(t.picker != nil && t.picker.IsSelected(shipmentID))
				check.Enable()
				check.OnChanged = func(checked bool) {
					t.onShipmentChecked(shipmentID, checked)
				}

				name.SetText(firstNonEmpty(shipment.ShipTo, shipment.ExternalID, shipment.ID))
				details.SetText(shipmentTreeDetails(shipment))
				created.SetText("")
			}
		},
	)
	t.tree.OnSelected = func(uid widget.TreeNodeID) {
		switch {
		case strings.HasPrefix(string(uid), batchNodePrefix):
			t.onBatchTapped(strings.TrimPrefix(string(uid), batchNodePrefix))
		case strings.HasPrefix(string(uid), shipmentNodePrefix):
			t.onShipmentTapped(strings.TrimPrefix(string(uid), shipmentNodePrefix))
		}
	}
	t.tree.OpenBranch(rootNodeID)

	t.batchCountEntry = widget.NewEntry()
	t.batchCountEntry.SetText(fmt.Sprintf("%d", initialBatchLoadLimit))
	t.batchCountEntry.SetMinRowsVisible(1)

	t.loadBtn = widget.NewButton("Load", func() {
		t.loadBatches(false)
	})
	t.forceRefreshBtn = widget.NewButton("Force Refresh", func() {
		t.loadBatches(true)
	})

	headers := container.NewGridWithColumns(4,
		widget.NewLabel("✓"),
		widget.NewLabel("Name"),
		widget.NewLabel("Count"),
		widget.NewLabel("Created (CT)"),
	)
	controls := container.NewHBox(
		widget.NewLabel("Batches:"),
		t.batchCountEntry,
		t.loadBtn,
		t.forceRefreshBtn,
		statusRow,
	)
	t.root = container.NewBorder(container.NewVBox(controls, headers), nil, nil, nil, t.tree)
	return t
}

func (t *ShipmentListTab) Object() fyne.CanvasObject {
	if t == nil {
		return widget.NewLabel("")
	}
	return t.root
}

func (t *ShipmentListTab) Refresh() {
	if t == nil || t.picker == nil {
		return
	}
	count := 0
	for _, batch := range t.picker.Batches {
		if batch == nil {
			continue
		}
		if batch.ShipmentsLoaded {
			count += len(batch.Shipments)
		} else {
			count += len(batch.ShipmentIDs)
		}
	}
	t.statusLabel.SetText(fmt.Sprintf("Batches: %d • Shipments: %d", len(t.picker.Batches), count))
	t.tree.Refresh()
	t.tree.OpenBranch(rootNodeID)
}

func (t *ShipmentListTab) loadBatches(forceRefresh bool) {
	if t == nil || t.picker == nil {
		return
	}
	limit := t.batchCount()
	if limit < 1 {
		t.statusLabel.SetText("Batch count must be at least 1")
		return
	}

	if forceRefresh {
		t.statusLabel.SetText("Force refreshing batches...")
	} else {
		t.statusLabel.SetText("Loading batches...")
	}
	t.picker.LoadBatches(limit, forceRefresh, func(count int, err error) {
		fyne.Do(func() {
			if err != nil {
				t.statusLabel.SetText(fmt.Sprintf("Load failed: %v", err))
				return
			}
			t.Refresh()
			if forceRefresh {
				t.statusLabel.SetText(fmt.Sprintf("Loaded %d batches (force refresh)", count))
			} else {
				t.statusLabel.SetText(fmt.Sprintf("Loaded %d batches", count))
			}
		})
	})
}

func (t *ShipmentListTab) batchCount() int {
	if t == nil || t.batchCountEntry == nil {
		return initialBatchLoadLimit
	}
	count, err := strconv.Atoi(strings.TrimSpace(t.batchCountEntry.Text))
	if err != nil {
		return 0
	}
	return count
}

func (t *ShipmentListTab) onBatchTapped(batchID string) {
	if t == nil || t.picker == nil {
		return
	}

	batch := t.batchByID(batchID)
	if batch == nil {
		return
	}

	selectedCount, totalCount := t.batchSelectionState(batch)
	targetChecked := !(totalCount > 0 && selectedCount == totalCount)
	t.onBatchChecked(batchID, targetChecked)
}

func (t *ShipmentListTab) onBatchChecked(batchID string, checked bool) {
	if t == nil || t.picker == nil {
		return
	}
	batch := t.batchByID(batchID)
	if batch == nil {
		return
	}
	if !batch.ShipmentsLoaded && checked {
		t.statusLabel.SetText(fmt.Sprintf("Loading shipments for %s...", batch.Name))
		t.picker.LoadBatchShipments(batchID, false, func(shipments []*domain.Shipment, err error) {
			fyne.Do(func() {
				if err != nil {
					t.statusLabel.SetText(fmt.Sprintf("Failed to load shipments: %v", err))
					return
				}
				for _, shipment := range shipments {
					if shipment != nil {
						t.picker.SelectShipment(shipment.ID, shipment)
					}
				}
				if t.onSelChanged != nil {
					t.onSelChanged()
				}
				t.Refresh()
			})
		})
		return
	}

	for _, shipment := range batch.Shipments {
		if shipment == nil {
			continue
		}
		if checked {
			t.picker.SelectShipment(shipment.ID, shipment)
		} else {
			t.picker.DeselectShipment(shipment.ID)
		}
	}
	if t.onSelChanged != nil {
		t.onSelChanged()
	}
	t.Refresh()
}

func (t *ShipmentListTab) onShipmentTapped(shipmentID string) {
	if t == nil || t.picker == nil {
		return
	}
	if t.picker.IsSelected(shipmentID) {
		t.onShipmentChecked(shipmentID, false)
	} else {
		t.onShipmentChecked(shipmentID, true)
	}
}

func (t *ShipmentListTab) onShipmentChecked(shipmentID string, checked bool) {
	if t == nil || t.picker == nil {
		return
	}

	shipment := t.shipmentByID(shipmentID)
	if shipment == nil {
		return
	}

	if checked {
		t.picker.SelectShipment(shipmentID, shipment)
	} else {
		t.picker.DeselectShipment(shipmentID)
	}
	if t.onSelChanged != nil {
		t.onSelChanged()
	}
	t.Refresh()
}

func (t *ShipmentListTab) batchNodeIDs() []widget.TreeNodeID {
	if t == nil || t.picker == nil {
		return nil
	}
	ids := make([]widget.TreeNodeID, 0, len(t.picker.Batches))
	for _, batch := range t.picker.Batches {
		if batch == nil || batch.ID == "" {
			continue
		}
		ids = append(ids, widget.TreeNodeID(batchNodePrefix+batch.ID))
	}
	return ids
}

func (t *ShipmentListTab) batchByID(batchID string) *domain.Batch {
	if t == nil || t.picker == nil {
		return nil
	}
	for _, batch := range t.picker.Batches {
		if batch != nil && batch.ID == batchID {
			return batch
		}
	}
	return nil
}

func (t *ShipmentListTab) shipmentsForBatch(batchID string) []*domain.Shipment {
	batch := t.batchByID(batchID)
	if batch == nil {
		return nil
	}
	return batch.Shipments
}

func (t *ShipmentListTab) shipmentByID(shipmentID string) *domain.Shipment {
	if t == nil || t.picker == nil {
		return nil
	}
	for _, batch := range t.picker.Batches {
		if batch == nil {
			continue
		}
		for _, shipment := range batch.Shipments {
			if shipment != nil && shipment.ID == shipmentID {
				return shipment
			}
		}
	}
	return nil
}

func (t *ShipmentListTab) batchSelectionState(batch *domain.Batch) (selectedCount int, totalCount int) {
	if t == nil || t.picker == nil || batch == nil {
		return 0, 0
	}
	if batch.ShipmentsLoaded {
		totalCount = len(batch.Shipments)
		for _, shipment := range batch.Shipments {
			if shipment != nil && t.picker.IsSelected(shipment.ID) {
				selectedCount++
			}
		}
		return selectedCount, totalCount
	}
	for _, shipmentID := range batch.ShipmentIDs {
		if strings.TrimSpace(shipmentID) == "" {
			continue
		}
		totalCount++
		if t.picker.IsSelected(shipmentID) {
			selectedCount++
		}
	}
	return selectedCount, totalCount
}

func shipmentTreeDetails(shipment *domain.Shipment) string {
	if shipment == nil {
		return ""
	}
	service := strings.TrimSpace(shipment.ServiceCode)
	tracking := strings.TrimSpace(shipment.TrackingNumber)
	if len(tracking) > 12 {
		tracking = tracking[:4] + "…" + tracking[len(tracking)-6:]
	}
	return firstNonEmpty(strings.TrimSpace(service+" "+tracking), service, tracking)
}

func formatBatchCreatedCT(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	loc, err := time.LoadLocation("America/Chicago")
	if err == nil {
		ts = ts.In(loc)
	}
	return ts.Format("01/02/06 3:04 PM")
}
