package odoo

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"pickcart/internal/domain"
)

type Provider struct {
	client *Client
}

func NewProvider(client *Client) *Provider {
	return &Provider{client: client}
}

func (p *Provider) GetRecentBatches(limit int, maxBatchSize int, forceRefresh bool) ([]*domain.Batch, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("odoo provider client is not initialized")
	}

	rows, err := p.client.GetBatches(limit, forceRefresh)
	if err != nil {
		return nil, err
	}

	batches := make([]*domain.Batch, 0, len(rows))
	for _, row := range rows {
		batchID := intFromAny(row["id"])
		if batchID <= 0 {
			continue
		}

		shipmentIDs := stringIDsFromAny(row["picking_ids"])
		if maxBatchSize > 0 && len(shipmentIDs) > maxBatchSize {
			continue
		}

		batches = append(batches, &domain.Batch{
			ID:              strconv.Itoa(batchID),
			Name:            firstNonEmpty(stringFromAny(row["name"]), strconv.Itoa(batchID)),
			State:           stringFromAny(row["state"]),
			CreatedAt:       parseOdooTime(stringFromAny(row["create_date"])),
			ShipmentIDs:     shipmentIDs,
			ShipmentsLoaded: false,
		})
	}

	return batches, nil
}

func (p *Provider) GetBatchShipments(batchID string, forceRefresh bool) ([]*domain.Shipment, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("odoo provider client is not initialized")
	}

	parsedBatchID, err := strconv.Atoi(strings.TrimSpace(batchID))
	if err != nil {
		return nil, fmt.Errorf("parse batch id %q: %w", batchID, err)
	}

	rows, err := p.client.GetBatchShipments(parsedBatchID, forceRefresh)
	if err != nil {
		return nil, err
	}

	shipments := make([]*domain.Shipment, 0, len(rows))
	for _, row := range rows {
		shipmentID := intFromAny(row["id"])
		if shipmentID <= 0 {
			continue
		}

		_, shipTo := p.client.ParseMany2One(row["partner_id"])
		_, serviceCode := p.client.ParseMany2One(row["carrier_id"])
		externalID := firstNonEmpty(stringFromAny(row["name"]), strconv.Itoa(shipmentID))

		shipments = append(shipments, &domain.Shipment{
			ID:             strconv.Itoa(shipmentID),
			ExternalID:     externalID,
			Name:           firstNonEmpty(stringFromAny(row["origin"]), externalID),
			ShipTo:         shipTo,
			ServiceCode:    serviceCode,
			TrackingNumber: stringFromAny(row["carrier_tracking_ref"]),
			ItemsLoaded:    false,
		})
	}

	return shipments, nil
}

func (p *Provider) GetBatchShipmentItemsBulk(ids []string, skuLocations map[string]string) (map[string]BulkResult, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("odoo provider client is not initialized")
	}

	return p.client.GetBatchShipmentItemsBulk(ids, false, skuLocations)
}

func (p *Provider) GetTrackingNumbers(ids []string) (map[string]string, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("odoo provider client is not initialized")
	}

	return p.client.GetTrackingNumbers(ids)
}

func (p *Provider) GetInventory() (map[string]string, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("odoo provider client is not initialized")
	}

	return p.client.GetInventory()
}

func parseOdooTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}

	return time.Time{}
}

func stringIDsFromAny(value any) []string {
	switch v := value.(type) {
	case []any:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			id := intFromAny(item)
			if id > 0 {
				ids = append(ids, strconv.Itoa(id))
			}
		}
		return ids
	case []int:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			if item > 0 {
				ids = append(ids, strconv.Itoa(item))
			}
		}
		return ids
	default:
		return nil
	}
}
