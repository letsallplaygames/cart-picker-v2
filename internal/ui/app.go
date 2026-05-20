package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"pickcart/internal/boxing"
	"pickcart/internal/config"
	"pickcart/internal/dimensions"
	"pickcart/internal/domain"
	"pickcart/internal/led"
	"pickcart/internal/odoo"
	"pickcart/internal/picker"
)

type App struct {
	fyneApp fyne.App
	window  fyne.Window

	cfg     *config.AppConfig
	profile domain.CartProfile
	picker  *picker.Picker
	led     *led.Controller

	tabs            *container.AppTabs
	shipmentListTab *ShipmentListTab
	shelfLayoutTab  *ShelfLayoutTab
	pickListTab     *PickListTab
	boxesTab        *ShelfLayoutTab
	findOrderTab    *FindOrderTab

	itemsByShipment map[string][]domain.Item
	shipmentsByID   map[string]*domain.Shipment
	selectionEpoch  int64
	batchLoading    bool
	statusLabel     *widget.Label
	currentTabName  string

	productDims map[string]dimensions.Dimensions
}

func NewApp(cfg *config.AppConfig, profile domain.CartProfile, p *picker.Picker, ledController *led.Controller) *App {
	fa := fyneapp.New()
	w := fa.NewWindow("PickCart")

	a := &App{
		fyneApp:         fa,
		window:          w,
		cfg:             cfg,
		profile:         profile,
		picker:          p,
		led:             ledController,
		itemsByShipment: map[string][]domain.Item{},
		shipmentsByID:   map[string]*domain.Shipment{},
		statusLabel:     widget.NewLabel("Ready"),
		productDims:     map[string]dimensions.Dimensions{},
	}

	a.tabs = a.buildTabs()
	a.currentTabName = "Shipments"
	a.window.SetContent(container.NewBorder(nil, a.statusLabel, nil, nil, a.tabs))
	a.window.Canvas().SetOnTypedKey(a.onTypedKey)
	a.window.Resize(fyne.NewSize(1400, 900))

	return a
}

func (a *App) Run() {
	if a == nil {
		return
	}

	a.batchLoading = true
	a.statusLabel.SetText("Loading batches...")
	a.picker.LoadBatches(initialBatchLoadLimit, false, func(count int, err error) {
		fyne.Do(func() {
			a.batchLoading = false
			if err != nil {
				a.statusLabel.SetText(fmt.Sprintf("Failed to load batches: %v", err))
				return
			}
			if a.shipmentListTab != nil {
				a.shipmentListTab.Refresh()
			}
			a.statusLabel.SetText(fmt.Sprintf("Loaded %d batches", count))
		})
	})

	a.window.ShowAndRun()
}

func (a *App) buildTabs() *container.AppTabs {
	a.shipmentListTab = NewShipmentListTab(a.picker, a.onSelectionChanged)
	a.shelfLayoutTab = NewShelfLayoutTab(a.profile, a.led, false)
	a.pickListTab = NewPickListTab()
	a.boxesTab = NewShelfLayoutTab(a.profile, a.led, true)
	a.findOrderTab = NewFindOrderTab(a.profile, a.led)

	tabs := container.NewAppTabs(
		container.NewTabItem("Shipments", a.shipmentListTab.Object()),
		container.NewTabItem("Pick Cart", a.shelfLayoutTab.Object()),
		container.NewTabItem("Pick List", a.pickListTab.Object()),
		container.NewTabItem("Boxes", a.boxesTab.Object()),
		container.NewTabItem("Find Order", a.findOrderTab.Object()),
	)
	tabs.OnSelected = a.onTabChanged
	return tabs
}

func (a *App) onSelectionChanged() {
	if a == nil {
		return
	}

	a.selectionEpoch++
	epoch := a.selectionEpoch

	selectedMap := a.picker.GetSelected()
	nextShipments := make(map[string]*domain.Shipment, len(selectedMap))
	nextItems := make(map[string][]domain.Item, len(selectedMap))
	for id, shipment := range selectedMap {
		if shipment == nil {
			continue
		}
		nextShipments[id] = shipment
		if items, ok := a.itemsByShipment[id]; ok {
			nextItems[id] = append([]domain.Item(nil), items...)
		}
	}
	a.shipmentsByID = nextShipments
	a.itemsByShipment = nextItems

	selectedShipments := a.selectedShipmentsOrdered()
	cells := a.buildShelfCells(selectedShipments)
	entries, details := a.buildFindOrderState(selectedShipments, cells)

	a.shelfLayoutTab.UpdateShipments(cells)
	a.boxesTab.UpdateShipments(cells)
	a.findOrderTab.UpdateShipments(entries)
	a.findOrderTab.UpdateShipmentDetails(details)

	if len(selectedShipments) == 0 {
		a.pickListTab.UpdateItems(nil)
		a.shelfLayoutTab.UpdatePickItems(nil)
		a.boxesTab.UpdatePickItems(nil)
		a.statusLabel.SetText("No shipments selected")
		return
	}

	shipmentIDs := make([]string, 0, len(selectedShipments))
	for _, shipment := range selectedShipments {
		shipmentIDs = append(shipmentIDs, shipment.ID)
	}

	a.statusLabel.SetText(fmt.Sprintf("Loading items for %d selected shipments...", len(shipmentIDs)))
	a.picker.LoadShipmentItemsBulk(shipmentIDs, func(results map[string]odoo.BulkResult, err error) {
		fyne.Do(func() {
			if epoch != a.selectionEpoch {
				return
			}
			if err != nil {
				a.statusLabel.SetText(fmt.Sprintf("Failed to load shipment items: %v", err))
				return
			}
			a.onBulkResultsLoaded(epoch, results)
		})
	})
}

func (a *App) onBulkResultsLoaded(epoch int64, results map[string]odoo.BulkResult) {
	if a == nil || epoch != a.selectionEpoch {
		return
	}

	for shipmentID, result := range results {
		items := append([]domain.Item(nil), result.Items...)
		a.itemsByShipment[shipmentID] = items

		if shipment, ok := a.shipmentsByID[shipmentID]; ok {
			shipment.Items = items
			shipment.ItemsLoaded = true
			shipment.Weight = result.TotalWeightOz
			shipment.BoxName = boxing.FindSmallestBox(items)
		}
	}

	selectedShipments := a.selectedShipmentsOrdered()
	cells := a.buildShelfCells(selectedShipments)
	pickItems := a.aggregatePickList()
	entries, details := a.buildFindOrderState(selectedShipments, cells)

	a.pickListTab.UpdateItems(pickItems)
	a.shelfLayoutTab.UpdateShipments(cells)
	a.shelfLayoutTab.UpdatePickItems(pickItems)
	a.boxesTab.UpdateShipments(cells)
	a.boxesTab.UpdatePickItems(pickItems)
	a.findOrderTab.UpdateShipments(entries)
	a.findOrderTab.UpdateShipmentDetails(details)
	a.statusLabel.SetText(fmt.Sprintf("Selected shipments: %d • Pick items: %d", len(selectedShipments), len(pickItems)))
}

func (a *App) aggregatePickList() []domain.PickItem {
	if a == nil {
		return nil
	}

	aggregated := make(map[string]*domain.PickItem)
	for _, shipment := range a.selectedShipmentsOrdered() {
		if shipment == nil || shipment.ID == "" {
			continue
		}

		items := a.itemsByShipment[shipment.ID]
		if len(items) == 0 {
			items = shipment.Items
		}

		byName := make(map[string]*domain.Item)
		for _, item := range items {
			key := strings.TrimSpace(item.Name)
			if key == "" {
				key = strings.TrimSpace(item.SKU)
			}
			if key == "" {
				key = "Unknown Item"
			}

			if existing, ok := byName[key]; ok {
				existing.Quantity += item.Quantity
				if existing.SKU == "" {
					existing.SKU = item.SKU
				}
				if existing.SKULocation == "" {
					existing.SKULocation = item.SKULocation
				}
				if existing.Height == 0 {
					existing.Height = item.Height
				}
				if existing.Width == 0 {
					existing.Width = item.Width
				}
				if existing.Length == 0 {
					existing.Length = item.Length
				}
				if shouldAdoptQtyAvailable(existing.QtyAvailable, item.QtyAvailable) {
					existing.QtyAvailable = item.QtyAvailable
				}
				continue
			}

			copy := item
			byName[key] = &copy
		}

		keys := make([]string, 0, len(byName))
		for key := range byName {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			item := byName[key]
			pickShipment := domain.PickShipment{
				ShipmentID: shipment.ID,
				ExternalID: shipment.ExternalID,
				Quantity:   item.Quantity,
				Location:   item.SKULocation,
			}

			if existing, ok := aggregated[key]; ok {
				existing.Quantity += item.Quantity
				if existing.SKU == "" {
					existing.SKU = item.SKU
				}
				if existing.Location == "" {
					existing.Location = item.SKULocation
				}
				if existing.Height == 0 {
					existing.Height = item.Height
				}
				if existing.Width == 0 {
					existing.Width = item.Width
				}
				if existing.Length == 0 {
					existing.Length = item.Length
				}
				if shouldAdoptQtyAvailable(existing.QtyAvailable, item.QtyAvailable) {
					existing.QtyAvailable = item.QtyAvailable
				}
				existing.Shipments = append(existing.Shipments, pickShipment)
				continue
			}

			aggregated[key] = &domain.PickItem{
				Name:         key,
				SKU:          item.SKU,
				Location:     item.SKULocation,
				Quantity:     item.Quantity,
				Height:       item.Height,
				Width:        item.Width,
				Length:       item.Length,
				QtyAvailable: item.QtyAvailable,
				Shipments:    []domain.PickShipment{pickShipment},
			}
		}
	}

	items := make([]domain.PickItem, 0, len(aggregated))
	for _, item := range aggregated {
		if item == nil {
			continue
		}
		items = append(items, *item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		gi, li, mi, si := a.locationSortKey(items[i].Location)
		gj, lj, mj, sj := a.locationSortKey(items[j].Location)
		if gi != gj {
			return gi < gj
		}
		if li != lj {
			return li < lj
		}
		if mi != mj {
			return mi < mj
		}
		if si != sj {
			return si < sj
		}
		return items[i].Name < items[j].Name
	})

	return items
}

func (a *App) locationSortKey(location string) (group int, letter string, mainNum int, subNum int) {
	location = strings.TrimSpace(strings.ToUpper(location))
	if location == "" || location == "-" || location == "UNKN" {
		return 2, "", 0, 0
	}
	if len(location) < 2 {
		return 1, location, 0, 0
	}

	letter = location[:1]
	rest := location[1:]
	parts := strings.SplitN(rest, ".", 2)

	main, err := strconv.Atoi(parts[0])
	if err != nil {
		return 1, location, 0, 0
	}
	mainNum = main

	if len(parts) > 1 {
		subNum, _ = strconv.Atoi(parts[1])
	}

	if rowUsesDescendingRoute(letter) {
		mainNum = -mainNum
	}

	return 0, letter, mainNum, subNum
}

func rowUsesDescendingRoute(letter string) bool {
	letter = strings.TrimSpace(strings.ToUpper(letter))
	if len(letter) != 1 {
		return false
	}
	row := int(letter[0] - 'A')
	if row < 0 || row > 25 {
		return false
	}
	return row%2 == 1
}

func (a *App) onTabChanged(tab *container.TabItem) {
	if a == nil || tab == nil {
		return
	}

	a.currentTabName = tab.Text

	pickCartActive := tab.Text == "Pick Cart"
	findOrderActive := tab.Text == "Find Order"
	if a.shelfLayoutTab != nil {
		a.shelfLayoutTab.SetActive(pickCartActive)
	}
	if a.findOrderTab != nil {
		a.findOrderTab.SetActive(findOrderActive)
	}
	if a.led != nil && !pickCartActive && !findOrderActive {
		a.led.ClearLEDs()
	}

	switch tab.Text {
	case "Pick Cart":
		if a.shelfLayoutTab != nil {
			a.shelfLayoutTab.refreshResponsiveLayout()
		}
	case "Find Order":
		if a.findOrderTab != nil {
			a.findOrderTab.refreshResponsiveLayout()
		}
		a.statusLabel.SetText("Find Order ready")
	case "Boxes":
		if a.boxesTab != nil {
			a.boxesTab.refreshResponsiveLayout()
		}
		a.statusLabel.SetText("Boxes view ready")
	}
}

func (a *App) onTypedKey(ev *fyne.KeyEvent) {
	if a == nil || ev == nil {
		return
	}

	switch a.currentTabName {
	case "Pick Cart":
		if a.shelfLayoutTab == nil {
			return
		}
		switch ev.Name {
		case fyne.KeyLeft:
			a.shelfLayoutTab.ShowPrevious()
		case fyne.KeyRight:
			a.shelfLayoutTab.ShowNext()
		}
	case "Boxes":
		if a.boxesTab == nil {
			return
		}
		switch ev.Name {
		case fyne.KeyLeft:
			a.boxesTab.ShowPreviousBox()
		case fyne.KeyRight:
			a.boxesTab.ShowNextBox()
		}
	case "Find Order":
		if a.findOrderTab == nil {
			return
		}
		if a.window != nil && a.window.Canvas().Focused() == a.findOrderTab.searchEntry {
			return
		}
		switch ev.Name {
		case fyne.KeyLeft:
			a.findOrderTab.ShowPrevious()
		case fyne.KeyRight:
			a.findOrderTab.ShowNext()
		}
	}
}

func (a *App) selectedShipmentsOrdered() []*domain.Shipment {
	selected := make([]*domain.Shipment, 0, len(a.shipmentsByID))
	for _, shipment := range a.shipmentsByID {
		if shipment != nil {
			selected = append(selected, shipment)
		}
	}

	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Weight != selected[j].Weight {
			return selected[i].Weight > selected[j].Weight
		}
		if selected[i].ExternalID != selected[j].ExternalID {
			return selected[i].ExternalID < selected[j].ExternalID
		}
		return selected[i].ID < selected[j].ID
	})

	return selected
}

func (a *App) buildShelfCells(shipments []*domain.Shipment) []ShelfCell {
	locations := cartLocations(a.profile)
	cells := make([]ShelfCell, 0, len(shipments))
	for idx, shipment := range shipments {
		if shipment == nil {
			continue
		}

		location := ""
		if idx < len(locations) {
			location = locations[idx]
		}

		cells = append(cells, ShelfCell{
			Location:   location,
			ExternalID: shipment.ExternalID,
			ShipmentID: shipment.ID,
			BoxName:    shipment.BoxName,
			HasOrder:   true,
		})
	}
	return cells
}

func (a *App) buildFindOrderState(shipments []*domain.Shipment, cells []ShelfCell) ([]FindOrderEntry, map[string]FindOrderDetail) {
	locationByShipment := make(map[string]string, len(cells))
	for _, cell := range cells {
		locationByShipment[cell.ShipmentID] = cell.Location
	}

	entries := make([]FindOrderEntry, 0, len(shipments))
	details := make(map[string]FindOrderDetail, len(shipments))
	for idx, shipment := range shipments {
		if shipment == nil {
			continue
		}

		entries = append(entries, FindOrderEntry{
			ShipmentID:     shipment.ID,
			ExternalID:     shipment.ExternalID,
			TrackingNumber: shipment.TrackingNumber,
			GridIndex:      idx,
		})
		details[shipment.ID] = FindOrderDetail{
			CustomerName:   shipment.ShipTo,
			TrackingNumber: shipment.TrackingNumber,
			Location:       locationByShipment[shipment.ID],
		}
	}

	return entries, details
}

func shouldAdoptQtyAvailable(current float64, candidate float64) bool {
	if candidate < 0 {
		return false
	}
	return current < 0
}
