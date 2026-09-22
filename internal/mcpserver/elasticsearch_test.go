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
			fmt.Fprint(w, `{"cluster":["monitor"],"indices":[{"names":["reports-*"],"privileges":["read","view_index_metadata"],"allow_restricted_indices":false}],"applications":[],"run_as":[],"global":[]}`)
		case "/_resolve/index/reports-*":
			fmt.Fprint(w, `{"indices":[{"name":"reports-2026","attributes":["open"]}],"aliases":[],"data_streams":[]}`)
		case "/reports-*/_mapping":
			fmt.Fprint(w, `{"reports-2026":{"mappings":{"_meta":{"number":9007199254740993}}}}`)
		case "/reports-*/_search":
			fmt.Fprint(w, `{"timed_out":false,"_shards":{"total":1,"failed":0,"successful":1},"hits":{"hits":[{"_index":"reports-2026","_id":"a","_source":{"number":9007199254740993}}]}}`)
		case "/_msearch":
			fmt.Fprint(w, `{"responses":[{"timed_out":false,"_shards":{"total":1,"failed":0,"successful":1},"hits":{"hits":[{"_index":"reports-2026","_id":"a","_source":{"number":9007199254740993}}]}}]}`)
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
