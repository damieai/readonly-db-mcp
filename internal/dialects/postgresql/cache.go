package postgresql

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
func (c *metadataCache) get(k string) (any, bool) {
	if !c.enabled {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[k]
	if !ok {
		return nil, false
	}
	v := e.Value.(*cacheEntry)
	if time.Now().After(v.expires) {
		c.remove(e)
		return nil, false
	}
	c.lru.MoveToFront(e)
	return cloneMetadata(v.value), true
}
func (c *metadataCache) lead(ctx context.Context, k string, refresh bool, cooldown time.Duration) (bool, error) {
	if !c.enabled {
		return true, nil
	}
	c.mu.Lock()
	if ch, ok := c.inflight[k]; ok {
		c.mu.Unlock()
		select {
		case <-ch:
			return false, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	if refresh {
		if last := c.lastRefresh[k]; !last.IsZero() && time.Since(last) < cooldown {
			c.mu.Unlock()
			return false, errRefreshCooldown
		}
		c.lastRefresh[k] = time.Now()
	}
	c.inflight[k] = make(chan struct{})
	c.mu.Unlock()
	return true, nil
}
func (c *metadataCache) finish(k string, v any, ttl time.Duration, store bool) {
	if !c.enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if store {
		b, _ := json.Marshal(v)
		if len(b) <= c.maxBytes {
			if old, ok := c.items[k]; ok {
				c.remove(old)
			}
			e := c.lru.PushFront(&cacheEntry{key: k, value: cloneMetadata(v), expires: time.Now().Add(ttl), bytes: len(b)})
			c.items[k] = e
			c.bytes += len(b)
			for len(c.items) > c.maxEntries || c.bytes > c.maxBytes {
				c.remove(c.lru.Back())
			}
		}
	}
	if ch, ok := c.inflight[k]; ok {
		delete(c.inflight, k)
		close(ch)
	}
}
func (c *metadataCache) remove(e *list.Element) {
	if e == nil {
		return
	}
	v := e.Value.(*cacheEntry)
	delete(c.items, v.key)
	c.bytes -= v.bytes
	c.lru.Remove(e)
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
func cloneMetadata(v any) any {
	switch x := v.(type) {
	case []core.TableSummary:
		return append([]core.TableSummary(nil), x...)
	case *core.TableDescription:
		y := *x
		y.Columns = append([]core.ColumnDescription(nil), x.Columns...)
		y.Indexes = make([]core.IndexDescription, len(x.Indexes))
		for i := range x.Indexes {
			y.Indexes[i] = x.Indexes[i]
			y.Indexes[i].Columns = append([]string(nil), x.Indexes[i].Columns...)
			y.Indexes[i].Expressions = append([]string(nil), x.Indexes[i].Expressions...)
		}
		return &y
	}
	return v
}
