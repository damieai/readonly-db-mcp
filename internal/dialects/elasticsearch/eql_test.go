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

const eqlEventResponse = `{"took":2,"timed_out":false,"is_partial":false,"is_running":false,"hits":{"total":{"value":1,"relation":"eq"},"events":[{"_index":"reports-2026","_id":"a","_source":{"value":9007199254740993,"script":"delete","index":"private-data"}}]}}`
const eqlSequenceResponse = `{"took":2,"timed_out":false,"is_partial":false,"is_running":false,"hits":{"total":{"value":1,"relation":"eq"},"sequences":[{"join_keys":[9007199254740993,"private-index"],"events":[{"_index":"reports-2026","_id":"a","_source":{"value":9007199254740993}},{"_index":"","_id":"","_source":{},"missing":true},{"_index":"reports-2026","_id":"b","_source":{}}]}]}}`
const eqlGroupedResponse = `{"took":2,"timed_out":false,"is_partial":false,"is_running":false,"hits":{"total":{"value":1,"relation":"eq"},"sequences":[{"join_keys":[9007199254740993],"events":[{"_index":"reports-2026","_id":"a","_source":{}},{"_index":"reports-2026","_id":"b","_source":{}}]}]}}`

func TestEQLNativeAdvancedQueriesAndParameters(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		t.Run(version, func(t *testing.T) {
			f := newESFixture(t)
			f.version = version
			cfg, limits, admission := fixtureConfig(t, f)
			cfg.Elasticsearch.Version = version
			target, err := Open(context.Background(), cfg, limits, admission, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			var executions atomic.Int64
			var expected string
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if !strings.HasSuffix(r.URL.Path, "/_eql/search") {
					return false
				}
				executions.Add(1)
				if r.Method != "POST" || r.URL.Path != "/reports-alias/_eql/search" {
					t.Error("native alias/method changed")
				}
				raw, _ := io.ReadAll(r.Body)
				body, err := decodeObject(raw)
				if err != nil || body["query"] != expected {
					t.Errorf("EQL query was rewritten: %s", raw)
				}
				if _, ok := body["timeout"]; ok {
					t.Error("DSL timeout injected into EQL")
				}
				if _, ok := body["wait_for_completion_timeout"]; ok {
					t.Error("synchronous EQL converted to asynchronous")
				}
				if r.URL.Query().Get("allow_partial_search_results") != "false" || r.URL.Query().Get("allow_partial_sequence_results") != "false" {
					t.Error("incomplete native results allowed")
				}
				if r.URL.Query().Get("expand_wildcards") != "open,hidden" || r.URL.Query().Get("ignore_unavailable") != "false" {
					t.Error("native index options changed")
				}
				if !strings.Contains(string(raw), "9007199254740993") {
					t.Error("runtime parameter precision lost")
				}
				switch {
				case strings.HasPrefix(expected, "sequence"):
					fmt.Fprint(w, eqlSequenceResponse)
				case strings.HasPrefix(expected, "sample"), strings.HasPrefix(expected, "join"):
					fmt.Fprint(w, eqlGroupedResponse)
				default:
					fmt.Fprint(w, eqlEventResponse)
				}
				return true
			})
			for _, query := range []string{
				`process where process.name : ("cmd*","powershell*") and ?user.id != null | head 3`,
				`sequence by host.id with maxspan=5m [process where true] ![file where file.name : "*.tmp"] [network where destination.port in (80,443)] until [any where event.type == "termination"]`,
				`sample by host.id [process where true] [network where true]`,
				`join by host.id [process where true] [file where true]`,
				`"private-index" where name == "DELETE FROM private-*" /* mutation-looking literal */`,
			} {
				expected = query
				body := map[string]any{"query": query, "size": 3, "fetch_size": 2000, "result_position": "head", "max_samples_per_key": 2, "timestamp_field": "@timestamp", "tiebreaker_field": "event.sequence", "event_category_field": "event.category", "fields": []any{"event.*", map[string]any{"field": "@timestamp", "format": "epoch_millis"}}, "filter": map[string]any{"terms": map[string]any{"tenant": map[string]any{"index": "reports-lookup", "id": "tenants", "path": "values"}}}, "runtime_mappings": map[string]any{"n": map[string]any{"type": "long", "script": map[string]any{"source": "emit(params.n)", "params": map[string]any{"n": json.Number("9007199254740993")}}}}}
				result, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "eql.search", Indices: []string{"reports-alias"}, Body: mustJSON(t, body), Options: map[string]json.RawMessage{"expand_wildcards": json.RawMessage(`["open","hidden"]`), "ignore_unavailable": json.RawMessage(`false`)}})
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(result.Data), "9007199254740993") || result.Partial || result.Truncated || result.Operation != "eql.search" {
					t.Fatal("native EQL result changed")
				}
			}
			if executions.Load() != 5 || admission.Stats().Active != 0 {
				t.Fatal("EQL replayed or leaked admission")
			}
		})
	}
}

func TestEQLRejectsAsyncEffectsScopeEscapesAndAmbiguity(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var dispatched atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_eql/search") {
			dispatched.Add(1)
			fmt.Fprint(w, eqlEventResponse)
			return true
		}
		return false
	})
	for _, test := range []struct{ body, code string }{
		{`{"query":"any where true; DELETE private"}`, "invalid_request"},
		{`{"query":"any where true","query":"any where false"}`, "invalid_response"},
		{`{"query":"any where true","wait_for_completion_timeout":"0ms"}`, "capability_unavailable"},
		{`{"query":"any where true","wait_for_completion_timeout":"1m","keep_on_completion":false}`, "capability_unavailable"},
		{`{"query":"any where true","keep_on_completion":true}`, "mutation_forbidden"},
		{`{"query":"any where true","allow_partial_search_results":true}`, "capability_unavailable"},
		{`{"query":"any where true","allow_partial_sequence_results":true}`, "capability_unavailable"},
		{`{"query":"any where true","filter":{"terms":{"tenant":{"index":"private","id":"x","path":"ids"}}}}`, "scope_denied"},
		{`{"query":"any where true","runtime_mappings":{"x":{"type":"lookup","target_index":"private","input_field":"id","target_field":"id","fetch_fields":["name"]}}}`, "scope_denied"},
		{`{"query":"any where true","runtime_mappings":{"x":{"type":"long","script":{"lang":"custom","source":"write()"}}}}`, "capability_unavailable"},
		{`{"query":"any where true","size":101}`, "resource_limit"},
		{`{"query":"any where true","fetch_size":100001}`, "resource_limit"},
		{`{"query":"any where true","max_samples_per_key":101}`, "resource_limit"},
		{`{"query":"any where true","cursor":"foreign"}`, "capability_unavailable"},
	} {
		_, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "eql.search", Body: json.RawMessage(test.body)})
		if err == nil || !strings.HasPrefix(err.Error(), test.code+":") {
			t.Fatalf("%s: %v", test.body, err)
		}
	}
	for _, options := range []map[string]json.RawMessage{
		{"wait_for_completion_timeout": json.RawMessage(`"-0ms"`)},
		{"keep_on_completion": json.RawMessage(`true`)},
		{"allow_partial_sequence_results": json.RawMessage(`true`)},
		{"source": json.RawMessage(`"{}"`)},
	} {
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "eql.search", Body: json.RawMessage(`{"query":"any where true"}`), Options: options}); err == nil {
			t.Fatal("unsafe option admitted")
		}
	}
	if dispatched.Load() != 0 {
		t.Fatal("rejected EQL reached execution")
	}
	// -1 is the pinned engine's explicit synchronous sentinel, not a positive
	// async wait. Both documented parameter locations preserve that value.
	for _, request := range []core.ElasticsearchQueryRequest{
		{Operation: "eql.search", Body: json.RawMessage(`{"query":"any where true","wait_for_completion_timeout":"-1","keep_on_completion":false}`)},
		{Operation: "eql.search", Body: json.RawMessage(`{"query":"any where true"}`), Options: map[string]json.RawMessage{"wait_for_completion_timeout": json.RawMessage(`"-1"`)}},
	} {
		if _, err := target.ElasticsearchQuery(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEQLResponseScopeCompletenessAndWholeSequences(t *testing.T) {
	for _, test := range []struct {
		name, body, response, code string
		rows                       int
	}{
		{"sequence", `{"query":"sequence with maxspan=1m [any where true] ![any where true] [any where true]","size":1}`, eqlSequenceResponse, "", 3},
		{"oversized sequence", `{"query":"sequence [any where true] [any where true]","size":1}`, eqlGroupedResponse, "resource_limit", 1},
		{"foreign event", `{"query":"any where true"}`, strings.Replace(eqlEventResponse, "reports-2026", "private", 1), "scope_denied", 100},
		{"forged missing", `{"query":"sequence [any where true] ![any where true]"}`, strings.Replace(eqlSequenceResponse, `"_index":"","_id":"","_source":{},"missing":true`, `"_index":"private","_id":"x","_source":{"secret":1},"missing":true`, 1), "scope_denied", 100},
		{"partial", `{"query":"any where true"}`, strings.Replace(eqlEventResponse, `"is_partial":false`, `"is_partial":true`, 1), "partial_result", 100},
		{"timeout", `{"query":"any where true"}`, strings.Replace(eqlEventResponse, `"timed_out":false`, `"timed_out":true`, 1), "partial_result", 100},
		{"shard failure", `{"query":"any where true"}`, strings.Replace(eqlEventResponse, `"took":2`, `"took":2,"shard_failures":[{"reason":"failed"}]`, 1), "partial_result", 100},
		{"missing completion flag", `{"query":"any where true"}`, strings.Replace(eqlEventResponse, `"is_partial":false,`, "", 1), "invalid_response", 100},
		{"unexpected async id", `{"query":"any where true"}`, strings.Replace(eqlEventResponse, `"took":2`, `"took":2,"id":"never-return-this-id"`, 1), "profile_mismatch", 100},
		{"unexpected running work", `{"query":"any where true"}`, strings.Replace(eqlEventResponse, `"is_running":false`, `"is_running":true`, 1), "profile_mismatch", 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newESFixture(t)
			target, _ := openFixture(t, f)
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if strings.HasSuffix(r.URL.Path, "/_eql/search") {
					fmt.Fprint(w, test.response)
					return true
				}
				return false
			})
			result, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "eql.search", Body: json.RawMessage(test.body), MaxRows: test.rows})
			if test.code == "" {
				if err != nil || string(result.Data) != test.response {
					t.Fatal("native sequence shape changed", err)
				}
				return
			}
			if result != nil || err == nil || !strings.HasPrefix(err.Error(), test.code+":") || strings.Contains(err.Error(), "never-return-this-id") {
				t.Fatal("unsafe or partial result accepted", err)
			}
			if test.code == "profile_mismatch" && target.Info().Healthy {
				t.Fatal("unexpected asynchronous engine behavior not quarantined")
			}
		})
	}
}

func TestEQLBatchPreflightAndCancellation(t *testing.T) {
	f := newESFixture(t)
	target, admission := openFixture(t, f)
	var dispatched atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_eql/search") {
			dispatched.Add(1)
			fmt.Fprint(w, eqlEventResponse)
			return true
		}
		return false
	})
	member := core.ElasticsearchQueryRequest{Operation: "eql.search", Body: json.RawMessage(`{"query":"any where true"}`)}
	result, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{member, member}})
	if err != nil || result.Consistency != "independent" || len(result.Results) != 2 {
		t.Fatal(err)
	}
	bad := member
	bad.Body = json.RawMessage(`{"query":"any where true","wait_for_completion_timeout":"1s"}`)
	if _, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{member, bad}}); err == nil {
		t.Fatal("batch async member admitted")
	}
	if _, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Consistency: "pit", Requests: []core.ElasticsearchQueryRequest{member}}); err == nil {
		t.Fatal("EQL falsely labeled shared PIT")
	}
	if dispatched.Load() != 2 {
		t.Fatal("batch did not preflight before execution")
	}
	canceled := make(chan struct{})
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/_eql/search") {
			_, _ = io.Copy(io.Discard, r.Body)
			<-r.Context().Done()
			close(canceled)
			return true
		}
		return false
	})
	member.Timeout = 80 * time.Millisecond
	started := time.Now()
	if _, err := target.ElasticsearchQuery(context.Background(), member); err == nil {
		t.Fatal("canceled EQL returned success")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("HTTP cancellation did not reach EQL handler")
	}
	if time.Since(started) > time.Second || admission.Stats().Active != 0 {
		t.Fatal("EQL cancellation exceeded budget or leaked admission")
	}
}
