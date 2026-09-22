package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

const sqlRows = `{"columns":[{"name":"value","type":"long"}],"rows":[[9007199254740993]]}`

func TestSQLNativeGrammarQueriesAndTranslate(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		t.Run(version, func(t *testing.T) {
			f := newESFixture(t)
			f.version = version
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Version = version
			target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			expected := ""
			var calls atomic.Int64
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/_sql" && r.URL.Path != "/_sql/translate" {
					return false
				}
				calls.Add(1)
				raw, _ := io.ReadAll(r.Body)
				m, e := decodeObject(raw)
				if e != nil || m["query"] != expected {
					t.Errorf("SQL text changed: %s", raw)
				}
				if m["mode"] != "jdbc" || m["version"] != version || !strings.Contains(string(raw), `"type":"long"`) {
					t.Error("native typed parameter/client mode changed")
				}
				if !strings.Contains(string(raw), "9007199254740993") {
					t.Error("SQL parameter precision lost")
				}
				if r.URL.Path == "/_sql" {
					if m["allow_partial_search_results"] != false || m["binary_format"] != false || r.URL.Query().Get("format") != "json" {
						t.Error("unsafe output/completion profile")
					}
					if _, ok := m["wait_for_completion_timeout"]; ok {
						t.Error("SQL converted to asynchronous")
					}
					if _, ok := m["request_timeout"]; !ok {
						t.Error("missing native deadline")
					}
					if _, ok := m["page_timeout"]; !ok {
						t.Error("missing native lease")
					}
					fmt.Fprint(w, sqlRows)
				} else {
					fmt.Fprint(w, `{"size":2,"query":{"range":{"value":{"gte":9007199254740993}}}}`)
				}
				return true
			})
			for _, sql := range []string{
				`SELECT SUM(value), DATE_TRUNC('day', ts) FROM "reports-alias" WHERE value > ? GROUP BY DATE_TRUNC('day', ts) HAVING SUM(value)>0 ORDER BY 1 DESC LIMIT 2`,
				`WITH "reports-cte" AS (SELECT value FROM "reports-alias" WHERE value > ?) SELECT * FROM "reports-cte"`,
				`SELECT * FROM (SELECT * FROM "reports-alias") x LEFT JOIN "reports-other" y ON x.id=y.id WHERE x.value > ?`,
				`EXPLAIN (PLAN ALL) SELECT * FROM "reports-alias" WHERE value > ?`,
				`SELECT CASE WHEN ? > 0 THEN 'DELETE FROM private-*' END`,
			} {
				expected = sql
				body := mustJSON(t, map[string]any{"query": sql, "mode": "jdbc", "version": version, "params": []any{map[string]any{"type": "long", "value": json.Number("9007199254740993")}}, "fetch_size": 2, "time_zone": "Asia/Shanghai", "filter": map[string]any{"terms": map[string]any{"tenant": map[string]any{"index": "reports-lookup", "id": "x", "path": "values"}}}, "runtime_mappings": map[string]any{"n": map[string]any{"type": "long", "script": "emit(1)"}}})
				for _, op := range []string{"sql.query", "sql.translate"} {
					out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: op, Body: body})
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(out.Data), "9007199254740993") || out.Partial || out.Truncated {
						t.Fatal("native SQL result changed")
					}
				}
			}
			if calls.Load() != 10 || len(target.cursorInventory()) != 0 || controller.Stats().Active != 0 {
				t.Fatal("SQL leaked state or replayed")
			}
		})
	}
}
func TestSQLRejectsEffectsAndAllSourceLocationsBeforeExecution(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var calls atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/_sql" || r.URL.Path == "/_sql/translate" {
			calls.Add(1)
			fmt.Fprint(w, sqlRows)
			return true
		}
		return false
	})
	for _, body := range []string{
		`{"query":"DELETE FROM reports"}`, `{"query":"SELECT 1; DELETE FROM reports"}`, `{"query":"SELECT 1","cursor":"foreign"}`,
		`{"query":"SELECT * FROM \"private-*\""}`, `{"query":"SELECT * FROM \"reports-*\" WHERE EXISTS (SELECT 1 FROM \"private-*\")"}`,
		`{"query":"WITH x AS (SELECT * FROM \"private-*\") SELECT * FROM \"reports-*\""}`,
		`{"query":"SELECT * FROM \"reports-*\" JOIN \"private-*\" ON true"}`,
		`{"query":"SHOW TABLES"}`, `{"query":"SHOW TABLES LIKE ?","params":["private-%"]}`,
		`{"query":"SELECT * FROM \"remote\":\"reports-*\""}`,
		`{"query":"SELECT 1","filter":{"terms":{"x":{"index":"private","id":"x","path":"x"}}}}`,
		`{"query":"SELECT 1","runtime_mappings":{"x":{"type":"lookup","target_index":"private","input_field":"x","target_field":"x","fetch_fields":["x"]}}}`,
		`{"query":"SELECT 1","wait_for_completion_timeout":"0ms"}`, `{"query":"SELECT 1","wait_for_completion_timeout":0}`, `{"query":"SELECT 1","keep_on_completion":true}`,
		`{"query":"SELECT 1","allow_partial_search_results":true}`, `{"query":"SELECT 1","binary_format":true}`, `{"query":"SELECT 1","fetch_size":101}`, `{"query":"SELECT 1","page_timeout":"16m"}`,
		`{"query":"SELECT 1","query":"SELECT 2"}`, `{"query":"SHOW TABLES LIKE ?","params":[{"type":"keyword","value":"private-%"}]}`,
	} {
		_, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "sql.query", Body: json.RawMessage(body)})
		if err == nil {
			t.Fatalf("admitted %s", body)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("rejected SQL dispatched")
	}
	// Explicit request scopes cannot be widened by an allowed but different SQL index.
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "sql.query", Indices: []string{"reports-alias"}, Body: json.RawMessage(`{"query":"SELECT * FROM \"reports-other\""}`)}); err == nil {
		t.Fatal("SQL widened envelope scope")
	}
	for _, body := range []string{`{"query":"SELECT 1","wait_for_completion_timeout":"-1"}`, `{"query":"SELECT 1","wait_for_completion_timeout":-1}`, `{"query":"SHOW TABLES LIKE 'reports-%'"}`, `{"query":"SYS COLUMNS TABLE LIKE 'reports-%' LIKE 'field_%'"}`} {
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "sql.query", Body: json.RawMessage(body)}); err != nil {
			t.Fatal(err)
		}
	}
}
func TestSQLStatelessDrainsPagesPreservesOrientationAndCleansLimits(t *testing.T) {
	for _, columnar := range []bool{false, true} {
		t.Run(fmt.Sprint(columnar), func(t *testing.T) {
			f := newESFixture(t)
			target, _ := openFixture(t, f)
			var calls, clears atomic.Int64
			endless := false
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/_sql/close" {
					clears.Add(1)
					fmt.Fprint(w, `{"succeeded":true}`)
					return true
				}
				if r.URL.Path != "/_sql" {
					return false
				}
				calls.Add(1)
				raw, _ := io.ReadAll(r.Body)
				body, _ := decodeObject(raw)
				_, next := body["cursor"]
				if body["columnar"] != columnar {
					t.Error("SQL orientation not retained")
				}
				response := map[string]any{}
				key := "rows"
				if columnar {
					key = "values"
				}
				response[key] = [][]any{{json.Number("9007199254740993")}}
				if !next {
					response["columns"] = []any{map[string]any{"name": "value", "type": "long"}}
				}
				if !next || endless {
					response["cursor"] = "sql-native-token"
				}
				_ = json.NewEncoder(w).Encode(response)
				return true
			})
			request := core.ElasticsearchQueryRequest{Operation: "sql.query", MaxRows: 3, Body: mustJSON(t, map[string]any{"query": `SELECT value FROM "reports-*"`, "fetch_size": 1, "columnar": columnar})}
			result, err := target.ElasticsearchQuery(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(result.Data), "9007199254740993") != 2 || strings.Contains(string(result.Data), "cursor") || calls.Load() != 2 || len(target.cursorInventory()) != 0 {
				t.Fatal("SQL pages not completely merged or cursor leaked")
			}
			endless = true
			_, err = target.ElasticsearchQuery(context.Background(), request)
			if err == nil || !strings.HasPrefix(err.Error(), "resource_limit:") || clears.Load() == 0 || len(target.cursorInventory()) != 0 {
				t.Fatalf("oversized SQL did not fail whole/clear: %v", err)
			}
		})
	}
}
func TestSQLOwnedCursorRotationScopeAndCleanup(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var calls, clears atomic.Int64
	finish := false
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/_sql/close" {
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), "sql-token-2") {
				t.Error("SQL clear did not use rotated ID")
			}
			clears.Add(1)
			fmt.Fprint(w, `{"succeeded":true}`)
			return true
		}
		if r.URL.Path != "/_sql" {
			return false
		}
		n := calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		m, _ := decodeObject(raw)
		if n == 1 {
			if m["query"] != `SELECT * FROM "reports-*"` {
				t.Error("query changed")
			}
		} else {
			if _, ok := m["query"]; ok {
				t.Error("SQL cursor reran original query")
			}
			if m["cursor"] != "sql-token-1" {
				t.Error("wrong continuation token")
			}
		}
		response := strings.TrimSuffix(sqlRows, "}")
		if !finish {
			response += fmt.Sprintf(`,"cursor":"sql-token-%d"`, n)
		}
		fmt.Fprint(w, response+"}")
		return true
	})
	open := core.ElasticsearchCursorRequest{Action: "open", Kind: "sql", Owner: "session-a", Body: json.RawMessage(`{"query":"SELECT * FROM \"reports-*\"","fetch_size":2,"page_timeout":"20s"}`)}
	first, err := target.ElasticsearchCursor(context.Background(), open)
	if err != nil {
		t.Fatal(err)
	}
	if first.Handle == "" || strings.Contains(string(first.Data), "sql-token") {
		t.Fatal("missing local handle or exposed native ID")
	}
	if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session-b", Handle: first.Handle}); err == nil {
		t.Fatal("cross-session SQL cursor accepted")
	}
	if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session-a", Handle: first.Handle, Body: json.RawMessage(`{"query":"SELECT 1"}`)}); err == nil {
		t.Fatal("SQL cursor query replaced")
	}
	next, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session-a", Handle: first.Handle})
	if err != nil || next.Handle != first.Handle {
		t.Fatal(err)
	}
	if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "close", Owner: "session-a", Handle: first.Handle}); err != nil {
		t.Fatal(err)
	}
	if clears.Load() != 1 || len(target.cursorInventory()) != 0 {
		t.Fatal("SQL cursor not cleared")
	}
	finish = true
	calls.Store(0)
	last, err := target.ElasticsearchCursor(context.Background(), open)
	if err != nil || !last.Exhausted || !last.Closed || last.Handle != "" {
		t.Fatal("last SQL page did not close", err)
	}
}
func TestSQLResponseProfileFailureAndNoLeak(t *testing.T) {
	for _, response := range []string{
		`{"id":"persisted","is_running":true,"is_partial":true,"columns":[],"rows":[],"cursor":"sql-owned"}`,
		`{"columns":[{"name":"x","type":"long"}],"rows":[[1,2]],"cursor":"sql-owned"}`,
		`{"columns":[{"name":"x","type":"long"}],"values":[[1]],"cursor":"sql-owned"}`,
		`{"columns":[],"rows":[],"cursor":"sql-owned"}`,
		`{"columns":[],"rows":[],"timed_out":true,"cursor":"sql-owned"}`,
	} {
		t.Run(response, func(t *testing.T) {
			f := newESFixture(t)
			target, _ := openFixture(t, f)
			var clears atomic.Int64
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				switch r.URL.Path {
				case "/_sql":
					fmt.Fprint(w, response)
					return true
				case "/_sql/close":
					clears.Add(1)
					fmt.Fprint(w, `{"succeeded":true}`)
					return true
				}
				return false
			})
			if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "sql.query", Body: json.RawMessage(`{"query":"SELECT 1"}`)}); err == nil {
				t.Fatal("invalid SQL response admitted")
			}
			if clears.Load() != 1 || len(target.cursorInventory()) != 0 {
				t.Fatal("failed SQL context not cleared")
			}
			if strings.Contains(response, `"id"`) && target.Info().Healthy {
				t.Fatal("async profile drift did not invalidate target")
			}
		})
	}
}
func TestSQLBatchPreflightAndCancellation(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	var calls atomic.Int64
	cancelled := make(chan struct{}, 1)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/_sql" {
			return false
		}
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
			cancelled <- struct{}{}
		case <-time.After(2 * time.Second):
		}
		return true
	})
	_, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{{Operation: "sql.query", Body: json.RawMessage(`{"query":"SELECT 1"}`)}, {Operation: "sql.query", Body: json.RawMessage(`{"query":"SELECT * FROM \"private-*\""}`)}}})
	if err == nil || calls.Load() != 0 {
		t.Fatal("SQL batch dispatched before complete preflight")
	}
	start := time.Now()
	_, err = target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "sql.query", Timeout: 80 * time.Millisecond, Body: json.RawMessage(`{"query":"SELECT 1","page_timeout":"1ms"}`)})
	if err == nil || time.Since(start) > time.Second || controller.Stats().Active != 0 {
		t.Fatal("SQL cancellation failed", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("HTTP cancellation did not reach SQL")
	}
	// Unknown opening outcomes keep a bounded orphan reservation; never replay.
	if calls.Load() != 1 {
		t.Fatal("SQL query retried")
	}
}

func TestSQLLocalCatalogFullTextAndSelectorProof(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	f.mu.Lock()
	f.mapping = `{"reports-2026":{"mappings":{"properties":{"title":{"type":"text"},"semantic":{"type":"semantic_text"},"semantic_alias":{"type":"alias","path":"semantic"}}}}}`
	f.mu.Unlock()
	var calls atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/_sql" {
			calls.Add(1)
			fmt.Fprint(w, sqlRows)
			return true
		}
		return false
	})
	for _, sql := range []string{`SELECT * FROM "fixture-cluster":"reports-*" WHERE MATCH(title, 'term')`, `SELECT * FROM "reports-*"::data WHERE MATCH('title^2', 'term')`, `SYS TABLES CATALOG LIKE '%' LIKE ''`, `SHOW CATALOGS`} {
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "sql.query", Body: mustJSON(t, map[string]any{"query": sql})}); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	allowed := calls.Load()
	for _, sql := range []string{`SELECT * FROM "reports-*" WHERE MATCH(semantic, 'term')`, `SELECT * FROM "reports-*" WHERE MATCH(semantic_alias, 'term')`, `SELECT * FROM "reports-*" WHERE QUERY('title:term')`, `SELECT * FROM "reports-*"::failures`, `SHOW TABLES CATALOG LIKE 'remote%' LIKE 'reports-%'`} {
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "sql.query", Body: mustJSON(t, map[string]any{"query": sql})}); err == nil {
			t.Fatalf("unproved SQL feature admitted: %s", sql)
		}
	}
	if calls.Load() != allowed {
		t.Fatal("unproved fulltext/catalog executed")
	}
}

func TestSQLFailedCleanupRetainsReservationAndRejectsReuse(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var pages atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/_sql":
			pages.Add(1)
			fmt.Fprint(w, strings.TrimSuffix(sqlRows, "}")+`,"cursor":"sql-owned"}`)
			return true
		case "/_sql/close":
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":"unavailable"}`)
			return true
		}
		return false
	})
	out, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "open", Kind: "sql", Owner: "owner", Body: json.RawMessage(`{"query":"SELECT 1"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "close", Owner: "owner", Handle: out.Handle}); err == nil || !strings.HasPrefix(err.Error(), "cleanup_pending:") {
		t.Fatal(err)
	}
	c, err := target.lookupCursor("owner", out.Handle)
	if err != nil {
		t.Fatal("cleanup failure released reservation")
	}
	if err := c.lock(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !c.invalid || c.released {
		t.Fatal("failed cleanup remained usable")
	}
	c.serverUntil = time.Now().Add(-time.Second)
	if err := target.cleanupCursor(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	c.unlock()
	if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "owner", Handle: out.Handle}); err == nil || pages.Load() != 1 {
		t.Fatal("disabled SQL handle executed again")
	}
}
