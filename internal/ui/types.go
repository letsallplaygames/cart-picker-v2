package ui

type ShelfCell struct {
	Location   string
	ExternalID string
	ShipmentID string
	BoxName    string
	HasOrder   bool
}

type FindOrderEntry struct {
	ShipmentID     string
	ExternalID     string
	TrackingNumber string
	GridIndex      int
}

type FindOrderDetail struct {
	CustomerName   string
	TrackingNumber string
	Location       string
}
