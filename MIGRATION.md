# PickCart Go + Fyne Migration Draft

## 1. Project Overview

**App name:** PickCart

**Purpose:**
PickCart is a Raspberry Pi-based warehouse order-picking system. It integrates with **Odoo ERP** to load batches and shipments, then uses **WS2812B addressable LEDs** connected to **GPIO 18** to highlight physical cart bins during picking.

**Current implementation:**
- Python 3.11+
- Tkinter UI
- Odoo API integration
- SQLite response cache
- Raspberry Pi LED control through `rpi_ws281x`

**Target implementation:**
- **Go 1.22+**
- **Fyne** (`fyne.io/fyne/v2`) for the GUI
- Same Odoo-backed picking workflow
- Same Raspberry Pi LED behavior
- Same cart profiles and physical layout behavior
- Better structure for a Pi 3b with 1GB RAM

**Primary target hardware:**
- Raspberry Pi 3b
- Linux ARM (`linux/arm` for 32-bit Raspberry Pi OS)
- Also support `linux/arm64`
- Our Raspberry Pi's are currently configured with a 64-bit OS

**Development platform:**
- macOS for local development
- Linux builds/releases for Raspberry Pi

---

## 2. Goals of the Rewrite

The rewrite should preserve the current app behavior while improving structure, maintainability, and deployment.

### Preserve
- Batch/shipment workflow
- Odoo integration
- SKU location lookup from Odoo inventory/location records
- Product dimension loading
- Box recommendation behavior
- LED mapping from Google Sheets
- Shelf/cart layout behavior
- Barcode-scanner-based order lookup

### Improve
- Clear package boundaries
- Lower memory overhead than Python on Pi 3b
- Better release packaging through GitHub Releases
- Simpler development on macOS using a stub LED controller
- Better separation between UI, state management, data access, and hardware

### Non-goals for the initial migration
- Do not redesign the business workflow
- Do not change warehouse layout behavior
- Do not switch away from Odoo
- Do not rebuild every dev/debug script immediately

---

## 3. Module & Dependencies

**Go module name:**

```go
module pickcart
```

**Dependencies:**
- `fyne.io/fyne/v2 v2.5.1` — GUI
- `github.com/joho/godotenv v1.5.1` — `.env` loading
- `modernc.org/sqlite v1.33.1` — pure-Go SQLite driver

**Important note about CGo:**
- CGo should be used **only** in `internal/led/controller_linux.go`
- That file links against the `libws2811` / `rpi_ws281x` C library on Linux
- All other packages should remain pure Go where possible

---

## 4. Project Layout

```text
pickcart-go/
├── go.mod
├── go.sum
├── .env.example
├── .gitignore
├── Makefile
├── .github/
│   └── workflows/
│       └── release.yml
│
├── cmd/
│   ├── pickcart/
│   │   └── main.go
│   ├── testleds/
│   │   └── main.go
│   └── mapleds/
│       └── main.go
│
└── internal/
    ├── domain/
    │   └── types.go
    ├── config/
    │   └── config.go
    ├── cache/
    │   └── cache.go
    ├── odoo/
    │   ├── client.go
    │   └── provider.go

    ├── dimensions/
    │   └── loader.go
    ├── picker/
    │   └── picker.go
    ├── boxing/
    │   └── calculator.go
    ├── led/
    │   ├── stub.go
    │   └── controller_linux.go
    └── ui/
        ├── app.go
        ├── shipment_list.go
        ├── shelf_layout.go
        ├── shelf_list.go
        └── find_order.go
```

---

## 5. Cross-Cutting Concerns

### 5.1 Concurrency model

The Python app uses background threads plus Tkinter callbacks. In Go:
- Use **goroutines** for background work
- Keep package APIs callback-friendly where appropriate
- Keep shared state behind `sync.RWMutex` or atomic fields where needed
- Guard against stale async results using an **epoch**

### 5.2 Stale result / epoch guard

The app should preserve the Python app's stale-selection protection.

`ui.App` should have:

```go
type App struct {
    selectionEpoch int64
}
```

Rules:
- Increment `selectionEpoch` every time shipment selection changes
- Any background load started from a selection change captures the current epoch
- When the result returns, compare captured epoch to current epoch
- If they differ, discard the result

This prevents old bulk item loads from overwriting a newer UI state.

### 5.3 Error handling policy

- **cache**: fail-open; log and return cache miss
- **odoo**: return errors to caller
- **picker**: return/callback with errors; never panic
- **dimensions loader**: fail-open; empty map on network failure
- **LED controller**: fail-open; if init fails, become a no-op controller
- **UI**: show load state and errors via status labels; never crash on expected failures

### 5.4 Build tags

Two LED implementations should exist with the same exported API:

- `internal/led/stub.go`
  - Build tag: `//go:build !linux`
  - Used on macOS and CI
  - No-op implementation

- `internal/led/controller_linux.go`
  - Build tag: `//go:build linux`
  - Used on Raspberry Pi Linux
  - Real hardware implementation through CGo + `libws2811`

### 5.5 Runtime behavior on Pi

The Linux LED controller must assume the same hardware settings as the Python version:
- GPIO pin: 18
- Frequency: 800000 Hz
- DMA: 10
- Brightness: 128
- Channel: 0
- Max LED count: 300
- Strip type: GRB

The Pi must also have:
- `dtparam=audio=off`
- `gpu_mem=64`
- `libws2811` installed
- the app running with sufficient privileges for `/dev/mem` access when using `rpi_ws281x`

### 5.6 Fyne UI updates

The Fyne UI layer should keep updates serialized and explicit. Background work should update shared state first, then refresh widgets in a controlled way. Avoid assuming widget state changes are free-threaded without care.

---

## 6. File Specifications

This section describes each file that should exist in the Go rewrite and what it should do.

---

## 6.1 `go.mod`

**Path:** `go.mod`

**Purpose:**
Defines the Go module and versioned dependencies.

**Expected contents:**

```go
module pickcart

go 1.22

require (
    fyne.io/fyne/v2 v2.5.1
    github.com/joho/godotenv v1.5.1
    modernc.org/sqlite v1.33.1
)
```

`go.sum` should be generated automatically.

---

## 6.2 `.env.example`

**Path:** `.env.example`

**Purpose:**
Documents runtime environment variables required for Odoo access.

**Expected contents:**

```env
ODOO_API_KEY=
ODOO_BASE_URL=
ODOO_DATABASE=
```

---

## 6.3 `internal/domain/types.go`

**Package:** `domain`

**Replaces:** `data_interface.py`

**Purpose:**
Defines all shared domain types used across the app.

**Types:**

```go
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
    Weight       float64 // per-unit weight in ounces, converted from Odoo `product.product.weight` (pounds)
    Volume       float64 // per-unit volume in cubic feet from Odoo `product.product.volume`
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
    Volume float64 // cubic feet, derived from the inch dimensions for box-fit comparison
}
```

**Implementation notes:**
- Keep this package dependency-light
- It should only depend on the standard library (`time`)
- All other packages should import these types rather than re-declaring their own copies

---

## 6.4 `internal/config/config.go`

**Package:** `config`

**Replaces:** `config.py`

**Purpose:**
Holds runtime configuration, hardware constants, and built-in cart profiles.

**Hardware constants:**

```go
const (
    LEDPin        = 18
    LEDFreqHz     = 800000
    LEDDMA        = 10
    LEDBrightness = 128
    LEDChannel    = 0
    LEDCount      = 300
)
```

**App config struct:**

```go
type AppConfig struct {
    OdooAPIKey  string
    OdooBaseURL string
    OdooDB      string
    CartNumber  int
    ProfileName string
}
```

**Built-in cart profiles:**

- `standard`
  - `DisplayName = "Standard Cart"`
  - `MaxBatchSize = 100`
  - `RowConfigs = [{28}, {28}, {14}, {14}, {8}, {8}]`

- `small_cart`
  - `DisplayName = "Small Cart"`
  - `MaxBatchSize = 126`
  - `RowConfigs = [{28}, {28}, {28}, {14}, {14}, {14}]`

- `large_cart`
  - `DisplayName = "Large Cart"`
  - `MaxBatchSize = 48`
  - `RowConfigs = [{8}, {8}, {8}, {8}, {8}, {8}]`

**Functions:**

```go
func Load(cartNumber int, profileName string) (*AppConfig, error)
func GetCartProfile(name string) (domain.CartProfile, error)
func AvailableProfiles() []string
func CartCapacity(p domain.CartProfile) int
func TotalCells(p domain.CartProfile) int
```

**Behavior:**
- `Load(...)`
  - Calls `godotenv.Load()` but ignores missing `.env`
  - Reads `ODOO_API_KEY`, `ODOO_BASE_URL`, and `ODOO_DATABASE`
  - Validates `profileName`
  - Returns populated `AppConfig`

- `GetCartProfile(...)`
  - Returns a deep copy of the selected profile
  - Must allocate a fresh `RowConfigs` slice

- `AvailableProfiles()`
  - Returns profile names sorted alphabetically

- `CartCapacity(...)`
  - Uses `MaxBatchSize` if non-zero
  - Falls back to sum of `RowConfigs[i].Cols`

- `TotalCells(...)`
  - Sums total cart cells from row configs

---

## 6.5 `internal/cache/cache.go`

**Package:** `cache`

**Replaces:** `api_cache.py`

**Purpose:**
Provides a SQLite-backed cache for Odoo API responses.

**Struct:**

```go
type Cache struct {
    db   *sql.DB
    mu   sync.Mutex
    path string
}
```

**Schema:**

```sql
CREATE TABLE IF NOT EXISTS api_cache (
    key TEXT PRIMARY KEY,
    data TEXT NOT NULL,
    timestamp INTEGER NOT NULL
)
```

**Functions:**

```go
func New(dbPath string) (*Cache, error)
func (c *Cache) Get(endpoint string, params any, ttlHours int) (any, bool)
func (c *Cache) Put(endpoint string, params any, data any) error
func (c *Cache) Clear(olderThanHours int) error
```

**Behavior:**
- Uses `modernc.org/sqlite` with `database/sql`
- Enables WAL mode and `synchronous=NORMAL`
- Cache key is MD5 hex of `endpoint + stable-json(params)`
- `Get(...)`
  - Returns `(value, true)` on hit
  - Returns `(_, false)` on miss, expiry, or any error
  - Deletes stale entries when found expired
- `Put(...)`
  - Stores JSON-encoded payload and current Unix timestamp
  - Uses `INSERT OR REPLACE`
- `Clear(...)`
  - `olderThanHours == 0` wipes everything
  - Otherwise removes old entries

**Important implementation detail:**
Go's `encoding/json` does not guarantee sorted map keys by API contract, so the cache implementation should normalize parameters into stable JSON before hashing. A simple approach is to recursively normalize maps into key-sorted structures before marshaling.

---

## 6.6 `internal/odoo/client.go`

**Package:** `odoo`

**Replaces:** `odoo_client.py`

**Purpose:**
Lowest-level Odoo JSON API client.

**Config struct:**

```go
type Config struct {
    APIKey   string
    BaseURL  string
    Database string
    Cache    *cache.Cache
    UseCache bool
}
```

**Bulk result struct:**

```go
type BulkResult struct {
    Items         []domain.Item
    TotalWeightOz float64
}
```

**Client struct:**

```go
type Client struct {
    cfg  Config
    http *http.Client
}
```

**Constants:**
- `maxRetries = 5`
- `backoffBase = 1 * time.Second`
- `backoffJitter = 0.25`
- `lenexaPickingTypeID = 446`

**Functions:**

```go
func New(cfg Config) *Client
func (c *Client) SearchRead(model string, domain []any, fields []string, limit, offset int, order string, forceRefresh bool) ([]map[string]any, error)
func (c *Client) Read(model string, ids []int, fields []string, forceRefresh bool) ([]map[string]any, error)
func (c *Client) ParseMany2One(value any) (id int, name string)
func (c *Client) ParseSKUFromLabel(label string) string
func (c *Client) FormatOdooLocation(completeName string) string
func (c *Client) GetInventory() (map[string]string, error)
func (c *Client) GetBatches(limit int, forceRefresh bool) ([]map[string]any, error)
func (c *Client) GetBatchShipments(batchID int, forceRefresh bool) ([]map[string]any, error)
func (c *Client) GetBatchShipmentItemsBulk(shipmentIDs []string, forceRefresh bool, skuLocations map[string]string) (map[string]BulkResult, error)
func (c *Client) GetTrackingNumbers(shipmentIDs []string) (map[string]string, error)
```

**Core internal helper:**

```go
func (c *Client) postJSON2(model, method string, payload map[string]any, forceRefresh bool) (any, error)
```

**Behavior details:**

### `postJSON2(...)`
- Builds URL as:
  - `{BaseURL}/json/2/{model}/{method}`
- Sets headers:
  - `Authorization: Bearer {APIKey}`
  - `Content-Type: application/json`
- Uses cache unless `forceRefresh` is true
- Retries on HTTP 429 and 503 with exponential backoff and jitter
- Backoff formula:
  - `backoffBase * 2^attempt`, then add ±25% jitter
- Extracts payload from Odoo response structure
- Caches successful responses

### `ParseMany2One(...)`
Must handle all common Odoo shapes:
- `[id, name]`
- `map[string]any{"id":..., "display_name":...}`
- bare `int`

### `ParseSKUFromLabel(...)`
If label is in form `[ABC123] Product Name`, extract `ABC123`.

### `FormatOdooLocation(...)`
Converts examples like:
- `LNX/STOCK/A/018/3` → `A018.3`
- `LNX/STOCK/A/022` → `A022`
- `LNX/STOCK/B/014/0` → `B014`

Rules:
- Split on `/`
- Find first single-letter path segment (aisle)
- Use next segment as the main shelf number, zero-padded to 3 digits
- Use optional third segment as sub-location if it is not `0`

### `GetInventory()`
- Calls `stock.quant/search_read`
- Domain:
  - `location_id.usage = internal`
  - `quantity > 0`
- Fields:
  - `product_id`
  - `location_id`
- Limit: `10000`
- Reads the corresponding `stock.location` records to get `complete_name`
- Builds a `map[string]string` of `SKU -> location`
- Converts each Odoo `complete_name` using `FormatOdooLocation(...)`
- Uses a separate file cache `sku_location_cache.json`
- File cache TTL: 4 hours based on file mtime

### `GetBatches(...)`
- Calls `stock.picking.batch/search_read`
- Filters by:
  - `picking_type_id = 446`
  - `state in [in_progress, done]`
- Fields:
  - `id`, `name`, `create_date`, `state`, `picking_ids`
- Order by `create_date desc`

### `GetBatchShipments(...)`
- Calls `stock.picking/search_read`
- Filters by:
  - `batch_id = batchID`
  - `picking_type_id = 446`
  - `state in [assigned, done]`
- Uses page size `500`
- Paginates until fewer than 500 records returned

### `GetBatchShipmentItemsBulk(...)`
This is the most important performance path.

It must fetch all items for a list of shipment IDs in exactly **two API calls**:
1. `stock.move/search_read` for all shipment IDs
2. `product.product/read` for all unique product IDs

`product.product/read` must request at least:
- `id`
- `default_code`
- `name`
- `weight`
- `volume`

Then:
- Group moves by `picking_id`
- Build `[]domain.Item` for each shipment
- Read `product.product.weight` (pounds) and `product.product.volume`
- Convert per-unit weight from pounds to ounces
- Preserve per-unit volume in cubic feet
- Calculate total shipment weight in ounces
- Return `map[shipmentID]BulkResult`

### `buildItemsFromMoves(...)`
Internal helper used by bulk loading.

Responsibilities:
- Extract product metadata
- Fill `domain.Item`
- Resolve `SKULocation` from `skuLocations`
- Populate `Item.Weight` from `product.product.weight` (stored in pounds, converted to ounces), falling back to move weight if needed
- Populate `Item.Volume` from `product.product.volume` (cubic feet)
- Sum total shipment weight in ounces

### `GetTrackingNumbers(...)`
- Reads `stock.picking` records by ID
- Returns `shipmentID -> carrier_tracking_ref`

---

## 6.7 `internal/odoo/provider.go`

**Package:** `odoo`

**Replaces:** `odoo_provider.py`

**Purpose:**
Thin facade converting raw Odoo rows into domain structs.

**Struct:**

```go
type Provider struct {
    client *Client
}
```

**Functions:**

```go
func NewProvider(client *Client) *Provider
func (p *Provider) GetRecentBatches(limit int, maxBatchSize int, forceRefresh bool) ([]*domain.Batch, error)
func (p *Provider) GetBatchShipments(batchID string, forceRefresh bool) ([]*domain.Shipment, error)
func (p *Provider) GetBatchShipmentItemsBulk(ids []string, skuLocations map[string]string) (map[string]BulkResult, error)
func (p *Provider) GetTrackingNumbers(ids []string) (map[string]string, error)
func (p *Provider) GetInventory() (map[string]string, error)
```

**Behavior:**
- `GetRecentBatches(...)`
  - Calls client `GetBatches(...)`
  - Maps each row to `domain.Batch`
  - Converts `id` to string
  - Uses `name`, `state`, `create_date`, `picking_ids`
  - Sets `ShipmentsLoaded = false`
  - Filters out oversized batches when `maxBatchSize > 0`

- `GetBatchShipments(...)`
  - Parses `batchID` string to int
  - Calls client `GetBatchShipments(...)`
  - Maps fields to `domain.Shipment`
  - Uses `partner_id` → `ShipTo`
  - Uses `carrier_id` → `ServiceCode`
  - Uses `carrier_tracking_ref` → `TrackingNumber`
  - Sets `ItemsLoaded = false`

- Other methods delegate directly to `Client`

---

## 6.8 SKU location loading

There is no standalone `internal/locations/loader.go` in the Go port.

**Reason:**
SKU locations now come directly from Odoo instead of Google Sheets.

**Implementation location:**
- `internal/odoo/client.go`
- `Client.GetInventory()` reads `stock.quant`
- It then reads `stock.location` to obtain each location `complete_name`
- `FormatOdooLocation(...)` converts Odoo locations into pick-cart format such as `A018.3`

**Migration note:**
`sku_location_importer.py` is not ported directly because its responsibility is now handled inside the Odoo data-access layer.

---

## 6.9 `internal/dimensions/loader.go`

**Package:** `dimensions`

**Replaces:** `product_dimension_loader.py`

**Purpose:**
Loads product dimensions from Google Sheets.

**Type:**

```go
type Dimensions struct {
    Length float64
    Width  float64
    Height float64
}
```

**Functions:**

```go
func GetCSVExportURL(sheetURL string) (string, error)
func LoadFromSheet(sheetURL string) (map[string]Dimensions, error)
```

**Behavior:**
- Converts Google Sheets URL to CSV export URL
- Parses CSV with expected layout:
  - Col A = SKU
  - Col E = Length
  - Col F = Width
  - Col G = Height
- Skips header row
- Skips malformed rows
- Skips rows with non-numeric or non-positive dimensions
- On network failure, returns an empty map instead of a hard error

**Important migration note:**
This loader must not be a startup hard dependency for the app to function. A dimensions failure should disable box sizing only, not LED control or core picking.

---

## 6.10 `internal/picker/picker.go`

**Package:** `picker`

**Replaces:** `shipment_picker.py`

**Purpose:**
Owns batch state, selection state, and async loading workflow.

**Struct:**

```go
type Picker struct {
    provider     *odoo.Provider
    Batches      []*domain.Batch
    SKULocations map[string]string
    selected     map[string]*domain.Shipment
    mu           sync.RWMutex
}
```

**Functions:**

```go
func New(provider *odoo.Provider) *Picker
func (p *Picker) LoadBatches(limit int, forceRefresh bool, callback func(count int, err error))
func (p *Picker) CheckForNewBatches(limit int, callback func(newCount int, err error))
func (p *Picker) LoadBatchShipments(batchID string, forceRefresh bool, callback func([]*domain.Shipment, error))
func (p *Picker) LoadShipmentItemsBulk(shipmentIDs []string, callback func(map[string]odoo.BulkResult, error))
func (p *Picker) SelectShipment(id string, s *domain.Shipment)
func (p *Picker) DeselectShipment(id string)
func (p *Picker) ClearSelection()
func (p *Picker) IsSelected(id string) bool
func (p *Picker) GetSelected() map[string]*domain.Shipment
```

**Behavior:**
- `LoadBatches(...)`
  - Starts a goroutine
  - Loads inventory and batches concurrently via `sync.WaitGroup`
  - Stores both in `Picker`
  - Calls callback with batch count

- `CheckForNewBatches(...)`
  - Loads fresh batches in background
  - Diffs batch IDs against current set
  - Updates `Batches` when changed
  - Calls callback with count of new batches

- `LoadBatchShipments(...)`
  - Fetches shipments for a batch
  - Stores them on the matching `domain.Batch`
  - Marks `ShipmentsLoaded = true`

- `LoadShipmentItemsBulk(...)`
  - Uses current `SKULocations`
  - Delegates to provider bulk loader

- Selection methods must be lock-safe
- `GetSelected()` returns a copy of the internal map

---

## 6.11 `internal/boxing/calculator.go`

**Package:** `boxing`

**Replaces:** `box_calculator.py`

**Purpose:**
Finds the smallest predefined box that can fit a shipment.

**Boxes:**

```text
UPS-Small:    9 × 6 × 3
UPS-Medium:   12.5 × 10 × 3
Two-Games:    11 × 4.375 × 7.25
Small:        13 × 9 × 4
Short-Medium: 13 × 11 × 6
Medium:       16 × 12 × 6
Tall-Medium:  13 × 13 × 13
Short-Large:  18 × 14 × 8
Tall-Large:   20 × 12 × 12
Long-Large:   24 × 8 × 8
Large-14:     22 × 14 × 14
Large-15:     26.875 × 14.625 × 15.625
```

**Exported variable:**

```go
var Boxes []domain.Box
```

**Function:**

```go
func FindSmallestBox(items []domain.Item) string
```

**Behavior:**
- Use `domain.Item.Volume` from Odoo `product.product.volume`
- Treat item volume as cubic feet per unit and multiply by quantity
- Sum total shipment volume
- Compare against predefined box volumes from smallest to largest
- `domain.Box.Volume` must also be in cubic feet; derive it from the listed inch dimensions (`L*W*H/1728`)
- Return first box name whose volume can hold the shipment total
- If any item is missing usable volume data, return:
  - `Oversize (Missing volume: SKU1,SKU2)`
- If none fit, return `Oversize`

**Note:**
For the Go port, box recommendation should be volume-based rather than dimension-based. Product dimensions may still be loaded separately for UI display, but they are not required for box selection.

**Implementation adjustment note:**
The current `internal/odoo/client.go` implementation already converts product weight from pounds to ounces, but it still needs to request `product.product.volume` and populate `domain.Item.Volume`. The future `internal/boxing/calculator.go` implementation should consume those Odoo-provided volumes directly instead of relying on product dimensions.

---

## 6.12 `internal/led/stub.go`

**Package:** `led`

**Build tag:** `//go:build !linux`

**Replaces:** development/simulation behavior of `led_controller.py`

**Purpose:**
No-op LED controller for macOS and CI.

**Struct:**

```go
type Controller struct {
    ledMap map[string]int
}
```

**Functions:**

```go
func New(cartNumber int, profileName string) *Controller
func (c *Controller) LoadMappings(m map[string]int)
func (c *Controller) HighlightLocations(locations []string, color [3]byte)
func (c *Controller) ClearLEDs()
func (c *Controller) Cleanup()
```

**Behavior:**
- Stores mapping in memory
- All LED methods are no-ops except debug logging
- Must expose the same API as Linux controller

---

## 6.13 `internal/led/controller_linux.go`

**Package:** `led`

**Build tag:** `//go:build linux`

**Replaces:** `led_controller.py`

**Purpose:**
Real WS2812B controller for Raspberry Pi using `libws2811`.

**CGo preamble requirements:**

```c
#cgo CFLAGS: -I/usr/local/include
#cgo LDFLAGS: -L/usr/local/lib -lws2811
#include <stdint.h>
#include <string.h>
#include "ws2811.h"

static ws2811_t ledstring;

static ws2811_return_t led_init(int count, uint8_t brightness) {
    memset(&ledstring, 0, sizeof(ledstring));
    ledstring.freq = 800000;
    ledstring.dmanum = 10;
    ledstring.channel[0].gpionum = 18;
    ledstring.channel[0].invert = 0;
    ledstring.channel[0].count = count;
    ledstring.channel[0].strip_type = WS2811_STRIP_GRB;
    ledstring.channel[0].brightness = brightness;
    return ws2811_init(&ledstring);
}
static void led_set(int index, uint32_t color) {
    if (index >= 0 && index < ledstring.channel[0].count)
        ledstring.channel[0].leds[index] = color;
}
static void led_clear_all(void) {
    for (int i = 0; i < ledstring.channel[0].count; i++)
        ledstring.channel[0].leds[i] = 0;
}
static ws2811_return_t led_render(void) { return ws2811_render(&ledstring); }
static void led_fini(void) { ws2811_fini(&ledstring); }
```

**Struct:**

```go
type Controller struct {
    ledCount int
    ledMap   map[string]int
    ok       bool
}
```

**Functions:**

```go
func New(cartNumber int, profileName string) *Controller
func (c *Controller) LoadMappings(m map[string]int)
func (c *Controller) HighlightLocations(locations []string, color [3]byte)
func (c *Controller) ClearLEDs()
func (c *Controller) Cleanup()
```

**Behavior:**
- `New(...)`
  - Initializes hardware with brightness 128 and LED count 300
  - If init fails, logs warning and sets `ok = false`

- `LoadMappings(...)`
  - Stores location → LED index map

- `HighlightLocations(...)`
  - Clears all LEDs first
  - Packs color as `0x00RRGGBB`
  - For each mapped location, sets pixel color and renders

- `ClearLEDs()`
  - Clears strip and renders

- `Cleanup()`
  - Clears LEDs, renders, and calls `led_fini()`

---

## 6.14 `internal/ui/app.go`

**Package:** `ui`

**Replaces:** `PickingApp` in `app.py`

**Purpose:**
Top-level Fyne application coordinator.

**Struct:**

```go
type App struct {
    fyneApp  fyne.App
    window   fyne.Window

    cfg     *config.AppConfig
    profile domain.CartProfile
    picker  *picker.Picker
    led     *led.Controller

    tabs            *container.AppTabs
    shipmentListTab *ShipmentListTab
    shelfLayoutTab  *ShelfLayoutTab
    shelfListTab    *ShelfListTab
    pickListTab     *PickListTab
    boxesTab        *ShelfLayoutTab
    findOrderTab    *FindOrderTab

    itemsByShipment map[string][]domain.Item
    shipmentsByID   map[string]*domain.Shipment
    selectionEpoch  int64
    batchLoading    bool
    statusLabel     *widget.Label

    productDims     map[string]dimensions.Dimensions
}
```

**Functions:**

```go
func NewApp(cfg *config.AppConfig, profile domain.CartProfile, p *picker.Picker, led *led.Controller) *App
func (a *App) Run()
func (a *App) buildTabs() *container.AppTabs
func (a *App) onSelectionChanged()
func (a *App) onBulkResultsLoaded(epoch int64, results map[string]odoo.BulkResult)
func (a *App) aggregatePickList() []domain.PickItem
func (a *App) locationSortKey(location string) (group int, letter string, mainNum int, subNum int)
func (a *App) onTabChanged(tab *container.TabItem)
```

**Responsibilities:**
- Create Fyne app/window
- Build all six tabs
- Kick off initial batch load
- Track selected shipments
- Trigger bulk item loading for selected shipments
- Aggregate pick items across shipments
- Update all dependent tabs after item loads
- Focus the Find Order tab when selected

**Tabs to build:**
1. Shipments
2. Pick Cart
3. Pick List
4. Next Pick
5. Boxes
6. Find Order

**Important behavior:**
- `onSelectionChanged()` must increment `selectionEpoch`
- Build shelf cells from selected shipments ordered by descending shipment weight
- Populate the cart from the bottom-left, moving left-to-right across each row, then upward row by row
- Trigger background item bulk load
- When results return, compare epoch before applying them

### Pick list aggregation rules
Port the Python `_update_pick_list` behavior:
- Aggregate items across selected shipments by **item name**
- Sum quantities
- Preserve SKU/location/dimension info
- Track which shipments need that item
- Sort by warehouse location

### Location sort behavior
Mimic the Python location parsing behavior:
- Empty/unknown locations sort last
- Parse locations like `A022`, `A022.1`
- Sort by aisle letter, main number, sub-number
- Rows `B, D, F, H, J` sort main number descending
- All others ascending

---

## 6.15 `internal/ui/shipment_list.go`

**Package:** `ui`

**Purpose:**
Displays batches and shipments, and lets the user toggle shipment selection.

**Struct:**

```go
type ShipmentListTab struct {
    widget.BaseWidget
    picker       *picker.Picker
    onSelChanged func()
    tree         *widget.Tree
    refreshBtn   *widget.Button
    statusLabel  *widget.Label
}
```

**Functions:**

```go
func NewShipmentListTab(p *picker.Picker, onSelChanged func()) *ShipmentListTab
func (t *ShipmentListTab) Refresh()
func (t *ShipmentListTab) onBatchTapped(batchID string)
func (t *ShipmentListTab) onShipmentTapped(shipmentID string)
```

**Behavior:**
- Uses `widget.Tree`
- Batch node shows batch name + shipment count
- Shipment node shows customer + service code
- Tapping a batch loads shipments if not already loaded
- Tapping a shipment toggles selection and calls `onSelChanged`

---

## 6.16 `internal/ui/shelf_layout.go`

**Package:** `ui`

**Replaces:** `shelf_layout_tab.py`

**Purpose:**
Displays the physical cart grid and current pick item details.

**Types:**

```go
type ShelfCell struct {
    Location   string
    ExternalID string
    ShipmentID string
    BoxName    string
    HasOrder   bool
}

type ShelfGrid struct {
    widget.BaseWidget
    profile     domain.CartProfile
    cells       []ShelfCell
    highlighted map[string]color.RGBA
    onCellTap   func(location string)
}

type ShelfLayoutTab struct {
    widget.BaseWidget
    profile          domain.CartProfile
    led              *led.Controller
    enableBoxCycling bool
    pickItems        []domain.PickItem
    currentIdx       int
    grid             *ShelfGrid

    quantityLabel   *widget.Label
    nameLabel       *widget.Label
    skuLabel        *widget.Label
    locationLabel   *widget.Label
    dimensionsLabel *widget.Label
    qtyAvailLabel   *widget.Label
    prevBtn         *widget.Button
    nextBtn         *widget.Button

    boxes         []domain.Box
    currentBoxIdx int
    boxNameLabel  *widget.Label
    boxDimsLabel  *widget.Label
    prevBoxBtn    *widget.Button
    nextBoxBtn    *widget.Button
}
```

**Functions:**

```go
func NewShelfLayoutTab(profile domain.CartProfile, led *led.Controller, enableBoxCycling bool) *ShelfLayoutTab
func (t *ShelfLayoutTab) UpdateShipments(cells []ShelfCell)
func (t *ShelfLayoutTab) UpdatePickItems(items []domain.PickItem)
func (t *ShelfLayoutTab) ShowNext()
func (t *ShelfLayoutTab) ShowPrevious()
func (t *ShelfLayoutTab) updateDisplay()
func (t *ShelfLayoutTab) updateLEDs()
```

**Renderer behavior for `ShelfGrid`:**
- One rectangle + one text label per cell
- Fixed row height: 80px
- Rows drawn bottom-to-top so row A is visually bottom
- Cell width is row width divided by number of columns in that row

**Color mapping:**
- Quantity `1` → light green `#90EE90`
- Quantity `2` → orange `#FFA500`
- Quantity `3` → purple `#800080`
- Quantity `>3` → red `#FF0000`
- Default/no order → white

**LED behavior:**
- When current pick item changes, highlight all locations that need that item
- Use the same color as the UI cell highlight

**Box mode:**
- Same grid widget
- Top panel changes to box navigation controls
- Cells can be highlighted by matching box type

---

## 6.17 `internal/ui/shelf_list.go`

**Package:** `ui`

**Replaces:** `shelf_list_tab.py`

**Purpose:**
Sequential pick-list view with large text and per-order shipment list.

**Struct:**

```go
type ShelfListTab struct {
    widget.BaseWidget
    items      []domain.PickItem
    currentIdx int

    quantityText *canvas.Text
    nameText     *canvas.Text
    locationText *canvas.Text
    list         *widget.List
    prevBtn      *widget.Button
    nextBtn      *widget.Button
}
```

**Functions:**

```go
func NewShelfListTab() *ShelfListTab
func (t *ShelfListTab) UpdateItems(items []domain.PickItem)
func (t *ShelfListTab) ShowNext()
func (t *ShelfListTab) ShowPrevious()
func (t *ShelfListTab) updateDisplay()
func (t *ShelfListTab) updateButtonStates()
```

**Behavior:**
- Large quantity text (72pt)
- Large location text (72pt)
- Name text (54pt)
- `widget.List` shows one row per `PickShipment`
- Each row should display:
  - location
  - external order ID
  - quantity
  - total volume for that line

---

## 6.18 `internal/ui/find_order.go`

**Package:** `ui`

**Replaces:** `find_order_tab.py`

**Purpose:**
Barcode-scanner-driven order lookup tab.

**Types:**

```go
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

type FindOrderTab struct {
    widget.BaseWidget
    profile        domain.CartProfile
    led            *led.Controller
    entries        []FindOrderEntry
    details        map[string]FindOrderDetail
    currentIdx     int
    trackingBuf    string
    trackingLoaded bool
    grid           *ShelfGrid

    quantityLabel  *widget.Label
    customerLabel  *widget.Label
    orderLabel     *widget.Label
    locationLabel  *widget.Label
    trackingLabel  *widget.Label
    prevBtn        *widget.Button
    nextBtn        *widget.Button
}
```

**Functions:**

```go
func NewFindOrderTab(profile domain.CartProfile, led *led.Controller) *FindOrderTab
func (t *FindOrderTab) UpdateShipments(entries []FindOrderEntry)
func (t *FindOrderTab) UpdateShipmentDetails(details map[string]FindOrderDetail)
func (t *FindOrderTab) navigateTo(idx int)
func (t *FindOrderTab) processTrackingInput()
func (t *FindOrderTab) findGridLocation(gridIndex int) string
```

**Focusable behavior:**
The widget should capture scanner input through Fyne keyboard/focus hooks.

**Tracking search rules:**
1. Trim buffer
2. If it does not start with `1Z`, remove first 8 characters
3. Search as substring against all known tracking numbers
4. If matched, navigate to that shipment
5. Clear buffer

**Grid location behavior:**
Convert linear shipment position into cart location using the active profile's `RowConfigs`.

---

## 6.19 `cmd/pickcart/main.go`

**Package:** `main`

**Purpose:**
Main application entry point.

**CLI flags:**
- `--cart` (int, default `1`)
- `--profile` (string, default `standard`)
- `--clear-cache` (bool)

**Startup sequence:**
1. Parse CLI flags
2. Load config via `config.Load(...)`
3. Configure logging (`app.log` + stderr)
4. Clear SQLite cache if requested
5. Create cache
6. Create Odoo client
7. Create Odoo provider
8. Create picker
9. Create LED controller
10. Load LED mappings in background
11. Load product dimensions in background
12. Create UI app and run it
13. On shutdown, call `led.Cleanup()`

**Hardcoded Google Sheets URLs:**
- LED mapping CSV:
  - `https://docs.google.com/spreadsheets/d/119MINHxDn9X0aUeO9ETIrusZMJiYrS7Ru7RhMWLA-vM/export?format=csv`
- Product dimensions sheet (optional UI/display data only; box calculations should use Odoo `product.product.volume` instead):
  - `https://docs.google.com/spreadsheets/d/1S1WMWEt0MhdIRl9h3bIVK0Ib9e2g93lTEKMVi796zQM/edit?usp=sharing`

**Important behavior:**
LED mapping load should not block UI startup.

---

## 6.20 `cmd/testleds/main.go`

**Package:** `main`

**Replaces:** `bin_leds.py`

**Purpose:**
Standalone LED diagnostic tool.

**Behavior:**
- CLI flags: `--cart`, `--profile`
- Loads cart profile and LED mappings
- Infinite loop:
  1. All LEDs white for 3 seconds
  2. All off for 1 second
  3. Each location lights green for 1 second, then off, with 0.2 second gap
  4. Repeat
- On Ctrl+C, clean up LEDs

---

## 6.21 `cmd/mapleds/main.go`

**Package:** `main`

**Replaces:** `map_leds.py`

**Purpose:**
Interactive CLI calibration tool for determining physical LED strip index mappings.

**Behavior:**
- CLI flags:
  - `--led-count` default `300`
  - `--start-led` default `0`
- Lights current LED white
- Commands:
  - Enter → next LED
  - `b` → previous LED
  - number → jump to LED index
  - `q` → quit and clear LEDs

---

## 7. Infrastructure Files

## 7.1 `Makefile`

**Purpose:**
Developer/build convenience.

**Targets:**
- `run`
  - `go run ./cmd/pickcart --cart 1 --profile standard`
- `build`
  - `go build -o pickcart ./cmd/pickcart`
- `build-pi`
  - `fyne-cross linux --arch=arm --app-id=io.pickcart.app --name=pickcart ./cmd/pickcart`
- `build-pi64`
  - `fyne-cross linux --arch=arm64 --app-id=io.pickcart.app --name=pickcart ./cmd/pickcart`
- `release-dry`
  - runs both Pi builds
- `lint`
  - `go vet ./...`
- `clean`
  - removes binary and `fyne-cross/`

---

## 7.2 `.github/workflows/release.yml`

**Purpose:**
Builds release artifacts on GitHub tag pushes.

**Behavior:**
- Trigger on tags matching `v*`
- Use `ubuntu-latest`
- Steps:
  1. checkout
  2. setup Go from `go.mod`
  3. install `fyne-cross`
  4. build `linux/arm`
  5. build `linux/arm64`
  6. create GitHub Release
  7. upload `.tar.gz` assets

**Permissions needed:**

```yaml
permissions:
  contents: write
```

---

## 8. Files Not Migrated Directly

| Python file | Reason |
|---|---|
| `shipstation_client.py` / `shipstation_provider.py` | Legacy ShipStation path; current app uses Odoo |
| `hardware_check.py` | Functionality absorbed into Linux LED init + deployment docs |
| `debug_*.py`, `test_*.py`, `simple_batch_test.py` | Dev/debug scripts, not core application |
| `data_interface.py` | Replaced by `internal/domain/types.go` |
| `clear_cache.py` | Replaced by `--clear-cache` flag |

---

## 9. Python → Go Translation Notes

| Python | Go |
|---|---|
| `Tkinter Notebook` | `Fyne AppTabs` |
| `Treeview` | `widget.Tree` |
| `Canvas` rectangle/text drawing | custom `widget.BaseWidget` + `canvas.Rectangle` / `canvas.Text` |
| `threading.Thread` | goroutine |
| `root.after(...)` scheduling | goroutine + explicit UI refresh flow |
| `threading.local()` SQLite pattern | `database/sql` + mutex |
| `try/except ImportError` hardware fallback | build tags + no-op stub controller |
| `selection_epoch` stale-result guard | `selectionEpoch int64` |
| `rpi_ws281x` Python wrapper | `libws2811` via CGo |

---

## 10. Suggested Migration Order

This rewrite should be implemented bottom-up.

### Phase 1 — Foundation
1. `go.mod`
2. `internal/domain/types.go`
3. `internal/config/config.go`
4. `internal/cache/cache.go`

### Phase 2 — Data access
5. `internal/odoo/client.go`
6. `internal/odoo/provider.go`
7. `internal/dimensions/loader.go`

### Phase 3 — Business logic
9. `internal/picker/picker.go`
10. `internal/boxing/calculator.go`
11. `internal/led/stub.go`
12. `internal/led/controller_linux.go`

### Phase 4 — UI
13. `internal/ui/shipment_list.go`
14. `internal/ui/shelf_layout.go`
15. `internal/ui/shelf_list.go`
16. `internal/ui/find_order.go`
17. `internal/ui/app.go`

### Phase 5 — Commands and release plumbing
18. `cmd/pickcart/main.go`
19. `cmd/testleds/main.go`
20. `cmd/mapleds/main.go`
21. `Makefile`
22. `.github/workflows/release.yml`

---

## 11. Final Notes for the Implementing Agent

- Prioritize preserving behavior over inventing new abstractions
- Keep package boundaries clean
- Treat LED control as optional at runtime but available on Pi Linux
- Keep box dimensions loading non-fatal
- Keep the bulk shipment item loader efficient; this is one of the main reasons for the rewrite
- The Raspberry Pi 3b is memory-constrained, so avoid unnecessary duplication of large in-memory structures
- The UI should remain usable even while background loads are in progress

This file is intended to be a working migration spec, not just a high-level idea list.
If implementation choices differ, preserve the behavioral contract described above.

---

## 12. Current Progress Summary

**Overall status:** The Go/Fyne rewrite is now far enough along for real workflow testing, including Pi hardware validation. Core Odoo data flow, cache integration, cart rendering, and LED control are all in place.

### Working now
- Odoo-backed batch, shipment, inventory, tracking, and bulk item loading are implemented.
- BoltDB is the active cache backend for both API responses and SKU location caching.
- LED mappings load from `pick_shelf_light_positions.csv` and are wired into both the stub and Linux controllers.
- The Linux LED controller is driving `libws2811` directly through the project’s small CGo bridge.
- Pick Cart LED behavior now targets cart-bin positions instead of warehouse source locations.
- LED highlight colors now follow the same quantity-based color mapping used by the cart UI.
- Active tabs now in use are:
  - `Shipments`
  - `Pick Cart`
  - `Pick List`
  - `Boxes`
  - `Find Order`
- The old `Next Pick` tab has been removed.
- Find Order search supports tracking number, customer name, and external order ID matching.
- Pick sorting now follows the warehouse serpentine route pattern (A ascending, B descending, etc.).
- Pick Cart, Boxes, and Find Order share a unified top header style.

### Recently improved
- Batch selection no longer auto-expands the branch when selected.
- Batch shipment loading now has a fallback path using `picking_ids` when direct batch queries return zero rows.
- Shipment/cart displays now trim `LNX/OUT/` prefixes down to the meaningful trailing numeric portion.
- Cart-grid bin location and quantity text were made easier to read.
- Cart-grid borders were visually softened.
- Product dimensions and quantity available are now populated from Odoo where available and shown in the Pick Cart header.
- The shelf/cart views have had multiple responsive-layout passes for smaller screens.

### Still in progress / follow-up items
- The cart-display views still need final responsive tuning so the full app scales down cleanly on narrower laptop windows while still looking correct on the target 24" 1080p monitor.
- Header wrapping / bounded vertical overflow in the shared cart header needs final visual validation across all cart-style tabs.
- The Pi LED bridge is intentionally custom and minimal; it is sufficient for current functionality, but could still be revisited later if adopting an upstream Go binding becomes worthwhile.
- Supporting project files like `.env.example`, `Makefile`, and release workflow are still not finalized.
