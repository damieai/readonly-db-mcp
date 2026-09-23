package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

const daySource = `<reports-{now/d{yyyy.MM.dd|+08:00}}>`

func TestDateMathSelectorGuardKeepsNativeStaticEscapes(t *testing.T) {
	if !dateMathSource(`<reports-\{literal\}-{now/d}>`) {
		t.Fatal("escaped native static braces were rejected")
	}
	for _, value := range []string{`<reports-\/{now/d}>`, `<reports-\,{now/d}>`, `<reports-\`, `<reports-{now/d>`, `<reports-{now/d}>/other`} {
		if dateMathSource(value) {
			t.Fatal("invalid native date route accepted", value)
		}
	}
}

func TestDateMathNativeSourcesAndProtocol(t *testing.T) {
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
			var queryCalls, sourceReads atomic.Int64
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if strings.Contains(r.URL.Path, daySource) {
					if !strings.Contains(r.RequestURI, "%3C") || !strings.Contains(r.RequestURI, "%7B") || !strings.Contains(r.RequestURI, "%2F") || !strings.Contains(r.RequestURI, "%2B08%3A00") || !strings.Contains(r.RequestURI, "%3E") || strings.Contains(r.RequestURI, "/../") {
						t.Errorf("date source was not URL escaped: %s", r.RequestURI)
					}
					sourceReads.Add(1)
				}
				switch {
				case r.URL.Path == "/"+daySource+"/_search":
					queryCalls.Add(1)
					fmt.Fprint(w, nativeSearchResult)
				case r.URL.Path == "/reports-*/_search":
					queryCalls.Add(1)
					fmt.Fprint(w, nativeSearchResult)
				case r.URL.Path == "/"+daySource+"/_eql/search":
					queryCalls.Add(1)
					fmt.Fprint(w, eqlEventResponse)
				case r.URL.Path == "/_sql":
					queryCalls.Add(1)
					fmt.Fprint(w, sqlRows)
				case r.URL.Path == "/_query":
					queryCalls.Add(1)
					fmt.Fprint(w, esqlRows)
				default:
					return false
				}
				return true
			})
			for _, operation := range []string{"resolve", "mappings"} {
				if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: operation, Indices: []string{daySource}}); err != nil {
					t.Fatal(operation, err)
				}
			}
			queries := []core.ElasticsearchQueryRequest{
				{Operation: "search", Indices: []string{daySource}, Body: json.RawMessage(`{"query":{"match_all":{}}}`)},
				{Operation: "eql.search", Indices: []string{daySource}, Body: json.RawMessage(`{"query":"any where true"}`)},
				{Operation: "sql.query", Body: mustJSON(t, map[string]any{"query": `SELECT id FROM "` + daySource + `"`})},
				{Operation: "esql.query", Body: mustJSON(t, map[string]any{"query": `FROM "` + daySource + `" | LIMIT 1`})},
				{Operation: "search", Body: mustJSON(t, map[string]any{"query": map[string]any{"terms": map[string]any{"label": map[string]any{"index": daySource, "id": "a", "path": "labels"}}}})},
			}
			for _, query := range queries {
				out, err := target.ElasticsearchQuery(context.Background(), query)
				if err != nil {
					t.Fatalf("%s: %v", query.Operation, err)
				}
				if out.Partial || out.Truncated {
					t.Fatal("native date result incomplete")
				}
			}
			if queryCalls.Load() != int64(len(queries)) || sourceReads.Load() < 8 || controller.Stats().Active != 0 {
				t.Fatal("date expression was rewritten, not proved, or replayed")
			}
		})
	}
}

func TestDateMathRejectsRouteEscapesAndScopeViolations(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var calls atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_search") {
			calls.Add(1)
			fmt.Fprint(w, nativeSearchResult)
			return true
		}
		return false
	})
	for _, source := range []string{
		`<reports-{now/d},private>`, `<remote:private-{now/d}>`, `<reports-{now/d}>/../_bulk`,
		`<reports-{now/d}>?x=1`, `<reports-{now/d}>%2F_bulk`, `<reports-{now/d}>:private`,
		`<reports-{now/d}>::failures`, `<reports-{now/d>`, `<reports-{now/d}}>`, `<reports-{now/d}>#x`,
	} {
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Indices: []string{source}, Body: json.RawMessage(`{"query":{"match_all":{}}}`)}); err == nil {
			t.Fatal("unsafe date selector accepted", source)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid date selector executed")
	}
	f.mu.Lock()
	f.resolution = `{"indices":[{"name":"private-2026","attributes":["open"]}],"aliases":[],"data_streams":[]}`
	f.mu.Unlock()
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Indices: []string{daySource}, Body: json.RawMessage(`{"query":{"match_all":{}}}`)}); err == nil || calls.Load() != 0 {
		t.Fatal("out-of-scope resolution reached execution")
	}
}

func TestDateMathRequestedScopesAndAliasSemantics(t *testing.T) {
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
				if r.URL.Path == "/_sql" {
					calls.Add(1)
					fmt.Fprint(w, sqlRows)
					return true
				}
				if r.URL.Path == "/_query" {
					calls.Add(1)
					fmt.Fprint(w, esqlRows)
					return true
				}
				return false
			})
			for _, operation := range []string{"sql.query", "esql.query"} {
				query := `SELECT id FROM "` + daySource + `"`
				if operation == "esql.query" {
					query = `FROM "` + daySource + `" | LIMIT 1`
				}
				body := mustJSON(t, map[string]any{"query": query})
				for _, requested := range [][]string{{"reports-*"}, {"reports-2026"}, {daySource}} {
					if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: operation, Indices: requested, Body: body}); err != nil {
						t.Fatalf("%s %v: %v", operation, requested, err)
					}
				}
				for _, requested := range [][]string{{"reports-other"}, {"reports-private"}} {
					if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: operation, Indices: requested, Body: body}); err == nil {
						t.Fatal("requested index scope widened")
					}
				}
			}
			if calls.Load() != 6 || controller.Stats().Active != 0 {
				t.Fatal("requested date scope dispatched a denied query")
			}
			f.mu.Lock()
			f.resolution = `{"indices":[{"name":"reports-2026","attributes":["open"]}],"aliases":[{"name":"reports-date","indices":["reports-2026"]}],"data_streams":[]}`
			f.mu.Unlock()
			body := mustJSON(t, map[string]any{"query": `FROM "` + daySource + `" | LIMIT 1`})
			if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Indices: []string{"reports-date"}, Body: body}); err != nil {
				t.Fatal("date alias logical scope lost", err)
			}
			if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Indices: []string{"reports-2026"}, Body: body}); err == nil {
				t.Fatal("alias was silently treated as its backing index")
			}
		})
	}
}

func TestDateMathInventoryDriftRejectsWholeBatch(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	var calls atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/_msearch" {
			return false
		}
		calls.Add(1)
		f.mu.Lock()
		f.resolution = `{"indices":[{"name":"reports-2027","attributes":["open"]}],"aliases":[],"data_streams":[]}`
		f.mu.Unlock()
		fmt.Fprintf(w, `{"responses":[%s]}`, nativeSearchResult)
		return true
	})
	query := core.ElasticsearchQueryRequest{Operation: "search", Indices: []string{daySource}, Body: json.RawMessage(`{"query":{"match_all":{}}}`)}
	out, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{query}})
	if err == nil || out != nil || calls.Load() != 1 || controller.Stats().Active != 0 {
		t.Fatal("date rollover drift returned partial batch or leaked admission")
	}
}
