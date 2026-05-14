package domain

import "time"

type RowConfig struct {
	Cols int
}

type CartProfile struct {
	Name           string
	DisplayName    string
	MaxBatchSize   int
	RowConfigs     []RowConfig
	LEDColumnIndex *int // nil means fall back to cart number
}

type Batch struct {
	ID              string
	Name            string
	State           string
	CreatedAt       time.Time
	ShipmentIDs     []string
	Shipments       []*Shipment
	ShipmentsLoaded bool
}

type Shipment struct {
	ID             string
	ExternalID     string
	Name           string
	ShipTo         string
	ServiceCode    string
	TrackingNumber string
	Items          []Item
	ItemsLoaded    bool
	Weight         float64 // ounces
	BoxName        string
}

type Item struct {
	SKU          string
	Name         string
	Quantity     int
	SKULocation  string
	Weight       float64
	Volume       float64
	Height       float64
	Width        float64
	Length       float64
	QtyAvailable float64
}

type PickItem struct {
	Name         string
	SKU          string
	Location     string
	Quantity     int
	Height       float64
	Width        float64
	Length       float64
	QtyAvailable float64
	Shipments    []PickShipment
}

type PickShipment struct {
	ShipmentID string
	ExternalID string
	Quantity   int
	Location   string
}

type Box struct {
	Name   string
	Length float64
	Width  float64
	Height float64
	Volume float64
}
