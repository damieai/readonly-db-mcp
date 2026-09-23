package mcpserver

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/registry"
)

func TestElasticsearchMetadataEnvelopeRejectsAmbiguity(t *testing.T) {
	for _, raw := range []string{`{"target":"a","target":"b","operation":"resolve"}`, `{"target":"a","operation":"resolve","headers":{"Authorization":"x"}}`, `{"target":"a","operation":"resolve"} {}`} {
		var input ElasticsearchMetadataInput
		if json.Unmarshal([]byte(raw), &input) == nil {
			t.Fatal("ambiguous metadata envelope accepted")
		}
	}
}

func TestElasticsearchThroughRegistryAndMCP(t *testing.T) {
	var cleared, sqlCleared atomic.Int64
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch r.URL.Path {
		case "/":
			fmt.Fprintf(w, `{"cluster_uuid":"abcdefghijklmnopqrstuv","version":{"number":"8.19.21","build_hash":%q,"build_flavor":"default","build_snapshot":false}}`, config.ElasticsearchBuilds["8.19.21"])
		case "/_security/_authenticate":
			fmt.Fprint(w, `{"username":"mcp_reader","enabled":true,"authentication_type":"realm"}`)
		case "/_nodes/plugins":
			fmt.Fprintf(w, `{"_nodes":{"total":1,"successful":1,"failed":0},"nodes":{"node":{"version":"8.19.21","build_hash":%q,"plugins":[]}}}`, config.ElasticsearchBuilds["8.19.21"])
		case "/_security/user/_privileges":
			fmt.Fprint(w, `{"cluster":["monitor","monitor_enrich"],"indices":[{"names":["reports-*"],"privileges":["read","view_index_metadata"],"allow_restricted_indices":false}],"applications":[],"run_as":[],"global":[]}`)
		case "/_cluster/state/metadata,version/.enrich-*":
			fmt.Fprint(w, `{"state_uuid":"state-1","version":1,"metadata":{"cluster_uuid":"abcdefghijklmnopqrstuv","indices":{".enrich-customers-1234":{"state":"open","aliases":[".enrich-customers"],"settings":{"index.uuid":"snapshot-uuid-123456789","index.blocks.write":"true","index.number_of_shards":"1"},"mappings":{"_doc":{"dynamic":"false","_meta":{"enrich_policy_name":"customers","enrich_policy_type":"match","enrich_match_field":"id"},"properties":{"id":{"type":"keyword"},"label":{"type":"keyword","index":false}}}}}}}}`)
		case "/_enrich/policy/customers":
			fmt.Fprint(w, `{"policies":[{"config":{"match":{"name":"customers","indices":["private-source-*"],"match_field":"id","enrich_fields":["label"]}}}]}`)
		case "/_resolve/index/reports-*":
			fmt.Fprint(w, `{"indices":[{"name":"reports-2026","attributes":["open"]}],"aliases":[],"data_streams":[]}`)
		case "/reports-*/_mapping":
			fmt.Fprint(w, `{"reports-2026":{"mappings":{"_meta":{"number":9007199254740993}}}}`)
		case "/reports-*/_search":
			fmt.Fprint(w, `{"timed_out":false,"_shards":{"total":1,"failed":0,"successful":1},"hits":{"hits":[{"_index":"reports-2026","_id":"a","_source":{"number":9007199254740993}}]}}`)
		case "/_render/template":
			fmt.Fprint(w, `{"template_output":{"query":{"match_all":{}},"profile":false,"explain":false}}`)
		case "/_sql":
			var body map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, ok := body["cursor"]; ok {
				fmt.Fprint(w, `{"rows":[[9007199254740993]]}`)
			} else {
				response := `{"columns":[{"name":"n","type":"long"}],"rows":[[9007199254740993]]`
				if _, open := body["columnar"]; open {
					response += `,"cursor":"native-private-sql"`
				}
				fmt.Fprint(w, response+"}")
			}
		case "/_sql/translate":
			fmt.Fprint(w, `{"query":{"term":{"n":9007199254740993}}}`)
		case "/_query":
			fmt.Fprint(w, `{"took":1,"is_partial":false,"columns":[{"name":"n","type":"long"}],"values":[[9007199254740993]]}`)
		case "/_sql/close":
			sqlCleared.Add(1)
			fmt.Fprint(w, `{"succeeded":true}`)
		case "/reports-*/_eql/search":
			fmt.Fprint(w, `{"took":1,"timed_out":false,"is_partial":false,"is_running":false,"hits":{"events":[{"_index":"reports-2026","_id":"a","_source":{"number":9007199254740993}}]}}`)
		case "/reports-*/_pit":
			fmt.Fprint(w, `{"id":"native-private-pit","_shards":{"total":1,"successful":1,"failed":0}}`)
		case "/_search":
			fmt.Fprint(w, `{"pit_id":"native-private-rotated","timed_out":false,"_shards":{"total":1,"successful":1,"failed":0},"hits":{"hits":[{"_index":"reports-2026","_id":"a","sort":[9007199254740993]}]}}`)
		case "/_pit":
			cleared.Add(1)
			fmt.Fprint(w, `{"succeeded":true,"num_freed":1}`)
		case "/_msearch":
			var lines []json.RawMessage
			decoder := json.NewDecoder(r.Body)
			for decoder.More() {
				var line json.RawMessage
				if decoder.Decode(&line) != nil {
					t.Error("invalid batch frame")
					break
				}
				lines = append(lines, line)
			}
			responses := make([]json.RawMessage, len(lines)/2)
			for i := range responses {
				responses[i] = json.RawMessage(`{"timed_out":false,"_shards":{"total":1,"failed":0,"successful":1},"hits":{"hits":[{"_index":"reports-2026","_id":"a","_source":{"number":9007199254740993}}]}}`)
			}
			json.NewEncoder(w).Encode(map[string]any{"responses": responses})
		default:
			t.Errorf("unexpected MCP upstream path %q", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	dir := t.TempDir()
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw})
	if err := os.WriteFile(filepath.Join(dir, "ca.pem"), ca, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCP_ES_TEST_PASSWORD", "fixture-only")
	text := fmt.Sprintf(`server:
  strict_startup: true
limits:
  global_concurrency: 4
  per_target_concurrency: 2
targets:
  es_test:
    engine: elasticsearch
    environment: test
    username: mcp_reader
    password_env: MCP_ES_TEST_PASSWORD
    tls:
      mode: verify-full
      ca_file: ca.pem
    elasticsearch:
      endpoints: [%q]
      version: 8.19.21
      cluster_uuid: abcdefghijklmnopqrstuv
      allowed_indices: [reports-*]
      enrich:
        scope: all_cluster_snapshots
`, upstream.URL)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	targets, err := registry.Open(ctx, cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer targets.Close()
	if _, err := targets.GetSQL("es_test"); err == nil {
		t.Fatal("ES routed through SQL")
	}
	if _, err := targets.GetRedis("es_test"); err == nil {
		t.Fatal("ES routed through Redis")
	}
	server := New(targets, slog.Default(), "test")
	a, b := mcp.NewInMemoryTransports()
	ss, err := server.mcp.Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name == "es_cursor" && (!tool.Annotations.ReadOnlyHint || tool.Annotations.IdempotentHint) {
			t.Fatal("cursor annotations must describe non-idempotent readonly operations")
		}
		if tool.Name == "es_metadata" {
			found = true
			if !tool.Annotations.ReadOnlyHint || !tool.Annotations.IdempotentHint {
				t.Fatal("incorrect metadata annotations")
			}
		}
	}
	if !found {
		t.Fatal("ES tool missing")
	}
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "es_metadata", Arguments: map[string]any{"target": "es_test", "operation": "mappings"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("MCP metadata failure: %#v", result.Content)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "9007199254740993") {
		t.Fatalf("native number precision lost: %s", encoded)
	}
	for _, call := range []struct {
		name string
		args json.RawMessage
	}{
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"search","body":{"query":{"script_score":{"query":{"match_all":{}},"script":{"source":"params.x","params":{"x":9007199254740993}}}}}}`)},
		{"es_batch", json.RawMessage(`{"target":"es_test","requests":[{"operation":"search","body":{"query":{"match_all":{}}}}]}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"search_template","body":{"source":{"query":{"match_all":{}}},"profile":true,"explain":true}}`)},
		{"es_batch", json.RawMessage(`{"target":"es_test","requests":[{"operation":"search_template","body":{"source":{"query":{"match_all":{}}},"profile":true}},{"operation":"search_template","body":{"source":{"query":{"match_all":{}}},"explain":true}}]}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"eql.search","body":{"query":"any where value == 9007199254740993"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"sql.query","body":{"query":"SELECT ? FROM \"reports-*\"","params":[9007199254740993]}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"sql.translate","body":{"query":"SELECT ? FROM \"reports-*\"","params":[9007199254740993]}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"esql.query","body":{"query":"FROM reports-* | EVAL n=? | LIMIT 1","params":[9007199254740993]}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"esql.query","body":{"query":"ROW id=? | ENRICH customers ON id WITH label","params":[9007199254740993]}}`)},
		{"es_batch", json.RawMessage(`{"target":"es_test","requests":[{"operation":"esql.query","body":{"query":"FROM reports-* | ENRICH customers WITH name=label"}},{"operation":"sql.query","body":{"query":"SELECT 1"}}]}`)},
		{"es_batch", json.RawMessage(`{"target":"es_test","requests":[{"operation":"esql.query","body":{"query":"FROM reports-* | STATS n=COUNT(*)"}},{"operation":"sql.query","body":{"query":"SELECT 1"}}]}`)},
		{"es_batch", json.RawMessage(`{"target":"es_test","requests":[{"operation":"sql.query","body":{"query":"SELECT 1 FROM \"reports-*\""}},{"operation":"eql.search","body":{"query":"any where true"}}]}`)},

		{"es_batch", json.RawMessage(`{"target":"es_test","requests":[{"operation":"eql.search","body":{"query":"any where true"}},{"operation":"search","body":{"query":{"match_all":{}}}}]}`)},
	} {
		r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.args})
		if err != nil || r.IsError {
			t.Fatalf("%s: %v %#v", call.name, err, r)
		}
		raw, _ := json.Marshal(r)
		if !strings.Contains(string(raw), "9007199254740993") {
			t.Fatal("query result lost precision")
		}
	}
	for _, call := range []struct {
		name string
		args json.RawMessage
	}{
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"search","body":{"size":1,"size":2}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"search_template","body":{"source":"{}","profile":"true"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"msearch_template","body":{"source":"{}"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"eql.search","body":{"query":"any where true","wait_for_completion_timeout":"1s"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"eql.search","body":{"query":"any where true","query":"any where false"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"eql.search","body":{"query":"any where true; any where false"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"esql.query","body":{"query":"FROM reports-* | LOOKUP JOIN private ON id"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"enrich.execute_policy","body":{"name":"customers"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"enrich.get_policy","body":{"name":"customers"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"esql.query","body":{"query":"ROW x=1","keep_on_completion":true}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"search","headers":{"Authorization":"x"}}`)},
		{"es_query", json.RawMessage(`{"target":"es_test","operation":"search","body":null}`)},
		{"es_batch", json.RawMessage(`{"target":"es_test","requests":[{"operation":"search","target":"different"}]}`)},
		{"es_batch", json.RawMessage(`{"target":"es_test","requests":[{"operation":"search","timeout_ms":1000}]}`)},
		{"es_batch", json.RawMessage(`{"target":"es_test","requests":[{"operation":"search","body":{"query":{"match_all":{}},"query":{"match_none":{}}}}]}`)},
	} {
		r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.args})
		if err == nil && !r.IsError {
			t.Fatalf("ambiguous %s envelope admitted", call.name)
		}
	}
	// In-memory/stdio sessions have an empty SDK ID; distinct connections must
	// still get distinct ownership and disconnect must release their contexts.
	opened, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "es_cursor", Arguments: json.RawMessage(`{"target":"es_test","action":"open","kind":"pit","body":{"sort":["_shard_doc"]}}`)})
	if err != nil || opened.IsError {
		t.Fatal("MCP cursor open", err, opened)
	}
	var cursor struct {
		Handle string `json:"handle"`
	}
	raw, _ := json.Marshal(opened.StructuredContent)
	if json.Unmarshal(raw, &cursor) != nil || cursor.Handle == "" || strings.Contains(string(raw), "native-private") {
		t.Fatal("invalid public handle", string(raw))
	}
	a2, b2 := mcp.NewInMemoryTransports()
	ss2, err := server.mcp.Connect(ctx, a2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss2.Close()
	cs2, err := client.Connect(ctx, b2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs2.Close()
	for _, args := range []any{
		map[string]any{"target": "es_test", "action": "next", "handle": cursor.Handle},
		json.RawMessage(`{"target":"es_test","action":"open","kind":"pit","owner":"forged"}`),
		json.RawMessage(`{"target":"es_test","action":"open","kind":"pit","body":{"size":1,"size":2}}`),
		json.RawMessage(`{"target":"es_test","action":"open","kind":"pit","scroll_id":"_all"}`),
	} {
		r, err := cs2.CallTool(ctx, &mcp.CallToolParams{Name: "es_cursor", Arguments: args})
		if err == nil && !r.IsError {
			t.Fatal("foreign/ambiguous cursor admitted")
		}
	}
	continued, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "es_cursor", Arguments: map[string]any{"target": "es_test", "action": "next", "handle": cursor.Handle}})
	if err != nil || continued.IsError {
		t.Fatal("owner continuation", err, continued)
	}
	sqlOpened, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "es_cursor", Arguments: json.RawMessage(`{"target":"es_test","action":"open","kind":"sql","body":{"query":"SELECT * FROM \"reports-*\"","columnar":false}}`)})
	if err != nil || sqlOpened.IsError {
		t.Fatal("MCP SQL cursor open", err, sqlOpened)
	}
	sqlRaw, _ := json.Marshal(sqlOpened.StructuredContent)
	var sqlHandle struct {
		Handle string `json:"handle"`
	}
	if json.Unmarshal(sqlRaw, &sqlHandle) != nil || sqlHandle.Handle == "" || strings.Contains(string(sqlRaw), "native-private") {
		t.Fatal("SQL cursor ID leaked")
	}
	foreign, err := cs2.CallTool(ctx, &mcp.CallToolParams{Name: "es_cursor", Arguments: map[string]any{"target": "es_test", "action": "next", "handle": sqlHandle.Handle}})
	if err == nil && !foreign.IsError {
		t.Fatal("foreign session advanced SQL cursor")
	}
	// Existing metadata assertions below use the second connection.
	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(time.Second)
	for (cleared.Load() == 0 || sqlCleared.Load() == 0) && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if cleared.Load() != 1 || sqlCleared.Load() != 1 {
		t.Fatal("session disconnect failed to release owned PIT")
	}
	cs = cs2

	for _, args := range []json.RawMessage{
		json.RawMessage(`{"target":"es_test","operation":"resolve","operation":"mappings"}`),
		json.RawMessage(`{"target":"es_test","operation":"resolve","headers":{"Host":"other"}}`),
	} {
		r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "es_metadata", Arguments: args})
		if err == nil && !r.IsError {
			t.Fatal("ambiguous MCP envelope reached handler")
		}
	}
}
