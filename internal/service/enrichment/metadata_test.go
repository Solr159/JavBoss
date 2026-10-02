package enrichment

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/jav"

	"gorm.io/gorm"
)

type metadataLookupCache struct {
	mu        sync.Mutex
	beforeGet func(string)
	values    map[string]jav.JavInfo
	keys      []string
}

func (c *metadataLookupCache) Get(key string, _ time.Time) ([]byte, bool, error) {
	if c.beforeGet != nil {
		c.beforeGet(key)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = append(c.keys, key)
	info, ok := c.values[key]
	if !ok {
		return []byte(`{"status":"not_found"}`), true, nil
	}
	raw, err := json.Marshal(struct {
		Status string      `json:"status"`
		Data   jav.JavInfo `json:"data"`
	}{
		Status: "hit",
		Data:   info,
	})
	return raw, true, err
}

func (c *metadataLookupCache) Set(string, []byte, time.Time) error {
	return nil
}

func openEnrichmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := db.Open(filepath.Join(t.TempDir(), "studios.db"))
	if err != nil {
		t.Fatal(err)
	}
	previous := common.DB
	common.DB = gdb
	t.Cleanup(func() {
		common.DB = previous
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return gdb
}
