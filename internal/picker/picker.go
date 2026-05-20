package picker

import (
	"fmt"
	"sync"

	"pickcart/internal/domain"
	"pickcart/internal/odoo"
)

type Picker struct {
	provider     *odoo.Provider
	Batches      []*domain.Batch
	SKULocations map[string]string
	selected     map[string]*domain.Shipment
	mu           sync.RWMutex
}

func New(provider *odoo.Provider) *Picker {
	return &Picker{
		provider:     provider,
		Batches:      []*domain.Batch{},
		SKULocations: map[string]string{},
		selected:     map[string]*domain.Shipment{},
	}
}

func (p *Picker) LoadBatches(limit int, forceRefresh bool, callback func(count int, err error)) {
	go func() {
		if p == nil || p.provider == nil {
			invokeBatchCallback(callback, 0, fmt.Errorf("picker provider is not initialized"))
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)

		var (
			batches      []*domain.Batch
			batchesErr   error
			skuLocations map[string]string
			inventoryErr error
		)

		go func() {
			defer wg.Done()
			skuLocations, inventoryErr = p.provider.GetInventory()
		}()

		go func() {
			defer wg.Done()
			batches, batchesErr = p.provider.GetRecentBatches(limit, 0, forceRefresh)
		}()

		wg.Wait()

		if inventoryErr != nil {
			invokeBatchCallback(callback, 0, inventoryErr)
			return
		}
		if batchesErr != nil {
			invokeBatchCallback(callback, 0, batchesErr)
			return
		}

		p.mu.Lock()
		p.SKULocations = copyStringMap(skuLocations)
		p.Batches = batches
		p.mu.Unlock()

		invokeBatchCallback(callback, len(batches), nil)
	}()
}

func (p *Picker) CheckForNewBatches(limit int, callback func(newCount int, err error)) {
	go func() {
		if p == nil || p.provider == nil {
			invokeNewBatchCallback(callback, 0, fmt.Errorf("picker provider is not initialized"))
			return
		}

		p.mu.RLock()
		currentBatchIDs := make(map[string]struct{}, len(p.Batches))
		currentBatchCount := len(p.Batches)
		for _, batch := range p.Batches {
			if batch == nil || batch.ID == "" {
				continue
			}
			currentBatchIDs[batch.ID] = struct{}{}
		}
		p.mu.RUnlock()

		freshBatches, err := p.provider.GetRecentBatches(limit, 0, true)
		if err != nil {
			invokeNewBatchCallback(callback, 0, err)
			return
		}

		newCount := 0
		for _, batch := range freshBatches {
			if batch == nil || batch.ID == "" {
				continue
			}
			if _, exists := currentBatchIDs[batch.ID]; !exists {
				newCount++
			}
		}

		if newCount > 0 || len(freshBatches) != currentBatchCount {
			p.mu.Lock()
			p.Batches = freshBatches
			p.mu.Unlock()
		}

		invokeNewBatchCallback(callback, newCount, nil)
	}()
}

func (p *Picker) LoadBatchShipments(batchID string, forceRefresh bool, callback func([]*domain.Shipment, error)) {
	go func() {
		if p == nil || p.provider == nil {
			invokeShipmentCallback(callback, nil, fmt.Errorf("picker provider is not initialized"))
			return
		}

		p.mu.RLock()
		fallbackIDs := []string(nil)
		for _, batch := range p.Batches {
			if batch == nil || batch.ID != batchID {
				continue
			}
			fallbackIDs = append([]string(nil), batch.ShipmentIDs...)
			break
		}
		p.mu.RUnlock()

		shipments, err := p.provider.GetBatchShipments(batchID, forceRefresh)
		if err != nil {
			invokeShipmentCallback(callback, nil, err)
			return
		}
		if len(shipments) == 0 && len(fallbackIDs) > 0 {
			shipments, err = p.provider.GetShipmentsByIDs(fallbackIDs, forceRefresh)
			if err != nil {
				invokeShipmentCallback(callback, nil, err)
				return
			}
		}

		shipmentIDs := make([]string, 0, len(shipments))
		for _, shipment := range shipments {
			if shipment == nil || shipment.ID == "" {
				continue
			}
			shipmentIDs = append(shipmentIDs, shipment.ID)
		}

		p.mu.Lock()
		for _, batch := range p.Batches {
			if batch == nil || batch.ID != batchID {
				continue
			}
			batch.Shipments = shipments
			batch.ShipmentIDs = shipmentIDs
			batch.ShipmentsLoaded = true
			break
		}
		p.mu.Unlock()

		invokeShipmentCallback(callback, shipments, nil)
	}()
}

func (p *Picker) LoadShipmentItemsBulk(shipmentIDs []string, callback func(map[string]odoo.BulkResult, error)) {
	go func() {
		if p == nil || p.provider == nil {
			invokeBulkCallback(callback, nil, fmt.Errorf("picker provider is not initialized"))
			return
		}

		p.mu.RLock()
		skuLocations := copyStringMap(p.SKULocations)
		p.mu.RUnlock()

		results, err := p.provider.GetBatchShipmentItemsBulk(shipmentIDs, skuLocations)
		invokeBulkCallback(callback, results, err)
	}()
}

func (p *Picker) SelectShipment(id string, s *domain.Shipment) {
	if p == nil || id == "" || s == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.selected[id] = s
}

func (p *Picker) DeselectShipment(id string) {
	if p == nil || id == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.selected, id)
}

func (p *Picker) ClearSelection() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.selected = map[string]*domain.Shipment{}
}

func (p *Picker) IsSelected(id string) bool {
	if p == nil || id == "" {
		return false
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.selected[id]
	return ok
}

func (p *Picker) GetSelected() map[string]*domain.Shipment {
	if p == nil {
		return map[string]*domain.Shipment{}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	selected := make(map[string]*domain.Shipment, len(p.selected))
	for id, shipment := range p.selected {
		selected[id] = shipment
	}
	return selected
}

func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}

	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func invokeBatchCallback(callback func(count int, err error), count int, err error) {
	if callback != nil {
		callback(count, err)
	}
}

func invokeNewBatchCallback(callback func(newCount int, err error), newCount int, err error) {
	if callback != nil {
		callback(newCount, err)
	}
}

func invokeShipmentCallback(callback func([]*domain.Shipment, error), shipments []*domain.Shipment, err error) {
	if callback != nil {
		callback(shipments, err)
	}
}

func invokeBulkCallback(callback func(map[string]odoo.BulkResult, error), results map[string]odoo.BulkResult, err error) {
	if callback != nil {
		callback(results, err)
	}
}
