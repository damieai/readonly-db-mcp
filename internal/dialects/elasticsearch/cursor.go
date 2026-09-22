package elasticsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func scopeKey(indices []string) string { return strings.Join(indices, ",") }

func (t *Target) cursorKeepAlive(requested time.Duration) (time.Duration, error) {
	if requested == 0 {
		requested = t.cfg.Elasticsearch.ContextKeepAlive
	}
	if requested < time.Millisecond || requested > 15*time.Minute {
		return 0, failure("resource_limit", "context keep_alive must be positive and at most 15 minutes")
	}
	return requested, nil
}

func (t *Target) ElasticsearchCursor(ctx context.Context, r core.ElasticsearchCursorRequest) (result *core.ElasticsearchCursorResult, err error) {
	started, id := time.Now(), uuid.NewString()
	raw, encodeErr := json.Marshal(r)
	defer func() { t.recordQuery(ctx, id, "es_cursor", raw, started, err) }()
	if encodeErr != nil {
		return nil, failure("invalid_request", "invalid cursor envelope")
	}
	if r.Owner == "" || len(r.Owner) > 256 {
		return nil, failure("invalid_request", "transport session identity is required")
	}
	if r.Action != "open" && r.Action != "next" && r.Action != "close" {
		return nil, failure("invalid_request", "cursor action must be open, next or close")
	}
	if r.Action == "open" {
		if r.Handle != "" || (r.Kind != "pit" && r.Kind != "scroll") {
			return nil, failure("invalid_request", "open requires pit/scroll kind and no handle")
		}
	} else if r.Handle == "" || r.Kind != "" || len(r.Indices) != 0 {
		return nil, failure("invalid_request", "next/close requires a handle and cannot replace kind or index scope")
	}
	if r.Action == "close" && (len(r.Body) != 0 || len(r.Options) != 0 || r.MaxRows != 0 || r.KeepAlive != 0) {
		return nil, failure("invalid_request", "close accepts only a handle and request timeout")
	}
	if r.Action == "next" && len(r.Options) != 0 {
		return nil, failure("invalid_request", "cursor options are fixed by open")
	}
	keep, err := t.cursorKeepAlive(r.KeepAlive)
	if err != nil {
		return nil, err
	}
	if r.OwnerContext != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
		unlink := context.AfterFunc(r.OwnerContext, cancel)
		defer unlink()
	}
	ctx, release, err := t.requestLease(ctx, r.Timeout, raw, admission.Interactive, r.Action != "close")
	if err != nil {
		return nil, err
	}
	defer release()
	if r.Action == "open" {
		return t.openCursor(ctx, r, keep, id, started)
	}
	c, err := t.lookupCursor(r.Owner, r.Handle)
	if err != nil {
		return nil, err
	}
	if err := c.lock(ctx); err != nil {
		return nil, err
	}
	defer c.unlock()
	if r.Action == "close" {
		if err := t.cleanupCursor(ctx, c); err != nil {
			return nil, err
		}
		return &core.ElasticsearchCursorResult{ElasticsearchResult: core.ElasticsearchResult{QueryID: id, Target: t.cfg.Name, Engine: "elasticsearch", Operation: "cursor_close", Consistency: c.kind, Data: json.RawMessage(`{"succeeded":true}`), DurationMS: time.Since(started).Milliseconds()}, Kind: c.kind, Closed: true}, nil
	}
	if err := t.usableCursor(c); err != nil {
		t.recoverCursor(ctx, c)
		return nil, err
	}
	ctx, cancel := context.WithDeadline(ctx, c.absolute)
	defer cancel()
	if c.kind == "scroll" && (len(r.Body) != 0 || r.MaxRows != 0) {
		return nil, failure("invalid_request", "scroll pages use the query and page size fixed by open")
	}
	p := newQueryProof(t, ctx, c.endpoint)
	p.snapshot = c
	request := c.query
	if len(r.Body) != 0 {
		request.Body = r.Body
	} else if c.kind == "pit" {
		body, err := decodeObject(request.Body)
		if err != nil {
			return nil, err
		}
		if len(c.after) != 0 {
			var after any
			d := json.NewDecoder(strings.NewReader(string(c.after)))
			d.UseNumber()
			if d.Decode(&after) != nil {
				return nil, failure("invalid_response", "invalid retained sort values")
			}
			body["search_after"] = after
		} else if c.lastHits > 0 {
			return nil, failure("invalid_request", "native page lacks sort values; provide a PIT search body with explicit continuation")
		}
		request.Body, _ = json.Marshal(body)
	}
	if r.MaxRows != 0 {
		request.MaxRows = r.MaxRows
	}
	// Continuation may re-materialize a large frozen query even when the public
	// envelope is only a handle. Include it in transient admission and enforce
	// the combined scope/options/body size after applying caller overrides.
	materialized, encodeErr := json.Marshal(request)
	if encodeErr != nil || len(materialized) > t.cfg.Elasticsearch.MaxRequestBytes {
		return nil, failure("resource_limit", "continued cursor query exceeds request byte limit")
	}
	extra := 6 * int64(max(0, len(materialized)-len(raw)))
	if !responseMemory.TryAcquire(extra) {
		return nil, failure("resource_limit", "cursor materialization memory is exhausted")
	}
	defer responseMemory.Release(extra)
	var q *preparedQuery
	if c.kind == "pit" {
		q, err = p.prepare(request)
		if err != nil {
			return nil, err
		}
	} else {
		if err := p.stockQueryProfile(); err != nil {
			t.recoverCursor(ctx, c)
			return nil, err
		}
		q = &preparedQuery{request: c.query, body: c.query.Body, physical: c.physical, rows: c.query.MaxRows, buckets: t.cfg.Elasticsearch.MaxAggregationBuckets, options: url.Values{}}
		// The scroll query is frozen, including embedded references. Prove those
		// references afresh, while leaving the original main snapshot untouched.
		for expression := range c.embedded {
			if _, err := p.sources(strings.Split(expression, ",")); err != nil {
				t.recoverCursor(ctx, c)
				return nil, err
			}
		}
	}
	ok := false
	defer func() {
		if !ok {
			t.recoverCursor(ctx, c)
		}
	}()
	data, err := t.cursorPage(ctx, c, q, keep, false)
	if err != nil {
		return nil, err
	}
	if err := p.verifySources(); err != nil {
		return nil, err
	}
	if err := t.usableCursor(c); err != nil {
		return nil, err
	}
	if c.kind == "pit" {
		// Account for changed queries before retaining them between pages.
		if err := t.updateCursorQuery(c, p, q); err != nil {
			return nil, err
		}
	}
	result, err = t.cursorResult(ctx, c, q, data, id, started)
	if err != nil {
		return nil, err
	}
	ok = true
	return result, nil
}

func (t *Target) openCursor(ctx context.Context, r core.ElasticsearchCursorRequest, keep time.Duration, id string, started time.Time) (result *core.ElasticsearchCursorResult, err error) {
	p := newQueryProof(t, ctx, int(t.next.Add(1)-1)%len(t.wire.clients))
	q, err := p.prepare(core.ElasticsearchQueryRequest{Operation: "search", Indices: r.Indices, Body: r.Body, Options: r.Options, MaxRows: r.MaxRows})
	if err != nil {
		return nil, err
	}
	c, err := t.reserveCursor(r.Owner, r.OwnerContext, r.Kind, p, q)
	if err != nil {
		return nil, err
	}
	defer c.unlock()
	ctx, cancel := context.WithDeadline(ctx, c.absolute)
	defer cancel()
	ok := false
	defer func() {
		if !ok {
			t.recoverCursor(ctx, c)
		}
	}()
	if err := t.updateCursorQuery(c, p, q); err != nil {
		return nil, err
	}
	if c.kind == "pit" {
		if err := t.openPIT(ctx, c, q, keep); err != nil {
			return nil, err
		}
	}
	data, err := t.cursorPage(ctx, c, q, keep, true)
	if err != nil {
		return nil, err
	}
	if err := p.verifySources(); err != nil {
		return nil, err
	}
	if err := t.usableCursor(c); err != nil {
		return nil, err
	}
	result, err = t.cursorResult(ctx, c, q, data, id, started)
	if err != nil {
		return nil, err
	}
	ok = true
	return result, nil
}

func nativeBaseBody(q *preparedQuery) json.RawMessage {
	body, _ := decodeObject(q.body)
	original, _ := decodeObject(q.request.Body)
	if value, ok := original["timeout"]; ok {
		body["timeout"] = value
	} else if value, ok := q.request.Options["timeout"]; ok {
		var timeout string
		if json.Unmarshal(value, &timeout) == nil {
			body["timeout"] = timeout
		}
	} else {
		delete(body, "timeout")
	}
	raw, _ := json.Marshal(body)
	return raw
}

func (t *Target) updateCursorQuery(c *ownedCursor, p *queryProof, q *preparedQuery) error {
	body := nativeBaseBody(q)
	// Original snapshot mappings stay retained. Per-page DSL may add new live
	// lookups; charge a conservative representation before replacing its state.
	options := map[string]json.RawMessage{}
	for k, v := range q.request.Options {
		if k == "size" || k == "from" || k == "timeout" {
			continue
		}
		options[k] = append(json.RawMessage(nil), v...)
	}
	retained := core.ElasticsearchQueryRequest{Operation: "search", Indices: append([]string(nil), q.request.Indices...), Body: body, Options: options, MaxRows: q.rows}
	queryRaw, _ := json.Marshal(retained)
	if len(queryRaw) > t.cfg.Elasticsearch.MaxRequestBytes {
		return failure("resource_limit", "retained cursor query exceeds request byte limit")
	}
	charge := int64(6*len(queryRaw) + 3*t.cfg.Elasticsearch.MaxContextIDBytes + 16384)
	mappingRaw, _ := json.Marshal(c.mappings)
	charge += int64(6*len(mappingRaw)) + retainedMappingMemory(c.mappings)
	for expression, physical := range p.embedded {
		charge += int64(6 * (len(expression) + 128))
		for name := range physical {
			charge += int64(6 * (len(name) + 128))
		}
	}
	for name := range c.physical {
		charge += int64(6 * (len(name) + 128))
	}
	if charge > c.bytes {
		if !cursorMemory.TryAcquire(charge - c.bytes) {
			return failure("resource_limit", "retained cursor query exceeds context memory budget")
		}
	} else {
		cursorMemory.Release(c.bytes - charge)
	}
	c.bytes = charge
	c.query = retained
	c.embedded = p.embedded
	return nil
}

// Maps/interfaces remain decoded between pages; byte length alone misses the
// overhead of many short fields and empty objects. Charge actual retained nodes.
func retainedMappingMemory(value any) int64 {
	charge := int64(256)
	switch v := value.(type) {
	case map[string]any:
		for _, child := range v {
			charge += retainedMappingMemory(child)
		}
	case []any:
		for _, child := range v {
			charge += retainedMappingMemory(child)
		}
	}
	return charge
}

// A native lease starts when ES processes the request, not when admission
// began. Unknown outcomes retain capacity through an additional max request
// interval; known outcomes use the observed response time as a safe upper bound.
func (t *Target) beginCursorDispatch(c *ownedCursor, keep time.Duration) (string, error) {
	if err := t.ready(); err != nil {
		return "", err
	}
	if c.epoch != t.authorityEpoch.Load() || c.ownerContext.Err() != nil || c.invalid {
		return "", failure("cursor_expired", "cursor authority or session changed")
	}
	remaining := time.Until(c.absolute)
	if remaining < time.Millisecond {
		return "", failure("cursor_expired", "absolute context lifetime expired")
	}
	keep = min(keep, remaining)
	c.expires = time.Now().Add(keep)
	c.nativeLease = max(c.nativeLease, keep)
	c.serverUntil = time.Now().Add(c.nativeLease + t.limits.MaxTimeout)
	c.uncertain = true
	// Even a shutdown without available cleanup memory must eventually release
	// local reservations. Renewal is rechecked under the entry gate by the timer.
	t.retireCursorAtLeaseEnd(c)
	return strconv.FormatInt(max(1, keep.Milliseconds()), 10) + "ms", nil
}

func (t *Target) openPIT(ctx context.Context, c *ownedCursor, q *preparedQuery, keep time.Duration) error {
	lease, err := t.beginCursorDispatch(c, keep)
	if err != nil {
		return err
	}
	options := url.Values{"keep_alive": {lease}, "allow_partial_search_results": {"false"}}
	for _, key := range []string{"routing", "preference", "expand_wildcards", "ignore_unavailable", "max_concurrent_shard_requests"} {
		if v := q.options.Get(key); v != "" {
			options.Set(key, v)
		}
	}
	raw, err := t.wire.request(ctx, c.endpoint, http.MethodPost, "/"+url.PathEscape(scopeKey(q.request.Indices))+"/_pit", options, nil, "application/json", t.limits.MaxResultBytes, false)
	if err != nil {
		return err
	}
	var response struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return failure("invalid_response", "invalid PIT creation response")
	}
	if err := t.captureCursorID(c, response.ID); err != nil {
		return err
	}
	root, err := decodeObject(raw)
	if err != nil {
		return err
	}
	if _, ok := root["_shards"]; !ok {
		return failure("invalid_response", "PIT shard outcome is missing")
	}
	if err := completeResponse(root); err != nil {
		return err
	}
	duration, _ := time.ParseDuration(lease)
	c.serverUntil = time.Now().Add(c.nativeLease)
	c.expires = minTime(time.Now().Add(duration), c.absolute)
	return nil
}

func (t *Target) cursorPage(ctx context.Context, c *ownedCursor, q *preparedQuery, keep time.Duration, first bool) ([]byte, error) {
	lease, err := t.beginCursorDispatch(c, keep)
	if err != nil {
		return nil, err
	}
	options := url.Values{}
	for key, value := range q.options {
		options[key] = append([]string(nil), value...)
	}
	path := q.path
	var body []byte
	if c.kind == "pit" {
		if len(c.ids) == 0 {
			return nil, failure("invalid_response", "owned PIT identifier is missing")
		}
		if err := searchTimeout(q, time.Until(deadline(ctx)).Milliseconds()); err != nil {
			return nil, err
		}
		native, _ := decodeObject(q.body)
		native["pit"] = map[string]any{"id": c.ids[0], "keep_alive": lease}
		body, _ = json.Marshal(native)
		path = "/_search"
		// Index selection, routing and preference belong to PIT creation only.
		for _, key := range []string{"routing", "preference", "expand_wildcards", "ignore_unavailable"} {
			options.Del(key)
		}
	} else if first {
		if err := searchTimeout(q, time.Until(deadline(ctx)).Milliseconds()); err != nil {
			return nil, err
		}
		body = q.body
		options.Set("scroll", lease)
	} else {
		path = "/_search/scroll"
		options = url.Values{}
		if value := c.query.Options["rest_total_hits_as_int"]; len(value) != 0 {
			var b bool
			if json.Unmarshal(value, &b) == nil {
				options.Set("rest_total_hits_as_int", strconv.FormatBool(b))
			} else {
				var text string
				if json.Unmarshal(value, &text) == nil {
					options.Set("rest_total_hits_as_int", text)
				}
			}
		}
		body, _ = json.Marshal(map[string]any{"scroll_id": c.ids[0], "scroll": lease})
	}
	if len(body) > t.cfg.Elasticsearch.MaxRequestBytes+2*t.cfg.Elasticsearch.MaxContextIDBytes {
		return nil, failure("resource_limit", "framed context request exceeds byte limit")
	}
	raw, err := t.wire.request(ctx, c.endpoint, http.MethodPost, path, options, body, "application/json", t.limits.MaxResultBytes, true)
	if err != nil {
		return nil, err
	}
	var native map[string]json.RawMessage
	if json.Unmarshal(raw, &native) != nil || native == nil {
		return nil, failure("invalid_response", "invalid native cursor response")
	}
	if _, ok := native["error"]; ok {
		return nil, failure("cursor_expired", "native context is unavailable; no new snapshot was created")
	}
	key := "pit_id"
	if c.kind == "scroll" {
		key = "_scroll_id"
	}
	if value, ok := native[key]; ok {
		var nativeID string
		if json.Unmarshal(value, &nativeID) != nil {
			return nil, failure("invalid_response", "invalid context identifier")
		}
		if err := t.captureCursorID(c, nativeID); err != nil {
			return nil, err
		}
	} else if c.kind == "scroll" {
		return nil, failure("invalid_response", "scroll response is missing its continuation identifier")
	} else {
		c.uncertain = false
	}
	other := "_scroll_id"
	if c.kind == "scroll" {
		other = "pit_id"
	}
	if _, ok := native[other]; ok {
		c.uncertain = true
		return nil, failure("invalid_response", "unexpected context type in native response")
	}
	delete(native, "pit_id")
	delete(native, "_scroll_id")
	clean, _ := json.Marshal(native)
	if err := validateQueryResponse(clean, q); err != nil {
		return nil, err
	}
	var hits struct {
		Hits struct {
			Hits []struct {
				Sort json.RawMessage `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if json.Unmarshal(clean, &hits) != nil || hits.Hits.Hits == nil {
		return nil, failure("invalid_response", "invalid cursor hits")
	}
	c.lastHits = len(hits.Hits.Hits)
	c.after = nil
	if c.lastHits > 0 {
		c.after = hits.Hits.Hits[c.lastHits-1].Sort
		if len(c.after) > t.cfg.Elasticsearch.MaxContextIDBytes {
			return nil, failure("resource_limit", "continuation sort values exceed context byte limit")
		}
	}
	duration, _ := time.ParseDuration(lease)
	c.serverUntil = time.Now().Add(c.nativeLease)
	c.expires = minTime(time.Now().Add(duration), c.absolute)
	if ctx.Err() != nil {
		return nil, failure("deadline_exceeded", "cursor page completed after its deadline")
	}
	return clean, nil
}

func (t *Target) cursorResult(ctx context.Context, c *ownedCursor, q *preparedQuery, data []byte, id string, started time.Time) (*core.ElasticsearchCursorResult, error) {
	base := t.queryResult(id, q, data, started)
	base.Consistency = c.kind
	base.Operation = "cursor_page"
	result := &core.ElasticsearchCursorResult{ElasticsearchResult: *base, Kind: c.kind, Handle: c.handle, ExpiresAt: c.expires.UTC().Format(time.RFC3339Nano)}
	body, _ := decodeObject(q.body)
	_, aggs := body["aggs"]
	_, aggregations := body["aggregations"]
	zeroSize := body["size"] == json.Number("0")
	if c.lastHits == 0 && (c.kind == "scroll" || (!aggs && !aggregations && !zeroSize)) {
		result.Exhausted = true
		result.Closed = true
		result.Handle = ""
		result.ExpiresAt = ""
		t.recoverCursor(ctx, c)
		result.CleanupPending = !c.released
	}
	if err := t.encodedLimit(result); err != nil {
		return nil, err
	}
	return result, nil
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
