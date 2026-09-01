package mysql

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

type metadataEntry struct {
	key     string
	value   any
	expires time.Time
	bytes   int
}
type missingDescription struct{}

type metadataCache struct {
	mu                          sync.Mutex
	enabled                     bool
	maxEntries, maxBytes, bytes int
	lru                         *list.List
	items                       map[string]*list.Element
	inflight                    map[string]chan struct{}
	lastRefresh                 map[string]time.Time
}

func newMetadataCache(enabled bool, maxEntries, maxBytes int) *metadataCache {
	return &metadataCache{enabled: enabled, maxEntries: maxEntries, maxBytes: maxBytes, lru: list.New(), items: map[string]*list.Element{}, inflight: map[string]chan struct{}{}, lastRefresh: map[string]time.Time{}}
}

func (c *metadataCache) get(key string) (any, bool) {
	if !c.enabled {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok {
		return nil, false
	}
	v := e.Value.(*metadataEntry)
	if time.Now().After(v.expires) {
		c.remove(e)
		return nil, false
	}
	c.lru.MoveToFront(e)
	return cloneMetadata(v.value), true
}

func (c *metadataCache) lead(ctx context.Context, key string) (bool, error) {
	if !c.enabled {
		return true, nil
	}
	for {
		c.mu.Lock()
		if ch, ok := c.inflight[key]; ok {
			c.mu.Unlock()
			select {
			case <-ch:
				return false, nil
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}
		c.inflight[key] = make(chan struct{})
		c.mu.Unlock()
		return true, nil
	}
}

func (c *metadataCache) leadRefresh(ctx context.Context, key string, cooldown time.Duration) (bool, error) {
	if !c.enabled {
		return true, nil
	}
	c.mu.Lock()
	if ch, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		select {
		case <-ch:
			return false, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	if last := c.lastRefresh[key]; !last.IsZero() && time.Since(last) < cooldown {
		c.mu.Unlock()
		return false, errRefreshCooldown
	}
	c.lastRefresh[key] = time.Now()
	c.inflight[key] = make(chan struct{})
	c.mu.Unlock()
	return true, nil
}

func (c *metadataCache) finish(key string, value any, ttl time.Duration, cache bool) {
	if !c.enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if cache {
		encoded, _ := json.Marshal(value)
		size := len(encoded)
		if size <= c.maxBytes {
			if old, ok := c.items[key]; ok {
				c.remove(old)
			}
			e := c.lru.PushFront(&metadataEntry{key: key, value: cloneMetadata(value), expires: time.Now().Add(ttl), bytes: size})
			c.items[key] = e
			c.bytes += size
			for len(c.items) > c.maxEntries || c.bytes > c.maxBytes {
				c.remove(c.lru.Back())
			}
		}
	}
	if ch, ok := c.inflight[key]; ok {
		delete(c.inflight, key)
		close(ch)
	}
}

func (c *metadataCache) remove(e *list.Element) {
	if e == nil {
		return
	}
	v := e.Value.(*metadataEntry)
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
	c.lastRefresh = map[string]time.Time{}
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
		}
		return &y
	default:
		return v
	}
}
