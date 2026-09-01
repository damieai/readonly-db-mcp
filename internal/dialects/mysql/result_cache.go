package mysql

import (
	"container/list"
	"encoding/json"
	"sync"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

type resultEntry struct {
	key             string
	result          *core.QueryResult
	stored, expires time.Time
	bytes           int
}
type resultCache struct {
	mu                                         sync.Mutex
	enabled                                    bool
	maxEntries, maxBytes, maxEntryBytes, bytes int
	ttl                                        time.Duration
	lru                                        *list.List
	items                                      map[string]*list.Element
}

func newResultCache(enabled bool, ttl time.Duration, entries, bytes, entryBytes int) *resultCache {
	return &resultCache{enabled: enabled, ttl: ttl, maxEntries: entries, maxBytes: bytes, maxEntryBytes: entryBytes, lru: list.New(), items: map[string]*list.Element{}}
}
func (c *resultCache) get(key string) (*core.QueryResult, time.Duration, bool) {
	if c == nil || !c.enabled {
		return nil, 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok {
		return nil, 0, false
	}
	v := e.Value.(*resultEntry)
	if time.Now().After(v.expires) {
		c.remove(e)
		return nil, 0, false
	}
	c.lru.MoveToFront(e)
	return cloneQueryResult(v.result), time.Since(v.stored), true
}
func (c *resultCache) put(key string, r *core.QueryResult) {
	if c == nil || !c.enabled || r.Truncated {
		return
	}
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) > c.maxEntryBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.items[key]; ok {
		c.remove(old)
	}
	now := time.Now()
	e := c.lru.PushFront(&resultEntry{key: key, result: cloneQueryResult(r), stored: now, expires: now.Add(c.ttl), bytes: len(encoded)})
	c.items[key] = e
	c.bytes += len(encoded)
	for len(c.items) > c.maxEntries || c.bytes > c.maxBytes {
		c.remove(c.lru.Back())
	}
}
func (c *resultCache) remove(e *list.Element) {
	if e == nil {
		return
	}
	v := e.Value.(*resultEntry)
	delete(c.items, v.key)
	c.bytes -= v.bytes
	c.lru.Remove(e)
}
func (c *resultCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.Init()
	c.items = map[string]*list.Element{}
	c.bytes = 0
}
func cloneQueryResult(r *core.QueryResult) *core.QueryResult {
	y := *r
	y.Columns = append([]core.Column(nil), r.Columns...)
	y.Rows = make([]map[string]any, len(r.Rows))
	for i, row := range r.Rows {
		y.Rows[i] = make(map[string]any, len(row))
		for k, v := range row {
			y.Rows[i][k] = v
		}
	}
	return &y
}
