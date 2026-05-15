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
	"pickcart/internal/led"
)

type FindOrderTab struct {
	profile        domain.CartProfile
	led            *led.Controller
	entries        []FindOrderEntry
	details        map[string]FindOrderDetail
	currentIdx     int
	trackingBuf    string
	trackingLoaded bool
	quantityText   *canvas.Text
	customerText   *canvas.Text
	orderText      *canvas.Text
	locationText   *canvas.Text
	trackingText   *canvas.Text
	searchEntry    *widget.Entry
	searchStatus   *widget.Label
	prevBtn        *widget.Button
	nextBtn        *widget.Button
	gridHolder     *fyne.Container
	root           fyne.CanvasObject
}

func NewFindOrderTab(profile domain.CartProfile, ledController *led.Controller) *FindOrderTab {
	t := &FindOrderTab{
		profile:      profile,
		led:          ledController,
		details:      map[string]FindOrderDetail{},
		currentIdx:   -1,
		quantityText: newHeaderText("0 / 0", 46, true),
		customerText: newHeaderText("No shipments available", 52, true),
		orderText:    newHeaderText("", 18, false),
		locationText: newHeaderText("", 46, true),
		trackingText: newHeaderText("", 18, false),
		searchEntry:  widget.NewEntry(),
		searchStatus: widget.NewLabel(""),
		gridHolder:   container.NewMax(widget.NewLabel("No shipments selected")),
	}
	for _, label := range []*widget.Label{t.searchStatus} {
		label.Alignment = fyne.TextAlignCenter
		label.Importance = widget.WarningImportance
	}

	t.searchEntry.SetPlaceHolder("Scan tracking or search customer name, then press Enter")
	t.searchEntry.OnSubmitted = func(string) { t.processSearchInput() }
	t.prevBtn = widget.NewButton("← Previous", t.ShowPrevious)
	t.prevBtn.Importance = widget.HighImportance
	t.nextBtn = widget.NewButton("Next →", t.ShowNext)
	t.nextBtn.Importance = widget.HighImportance
	header := makeUnifiedHeader(
		t.prevBtn,
		t.quantityText,
		t.nextBtn,
		t.locationText,
		t.customerText,
		t.orderText,
		t.trackingText,
	)
	t.root = container.NewBorder(
		container.NewVBox(
			t.searchEntry,
			t.searchStatus,
			header,
		),
		nil,
		nil,
		nil,
		container.NewVScroll(t.gridHolder),
	)
	t.navigateTo(-1)
	return t
}

func (t *FindOrderTab) Object() fyne.CanvasObject {
	if t == nil {
		return widget.NewLabel("")
	}
	return t.root
}

func (t *FindOrderTab) UpdateShipments(entries []FindOrderEntry) {
	t.entries = append([]FindOrderEntry(nil), entries...)
	if len(t.entries) == 0 {
		t.currentIdx = -1
	} else if t.currentIdx < 0 || t.currentIdx >= len(t.entries) {
		t.currentIdx = 0
	}
	t.navigateTo(t.currentIdx)
}

func (t *FindOrderTab) UpdateShipmentDetails(details map[string]FindOrderDetail) {
	t.details = make(map[string]FindOrderDetail, len(details))
	for key, value := range details {
		t.details[key] = value
	}
	t.trackingLoaded = len(details) > 0
	t.navigateTo(t.currentIdx)
}

func (t *FindOrderTab) ShowNext() {
	t.navigateTo(t.currentIdx + 1)
}

func (t *FindOrderTab) ShowPrevious() {
	t.navigateTo(t.currentIdx - 1)
}

func (t *FindOrderTab) navigateTo(idx int) {
	if idx < 0 || idx >= len(t.entries) {
		t.currentIdx = -1
		setHeaderText(t.quantityText, "0 / 0")
		setHeaderText(t.customerText, "No shipments available")
		setHeaderText(t.orderText, "")
		setHeaderText(t.locationText, "")
		setHeaderText(t.trackingText, "")
		t.prevBtn.Disable()
		t.nextBtn.Disable()
		t.rebuildGrid("")
		if t.led != nil {
			t.led.ClearLEDs()
		}
		return
	}

	t.currentIdx = idx
	entry := t.entries[idx]
	detail := t.details[entry.ShipmentID]
	location := firstNonEmpty(detail.Location, t.findGridLocation(entry.GridIndex))
	tracking := firstNonEmpty(detail.TrackingNumber, entry.TrackingNumber)

	setHeaderText(t.quantityText, fmt.Sprintf("%d / %d", idx+1, len(t.entries)))
	setHeaderText(t.customerText, firstNonEmpty(detail.CustomerName, "Unknown Customer"))
	setHeaderText(t.orderText, fmt.Sprintf("Order %s", firstNonEmpty(entry.ExternalID, entry.ShipmentID)))
	setHeaderText(t.locationText, fallbackLocation(location))
	if tracking == "" && !t.trackingLoaded {
		tracking = "Tracking not loaded yet"
	}
	setHeaderText(t.trackingText, fmt.Sprintf("Tracking %s", tracking))

	if idx <= 0 {
		t.prevBtn.Disable()
	} else {
		t.prevBtn.Enable()
	}
	if idx >= len(t.entries)-1 {
		t.nextBtn.Disable()
	} else {
		t.nextBtn.Enable()
	}

	if t.led != nil {
		t.led.ClearLEDs()
		if location != "" {
			t.led.HighlightLocations([]string{location}, [3]byte{0, 255, 0})
		}
	}
	t.rebuildGrid(entry.ShipmentID)
}

func (t *FindOrderTab) processSearchInput() {
	if t == nil || t.searchEntry == nil {
		return
	}

	raw := strings.TrimSpace(t.searchEntry.Text)
	if raw == "" {
		return
	}
	queries := normalizedSearchQueries(raw)
	t.searchEntry.SetText("")
	if len(queries) == 0 {
		return
	}

	for idx, entry := range t.entries {
		detail := t.details[entry.ShipmentID]
		tracking := strings.ToLower(firstNonEmpty(detail.TrackingNumber, entry.TrackingNumber))
		customer := strings.ToLower(strings.TrimSpace(detail.CustomerName))
		orderID := strings.ToLower(strings.TrimSpace(entry.ExternalID))
		for _, query := range queries {
			if query == "" {
				continue
			}
			if tracking != "" && strings.Contains(tracking, query) {
				t.searchStatus.SetText(fmt.Sprintf("Matched tracking %s", raw))
				t.navigateTo(idx)
				return
			}
			if customer != "" && strings.Contains(customer, query) {
				t.searchStatus.SetText(fmt.Sprintf("Matched customer %s", detail.CustomerName))
				t.navigateTo(idx)
				return
			}
			if orderID != "" && strings.Contains(orderID, query) {
				t.searchStatus.SetText(fmt.Sprintf("Matched order %s", entry.ExternalID))
				t.navigateTo(idx)
				return
			}
		}
	}

	t.searchStatus.SetText(fmt.Sprintf("No shipment found for %s", raw))
}

func normalizedSearchQueries(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	queries := []string{strings.ToLower(raw)}
	if !strings.HasPrefix(strings.ToUpper(raw), "1Z") && len(raw) > 8 {
		suffix := strings.TrimSpace(raw[8:])
		if suffix != "" {
			queries = append(queries, strings.ToLower(suffix))
		}
	}

	seen := make(map[string]struct{}, len(queries))
	unique := make([]string, 0, len(queries))
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		if _, ok := seen[query]; ok {
			continue
		}
		seen[query] = struct{}{}
		unique = append(unique, query)
	}
	return unique
}

func (t *FindOrderTab) findGridLocation(gridIndex int) string {
	locations := cartLocations(t.profile)
	if gridIndex < 0 || gridIndex >= len(locations) {
		return ""
	}
	return locations[gridIndex]
}

func (t *FindOrderTab) rebuildGrid(activeShipmentID string) {
	rows := locationRows(t.profile)
	entryByLocation := make(map[string]FindOrderEntry, len(t.entries))
	for _, entry := range t.entries {
		location := t.findGridLocation(entry.GridIndex)
		if location != "" {
			entryByLocation[location] = entry
		}
	}

	rowObjects := make([]fyne.CanvasObject, 0, len(rows))
	for _, row := range rows {
		cellObjects := make([]fyne.CanvasObject, 0, len(row))
		for _, location := range row {
			entry, ok := entryByLocation[location]
			fill := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
			centerText := ""
			footerText := ""
			if ok {
				centerText = entry.ExternalID
				if entry.ShipmentID == activeShipmentID {
					fill = color.NRGBA{R: 0x90, G: 0xee, B: 0x90, A: 0xff}
					footerText = "Active"
				}
			}
			cellObjects = append(cellObjects, makeGridCell(location, centerText, footerText, fill))
		}
		rowObjects = append(rowObjects, container.NewGridWithColumns(len(cellObjects), cellObjects...))
	}

	t.gridHolder.Objects = []fyne.CanvasObject{container.NewVBox(rowObjects...)}
	t.gridHolder.Refresh()
}
