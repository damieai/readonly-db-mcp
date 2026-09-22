package elasticsearch

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

const nativeSearchResult = `{"took":2,"timed_out":false,"_shards":{"total":1,"successful":1,"skipped":0,"failed":0},"hits":{"total":{"value":1,"relation":"eq"},"max_score":1,"hits":[{"_index":"reports-2026","_id":"a","_score":1,"_source":{"precise":9007199254740993,"script":"delete","index":"private-data"},"sort":[9007199254740993]}]},"aggregations":{"total":{"value":9007199254740993},"page":{"after_key":{"id":9007199254740993},"buckets":[]}}}`

func interceptQuery(f *esFixture, fn func(http.ResponseWriter, *http.Request) bool) {
	f.mu.Lock()
	f.intercept = fn
	f.mu.Unlock()
}

func TestNativeAdvancedSearchPreservesSemantics(t *testing.T) {
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
			var executions atomic.Int64
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if !strings.HasSuffix(r.URL.Path, "/_search") {
					return false
				}
				executions.Add(1)
				if r.Method != "POST" || r.URL.Path != "/reports-alias/_search" {
					t.Errorf("lost alias or method: %s %s", r.Method, r.URL.Path)
				}
				raw, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(raw), "9007199254740993") || strings.Contains(string(raw), "terminate_after") {
					t.Errorf("query changed: %s", raw)
				}
				if r.URL.Query().Get("routing") != "tenant/a&b" || r.URL.Query().Get("allow_partial_search_results") != "false" {
					t.Errorf("options lost: %s", r.URL.RawQuery)
				}
				m, _ := decodeObject(raw)
				if _, ok := m["timeout"]; !ok {
					t.Error("native search deadline missing")
				}
				fmt.Fprint(w, nativeSearchResult)
				return true
			})
			body := `{"size":5,"query":{"bool":{"must":[{"match":{"title":{"query":"delete update scripts"}}}],"filter":[{"terms":{"tenant":{"index":"reports-lookup","id":"a","path":"values"}}},{"geo_shape":{"shape":{"indexed_shape":{"index":"reports-shapes","id":"s"}}}}]}},"runtime_mappings":{"amount_tax":{"type":"double","script":{"source":"emit(doc['amount'].value * params.factor)","params":{"factor":1.1,"index":"private-data"}}},"customer":{"type":"lookup","target_index":"reports-customers","input_field":"customer_id","target_field":"id","fetch_fields":["name"]}},"script_fields":{"calculated":{"script":{"source":"params.x + 1","params":{"x":9007199254740993}}}},"sort":[{"_script":{"type":"number","script":{"source":"doc['amount'].value"},"order":"desc"}}],"aggs":{"delete":{"scripted_metric":{"init_script":"state.values = []","map_script":"state.values.add(doc.amount.value)","combine_script":"return state.values","reduce_script":"def x=0; for (s in states) { x += s.size(); } return x;"}},"group":{"composite":{"size":20,"sources":[{"id":{"terms":{"field":"id"}}}],"after":{"id":9007199254740993}},"aggs":{"total":{"sum":{"field":"amount"}}}}},"profile":true,"search_after":[9007199254740993]}`
			result, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Indices: []string{"reports-alias"}, Body: json.RawMessage(body), Options: map[string]json.RawMessage{"routing": json.RawMessage(`"tenant/a&b"`)}})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(result.Data), "9007199254740993") || result.Partial || result.Truncated || result.Consistency != "eventual" {
				t.Fatal("native result changed")
			}
			if executions.Load() != 1 {
				t.Fatal("unexpected retry")
			}
		})
	}
}

func TestNativeSearchScopeAndEffectNegatives(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var dispatched atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_search") {
			dispatched.Add(1)
			fmt.Fprint(w, nativeSearchResult)
			return true
		}
		return false
	})
	wrapped := base64.StdEncoding.EncodeToString([]byte(`{"terms":{"x":{"index":"private-lookup","id":"1","path":"x"}}}`))
	cases := []struct{ body, code string }{
		{`{"query":{"terms":{"x":{"index":"private-lookup","id":"1","path":"x"}}}}`, "scope_denied"},
		{`{"query":{"geo_shape":{"shape":{"indexed_shape":{"id":"1"}}}}}`, "scope_denied"},
		{`{"query":{"more_like_this":{"like":[{"_index":"private-doc","_id":"1"}]}}}`, "scope_denied"},
		{`{"query":{"percolate":{"field":"query","index":"private-doc","id":"1"}}}`, "scope_denied"},
		{`{"query":{"pinned":{"organic":{"match_all":{}},"docs":[{"_index":"private-doc","_id":"1"}]}}}`, "scope_denied"},
		{`{"runtime_mappings":{"r":{"type":"lookup","target_index":"private-doc"}}}`, "scope_denied"},
		{`{"query":{"wrapper":{"query":"` + wrapped + `"}}}`, "scope_denied"},
		{`{"aggs":{"a":{"filter":{"terms":{"x":{"index":"private-doc","id":"1","path":"x"}}}}}}`, "scope_denied"},
		{`{"sort":[{"x":{"nested":{"path":"a","filter":{"terms":{"x":{"index":"private-doc","id":"1","path":"x"}}}}}}]}`, "scope_denied"},
		{`{"highlight":{"fields":{"a":{"highlight_query":{"terms":{"x":{"index":"private-doc","id":"1","path":"x"}}}}}}}`, "scope_denied"},
		{`{"retriever":{"rrf":{"retrievers":[{"standard":{"query":{"terms":{"x":{"index":"private-doc","id":"1","path":"x"}}}}}]}}}`, "scope_denied"},
		{`{"query":{"script":{"script":{"lang":"custom","source":"external.write()"}}}}`, "capability_unavailable"},
		{`{"knn":{"field":"v","query_vector_builder":{"text_embedding":{"model_id":"remote","model_text":"a"}},"k":10}}`, "capability_unavailable"},
		{`{"pit":{"id":"raw-context"}}`, "capability_unavailable"},
		{`{"ext":{"plugin":{"index":"private"}}}`, "capability_unavailable"},
		{`{"size":101}`, "resource_limit"},
		{`{"query":{"match_all":{}},"query":{"match_none":{}}}`, "invalid_response"},
	}
	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			_, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Body: json.RawMessage(c.body)})
			if err == nil || !strings.HasPrefix(err.Error(), c.code+":") {
				t.Fatalf("got %v want %s", err, c.code)
			}
		})
	}
	for _, op := range []string{"index", "bulk", "delete", "update_by_query", "indices.refresh"} {
		_, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: op})
		if err == nil || !strings.HasPrefix(err.Error(), "mutation_forbidden:") {
			t.Fatal(op, err)
		}
	}
	if dispatched.Load() != 0 {
		t.Fatal("rejected query reached execution")
	}
}

func TestSuppliedVectorsFusionAndOpaqueFieldNames(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_search") {
			fmt.Fprint(w, nativeSearchResult)
			return true
		}
		return false
	})
	for _, body := range []string{
		`{"knn":{"field":"embedding","query_vector":[0.1,0.2,0.3],"k":10,"num_candidates":100,"filter":{"term":{"script":"delete"}}}}`,
		`{"query":{"script_score":{"query":{"match_all":{}},"script":{"source":"cosineSimilarity(params.v, 'embedding') + 1","params":{"v":[0.1,0.2,0.3],"script":{"lang":"custom","index":"private"}}}}}}`,
		`{"retriever":{"rrf":{"retrievers":[{"standard":{"query":{"match":{"title":"dense"}}}},{"knn":{"field":"embedding","query_vector":[0.1,0.2],"k":10,"num_candidates":100}}],"rank_window_size":50}}}`,
		`{"aggs":{"script":{"terms":{"field":"index"},"meta":{"script":{"lang":"custom"},"index":"private"}}},"query":{"term":{"index":"private"}}}`,
	} {
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Body: json.RawMessage(body)}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTemplatesAndStoredScriptsAreFrozen(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var executions atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/_scripts/score/a":
			if !strings.Contains(r.RequestURI, "score%2Fa") {
				t.Errorf("script id was not encoded once: %s", r.RequestURI)
			}
			fmt.Fprint(w, `{"_id":"score/a","found":true,"script":{"lang":"painless","source":"params.x + 1"}}`)
			return true
		case "/_scripts/template":
			fmt.Fprint(w, `{"_id":"template","found":true,"script":{"lang":"mustache","source":"{\"query\":{\"match_all\":{}}}"}}`)
			return true
		case "/_render/template":
			raw, _ := io.ReadAll(r.Body)
			if strings.Contains(string(raw), `"id"`) {
				t.Error("mutable template id executed")
			}
			fmt.Fprint(w, `{"template_output":{"query":{"script_score":{"query":{"match_all":{}},"script":{"id":"score/a","params":{"x":9007199254740993}}}}}}`)
			return true
		case "/reports-*/_search":
			executions.Add(1)
			raw, _ := io.ReadAll(r.Body)
			if strings.Contains(string(raw), `"id"`) || !strings.Contains(string(raw), "params.x + 1") || !strings.Contains(string(raw), "9007199254740993") {
				t.Errorf("script not frozen: %s", raw)
			}
			fmt.Fprint(w, nativeSearchResult)
			return true
		}
		return false
	})
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search_template", Body: json.RawMessage(`{"id":"template"}`)}); err != nil {
		t.Fatal(err)
	}
	if executions.Load() != 1 {
		t.Fatal("template did not execute exactly once")
	}
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/_render/template" {
			fmt.Fprint(w, `{"template_output":{"query":{"terms":{"x":{"index":"private-data","id":"a","path":"x"}}}}}`)
			return true
		}
		if strings.HasSuffix(r.URL.Path, "/_search") {
			executions.Add(1)
			return true
		}
		return false
	})
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search_template", Body: json.RawMessage(`{"source":"unsafe"}`)}); err == nil {
		t.Fatal("rendered scope escape accepted")
	}
	if executions.Load() != 1 {
		t.Fatal("invalid rendered template executed")
	}
}

func TestIndependentBatchFramesAndPreflightsMembers(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var executions atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/_msearch" {
			return false
		}
		executions.Add(1)
		raw, _ := io.ReadAll(r.Body)
		lines := strings.Split(string(raw), "\n")
		if len(lines) != 5 || lines[4] != "" || r.Header.Get("Content-Type") != "application/x-ndjson" {
			t.Errorf("bad framing: %q", raw)
		}
		for _, i := range []int{0, 2} {
			m, _ := decodeObject([]byte(lines[i]))
			if _, ok := m["index"]; !ok {
				t.Error("unscoped msearch header")
			}
		}
		fmt.Fprintf(w, `{"responses":[%s,%s]}`, nativeSearchResult, nativeSearchResult)
		return true
	})
	request := core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{{Operation: "search", Body: json.RawMessage(`{"query":{"match_all":{}}}`)}, {Operation: "search", Body: json.RawMessage(`{"size":0,"aggs":{"n":{"sum":{"field":"amount"}}}}`)}}}
	result, err := target.ElasticsearchBatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Consistency != "independent" || len(result.Results) != 2 {
		t.Fatal("wrong batch semantics")
	}
	request.Requests[1].Body = json.RawMessage(`{"query":{"terms":{"x":{"index":"private-data","id":"1","path":"x"}}}}`)
	if _, err := target.ElasticsearchBatch(context.Background(), request); err == nil {
		t.Fatal("bad member admitted")
	}
	if executions.Load() != 1 {
		t.Fatal("batch executed before validating all members")
	}
	request.Consistency = "pit"
	if _, err := target.ElasticsearchBatch(context.Background(), request); err == nil {
		t.Fatal("unsupported snapshot claimed")
	}
}

func TestPartialOversizedAndEscapedResponsesFail(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	for _, raw := range []string{
		strings.Replace(nativeSearchResult, `"timed_out":false`, `"timed_out":true`, 1),
		strings.Replace(nativeSearchResult, `"failed":0`, `"failed":1`, 1),
		strings.Replace(nativeSearchResult, `"_index":"reports-2026"`, `"_index":"private-data"`, 1),
		strings.Replace(nativeSearchResult, `"_id":"a"`, `"_id":"`+strings.Repeat("x", 32000)+`"`, 1),
		`{"hits":{"hits":[]}}`,
	} {
		interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
			if strings.HasSuffix(r.URL.Path, "/_search") {
				fmt.Fprint(w, raw)
				return true
			}
			return false
		})
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search"}); err == nil {
			t.Fatal("invalid/incomplete result returned as complete")
		}
	}
}

func TestDocumentIDsMissingAndEmbeddedIndices(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/_doc/") {
			if r.RequestURI != "/reports-2026/_doc/a%2Fb%3F%23%25" {
				t.Errorf("id/path escaped incorrectly: %s", r.RequestURI)
			}
			w.WriteHeader(404)
			fmt.Fprint(w, `{"_index":"reports-2026","_id":"a/b?#%","found":false}`)
			return true
		}
		return false
	})
	result, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "get", Indices: []string{"reports-2026"}, ID: "a/b?#%"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Consistency != "realtime" || !strings.Contains(string(result.Data), `"found":false`) {
		t.Fatal("missing document changed")
	}
	for _, operation := range []string{"mget", "mtermvectors"} {
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: operation, Body: json.RawMessage(`{"docs":[{"_index":"private-data","_id":"a"}]}`)}); err == nil {
			t.Fatal("document index escaped scope")
		}
	}
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "get", Indices: []string{"reports-2026"}, ID: "a", Options: map[string]json.RawMessage{"refresh": json.RawMessage(`true`)}}); err == nil || !strings.HasPrefix(err.Error(), "mutation_forbidden:") {
		t.Fatal("refresh side effect admitted", err)
	}
}

func TestQueryCancellationReleasesAdmission(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	var cancelled atomic.Bool
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_search") {
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
				cancelled.Store(true)
			case <-time.After(2 * time.Second):
			}
			return true
		}
		return false
	})
	_, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Timeout: 50 * time.Millisecond})
	if err == nil {
		t.Fatal("cancelled query succeeded")
	}
	limit := time.Now().Add(time.Second)
	for !cancelled.Load() && time.Now().Before(limit) {
		time.Sleep(time.Millisecond)
	}
	if !cancelled.Load() || controller.Stats().Active != 0 {
		t.Fatal("cancellation leaked transport or admission")
	}
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_search") {
			fmt.Fprint(w, nativeSearchResult)
			return true
		}
		return false
	})
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search"}); err != nil {
		t.Fatal("pool did not recover", err)
	}
}

func TestSemanticTextInferenceChecksIncludeFieldAliases(t *testing.T) {
	f := newESFixture(t)
	f.mapping = `{"reports-2026":{"mappings":{"properties":{"title":{"type":"text"},"meaning":{"type":"semantic_text"},"meaning_alias":{"type":"alias","path":"meaning"}}}}}`
	target, _ := openFixture(t, f)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_search") {
			fmt.Fprint(w, nativeSearchResult)
			return true
		}
		return false
	})
	for _, field := range []string{"meaning", "meaning_alias"} {
		_, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Body: json.RawMessage(`{"query":{"match":{"` + field + `":"text"}}}`)})
		if err == nil || !strings.HasPrefix(err.Error(), "capability_unavailable:") {
			t.Fatal("implicit inference admitted", err)
		}
	}
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Body: json.RawMessage(`{"query":{"match":{"title":"text"}}}`)}); err != nil {
		t.Fatal("ordinary field on mixed mapping blocked", err)
	}
}

func TestTermVectorDefaultsCannotHideSourcesOrChangeOrder(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var executed atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasSuffix(r.URL.Path, "/_mtermvectors") {
			return false
		}
		executed.Add(1)
		raw, _ := io.ReadAll(r.Body)
		body, _ := decodeObject(raw)
		if _, ok := body["parameters"]; ok {
			t.Fatal("defaults were not materialized")
		}
		docs := body["docs"].([]any)
		first := docs[0].(map[string]any)
		last := docs[len(docs)-1].(map[string]any)
		if _, ok := first["_index"]; ok {
			t.Error("later parameters changed earlier doc")
		}
		if last["_index"] != "reports-default" {
			t.Error("ids lost final template defaults")
		}
		fmt.Fprint(w, `{"docs":[{"_index":"reports-2026","_id":"a","found":true},{"_index":"reports-2026","_id":"b","found":true}]}`)
		return true
	})
	request := core.ElasticsearchQueryRequest{Operation: "mtermvectors", Indices: []string{"reports-2026"}, Body: json.RawMessage(`{"docs":[{"_id":"a"}],"parameters":{"_index":"reports-default","fields":["title"]},"ids":["b"]}`)}
	if _, err := target.ElasticsearchQuery(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"parameters":{"_index":"private-doc"},"ids":["a"]}`, `{"parameters":{"_index":"private-doc"},"docs":[{"_id":"a"}]}`} {
		request.Body = json.RawMessage(body)
		if _, err := target.ElasticsearchQuery(context.Background(), request); err == nil || !strings.HasPrefix(err.Error(), "scope_denied:") {
			t.Fatal("default index escaped scope", err)
		}
	}
	if executed.Load() != 1 {
		t.Fatal("rejected member reached execution")
	}
}

func TestQueryOptionsCannotHideInferenceOrCreateContexts(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_search") {
			raw, _ := io.ReadAll(r.Body)
			m, _ := decodeObject(raw)
			if m["size"] != json.Number("1") || m["timeout"] != "20ms" {
				t.Errorf("explicit size/timeout changed: %s", raw)
			}
			fmt.Fprint(w, nativeSearchResult)
			return true
		}
		return false
	})
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Options: map[string]json.RawMessage{"size": json.RawMessage(`1`), "timeout": json.RawMessage(`"20ms"`)}}); err != nil {
		t.Fatal(err)
	}
	for _, options := range []map[string]json.RawMessage{{"scroll": json.RawMessage(`"1m"`)}, {"source": json.RawMessage(`"{}"`)}, {"filter_path": json.RawMessage(`"hits"`)}, {"allow_partial_search_results": json.RawMessage(`true`)}} {
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Options: options}); err == nil {
			t.Fatal("control escape admitted")
		}
	}
	f.mu.Lock()
	f.mapping = `{"reports-2026":{"mappings":{"properties":{"meaning":{"type":"semantic_text"}}}}}`
	f.mu.Unlock()
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Options: map[string]json.RawMessage{"q": json.RawMessage(`"meaning:text"`)}}); err == nil {
		t.Fatal("query-string option bypassed mapping proof")
	}
}

func TestQueryProfileRejectsUnprovenPluginsWithoutDisablingMetadata(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/_nodes/plugins" {
			fmt.Fprintf(w, `{"_nodes":{"total":1,"successful":1,"failed":0},"nodes":{"node":{"version":"8.19.21","build_hash":%q,"plugins":[{"name":"custom-script-engine"}]}}}`, target.cfg.Elasticsearch.Version)
			return true
		}
		return false
	})
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search"}); err == nil {
		t.Fatal("unproven plugin admitted")
	}
	// Use the real pinned build in the plugin rejection proof, so the failure is
	// capability-specific and does not mark a matching target unhealthy.
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/_nodes/plugins" {
			fmt.Fprintf(w, `{"_nodes":{"total":1,"successful":1,"failed":0},"nodes":{"node":{"version":"8.19.21","build_hash":%q,"plugins":[{"name":"custom"}]}}}`, config.ElasticsearchBuilds["8.19.21"])
			return true
		}
		return false
	})
	if err := target.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search"}); err == nil || !strings.HasPrefix(err.Error(), "capability_unavailable:") {
		t.Fatal(err)
	}
	if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "mappings"}); err != nil {
		t.Fatal("plugin capability disabled metadata", err)
	}
}

func TestAggregateOnlySearchDetectsAliasInventoryDrift(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_search") {
			f.mu.Lock()
			f.resolution = `{"indices":[],"aliases":[{"name":"reports-alias","indices":["private-data"]}],"data_streams":[]}`
			f.mu.Unlock()
			fmt.Fprint(w, `{"timed_out":false,"_shards":{"failed":0},"hits":{"hits":[]},"aggregations":{"sum":{"value":99}}}`)
			return true
		}
		return false
	})
	result, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Indices: []string{"reports-alias"}, Body: json.RawMessage(`{"size":0,"aggs":{"sum":{"sum":{"field":"amount"}}}}`)})
	if result != nil || err == nil || !strings.HasPrefix(err.Error(), "scope_denied:") {
		t.Fatal("aggregate escaped postflight source proof", err)
	}
}

func TestAggregationNamesCannotBypassHitOrBucketBounds(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	target.cfg.Elasticsearch.MaxAggregationBuckets = 1
	for _, aggs := range []string{
		`{"value":{"buckets":[{"key":"a","doc_count":1},{"key":"b","doc_count":1}]}}`,
		`{"hits":{"hits":{"hits":[{"_index":"private-data","_id":"x"}]}}}`,
		`{"group":{"doc_count":1,"value":{"hits":{"hits":[{"_index":"private-data","_id":"x"}]}}}}`,
	} {
		interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
			if strings.HasSuffix(r.URL.Path, "/_search") {
				fmt.Fprintf(w, `{"timed_out":false,"_shards":{"failed":0},"hits":{"hits":[]},"aggregations":%s}`, aggs)
				return true
			}
			return false
		})
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Body: json.RawMessage(`{"size":0}`)}); err == nil {
			t.Fatal("aggregation name bypassed bounds")
		}
	}
}
