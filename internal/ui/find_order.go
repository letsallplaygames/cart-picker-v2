package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
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
	quantityLabel  *widget.Label
	customerLabel  *widget.Label
	orderLabel     *widget.Label
	locationLabel  *widget.Label
	trackingLabel  *widget.Label
	searchEntry    *widget.Entry
	searchStatus   *widget.Label
	prevBtn        *widget.Button
	nextBtn        *widget.Button
	gridHolder     *fyne.Container
	root           fyne.CanvasObject
}

func NewFindOrderTab(profile domain.CartProfile, ledController *led.Controller) *FindOrderTab {
	t := &FindOrderTab{
		profile:       profile,
		led:           ledController,
		details:       map[string]FindOrderDetail{},
		currentIdx:    -1,
		quantityLabel: widget.NewLabel("0 / 0"),
		customerLabel: widget.NewLabel("No shipments available"),
		orderLabel:    widget.NewLabel(""),
		locationLabel: widget.NewLabel(""),
		trackingLabel: widget.NewLabel(""),
		searchEntry:   widget.NewEntry(),
		searchStatus:  widget.NewLabel(""),
		gridHolder:    container.NewMax(widget.NewLabel("No shipments selected")),
	}
	for _, label := range []*widget.Label{t.quantityLabel, t.customerLabel, t.orderLabel, t.locationLabel, t.trackingLabel, t.searchStatus} {
		label.Alignment = fyne.TextAlignCenter
	}
	for _, label := range []*widget.Label{t.customerLabel, t.orderLabel, t.trackingLabel} {
		label.Wrapping = fyne.TextWrapWord
	}
	for _, label := range []*widget.Label{t.quantityLabel, t.customerLabel, t.orderLabel, t.locationLabel, t.trackingLabel} {
		label.Importance = widget.HighImportance
	}
	for _, label := range []*widget.Label{t.searchStatus} {
		label.Importance = widget.WarningImportance
	}

	t.searchEntry.SetPlaceHolder("Scan or enter tracking number and press Enter")
	t.searchEntry.OnSubmitted = func(string) { t.processTrackingInput() }
	t.prevBtn = widget.NewButton("Previous", func() { t.navigateTo(t.currentIdx - 1) })
	t.nextBtn = widget.NewButton("Next", func() { t.navigateTo(t.currentIdx + 1) })
	t.root = container.NewBorder(
		container.NewVBox(
			t.searchEntry,
			t.searchStatus,
			container.NewHBox(t.prevBtn, t.quantityLabel, t.nextBtn),
			t.customerLabel,
			t.orderLabel,
			t.locationLabel,
			t.trackingLabel,
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

func (t *FindOrderTab) navigateTo(idx int) {
	if idx < 0 || idx >= len(t.entries) {
		t.currentIdx = -1
		t.quantityLabel.SetText("0 / 0")
		t.customerLabel.SetText("No shipments available")
		t.orderLabel.SetText("")
		t.locationLabel.SetText("")
		t.trackingLabel.SetText("")
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

	t.quantityLabel.SetText(fmt.Sprintf("%d / %d", idx+1, len(t.entries)))
	t.customerLabel.SetText(firstNonEmpty(detail.CustomerName, "Unknown Customer"))
	t.orderLabel.SetText(fmt.Sprintf("Order %s", firstNonEmpty(entry.ExternalID, entry.ShipmentID)))
	t.locationLabel.SetText(fmt.Sprintf("Location %s", fallbackLocation(location)))
	if tracking == "" && !t.trackingLoaded {
		tracking = "Tracking not loaded yet"
	}
	t.trackingLabel.SetText(fmt.Sprintf("Tracking %s", tracking))

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

func (t *FindOrderTab) processTrackingInput() {
	if t == nil || t.searchEntry == nil {
		return
	}

	raw := strings.TrimSpace(t.searchEntry.Text)
	if raw == "" {
		return
	}
	if !strings.HasPrefix(strings.ToUpper(raw), "1Z") && len(raw) > 8 {
		raw = raw[8:]
	}
	search := strings.ToLower(strings.TrimSpace(raw))
	t.searchEntry.SetText("")
	if search == "" {
		return
	}

	for idx, entry := range t.entries {
		tracking := strings.ToLower(firstNonEmpty(t.details[entry.ShipmentID].TrackingNumber, entry.TrackingNumber))
		if tracking != "" && strings.Contains(tracking, search) {
			t.searchStatus.SetText(fmt.Sprintf("Matched tracking %s", raw))
			t.navigateTo(idx)
			return
		}
	}

	t.searchStatus.SetText(fmt.Sprintf("No shipment found for %s", raw))
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
