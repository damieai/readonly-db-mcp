package elasticsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func (t *Target) reserveSQL(owner string, ownerCtx context.Context, p *queryProof, q *preparedQuery) (*ownedCursor, error) {
	c, err := t.reserveCursor(owner, ownerCtx, "sql", p, q)
	if err != nil {
		return nil, err
	}
	var original map[string]json.RawMessage
	_ = json.Unmarshal(q.body, &original)
	controls := map[string]json.RawMessage{}
	for _, key := range []string{"mode", "version", "client_id"} {
		if value, exists := original[key]; exists {
			controls[key] = value
		}
	}
	c.query.Body, _ = json.Marshal(controls)
	c.query.MaxRows = q.rows
	c.sql = &sqlProfile{columnar: q.sql.columnar, keep: q.sql.keep, timeout: q.sql.timeout}
	c.sqlColumns = -1
	// Unlike search cursors, SQL continuation needs only the frozen native token
	// and output controls. Do not retain parse/proof objects or decoded mappings.
	c.mappings = nil
	return c, nil
}
func (t *Target) openSQLCursor(ctx context.Context, r core.ElasticsearchCursorRequest, keep time.Duration, id string, started time.Time) (*core.ElasticsearchCursorResult, error) {
	p := newQueryProof(t, ctx, int(t.next.Add(1)-1)%len(t.wire.clients))
	q, err := p.prepare(core.ElasticsearchQueryRequest{Operation: "sql.query", Indices: r.Indices, Body: r.Body, Options: r.Options, MaxRows: r.MaxRows})
	if err != nil {
		return nil, err
	}
	original, _ := decodeObject(r.Body)
	if _, exists := original["page_timeout"]; exists {
		if r.KeepAlive != 0 {
			return nil, failure("invalid_request", "SQL page_timeout and keep_alive_ms cannot both set the lease")
		}
		keep = q.sql.keep
	}
	q.sql.keep = keep
	c, err := t.reserveSQL(r.Owner, r.OwnerContext, p, q)
	if err != nil {
		return nil, err
	}
	defer c.unlock()
	ctx, cancel := context.WithDeadline(ctx, c.absolute)
	defer cancel()
	success := false
	defer func() {
		if !success {
			t.recoverCursor(ctx, c)
		}
	}()
	data, err := t.sqlPage(ctx, c, q, keep, true)
	if err != nil {
		return nil, err
	}
	if err := p.verifySources(); err != nil {
		return nil, err
	}
	if err := t.usableCursor(c); err != nil {
		return nil, err
	}
	result, err := t.sqlCursorResult(c, q, data, id, started)
	if err != nil {
		return nil, err
	}
	success = true
	return result, nil
}
func (t *Target) nextSQLCursor(ctx context.Context, c *ownedCursor, r core.ElasticsearchCursorRequest, keep time.Duration, id string, started time.Time) (*core.ElasticsearchCursorResult, error) {
	if len(r.Body) != 0 || r.MaxRows != 0 {
		return nil, failure("invalid_request", "SQL cursor query, parameters, output orientation and page size are fixed by open")
	}
	if r.KeepAlive == 0 {
		keep = c.sql.keep
	}
	p := newQueryProof(t, ctx, c.endpoint)
	success := false
	defer func() {
		if !success {
			t.recoverCursor(ctx, c)
		}
	}()
	if err := p.stockQueryProfile(); err != nil {
		return nil, err
	}
	for source := range c.embedded {
		if _, err := p.sources(strings.Split(source, ",")); err != nil {
			return nil, err
		}
	}
	q := &preparedQuery{request: c.query, rows: c.query.MaxRows, sql: c.sql}
	data, err := t.sqlPage(ctx, c, q, keep, false)
	if err != nil {
		return nil, err
	}
	if err := p.verifySources(); err != nil {
		return nil, err
	}
	if err := t.usableCursor(c); err != nil {
		return nil, err
	}
	result, err := t.sqlCursorResult(c, q, data, id, started)
	if err != nil {
		return nil, err
	}
	success = true
	return result, nil
}

func (t *Target) sqlPage(ctx context.Context, c *ownedCursor, q *preparedQuery, keep time.Duration, first bool) ([]byte, error) {
	lease, err := t.beginCursorDispatch(c, keep)
	if err != nil {
		return nil, err
	}
	var body map[string]any
	if first {
		body, err = decodeObject(q.body)
		if err != nil {
			return nil, err
		}
	} else {
		if len(c.ids) == 0 {
			return nil, failure("invalid_response", "SQL cursor identifier is missing")
		}
		body = map[string]any{"cursor": c.ids[0], "columnar": c.sql.columnar, "binary_format": false, "allow_partial_search_results": false}
		original, _ := decodeObject(c.query.Body)
		for _, key := range []string{"mode", "version", "client_id"} {
			if value, exists := original[key]; exists {
				body[key] = value
			}
		}
	}
	body["page_timeout"] = lease
	if err := sqlTimeout(ctx, body, c.sql.timeout); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(body)
	if err != nil || len(raw) > t.cfg.Elasticsearch.MaxRequestBytes+2*t.cfg.Elasticsearch.MaxContextIDBytes {
		return nil, failure("resource_limit", "framed SQL page exceeds request byte limit")
	}
	data, err := t.wire.request(ctx, c.endpoint, http.MethodPost, "/_sql", url.Values{"format": {"json"}}, raw, "application/json", t.limits.MaxResultBytes, false)
	if err != nil {
		return nil, err
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil || root == nil {
		return nil, failure("invalid_response", "invalid native SQL response")
	}
	// Capture only the SQL cursor, before any other response check can fail.
	// Async execution IDs are never mistaken for owned transient contexts.
	if value, exists := root["cursor"]; exists {
		var token *string
		if json.Unmarshal(value, &token) != nil || token == nil {
			return nil, failure("invalid_response", "malformed SQL cursor")
		}
		if *token != "" {
			if err := t.captureCursorID(c, *token); err != nil {
				return nil, err
			}
		} else {
			c.sqlDone = true
			c.uncertain = false
		}
	} else {
		c.sqlDone = true
		c.uncertain = false
	}
	delete(root, "cursor")
	clean, _ := json.Marshal(root)
	rows, columns, err := validateSQLPage(clean, c.sql.columnar, c.sqlColumns, first, q.rows)
	if err != nil {
		return nil, err
	}
	c.lastHits = rows
	c.sqlColumns = columns
	if rows == 0 && !c.sqlDone {
		return nil, failure("invalid_response", "SQL returned an empty page with more results")
	}
	duration, _ := time.ParseDuration(lease)
	c.expires = minTime(time.Now().Add(duration), c.absolute)
	c.serverUntil = time.Now().Add(c.nativeLease)
	if ctx.Err() != nil {
		return nil, failure("deadline_exceeded", "SQL page completed after its deadline")
	}
	return clean, nil
}

func validateSQLPage(raw []byte, columnar bool, columns int, first bool, limit int) (int, int, error) {
	root, err := decodeObject(raw)
	if err != nil {
		return 0, columns, failure("invalid_response", "invalid SQL response object")
	}
	if _, ok := root["id"]; ok {
		return 0, columns, failure("profile_mismatch", "synchronous SQL returned a persisted-result identifier")
	}
	for _, key := range []string{"is_partial", "is_running"} {
		if value, exists := root[key]; exists {
			flag, ok := value.(bool)
			if !ok {
				return 0, columns, failure("invalid_response", "invalid SQL completion flag")
			}
			if flag {
				return 0, columns, failure("profile_mismatch", "synchronous SQL returned asynchronous or incomplete work")
			}
		}
	}
	if _, ok := root["error"]; ok {
		return 0, columns, failure("upstream_error", "native SQL returned an error")
	}
	if err := completeResponse(root); err != nil {
		return 0, columns, err
	}
	for key := range root {
		switch key {
		case "columns", "rows", "values", "is_partial", "is_running":
		default:
			return 0, columns, failure("invalid_response", "unexpected SQL response control")
		}
	}
	if value, exists := root["columns"]; exists {
		fields, ok := value.([]any)
		if !ok {
			return 0, columns, failure("invalid_response", "invalid SQL columns")
		}
		if columns >= 0 && len(fields) != columns {
			return 0, columns, failure("invalid_response", "SQL cursor column count changed")
		}
		columns = len(fields)
		for _, value := range fields {
			m, ok := value.(map[string]any)
			if !ok {
				return 0, columns, failure("invalid_response", "invalid SQL column")
			}
			if _, ok := m["name"].(string); !ok {
				return 0, columns, failure("invalid_response", "SQL column name missing")
			}
			if _, ok := m["type"].(string); !ok {
				return 0, columns, failure("invalid_response", "SQL column type missing")
			}
		}
	} else if first {
		return 0, columns, failure("invalid_response", "first SQL page lacks its column schema")
	}
	key, other := "rows", "values"
	if columnar {
		key, other = other, key
	}
	if _, exists := root[other]; exists {
		return 0, columns, failure("invalid_response", "SQL output orientation changed")
	}
	arrays, ok := root[key].([]any)
	if !ok {
		return 0, columns, failure("invalid_response", "SQL result array is missing")
	}
	rows := len(arrays)
	if columnar {
		rows = 0
		if len(arrays) != columns && len(arrays) != 0 {
			return 0, columns, failure("invalid_response", "SQL columnar width changed")
		}
	}
	for i, value := range arrays {
		cells, ok := value.([]any)
		if !ok {
			return 0, columns, failure("invalid_response", "SQL result vector is malformed")
		}
		if columnar {
			if i == 0 {
				rows = len(cells)
			} else if len(cells) != rows {
				return 0, columns, failure("invalid_response", "SQL columnar vectors have different lengths")
			}
		} else if len(cells) != columns {
			return 0, columns, failure("invalid_response", "SQL row width changed")
		}
	}
	if rows > limit {
		return 0, columns, failure("resource_limit", "SQL result exceeds max_rows")
	}
	return rows, columns, nil
}
func (t *Target) sqlCursorResult(c *ownedCursor, q *preparedQuery, data []byte, id string, started time.Time) (*core.ElasticsearchCursorResult, error) {
	base := t.queryResult(id, q, data, started)
	base.Operation = "cursor_page"
	base.Consistency = "sql"
	result := &core.ElasticsearchCursorResult{ElasticsearchResult: *base, Kind: "sql", Handle: c.handle, ExpiresAt: c.expires.UTC().Format(time.RFC3339Nano)}
	if c.sqlDone {
		result.Exhausted = true
		result.Closed = true
		result.Handle = ""
		result.ExpiresAt = ""
		t.releaseCursor(c)
	}
	if err := t.encodedLimit(result); err != nil {
		return nil, err
	}
	return result, nil
}

// Stateless SQL reads drain bounded native pages under the one request deadline.
// Larger results fail whole and clear owned state; es_cursor exposes paging.
func (t *Target) executeSQL(ctx context.Context, q *preparedQuery) ([]byte, error) {
	c, err := t.reserveSQL("internal-sql-"+uuid.NewString(), ctx, q.sql.proof, q)
	if err != nil {
		return nil, err
	}
	defer c.unlock()
	defer func() {
		if !c.released {
			t.recoverCursor(ctx, c)
		}
	}()
	var merged map[string]json.RawMessage
	var result [][]json.RawMessage
	key := "rows"
	if q.sql.columnar {
		key = "values"
	}
	count := 0
	for page := 0; page <= q.rows; page++ {
		raw, err := t.sqlPage(ctx, c, q, q.sql.keep, page == 0)
		if err != nil {
			return nil, err
		}
		count += c.lastHits
		if count > q.rows {
			return nil, failure("resource_limit", "complete SQL result exceeds max_rows; use es_cursor for pagination")
		}
		var root map[string]json.RawMessage
		_ = json.Unmarshal(raw, &root)
		var values [][]json.RawMessage
		_ = json.Unmarshal(root[key], &values)
		if page == 0 {
			merged = root
			result = values
		} else if q.sql.columnar {
			for i := range values {
				result[i] = append(result[i], values[i]...)
			}
		} else {
			result = append(result, values...)
		}
		merged[key], _ = json.Marshal(result)
		combined, _ := json.Marshal(merged)
		if len(combined) > t.limits.MaxResultBytes {
			return nil, failure("resource_limit", "combined SQL pages exceed the result byte limit")
		}
		if c.sqlDone {
			t.releaseCursor(c)
			return combined, nil
		}
		if err := t.usableCursor(c); err != nil {
			return nil, err
		}
	}
	return nil, failure("resource_limit", "SQL page count exceeds bounded query execution")
}
