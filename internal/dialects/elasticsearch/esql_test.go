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

const esqlRows = `{"took":1,"is_partial":false,"columns":[{"name":"value","type":"long"}],"values":[[9007199254740993]]}`

func TestESQLNativeQueriesPreserveTextParametersAndProfile(t *testing.T) {
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
			var calls atomic.Int64
			expected := ""
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/_query" {
					return false
				}
				calls.Add(1)
				raw, _ := io.ReadAll(r.Body)
				body, err := decodeObject(raw)
				if err != nil || body["query"] != expected || r.Method != http.MethodPost {
					t.Errorf("native ES|QL changed: %s", raw)
				}
				if !strings.Contains(string(raw), "9007199254740993") || body["locale"] != "en-US" || body["profile"] != true {
					t.Error("native parameters/controls changed")
				}
				if r.URL.Query().Get("allow_partial_results") != "false" || r.URL.Query().Get("format") != "json" {
					t.Error("incorrect completion/format profile")
				}
				fmt.Fprint(w, esqlRows)
				return true
			})
			queries := []string{
				`FROM reports-alias | WHERE amount > ?n | STATS total=SUM(amount) WHERE amount>0 BY category | SORT total DESC | LIMIT 2`,
				`FROM "reports-alias, reports-other" | LOOKUP JOIN reports-lookup ON id | EVAL label=COALESCE(title,"DELETE FROM private") | KEEP label | LIMIT 2`,
				`FROM "reports-*, -reports-excluded" METADATA _index | WHERE amount>?n | LIMIT 2`,
				`FROM reports-*::data | WHERE KNN(vector, TO_DENSE_VECTOR([0.1,0.2,0.3]), {"k": 2}) | LIMIT 2`,
				`ROW x=?n | EVAL y=ABS(x)`, `SHOW INFO`,
			}
			if version == "9.1.10" {
				queries = append(queries, `FROM reports-alias | FORK (WHERE amount>?n | LOOKUP JOIN reports-lookup ON id) (STATS n=COUNT(*)) | LIMIT 2`)
			} else {
				queries = append(queries, `EXPLAIN [FROM reports-alias | LOOKUP JOIN reports-lookup ON id]`)
			}
			for _, query := range queries {
				expected = query
				body := mustJSON(t, map[string]any{"query": query, "params": []any{map[string]any{"n": json.Number("9007199254740993")}}, "locale": "en-US", "profile": true, "include_ccs_metadata": true, "filter": map[string]any{"terms": map[string]any{"tenant": map[string]any{"index": "reports-lookup", "id": "x", "path": "values"}}}})
				out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: body})
				if err != nil {
					t.Fatalf("%s: %v", query, err)
				}
				if !strings.Contains(string(out.Data), "9007199254740993") || out.Partial || out.Truncated {
					t.Fatal("native result changed")
				}
			}
			if calls.Load() != int64(len(queries)) || controller.Stats().Active != 0 || len(target.cursorInventory()) != 0 {
				t.Fatal("replayed or leaked ES|QL execution")
			}
		})
	}
}

func TestESQLRejectsUnprovedEffectsAndSourcesBeforeDispatch(t *testing.T) {
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
			var calls atomic.Int64
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/_query" {
					return false
				}
				calls.Add(1)
				fmt.Fprint(w, esqlRows)
				return true
			})
			bodies := []string{
				`{"query":"DELETE FROM reports-*"}`, `{"query":"ROW x=1; FROM private"}`, `{"query":"FROM private"}`,
				`{"query":"FROM reports-* | LOOKUP JOIN private ON id"}`, `{"query":"FROM \"reports-*,private\""}`,
				`{"query":"FROM remote:reports-*"}`, `{"query":"FROM reports-*::failures"}`, `{"query":"FROM -reports-*"}`,
				`{"query":"FROM reports-* | ENRICH policy"}`, `{"query":"ROW x=\"text\" | COMPLETION x WITH endpoint"}`,
				`{"query":"ROW x=1","wait_for_completion_timeout":"0ms"}`, `{"query":"ROW x=1","keep_on_completion":true}`,
				`{"query":"ROW x=1","tables":{"t":{}}}`, `{"query":"ROW x=1","pragma":{"page_size":999999},"accept_pragma_risks":true}`,
				`{"query":"ROW x=1","filter":{"terms":{"x":{"index":"private","id":"x","path":"x"}}}}`,
				`{"query":"ROW x=1","query":"FROM private"}`, `{"query":"ROW x=1","columnar":"true"}`,
			}
			if version == "9.1.10" {
				bodies = append(bodies, `{"query":"FROM reports-* | FORK (LIMIT 1) (LOOKUP JOIN private ON id)"}`)
			} else {
				bodies = append(bodies, `{"query":"EXPLAIN [FROM private]"}`)
			}
			for _, body := range bodies {
				if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: json.RawMessage(body)}); err == nil {
					t.Fatalf("admitted %s", body)
				}
			}
			for _, options := range []map[string]json.RawMessage{{"allow_partial_results": json.RawMessage(`true`)}, {"format": json.RawMessage(`"csv"`)}, {"url": json.RawMessage(`"/_bulk"`)}} {
				if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: json.RawMessage(`{"query":"ROW x=1"}`), Options: options}); err == nil {
					t.Fatal("unproved option admitted")
				}
			}
			if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Indices: []string{"reports-one"}, Body: json.RawMessage(`{"query":"FROM reports-two"}`)}); err == nil {
				t.Fatal("widened request scope")
			}
			if calls.Load() != 0 {
				t.Fatal("rejected ES|QL dispatched")
			}
		})
	}
}

func TestESQLFullTextTracksFieldAliasesAndNativeParameters(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	f.mu.Lock()
	f.mapping = `{"reports-2026":{"mappings":{"properties":{"title":{"type":"text"},"semantic":{"type":"semantic_text"},"semantic^field":{"type":"semantic_text"},"semantic_alias":{"type":"alias","path":"semantic"}}}}}`
	f.mu.Unlock()
	var calls atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/_query" {
			return false
		}
		calls.Add(1)
		fmt.Fprint(w, esqlRows)
		return true
	})
	for _, query := range []string{`FROM reports-* | WHERE MATCH(title,"text")`, `FROM reports-* | WHERE title:"text"`, `FROM reports-* | EVAL a=title | RENAME a AS b | WHERE MATCH(b,"text")`, `FROM reports-* | WHERE ??fn(title,"text")`, `FROM reports-* | EVAL n=ABS(price) | STATS n=SUM(n)`} {
		body := mustJSON(t, map[string]any{"query": query, "params": []any{map[string]any{"fn": "match"}}})
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: body}); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	allowed := calls.Load()
	for _, query := range []string{`FROM reports-* | WHERE MATCH(` + "`semantic^field`" + `,"text")`, `FROM reports-* | WHERE MATCH(semantic,"text")`, `FROM reports-* | WHERE semantic_alias:"text"`, `FROM reports-* | EVAL a=semantic | RENAME a AS b | WHERE MATCH(b,"text")`, `FROM reports-* | RENAME title = semantic | WHERE MATCH(title,"text")`, `FROM reports-* | RENAME ` + "`sem`antic" + ` AS title | WHERE MATCH(title,"text")`, `FROM reports-* | WHERE ??fn(semantic,"text")`, `FROM reports-* | WHERE QSTR("title:text")`} {
		body := mustJSON(t, map[string]any{"query": query, "params": []any{map[string]any{"fn": "match"}}})
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: body}); err == nil {
			t.Fatalf("unproved inference admitted: %s", query)
		}
	}
	if calls.Load() != allowed {
		t.Fatal("semantic inference dispatched")
	}
	for _, field := range []string{"title", "semantic"} {
		for _, query := range []string{`FROM reports-* | EVAL alias=?field | WHERE MATCH(alias,"text")`, `FROM reports-* | WHERE MATCH(?field,"text")`} {
			body := mustJSON(t, map[string]any{"query": query, "params": []any{map[string]any{"field": map[string]any{"identifier": field}}}})
			_, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: body})
			if (err == nil) != (field == "title") {
				t.Fatalf("typed parameter lineage %s: %v", field, err)
			}
		}
	}
}

func TestESQLResponseShapeLimitsAndCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		columnar  bool
		rows      int
		allowed   bool
	}{
		{"rows", esqlRows, false, 1, true}, {"columns", esqlRows, true, 1, true},
		{"partial", `{"took":1,"is_partial":true,"columns":[],"values":[]}`, false, 1, false},
		{"missing-completion", `{"took":1,"columns":[],"values":[]}`, false, 1, false},
		{"async-id", `{"took":1,"is_partial":false,"id":"secret","columns":[],"values":[]}`, false, 1, false},
		{"async-flag", `{"took":1,"is_partial":false,"is_running":false,"columns":[],"values":[]}`, false, 1, false},
		{"remote", `{"took":1,"is_partial":false,"_clusters":{},"columns":[],"values":[]}`, false, 1, false},
		{"row-limit", `{"took":1,"is_partial":false,"columns":[{"name":"n","type":"long"}],"values":[[1],[2]]}`, false, 1, false},
		{"column-limit", `{"took":1,"is_partial":false,"columns":[{"name":"n","type":"long"}],"values":[[1,2]]}`, true, 1, false},
		{"width", `{"took":1,"is_partial":false,"columns":[{"name":"n","type":"long"}],"values":[[1,2]]}`, false, 1, false},
		{"height", `{"took":1,"is_partial":false,"columns":[{"name":"n","type":"long"},{"name":"m","type":"long"}],"values":[[1,2],[3]]}`, true, 3, false},
		{"multivalue", `{"took":1,"is_partial":false,"columns":[{"name":"n","type":"long"}],"values":[[[1,2,3]]]}`, false, 1, true},
		{"drop-null", `{"took":1,"is_partial":false,"all_columns":[{"name":"n","type":"long"},{"name":"m","type":"null"}],"columns":[{"name":"n","type":"long"}],"values":[[1]]}`, false, 1, true},
		{"all-null-columns", `{"took":1,"is_partial":false,"all_columns":[{"name":"n","type":"null"}],"columns":[],"values":[]}`, true, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newESFixture(t)
			target, _ := openFixture(t, f)
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/_query" {
					return false
				}
				fmt.Fprint(w, tc.raw)
				return true
			})
			_, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", MaxRows: tc.rows, Body: mustJSON(t, map[string]any{"query": "ROW x=1", "columnar": tc.columnar}), Options: map[string]json.RawMessage{"drop_null_columns": json.RawMessage(`true`)}})
			if (err == nil) != tc.allowed {
				t.Fatal(err)
			}
		})
	}
}

func TestESQLSourceDriftFailsWholeResult(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/_query" {
			return false
		}
		f.mu.Lock()
		f.resolution = `{"indices":[{"name":"reports-new","attributes":["open"]}],"aliases":[],"data_streams":[]}`
		f.mu.Unlock()
		fmt.Fprint(w, esqlRows)
		return true
	})
	if result, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: json.RawMessage(`{"query":"FROM reports-* | STATS n=COUNT(*)"}`)}); err == nil || result != nil {
		t.Fatal("source drift released ES|QL data")
	}
}

func TestESQLBatchPreflightAndCancellation(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	var calls atomic.Int64
	cancelled := make(chan struct{}, 1)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/_query" {
			return false
		}
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
			cancelled <- struct{}{}
		case <-time.After(time.Second):
			t.Error("native cancellation did not propagate")
		}
		return true
	})
	_, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{{Operation: "esql.query", Body: json.RawMessage(`{"query":"ROW x=1"}`)}, {Operation: "esql.query", Body: json.RawMessage(`{"query":"FROM reports-* | LOOKUP JOIN private ON id"}`)}}})
	if err == nil || calls.Load() != 0 {
		t.Fatal("batch executed before all sources proved")
	}
	_, err = target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: json.RawMessage(`{"query":"ROW x=1"}`), Timeout: 80 * time.Millisecond})
	if err == nil {
		t.Fatal("cancelled query succeeded")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("request did not cancel")
	}
	if calls.Load() != 1 || controller.Stats().Active != 0 || len(target.cursorInventory()) != 0 {
		t.Fatal("cancelled request retried/leaked")
	}
}
