package elasticsearch

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"golang.org/x/sync/semaphore"
)

const cursorCleanupTimeout = 5 * time.Second
const cursorMemoryLimit = 64 << 20

var cursorSlots = semaphore.NewWeighted(128)
var cursorMemory = semaphore.NewWeighted(cursorMemoryLimit)

type ownedCursor struct {
	gate                                                chan struct{}
	handle, owner, kind                                 string
	ownerContext                                        context.Context
	nativeLease                                         time.Duration
	endpoint                                            int
	epoch                                               uint64
	created, expires, absolute, serverUntil, retryAfter time.Time
	ids                                                 []string // newest, then previous; native rotated IDs refer to the same context
	recoveryQueued                                      bool
	uncertain, invalid, released                        bool
	bytes                                               int64
	releaseOnce                                         sync.Once
	retireTimer                                         *time.Timer
	query                                               core.ElasticsearchQueryRequest
	physical                                            map[string]bool
	mappings                                            map[string]any
	embedded                                            map[string]map[string]bool
	after                                               json.RawMessage
	lastHits                                            int
}

func (c *ownedCursor) lock(ctx context.Context) error {
	select {
	case c.gate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return failure("deadline_exceeded", "cursor is busy or request deadline expired")
	}
}
func (c *ownedCursor) unlock() { <-c.gate }

func (t *Target) reserveCursor(owner string, ownerCtx context.Context, kind string, p *queryProof, q *preparedQuery) (*ownedCursor, error) {
	// Proof objects were allocated under the active request lease. Reserve their
	// retained representation, query copies and two maximum-sized IDs BEFORE any
	// server context creation; this is separate from transient response memory.
	charge := int64(6*p.proofBytes + 6*len(q.body) + 3*t.cfg.Elasticsearch.MaxContextIDBytes + 16384)
	if !cursorMemory.TryAcquire(charge) {
		return nil, failure("resource_limit", "retained Elasticsearch context memory is exhausted")
	}
	if !cursorSlots.TryAcquire(1) {
		cursorMemory.Release(charge)
		return nil, failure("resource_limit", "process context limit reached")
	}
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		cursorMemory.Release(charge)
		cursorSlots.Release(1)
		return nil, failure("resource_limit", "cannot allocate cursor handle")
	}
	now := time.Now()
	c := &ownedCursor{gate: make(chan struct{}, 1), handle: base64.RawURLEncoding.EncodeToString(entropy[:]), owner: owner, ownerContext: ownerCtx, kind: kind, endpoint: p.endpoint, epoch: t.authorityEpoch.Load(), created: now, absolute: now.Add(t.cfg.Elasticsearch.MaxContextLifetime), bytes: charge, physical: q.physical, mappings: p.mappings[scopeKey(q.request.Indices)], embedded: p.embedded, query: q.request}
	if ownerCtx == nil {
		c.ownerContext = context.Background()
	}
	c.gate <- struct{}{} // caller owns the unpublished entry through open/first page
	t.cursorMu.Lock()
	if t.closed.Load() || len(t.cursors) >= t.cfg.Elasticsearch.MaxOpenContexts {
		t.cursorMu.Unlock()
		cursorMemory.Release(charge)
		cursorSlots.Release(1)
		return nil, failure("resource_limit", "target context limit reached or target closed")
	}
	t.cursors[c.handle] = c
	t.cursorMu.Unlock()
	return c, nil
}

func (t *Target) releaseCursor(c *ownedCursor) {
	c.releaseOnce.Do(func() {
		c.released = true
		if c.retireTimer != nil {
			c.retireTimer.Stop()
		}
		t.cursorMu.Lock()
		delete(t.cursors, c.handle)
		t.cursorMu.Unlock()
		cursorMemory.Release(c.bytes)
		cursorSlots.Release(1)
	})
}

func (t *Target) lookupCursor(owner, handle string) (*ownedCursor, error) {
	if len(handle) != 43 {
		return nil, failure("cursor_expired", "cursor is unknown, expired or belongs to another session")
	}
	t.cursorMu.Lock()
	c := t.cursors[handle]
	t.cursorMu.Unlock()
	if c == nil || c.owner != owner {
		return nil, failure("cursor_expired", "cursor is unknown, expired or belongs to another session")
	}
	return c, nil
}

func (t *Target) usableCursor(c *ownedCursor) error {
	if c.released || c.invalid || c.ownerContext.Err() != nil || time.Now().After(c.expires) || time.Now().After(c.absolute) || c.epoch != t.authorityEpoch.Load() {
		return failure("cursor_expired", "cursor lease, session or authority revision is no longer valid")
	}
	return t.ready()
}

// The most recent ID is sufficient for native continuation. Keep its immediate
// predecessor until cleanup too, covering a partially observed rotation without
// retaining an unbounded history of large native tokens.
func (t *Target) captureCursorID(c *ownedCursor, id string) error {
	if id == "" || id == "_all" || len(id) > t.cfg.Elasticsearch.MaxContextIDBytes {
		return failure("invalid_response", "invalid or oversized native context identifier")
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '+' || r == '/' || r == '=' || r == '_' || r == '-') {
			return failure("invalid_response", "invalid native context identifier")
		}
	}
	if len(c.ids) == 0 || c.ids[0] != id {
		if len(c.ids) == 0 {
			c.ids = []string{id}
		} else {
			c.ids = []string{id, c.ids[0]}
		}
	}
	c.uncertain = false
	return nil
}

func (t *Target) cleanupCursor(ctx context.Context, c *ownedCursor) error {
	c.invalid = true
	if c.released {
		return nil
	}
	if !c.serverUntil.IsZero() && time.Now().After(c.serverUntil) {
		t.releaseCursor(c)
		return nil
	}
	if c.serverUntil.IsZero() {
		t.releaseCursor(c)
		return nil
	} // no creation was dispatched
	var cleanupErr error
	for i, id := range c.ids {
		path := "/_pit"
		body := map[string]any{"id": id}
		if c.kind == "scroll" {
			path = "/_search/scroll"
			body = map[string]any{"scroll_id": []string{id}}
		}
		raw, _ := json.Marshal(body)
		data, err := t.wire.request(ctx, c.endpoint, http.MethodDelete, path, nil, raw, "application/json", min(t.limits.MaxResultBytes, 64<<10), true)
		if err != nil {
			cleanupErr = err
			continue
		}
		var result struct {
			Succeeded bool `json:"succeeded"`
			NumFreed  *int `json:"num_freed"`
		}
		if json.Unmarshal(data, &result) != nil || !result.Succeeded || result.NumFreed == nil || *result.NumFreed < 0 {
			cleanupErr = failure("cleanup_failed", "Elasticsearch did not confirm context cleanup")
		}
		if i == 0 && cleanupErr == nil && !c.uncertain {
			t.releaseCursor(c)
			return nil
		}
	}
	if cleanupErr == nil && !c.uncertain && len(c.ids) > 0 {
		t.releaseCursor(c)
		return nil
	}
	// Unknown IDs and failed clears retain their slot AND memory until the
	// conservative native lease horizon. A timeout must not allow infinite orphans.
	c.retryAfter = time.Now().Add(cursorCleanupTimeout)
	t.retireCursorAtLeaseEnd(c)
	if t.metrics != nil {
		t.metrics.Add("elasticsearch_context_cleanup", 1, c.kind, "pending")
	}
	return failure("cleanup_pending", "context disabled; native cleanup or lease expiry is pending")
}

func (t *Target) retireCursorAtLeaseEnd(c *ownedCursor) {
	if c.serverUntil.IsZero() || time.Now().After(c.serverUntil) {
		t.releaseCursor(c)
		return
	}
	if c.retireTimer == nil {
		c.retireTimer = time.AfterFunc(max(time.Until(c.serverUntil), time.Millisecond), func() {
			_ = c.lock(context.Background())
			defer c.unlock()
			c.retireTimer = nil
			if !c.released {
				// A successful page may have renewed the native lease while this
				// timer was waiting for the entry gate. Recheck before releasing.
				t.retireCursorAtLeaseEnd(c)
			}
		})
	}
}

// Normal cleanup consumes the parent deadline. Expired requests only disable
// the handle and enqueue bounded maintenance; they never wait another five
// seconds while retaining their interactive/batch permit.
func (t *Target) recoverCursor(parent context.Context, c *ownedCursor) {
	c.invalid = true
	if c.released {
		return
	}
	if parent.Err() == nil {
		ctx, cancel := context.WithTimeout(parent, cursorCleanupTimeout)
		err := t.cleanupCursor(ctx, c)
		cancel()
		if err == nil {
			return
		}
	}
	t.retireCursorAtLeaseEnd(c)
	if c.released || c.recoveryQueued {
		return
	}
	c.recoveryQueued = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), cursorCleanupTimeout)
		defer cancel()
		permit, err := t.admission.Acquire(ctx, t.cfg.Name, admission.Maintenance)
		if err != nil {
			return
		}
		defer permit.Release()
		if !responseMemory.TryAcquire(6 * (64 << 10)) {
			return
		}
		defer responseMemory.Release(6 * (64 << 10))
		if err := c.lock(ctx); err != nil {
			return
		}
		defer c.unlock()
		c.recoveryQueued = false
		_ = t.cleanupCursor(ctx, c)
	}()
}

func (t *Target) cursorInventory() []*ownedCursor {
	t.cursorMu.Lock()
	defer t.cursorMu.Unlock()
	result := make([]*ownedCursor, 0, len(t.cursors))
	for _, c := range t.cursors {
		result = append(result, c)
	}
	return result
}

func (t *Target) maintainCursors() {
	defer t.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(t.ctx, cursorCleanupTimeout)
			t.reapCursors(ctx)
			cancel()
		}
	}
}

func (t *Target) reapCursors(ctx context.Context) {
	for _, c := range t.cursorInventory() {
		if ctx.Err() != nil {
			return
		}
		select {
		case c.gate <- struct{}{}:
		default:
			continue
		}
		if t.usableCursor(c) == nil || time.Now().Before(c.retryAfter) {
			c.unlock()
			continue
		}
		// Sweep work uses maintenance admission; never retains execution permits
		// between pages and never holds the registry mutex while waiting.
		permit, err := t.admission.Acquire(ctx, t.cfg.Name, admission.Maintenance)
		if err == nil {
			if responseMemory.TryAcquire(6 * (64 << 10)) {
				_ = t.cleanupCursor(ctx, c)
				responseMemory.Release(6 * (64 << 10))
			}
			permit.Release()
		}
		c.unlock()
	}
}

func (t *Target) closeOwnedCursors(ctx context.Context, owner string) error {
	if !responseMemory.TryAcquire(6 * (64 << 10)) {
		return failure("resource_limit", "cleanup response memory is exhausted")
	}
	defer responseMemory.Release(6 * (64 << 10))
	var last error
	for _, c := range t.cursorInventory() {
		if owner != "" && c.owner != owner {
			continue
		}
		if err := c.lock(ctx); err != nil {
			last = err
			continue
		}
		if err := t.cleanupCursor(ctx, c); err != nil {
			last = err
		}
		c.unlock()
	}
	return last
}

func (t *Target) CloseElasticsearchSession(ctx context.Context, owner string) error {
	if owner == "" {
		return failure("invalid_request", "session identity is required")
	}
	ctx, cancel := context.WithTimeout(ctx, cursorCleanupTimeout)
	defer cancel()
	permit, err := t.admission.Acquire(ctx, t.cfg.Name, admission.Maintenance)
	if err != nil {
		return err
	}
	defer permit.Release()
	return t.closeOwnedCursors(ctx, owner)
}
