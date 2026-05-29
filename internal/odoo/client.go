package odoo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"pickcart/internal/cache"
	"pickcart/internal/domain"
)

const (
	maxRetries          = 5
	backoffBase         = 1 * time.Second
	backoffJitter       = 0.25
	lenexaPickingTypeID = 446
	apiCacheTTLHours    = 24
	inventoryCacheTTL   = 4 * time.Hour
	inventoryCacheKey   = "odoo.inventory.sku_locations"
	lenexaQtyCacheKey   = "odoo.inventory.lenexa_qty_by_sku"
	lenexaLocationPrefix = "LNX/"
)

type Config struct {
	APIKey   string
	BaseURL  string
	Database string
	Cache    *cache.Cache
	UseCache bool
}

type BulkResult struct {
	Items         []domain.Item
	TotalWeightOz float64
}

type Client struct {
	cfg  Config
	http *http.Client
}

type productFieldSelection struct {
	lengthField string
	widthField  string
	heightField string
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("odoo request failed with status %d: %s", e.StatusCode, e.Body)
}

func New(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) SearchRead(model string, domainExpr []any, fields []string, limit, offset int, order string, forceRefresh bool) ([]map[string]any, error) {
	payload := map[string]any{
		"domain":  domainExpr,
		"context": map[string]any{"lang": "en_US"},
	}
	if payload["domain"] == nil {
		payload["domain"] = []any{}
	}
	if len(fields) > 0 {
		payload["fields"] = fields
	}
	if limit > 0 {
		payload["limit"] = limit
	}
	if offset > 0 {
		payload["offset"] = offset
	}
	if strings.TrimSpace(order) != "" {
		payload["order"] = order
	}

	result, err := c.postJSON2(model, "search_read", payload, forceRefresh)
	if err != nil {
		return nil, err
	}

	return rowsFromAny(result)
}

func (c *Client) Read(model string, ids []int, fields []string, forceRefresh bool) ([]map[string]any, error) {
	payload := map[string]any{
		"ids":     ids,
		"context": map[string]any{"lang": "en_US"},
	}
	if len(fields) > 0 {
		payload["fields"] = fields
	}

	result, err := c.postJSON2(model, "read", payload, forceRefresh)
	if err != nil {
		return nil, err
	}

	return rowsFromAny(result)
}

func (c *Client) ParseMany2One(value any) (id int, name string) {
	switch v := value.(type) {
	case []any:
		if len(v) == 0 {
			return 0, ""
		}
		id = intFromAny(v[0])
		if len(v) > 1 {
			name = stringFromAny(v[1])
		}
		return id, name
	case map[string]any:
		id = intFromAny(v["id"])
		name = firstNonEmpty(
			stringFromAny(v["display_name"]),
			stringFromAny(v["name"]),
		)
		return id, name
	case int, int32, int64, float64, json.Number:
		return intFromAny(v), ""
	default:
		return 0, ""
	}
}

func (c *Client) ParseSKUFromLabel(label string) string {
	label = strings.TrimSpace(label)
	if !strings.HasPrefix(label, "[") {
		return ""
	}

	end := strings.Index(label, "]")
	if end <= 1 {
		return ""
	}

	return strings.TrimSpace(label[1:end])
}

func (c *Client) FormatOdooLocation(completeName string) string {
	parts := strings.Split(strings.TrimSpace(completeName), "/")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if !isSingleLetterSegment(part) {
			continue
		}
		if i+1 >= len(parts) {
			break
		}

		aisle := strings.ToUpper(part)
		mainShelf := normalizeShelfSegment(parts[i+1])
		if mainShelf == "" {
			break
		}

		location := aisle + mainShelf
		if i+2 < len(parts) {
			sub := normalizeSubLocation(parts[i+2])
			if sub != "" {
				location += "." + sub
			}
		}

		return location
	}

	return strings.TrimSpace(completeName)
}

func (c *Client) GetInventory() (map[string]string, error) {
	if cached, ok, err := c.loadInventoryCache(inventoryCacheTTL); err == nil && ok {
		return cached, nil
	}

	rows, err := c.SearchRead(
		"stock.quant",
		[]any{
			[]any{"location_id.usage", "=", "internal"},
			[]any{"quantity", ">", 0},
		},
		[]string{"product_id", "location_id"},
		10000,
		0,
		"",
		false,
	)
	if err != nil {
		if stale, staleErr := c.loadInventoryCacheStale(); staleErr == nil && len(stale) > 0 {
			slog.Warn("using stale inventory cache after Odoo inventory fetch failed", "error", err)
			return stale, nil
		}
		return nil, err
	}

	locationIDs := make(map[int]struct{})
	for _, row := range rows {
		locationID, _ := c.ParseMany2One(row["location_id"])
		if locationID > 0 {
			locationIDs[locationID] = struct{}{}
		}
	}

	locationNames := make(map[int]string, len(locationIDs))
	if len(locationIDs) > 0 {
		ids := make([]int, 0, len(locationIDs))
		for locationID := range locationIDs {
			ids = append(ids, locationID)
		}
		sort.Ints(ids)

		locationRows, err := c.Read("stock.location", ids, []string{"id", "complete_name"}, false)
		if err != nil {
			if stale, staleErr := c.loadInventoryCacheStale(); staleErr == nil && len(stale) > 0 {
				slog.Warn("using stale inventory cache after Odoo location lookup failed", "error", err)
				return stale, nil
			}
			return nil, err
		}

		for _, row := range locationRows {
			locationID := intFromAny(row["id"])
			if locationID <= 0 {
				continue
			}
			locationNames[locationID] = stringFromAny(row["complete_name"])
		}
	}

	inventory := make(map[string]string, len(rows))
	for _, row := range rows {
		_, productLabel := c.ParseMany2One(row["product_id"])
		sku := c.ParseSKUFromLabel(productLabel)
		if sku == "" {
			continue
		}

		locationID, locationLabel := c.ParseMany2One(row["location_id"])
		completeName := firstNonEmpty(locationNames[locationID], locationLabel)
		candidate := c.FormatOdooLocation(completeName)
		inventory[sku] = preferredLocation(inventory[sku], candidate)
	}

	if err := c.saveInventoryCache(inventory); err != nil {
		slog.Warn("failed to save inventory cache", "error", err)
	}

	return inventory, nil
}

func isLenexaWarehouseLocation(completeName string) bool {
	name := strings.ToUpper(strings.TrimSpace(completeName))
	return strings.HasPrefix(name, lenexaLocationPrefix)
}

func (c *Client) GetLenexaQtyBySKU() (map[string]float64, error) {
	if cached, ok, err := c.loadLenexaQtyCache(inventoryCacheTTL); err == nil && ok {
		return cached, nil
	}

	rows, err := c.SearchRead(
		"stock.quant",
		[]any{
			[]any{"location_id.usage", "=", "internal"},
			[]any{"quantity", ">", 0},
		},
		[]string{"product_id", "location_id", "quantity"},
		10000,
		0,
		"",
		false,
	)
	if err != nil {
		if stale, staleErr := c.loadLenexaQtyCacheStale(); staleErr == nil && len(stale) > 0 {
			slog.Warn("using stale Lenexa qty cache after Odoo inventory fetch failed", "error", err)
			return stale, nil
		}
		return nil, err
	}

	locationIDs := make(map[int]struct{})
	for _, row := range rows {
		locationID, _ := c.ParseMany2One(row["location_id"])
		if locationID > 0 {
			locationIDs[locationID] = struct{}{}
		}
	}

	locationNames := make(map[int]string, len(locationIDs))
	if len(locationIDs) > 0 {
		ids := make([]int, 0, len(locationIDs))
		for locationID := range locationIDs {
			ids = append(ids, locationID)
		}
		sort.Ints(ids)

		locationRows, err := c.Read("stock.location", ids, []string{"id", "complete_name"}, false)
		if err != nil {
			if stale, staleErr := c.loadLenexaQtyCacheStale(); staleErr == nil && len(stale) > 0 {
				slog.Warn("using stale Lenexa qty cache after Odoo location lookup failed", "error", err)
				return stale, nil
			}
			return nil, err
		}

		for _, row := range locationRows {
			locationID := intFromAny(row["id"])
			if locationID <= 0 {
				continue
			}
			locationNames[locationID] = stringFromAny(row["complete_name"])
		}
	}

	qtyBySKU := make(map[string]float64, len(rows))
	for _, row := range rows {
		locationID, locationLabel := c.ParseMany2One(row["location_id"])
		completeName := firstNonEmpty(locationNames[locationID], locationLabel)
		if !isLenexaWarehouseLocation(completeName) {
			continue
		}

		_, productLabel := c.ParseMany2One(row["product_id"])
		sku := c.ParseSKUFromLabel(productLabel)
		if sku == "" {
			continue
		}

		qtyBySKU[sku] += floatFromAny(row["quantity"])
	}

	if err := c.saveLenexaQtyCache(qtyBySKU); err != nil {
		slog.Warn("failed to save Lenexa qty cache", "error", err)
	}

	return qtyBySKU, nil
}

func (c *Client) GetBatches(limit int, forceRefresh bool) ([]map[string]any, error) {
	rows, err := c.SearchRead(
		"stock.picking.batch",
		[]any{
			[]any{"picking_type_id", "=", lenexaPickingTypeID},
			[]any{"state", "in", []any{"in_progress", "done"}},
		},
		[]string{"id", "name", "create_date", "state", "picking_ids"},
		limit,
		0,
		"create_date desc",
		forceRefresh,
	)
	if err != nil {
		var httpErr *HTTPError
		if ok := asHTTPError(err, &httpErr); ok && httpErr.StatusCode == http.StatusNotFound {
			slog.Warn("stock.picking.batch/search_read returned 404; falling back to no batches")
			return []map[string]any{}, nil
		}
		return nil, err
	}

	return rows, nil
}

func (c *Client) GetBatchShipments(batchID int, forceRefresh bool) ([]map[string]any, error) {
	const pageSize = 500

	var allRows []map[string]any
	offset := 0
	for {
		rows, err := c.SearchRead(
			"stock.picking",
			[]any{
				[]any{"batch_id", "=", batchID},
				[]any{"picking_type_id", "=", lenexaPickingTypeID},
				[]any{"state", "in", []any{"assigned", "done"}},
			},
			stockPickingFields(),
			pageSize,
			offset,
			"id desc",
			forceRefresh,
		)
		if err != nil {
			return nil, err
		}

		allRows = append(allRows, rows...)
		if len(rows) < pageSize {
			break
		}
		offset += pageSize
	}

	return allRows, nil
}

func (c *Client) GetShipmentsByIDs(shipmentIDs []string, forceRefresh bool) ([]map[string]any, error) {
	intIDs, err := parseStringIDs(shipmentIDs)
	if err != nil {
		return nil, err
	}
	if len(intIDs) == 0 {
		return nil, nil
	}

	rows, err := c.Read("stock.picking", intIDs, stockPickingFields(), forceRefresh)
	if err != nil {
		return nil, err
	}

	rowsByID := make(map[int]map[string]any, len(rows))
	for _, row := range rows {
		id := intFromAny(row["id"])
		if id > 0 {
			rowsByID[id] = row
		}
	}

	ordered := make([]map[string]any, 0, len(intIDs))
	for _, id := range intIDs {
		if row, ok := rowsByID[id]; ok {
			ordered = append(ordered, row)
		}
	}
	return ordered, nil
}

func (c *Client) GetBatchShipmentItemsBulk(shipmentIDs []string, forceRefresh bool, skuLocations map[string]string) (map[string]BulkResult, error) {
	results := make(map[string]BulkResult, len(shipmentIDs))
	if len(shipmentIDs) == 0 {
		return results, nil
	}

	intIDs, err := parseStringIDs(shipmentIDs)
	if err != nil {
		return nil, err
	}
	for _, shipmentID := range shipmentIDs {
		results[shipmentID] = BulkResult{}
	}

	moves, err := c.SearchRead(
		"stock.move",
		[]any{
			[]any{"picking_id", "in", intIDs},
			[]any{"state", "!=", "cancel"},
		},
		[]string{"id", "picking_id", "product_id", "location_id", "product_uom_qty", "quantity", "state", "description_picking", "display_name", "weight"},
		0,
		0,
		"id asc",
		forceRefresh,
	)
	if err != nil {
		return nil, err
	}

	productIDSet := make(map[int]struct{})
	for _, move := range moves {
		productID, _ := c.ParseMany2One(move["product_id"])
		if productID > 0 {
			productIDSet[productID] = struct{}{}
		}
	}

	productIDs := make([]int, 0, len(productIDSet))
	for productID := range productIDSet {
		productIDs = append(productIDs, productID)
	}
	sort.Ints(productIDs)

	productMap := make(map[int]map[string]any, len(productIDs))
	fieldSelection := productFieldSelection{}
	if len(productIDs) > 0 {
		selection, err := c.detectProductFieldSelection(forceRefresh)
		if err != nil {
			slog.Warn("failed to detect optional product fields", "error", err)
		} else {
			fieldSelection = selection
		}

		productFields := appendProductReadFields([]string{"id", "default_code", "name", "weight", "volume"}, fieldSelection)
		products, err := c.Read("product.product", productIDs, productFields, forceRefresh)
		if err != nil {
			return nil, err
		}
		for _, product := range products {
			productID := intFromAny(product["id"])
			if productID > 0 {
				productMap[productID] = product
			}
		}
	}

	movesByShipment := make(map[string][]map[string]any, len(shipmentIDs))
	for _, move := range moves {
		shipmentID, _ := c.ParseMany2One(move["picking_id"])
		if shipmentID <= 0 {
			continue
		}
		shipmentKey := strconv.Itoa(shipmentID)
		movesByShipment[shipmentKey] = append(movesByShipment[shipmentKey], move)
	}

	lenexaQtyBySKU, lenexaQtyErr := c.GetLenexaQtyBySKU()
	if lenexaQtyErr != nil {
		slog.Warn("failed to load Lenexa warehouse quantities; available counts will be hidden", "error", lenexaQtyErr)
		lenexaQtyBySKU = nil
	}

	for shipmentID, shipmentMoves := range movesByShipment {
		results[shipmentID] = c.buildItemsFromMoves(shipmentMoves, productMap, skuLocations, fieldSelection, lenexaQtyBySKU)
	}

	return results, nil
}

func stockPickingFields() []string {
	return []string{"id", "name", "partner_id", "carrier_id", "carrier_tracking_ref", "origin", "batch_id", "state"}
}

func (c *Client) GetTrackingNumbers(shipmentIDs []string) (map[string]string, error) {
	tracking := make(map[string]string, len(shipmentIDs))
	if len(shipmentIDs) == 0 {
		return tracking, nil
	}

	intIDs, err := parseStringIDs(shipmentIDs)
	if err != nil {
		return nil, err
	}

	rows, err := c.Read("stock.picking", intIDs, []string{"id", "carrier_tracking_ref"}, false)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		shipmentID := intFromAny(row["id"])
		if shipmentID <= 0 {
			continue
		}
		tracking[strconv.Itoa(shipmentID)] = stringFromAny(row["carrier_tracking_ref"])
	}

	return tracking, nil
}

func (c *Client) postJSON2(model, method string, payload map[string]any, forceRefresh bool) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, fmt.Errorf("odoo base URL is required")
	}
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return nil, fmt.Errorf("odoo API key is required")
	}
	if payload == nil {
		payload = map[string]any{}
	}

	endpoint := fmt.Sprintf("/json/2/%s/%s", model, method)
	url := c.cfg.BaseURL + endpoint

	if c.cfg.UseCache && c.cfg.Cache != nil && !forceRefresh {
		if cached, ok := c.cfg.Cache.Get(endpoint, payload, apiCacheTTLHours); ok {
			return extractPayload(cached), nil
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request payload: %w", err)
	}

	for attempt := range maxRetries {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")
		if strings.TrimSpace(c.cfg.Database) != "" {
			req.Header.Set("X-Odoo-Database", strings.TrimSpace(c.cfg.Database))
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("perform request: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response body: %w", readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			if attempt < maxRetries-1 {
				sleep := retryDelay(attempt)
				slog.Warn("retrying Odoo request after transient status", "status", resp.StatusCode, "attempt", attempt+1, "sleep", sleep)
				time.Sleep(sleep)
				continue
			}
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &HTTPError{StatusCode: resp.StatusCode, Body: truncateBody(respBody, 1200)}
		}

		var data any
		if err := json.Unmarshal(respBody, &data); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		if c.cfg.UseCache && c.cfg.Cache != nil {
			if err := c.cfg.Cache.Put(endpoint, payload, data); err != nil {
				slog.Warn("failed to store Odoo response in cache", "error", err, "endpoint", endpoint)
			}
		}

		return extractPayload(data), nil
	}

	return nil, fmt.Errorf("odoo request failed after retries")
}

func (c *Client) buildItemsFromMoves(moves []map[string]any, productMap map[int]map[string]any, skuLocations map[string]string, fieldSelection productFieldSelection, lenexaQtyBySKU map[string]float64) BulkResult {
	result := BulkResult{Items: make([]domain.Item, 0, len(moves))}

	for _, move := range moves {
		productID, productLabel := c.ParseMany2One(move["product_id"])
		product := productMap[productID]

		sku := firstNonEmpty(
			stringFromAny(product["default_code"]),
			c.ParseSKUFromLabel(productLabel),
		)
		_, moveLocationLabel := c.ParseMany2One(move["location_id"])
		itemLocation := preferredLocation(c.FormatOdooLocation(moveLocationLabel), skuLocations[sku])
		name := firstNonEmpty(
			stringFromAny(product["name"]),
			stringFromAny(move["description_picking"]),
			productLabel,
			stringFromAny(move["display_name"]),
			sku,
			"Unknown Product",
		)

		quantity := intFromAny(move["product_uom_qty"])
		if quantity == 0 {
			quantity = intFromAny(move["quantity"])
		}

		unitWeightOz := 0.0
		lineWeightOz := 0.0
		if _, exists := product["weight"]; exists && product["weight"] != nil {
			unitWeightOz = floatFromAny(product["weight"]) * 16
			lineWeightOz = unitWeightOz * float64(quantity)
		} else {
			lineWeightOz = floatFromAny(move["weight"]) * 16
			if quantity > 0 {
				unitWeightOz = lineWeightOz / float64(quantity)
			}
		}

		qtyAvailable := lenexaQtyAvailable(sku, lenexaQtyBySKU)

		result.TotalWeightOz += lineWeightOz
		result.Items = append(result.Items, domain.Item{
			SKU:          sku,
			Name:         name,
			Quantity:     quantity,
			SKULocation:  itemLocation,
			Weight:       unitWeightOz,
			Volume:       floatFromAny(product["volume"]),
			Height:       floatFromSelectedField(product, fieldSelection.heightField),
			Width:        floatFromSelectedField(product, fieldSelection.widthField),
			Length:       floatFromSelectedField(product, fieldSelection.lengthField),
			QtyAvailable: qtyAvailable,
		})
	}

	return result
}

func (c *Client) detectProductFieldSelection(forceRefresh bool) (productFieldSelection, error) {
	payload := map[string]any{
		"attributes": []string{"string", "type"},
	}

	result, err := c.postJSON2("product.product", "fields_get", payload, forceRefresh)
	if err != nil {
		return productFieldSelection{}, err
	}

	fields, ok := result.(map[string]any)
	if !ok {
		return productFieldSelection{}, fmt.Errorf("unexpected product.product.fields_get payload type %T", result)
	}

	selection := productFieldSelection{}
	used := map[string]struct{}{}
	selection.lengthField = selectNumericField(fields, []string{"length", "product_length", "x_studio_length"}, []string{"length"}, used)
	if selection.lengthField != "" {
		used[selection.lengthField] = struct{}{}
	}
	selection.widthField = selectNumericField(fields, []string{"width", "product_width", "x_studio_width"}, []string{"width"}, used)
	if selection.widthField != "" {
		used[selection.widthField] = struct{}{}
	}
	selection.heightField = selectNumericField(fields, []string{"height", "product_height", "x_studio_height"}, []string{"height"}, used)

	return selection, nil
}

func appendProductReadFields(base []string, selection productFieldSelection) []string {
	fields := append([]string(nil), base...)
	for _, field := range []string{selection.lengthField, selection.widthField, selection.heightField} {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		alreadyIncluded := false
		for _, existing := range fields {
			if existing == field {
				alreadyIncluded = true
				break
			}
		}
		if !alreadyIncluded {
			fields = append(fields, field)
		}
	}
	return fields
}

func selectNumericField(fields map[string]any, preferredNames []string, labelTokens []string, exclude map[string]struct{}) string {
	for _, preferred := range preferredNames {
		preferred = strings.TrimSpace(preferred)
		if preferred == "" {
			continue
		}
		if _, skip := exclude[preferred]; skip {
			continue
		}
		meta, ok := fields[preferred].(map[string]any)
		if ok && isNumericFieldMeta(meta) {
			return preferred
		}
	}

	type candidate struct {
		name  string
		score int
	}

	best := candidate{}
	for name, rawMeta := range fields {
		if _, skip := exclude[name]; skip {
			continue
		}
		meta, ok := rawMeta.(map[string]any)
		if !ok || !isNumericFieldMeta(meta) {
			continue
		}

		nameLower := strings.ToLower(strings.TrimSpace(name))
		labelLower := strings.ToLower(strings.TrimSpace(stringFromAny(meta["string"])))
		score := 0
		for _, token := range labelTokens {
			token = strings.ToLower(strings.TrimSpace(token))
			if token == "" {
				continue
			}
			if labelLower == token {
				score = max(score, 4)
			}
			if strings.Contains(nameLower, token) {
				score = max(score, 3)
			}
			if strings.Contains(labelLower, token) {
				score = max(score, 2)
			}
		}
		if score > best.score {
			best = candidate{name: name, score: score}
		}
	}

	return best.name
}

func isNumericFieldMeta(meta map[string]any) bool {
	switch strings.TrimSpace(strings.ToLower(stringFromAny(meta["type"]))) {
	case "float", "integer", "monetary":
		return true
	default:
		return false
	}
}

func floatFromSelectedField(values map[string]any, fieldName string) float64 {
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		return 0
	}
	return floatFromAny(values[fieldName])
}

func extractPayload(data any) any {
	if m, ok := data.(map[string]any); ok {
		if result, exists := m["result"]; exists {
			return result
		}
	}
	return data
}

func rowsFromAny(value any) ([]map[string]any, error) {
	switch rows := value.(type) {
	case []map[string]any:
		return rows, nil
	case []any:
		result := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			rowMap, ok := row.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unexpected row type %T", row)
			}
			result = append(result, rowMap)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unexpected rows payload type %T", value)
	}
}

func parseStringIDs(ids []string) ([]int, error) {
	parsed := make([]int, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		parsedID, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("parse id %q: %w", id, err)
		}
		parsed = append(parsed, parsedID)
	}
	return parsed, nil
}

func preferredLocation(current string, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	current = strings.TrimSpace(current)
	if candidate == "" {
		return current
	}
	if current == "" {
		return candidate
	}

	candidateScore := locationQualityScore(candidate)
	currentScore := locationQualityScore(current)
	if candidateScore > currentScore {
		return candidate
	}
	return current
}

func locationQualityScore(location string) int {
	location = strings.TrimSpace(strings.ToUpper(location))
	if location == "" {
		return 0
	}
	if looksLikeShelfLocation(location) {
		return 3
	}
	if strings.Contains(location, "CON/STOCK") || strings.Contains(location, "/STOCK") {
		return 1
	}
	if strings.Contains(location, "/") {
		return 1
	}
	return 2
}

func looksLikeShelfLocation(location string) bool {
	location = strings.TrimSpace(strings.ToUpper(location))
	if location == "" {
		return false
	}

	parts := strings.SplitN(location, ".", 2)
	base := parts[0]
	if len(base) < 2 || base[0] < 'A' || base[0] > 'Z' {
		return false
	}
	for i := 1; i < len(base); i++ {
		if base[i] < '0' || base[i] > '9' {
			return false
		}
	}
	if len(parts) == 2 {
		if parts[1] == "" {
			return false
		}
		for i := 0; i < len(parts[1]); i++ {
			if parts[1][i] < '0' || parts[1][i] > '9' {
				return false
			}
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func isSingleLetterSegment(value string) bool {
	if len(value) != 1 {
		return false
	}
	ch := value[0]
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func normalizeShelfSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if n, err := strconv.Atoi(value); err == nil {
		return fmt.Sprintf("%03d", n)
	}
	return value
}

func normalizeSubLocation(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if n, err := strconv.Atoi(value); err == nil {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	if value == "0" {
		return ""
	}
	return value
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(math.Round(float64(v)))
	case float64:
		return int(math.Round(v))
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return int(i)
		}
		f, err := v.Float64()
		if err == nil {
			return int(math.Round(f))
		}
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return i
		}
	}
	return 0
}

func floatFromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, err := v.Float64()
		if err == nil {
			return f
		}
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return f
		}
	}
	return 0
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case int, int8, int16, int32, int64, float32, float64:
		return fmt.Sprintf("%v", v)
	default:
		return ""
	}
}

func retryDelay(attempt int) time.Duration {
	base := float64(backoffBase) * math.Pow(2, float64(attempt))
	jitterFactor := 1 + ((rand.Float64() * 2 * backoffJitter) - backoffJitter)
	return time.Duration(base * jitterFactor)
}

func truncateBody(body []byte, max int) string {
	text := string(body)
	if len(text) <= max {
		return text
	}
	return text[:max]
}

func asHTTPError(err error, target **HTTPError) bool {
	httpErr, ok := err.(*HTTPError)
	if ok {
		*target = httpErr
	}
	return ok
}

func (c *Client) loadInventoryCache(ttl time.Duration) (map[string]string, bool, error) {
	if c == nil || !c.cfg.UseCache || c.cfg.Cache == nil {
		return nil, false, nil
	}

	cached, storedAt, ok, err := c.cfg.Cache.GetWithTimestamp(inventoryCacheKey, nil)
	if err != nil || !ok {
		return nil, false, err
	}
	if time.Since(storedAt) > ttl {
		return nil, false, nil
	}

	locs, err := stringMapFromAny(cached)
	if err != nil {
		return nil, false, err
	}
	return locs, true, nil
}

func (c *Client) loadInventoryCacheStale() (map[string]string, error) {
	if c == nil || !c.cfg.UseCache || c.cfg.Cache == nil {
		return nil, nil
	}

	cached, _, ok, err := c.cfg.Cache.GetWithTimestamp(inventoryCacheKey, nil)
	if err != nil || !ok {
		return nil, err
	}
	return stringMapFromAny(cached)
}

func (c *Client) saveInventoryCache(locs map[string]string) error {
	if c == nil || !c.cfg.UseCache || c.cfg.Cache == nil {
		return nil
	}
	return c.cfg.Cache.Put(inventoryCacheKey, nil, locs)
}

func lenexaQtyAvailable(sku string, lenexaQtyBySKU map[string]float64) float64 {
	sku = strings.TrimSpace(sku)
	if sku == "" || lenexaQtyBySKU == nil {
		return -1
	}
	qty, ok := lenexaQtyBySKU[sku]
	if !ok {
		return 0
	}
	return qty
}

func (c *Client) loadLenexaQtyCache(ttl time.Duration) (map[string]float64, bool, error) {
	if c == nil || !c.cfg.UseCache || c.cfg.Cache == nil {
		return nil, false, nil
	}

	cached, storedAt, ok, err := c.cfg.Cache.GetWithTimestamp(lenexaQtyCacheKey, nil)
	if err != nil || !ok {
		return nil, false, err
	}
	if time.Since(storedAt) > ttl {
		return nil, false, nil
	}

	qty, err := floatMapFromAny(cached)
	if err != nil {
		return nil, false, err
	}
	return qty, true, nil
}

func (c *Client) loadLenexaQtyCacheStale() (map[string]float64, error) {
	if c == nil || !c.cfg.UseCache || c.cfg.Cache == nil {
		return nil, nil
	}

	cached, _, ok, err := c.cfg.Cache.GetWithTimestamp(lenexaQtyCacheKey, nil)
	if err != nil || !ok {
		return nil, err
	}
	return floatMapFromAny(cached)
}

func (c *Client) saveLenexaQtyCache(qty map[string]float64) error {
	if c == nil || !c.cfg.UseCache || c.cfg.Cache == nil {
		return nil
	}
	return c.cfg.Cache.Put(lenexaQtyCacheKey, nil, qty)
}

func stringMapFromAny(value any) (map[string]string, error) {
	switch v := value.(type) {
	case nil:
		return map[string]string{}, nil
	case map[string]string:
		copy := make(map[string]string, len(v))
		for key, entry := range v {
			copy[key] = entry
		}
		return copy, nil
	case map[string]any:
		copy := make(map[string]string, len(v))
		for key, entry := range v {
			copy[key] = stringFromAny(entry)
		}
		return copy, nil
	default:
		return nil, fmt.Errorf("unexpected inventory cache payload type %T", value)
	}
}

func floatMapFromAny(value any) (map[string]float64, error) {
	switch v := value.(type) {
	case nil:
		return map[string]float64{}, nil
	case map[string]float64:
		copy := make(map[string]float64, len(v))
		for key, entry := range v {
			copy[key] = entry
		}
		return copy, nil
	case map[string]any:
		copy := make(map[string]float64, len(v))
		for key, entry := range v {
			copy[key] = floatFromAny(entry)
		}
		return copy, nil
	default:
		return nil, fmt.Errorf("unexpected Lenexa qty cache payload type %T", value)
	}
}
