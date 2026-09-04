package cache

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	bbolt "go.etcd.io/bbolt"
)

func backdate(t *testing.T, c *Cache, endpoint string, params any, age time.Duration) {
	t.Helper()
	key, err := makeCacheKey(endpoint, params)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal("payload for " + endpoint)
	if err != nil {
		t.Fatal(err)
	}
	entry := encodeEntry(time.Now().Add(-age).Unix(), payload)
	if err := c.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).Put([]byte(key), entry)
	}); err != nil {
		t.Fatal(err)
	}
}

func has(t *testing.T, c *Cache, endpoint string, params any) bool {
	t.Helper()
	_, _, ok, err := c.GetWithTimestamp(endpoint, params)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestClearKeepsEntriesInsideRetentionWindow(t *testing.T) {
	c, err := New(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Mirrors the real entries: a fresh response, a day-old one still inside the
	// retention window, and one past it.
	if err := c.Put("/json/2/product.product/read", map[string]any{"ids": []int{1}}, "fresh"); err != nil {
		t.Fatal(err)
	}
	backdate(t, c, "/json/2/stock.move/search_read", map[string]any{"ids": []int{2}}, 25*time.Hour)
	backdate(t, c, "/json/2/product.product/read", map[string]any{"ids": []int{3}}, 8*24*time.Hour)
	backdate(t, c, "odoo.inventory.lenexa_qty_by_sku", nil, 30*time.Hour)

	const retentionHours = 24 * 7
	if err := c.Clear(retentionHours); err != nil {
		t.Fatal(err)
	}

	if !has(t, c, "/json/2/product.product/read", map[string]any{"ids": []int{1}}) {
		t.Error("fresh entry was purged")
	}
	if !has(t, c, "/json/2/stock.move/search_read", map[string]any{"ids": []int{2}}) {
		t.Error("25h entry inside retention window was purged")
	}
	if has(t, c, "/json/2/product.product/read", map[string]any{"ids": []int{3}}) {
		t.Error("8 day old entry survived the purge")
	}
	// The stale inventory fallback must still be readable after a purge.
	if !has(t, c, "odoo.inventory.lenexa_qty_by_sku", nil) {
		t.Error("stale inventory fallback was purged")
	}
}

func TestClearZeroWipesEverything(t *testing.T) {
	c, err := New(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.Put("/json/2/product.product/read", nil, "fresh"); err != nil {
		t.Fatal(err)
	}
	if err := c.Clear(0); err != nil {
		t.Fatal(err)
	}
	if has(t, c, "/json/2/product.product/read", nil) {
		t.Error("Clear(0) should wipe the cache")
	}
}
