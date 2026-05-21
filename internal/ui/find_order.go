package ui

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"pickcart/internal/domain"
	"pickcart/internal/led"
)

type FindOrderTab struct {
	profile        domain.CartProfile
	led            *led.Controller
	active         bool
	entries        []FindOrderEntry
	details        map[string]FindOrderDetail
	currentIdx     int
	trackingBuf    string
	trackingLoaded bool
	quantityText   *canvas.Text
	customerLabel  *canvas.Text
	detailsLabel   *widget.Label
	locationText   *canvas.Text
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
		quantityText:  newHeaderText("0 / 0", 46, true),
		customerLabel: newHeaderTitleText("No shipments available", 64),
		detailsLabel:  widget.NewLabel(""),
		locationText:  newHeaderText("", 46, true),
		searchEntry:   widget.NewEntry(),
		searchStatus:  widget.NewLabel(""),
		gridHolder:    container.NewMax(widget.NewLabel("No shipments selected")),
	}
	t.detailsLabel.Alignment = fyne.TextAlignCenter
	t.detailsLabel.Wrapping = fyne.TextWrapOff
	t.detailsLabel.TextStyle = fyne.TextStyle{}
	t.detailsLabel.Importance = widget.HighImportance
	t.detailsLabel.SizeName = theme.SizeNameText
	for _, label := range []*widget.Label{t.searchStatus} {
		label.Alignment = fyne.TextAlignCenter
		label.Importance = widget.WarningImportance
	}
	t.searchStatus.Hide()

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
		t.customerLabel,
		t.detailsLabel,
	)
	searchArea := wrapWithMargin(container.NewVBox(t.searchEntry, t.searchStatus), 14, 4)
	t.root = container.NewBorder(
		container.NewVBox(
			searchArea,
			header,
		),
		nil,
		nil,
		nil,
		wrapWithMargin(container.NewVScroll(t.gridHolder), 14, 10),
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

func (t *FindOrderTab) SetActive(active bool) {
	if t == nil {
		return
	}
	if t.active == active {
		if active {
			t.syncLEDs()
		}
		return
	}
	t.active = active
	if active {
		t.syncLEDs()
	}
}

func (t *FindOrderTab) SetData(entries []FindOrderEntry, details map[string]FindOrderDetail) {
	if t == nil {
		return
	}
	t.entries = append([]FindOrderEntry(nil), entries...)
	if len(t.entries) == 0 {
		t.currentIdx = -1
	} else if t.currentIdx < 0 || t.currentIdx >= len(t.entries) {
		t.currentIdx = 0
	}
	t.details = make(map[string]FindOrderDetail, len(details))
	for key, value := range details {
		t.details[key] = value
	}
	t.trackingLoaded = len(details) > 0
	t.navigateTo(t.currentIdx)
}

func (t *FindOrderTab) UpdateShipments(entries []FindOrderEntry) {
	if t == nil {
		return
	}
	t.SetData(entries, t.details)
}

func (t *FindOrderTab) UpdateShipmentDetails(details map[string]FindOrderDetail) {
	if t == nil {
		return
	}
	t.SetData(t.entries, details)
}

func (t *FindOrderTab) ShowNext() {
	t.navigateTo(t.currentIdx + 1)
}

func (t *FindOrderTab) ShowPrevious() {
	t.navigateTo(t.currentIdx - 1)
}

func (t *FindOrderTab) navigateTo(idx int) {
	t.applyResponsiveHeaderScale()

	if idx < 0 || idx >= len(t.entries) {
		t.currentIdx = -1
		setHeaderText(t.quantityText, "0 / 0")
		setHeaderTitleText(t.customerLabel, "No shipments available")
		setHeaderLabelText(t.detailsLabel, "")
		setHeaderText(t.locationText, "")
		t.prevBtn.Disable()
		t.nextBtn.Disable()
		t.rebuildGrid("")
		if t.active && t.led != nil {
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
	setHeaderTitleText(t.customerLabel, firstNonEmpty(detail.CustomerName, "Unknown Customer"))
	setHeaderLabelText(t.detailsLabel, formatFindOrderDebugLine(entry, tracking, t.trackingLoaded))
	setHeaderText(t.locationText, fallbackLocation(location))

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

	if t.active && t.led != nil {
		t.led.ClearLEDs()
		if location != "" {
			t.led.HighlightLocations([]string{location}, quantityLEDColor(1))
		}
	}
	t.rebuildGrid(entry.ShipmentID)
}

func (t *FindOrderTab) processSearchInput() {
	if t == nil || t.searchEntry == nil {
		return
	}

	raw := t.searchEntry.Text
	t.searchEntry.SetText("")
	t.processSearchValue(raw)
}

func (t *FindOrderTab) processSearchValue(raw string) {
	if t == nil {
		return
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	queries := normalizedSearchQueries(raw)
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
				t.setSearchStatus(fmt.Sprintf("Matched tracking %s", raw))
				t.navigateTo(idx)
				return
			}
			if customer != "" && strings.Contains(customer, query) {
				t.setSearchStatus(fmt.Sprintf("Matched customer %s", detail.CustomerName))
				t.navigateTo(idx)
				return
			}
			if orderID != "" && strings.Contains(orderID, query) {
				t.setSearchStatus(fmt.Sprintf("Matched order %s", entry.ExternalID))
				t.navigateTo(idx)
				return
			}
		}
	}

	t.setSearchStatus(fmt.Sprintf("No shipment found for %s", raw))
}

func (t *FindOrderTab) HandleScannerRune(r rune) bool {
	if t == nil || !t.active {
		return false
	}
	if !unicode.IsPrint(r) {
		return false
	}
	t.trackingBuf += string(r)
	return true
}

func (t *FindOrderTab) HandleScannerKey(ev *fyne.KeyEvent) bool {
	if t == nil || !t.active || ev == nil {
		return false
	}

	switch ev.Name {
	case fyne.KeyEscape:
		t.trackingBuf = ""
		t.setSearchStatus("Tracking input cleared")
		return true
	case fyne.KeyBackspace:
		if t.trackingBuf == "" {
			return true
		}
		_, size := utf8.DecodeLastRuneInString(t.trackingBuf)
		if size > 0 {
			t.trackingBuf = t.trackingBuf[:len(t.trackingBuf)-size]
		}
		return true
	case fyne.KeyReturn, fyne.KeyEnter:
		if strings.TrimSpace(t.trackingBuf) == "" {
			return false
		}
		raw := t.trackingBuf
		t.trackingBuf = ""
		t.processSearchValue(raw)
		return true
	default:
		return false
	}
}

func (t *FindOrderTab) FocusSearch(canvas fyne.Canvas) {
	if t == nil {
		return
	}
	t.trackingBuf = ""
	t.setSearchStatus("")
	if canvas != nil && t.searchEntry != nil {
		canvas.Focus(t.searchEntry)
	}
}

func (t *FindOrderTab) setSearchStatus(value string) {
	if t == nil || t.searchStatus == nil {
		return
	}
	value = strings.TrimSpace(value)
	t.searchStatus.SetText(value)
	if value == "" {
		t.searchStatus.Hide()
	} else {
		t.searchStatus.Show()
	}
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
	gridScale := gridScaleForWidth(t.root.Size().Width, maxColumnsForRows(rows))
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
				centerText = compactShipmentDisplayID(entry.ExternalID)
				if entry.ShipmentID == activeShipmentID {
					fill = color.NRGBA{R: 0x90, G: 0xee, B: 0x90, A: 0xff}
					footerText = "Active"
				}
			}
			cellObjects = append(cellObjects, makeGridCell(location, centerText, footerText, fill, gridScale))
		}
		rowObjects = append(rowObjects, container.NewGridWithColumns(len(cellObjects), cellObjects...))
	}

	t.gridHolder.Objects = []fyne.CanvasObject{container.NewVBox(rowObjects...)}
	t.gridHolder.Refresh()
}

func (t *FindOrderTab) refreshResponsiveLayout() {
	if t == nil {
		return
	}
	t.applyResponsiveHeaderScale()
	t.rebuildGrid(t.activeShipmentID())
	if t.active {
		t.syncLEDs()
	}
}

func (t *FindOrderTab) syncLEDs() {
	if t == nil || !t.active || t.led == nil {
		return
	}
	location := t.activeLocation()
	t.led.ClearLEDs()
	if location != "" {
		t.led.HighlightLocations([]string{location}, quantityLEDColor(1))
	}
}

func (t *FindOrderTab) activeLocation() string {
	if t == nil || t.currentIdx < 0 || t.currentIdx >= len(t.entries) {
		return ""
	}
	entry := t.entries[t.currentIdx]
	detail := t.details[entry.ShipmentID]
	return firstNonEmpty(detail.Location, t.findGridLocation(entry.GridIndex))
}

func (t *FindOrderTab) activeShipmentID() string {
	if t == nil || t.currentIdx < 0 || t.currentIdx >= len(t.entries) {
		return ""
	}
	return t.entries[t.currentIdx].ShipmentID
}

func (t *FindOrderTab) applyResponsiveHeaderScale() {
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
	applyHeaderTitleScale(scale, t.customerLabel)
	applyHeaderLabelScale(scale, nil, t.detailsLabel)
}

func formatFindOrderDebugLine(entry FindOrderEntry, tracking string, trackingLoaded bool) string {
	parts := make([]string, 0, 2)
	orderID := compactShipmentDisplayID(firstNonEmpty(entry.ExternalID, entry.ShipmentID))
	if orderID != "" {
		parts = append(parts, fmt.Sprintf("Order %s", orderID))
	}
	tracking = strings.TrimSpace(tracking)
	if tracking == "" && !trackingLoaded {
		tracking = "not loaded yet"
	}
	if tracking != "" {
		parts = append(parts, fmt.Sprintf("Tracking %s", tracking))
	}
	return strings.Join(parts, " • ")
}
