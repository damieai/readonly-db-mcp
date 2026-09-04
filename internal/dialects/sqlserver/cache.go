package sqlserver

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

var errRefreshCooldown = errors.New("metadata refresh cooldown is active")

type cacheEntry struct {
	key     string
	value   any
	expires time.Time
	bytes   int
}

type metadataCache struct {
	mu                          sync.Mutex
	enabled                     bool
	maxEntries, maxBytes, bytes int
	lru                         *list.List
	items                       map[string]*list.Element
	inflight                    map[string]chan struct{}
	lastRefresh                 map[string]time.Time
}

func newMetadataCache(enabled bool, entries, bytes int) *metadataCache {
	return &metadataCache{enabled: enabled, maxEntries: entries, maxBytes: bytes, lru: list.New(), items: map[string]*list.Element{}, inflight: map[string]chan struct{}{}, lastRefresh: map[string]time.Time{}}
}

func (c *metadataCache) get(key string) (any, bool) {
	if !c.enabled {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.items[key]
	if !ok {
		return nil, false
	}
	entry := element.Value.(*cacheEntry)
	if time.Now().After(entry.expires) {
		c.remove(element)
		return nil, false
	}
	c.lru.MoveToFront(element)
	return cloneMetadata(entry.value), true
}

func (c *metadataCache) lead(ctx context.Context, key string, refresh bool, cooldown time.Duration) (bool, error) {
	if !c.enabled {
		return true, nil
	}
	c.mu.Lock()
	if ready, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		select {
		case <-ready:
			return false, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	if refresh {
		if last := c.lastRefresh[key]; !last.IsZero() && time.Since(last) < cooldown {
			c.mu.Unlock()
			return false, errRefreshCooldown
		}
		c.lastRefresh[key] = time.Now()
	}
	c.inflight[key] = make(chan struct{})
	c.mu.Unlock()
	return true, nil
}

func (c *metadataCache) finish(key string, value any, ttl time.Duration, store bool) {
	if !c.enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if store {
		encoded, _ := json.Marshal(value)
		if len(encoded) <= c.maxBytes {
			if previous, ok := c.items[key]; ok {
				c.remove(previous)
			}
			element := c.lru.PushFront(&cacheEntry{key: key, value: cloneMetadata(value), expires: time.Now().Add(ttl), bytes: len(encoded)})
			c.items[key] = element
			c.bytes += len(encoded)
			for len(c.items) > c.maxEntries || c.bytes > c.maxBytes {
				c.remove(c.lru.Back())
			}
		}
	}
	if ready, ok := c.inflight[key]; ok {
		delete(c.inflight, key)
		close(ready)
	}
}

func (c *metadataCache) remove(element *list.Element) {
	if element == nil {
		return
	}
	entry := element.Value.(*cacheEntry)
	delete(c.items, entry.key)
	c.bytes -= entry.bytes
	c.lru.Remove(element)
}

func (c *metadataCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.Init()
	c.items = map[string]*list.Element{}
	c.bytes = 0
}

func cloneMetadata(value any) any {
	switch x := value.(type) {
	case []core.TableSummary:
		return append([]core.TableSummary(nil), x...)
	case *core.TableDescription:
		copyValue := *x
		copyValue.Columns = append([]core.ColumnDescription(nil), x.Columns...)
		copyValue.Indexes = make([]core.IndexDescription, len(x.Indexes))
		for i := range x.Indexes {
			copyValue.Indexes[i] = x.Indexes[i]
			copyValue.Indexes[i].Columns = append([]string(nil), x.Indexes[i].Columns...)
			copyValue.Indexes[i].Includes = append([]string(nil), x.Indexes[i].Includes...)
		}
		return &copyValue
	default:
		return value
	}
}
