package cache

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	bbolt "go.etcd.io/bbolt"
)

type Cache struct {
	db   *bbolt.DB
	mu   sync.Mutex
	path string
}

const bucketName = "api_cache"

func New(dbPath string) (*Cache, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("db path is required")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open cache database: %w", err)
	}

	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create cache bucket: %w", err)
	}

	return &Cache{db: db, path: dbPath}, nil
}

func (c *Cache) Get(endpoint string, params any, ttlHours int) (any, bool) {
	if c == nil || c.db == nil {
		return nil, false
	}

	cacheKey, err := makeCacheKey(endpoint, params)
	if err != nil {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var raw []byte
	if err := c.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return nil
		}

		value := bucket.Get([]byte(cacheKey))
		if value == nil {
			return nil
		}

		raw = append([]byte(nil), value...)
		return nil
	}); err != nil || raw == nil {
		return nil, false
	}

	timestamp, payload, err := decodeEntry(raw)
	if err != nil {
		return nil, false
	}

	if ttlHours > 0 {
		expiresAt := time.Unix(timestamp, 0).Add(time.Duration(ttlHours) * time.Hour)
		if time.Now().After(expiresAt) {
			_ = c.deleteLocked(cacheKey)
			return nil, false
		}
	}

	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, false
	}

	return value, true
}

func (c *Cache) Put(endpoint string, params any, data any) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("cache is not initialized")
	}

	cacheKey, err := makeCacheKey(endpoint, params)
	if err != nil {
		return fmt.Errorf("build cache key: %w", err)
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal cache payload: %w", err)
	}

	encoded := encodeEntry(time.Now().Unix(), payload)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return fmt.Errorf("cache bucket %q not found", bucketName)
		}
		return bucket.Put([]byte(cacheKey), encoded)
	}); err != nil {
		return fmt.Errorf("store cache entry: %w", err)
	}

	return nil
}

func (c *Cache) Clear(olderThanHours int) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("cache is not initialized")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if olderThanHours == 0 {
		if err := c.db.Update(func(tx *bbolt.Tx) error {
			if err := tx.DeleteBucket([]byte(bucketName)); err != nil && err != bbolt.ErrBucketNotFound {
				return err
			}
			_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
			return err
		}); err != nil {
			return fmt.Errorf("clear cache: %w", err)
		}
		return nil
	}

	cutoff := time.Now().Add(-time.Duration(olderThanHours) * time.Hour).Unix()
	if err := c.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return nil
		}

		var keysToDelete [][]byte
		if err := bucket.ForEach(func(key, value []byte) error {
			timestamp, _, err := decodeEntry(value)
			if err != nil || timestamp < cutoff {
				keysToDelete = append(keysToDelete, append([]byte(nil), key...))
			}
			return nil
		}); err != nil {
			return err
		}

		for _, key := range keysToDelete {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("clear old cache entries: %w", err)
	}

	return nil
}

func (c *Cache) Close() error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return nil
	}

	err := c.db.Close()
	c.db = nil
	return err
}

func (c *Cache) deleteLocked(cacheKey string) error {
	return c.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte(cacheKey))
	})
}

func encodeEntry(timestamp int64, payload []byte) []byte {
	buf := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint64(buf[:8], uint64(timestamp))
	copy(buf[8:], payload)
	return buf
}

func decodeEntry(value []byte) (int64, []byte, error) {
	if len(value) < 8 {
		return 0, nil, fmt.Errorf("invalid cache entry")
	}

	timestamp := int64(binary.BigEndian.Uint64(value[:8]))
	payload := append([]byte(nil), value[8:]...)
	return timestamp, payload, nil
}

func makeCacheKey(endpoint string, params any) (string, error) {
	stable, err := stableJSON(params)
	if err != nil {
		return "", err
	}

	sum := md5.Sum([]byte(endpoint + string(stable)))
	return fmt.Sprintf("%x", sum), nil
}

func stableJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := writeStableJSON(&buf, normalized); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func writeStableJSON(buf *bytes.Buffer, value any) error {
	switch v := value.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case bool, string, float64:
		encoded, err := json.Marshal(v)
		if err != nil {
			return err
		}
		buf.Write(encoded)
		return nil
	case []any:
		buf.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeStableJSON(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		buf.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			encodedKey, err := json.Marshal(key)
			if err != nil {
				return err
			}
			buf.Write(encodedKey)
			buf.WriteByte(':')
			if err := writeStableJSON(buf, v[key]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	default:
		return fmt.Errorf("unsupported stable json type %T", value)
	}
}
