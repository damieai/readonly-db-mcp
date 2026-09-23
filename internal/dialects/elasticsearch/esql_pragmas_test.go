package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func esqlPragmaFixtureProfile() *config.ElasticsearchESQLPragmaConfig {
	return &config.ElasticsearchESQLPragmaConfig{
		MaxExchangeBufferSize: 32, MaxExchangeConcurrentClients: 4,
		MaxEnrichWorkers: 2, MaxTaskConcurrency: 8, MaxPageSize: 2048,
		MaxConcurrentNodesPerCluster: 4, MaxConcurrentShardsPerNode: 20,
		MaxShardResolutionAttempts: 5, MaxFoldBytes: 64 << 20,
		MaxFoldPercent: 10, MinStatusInterval: 100 * time.Millisecond,
	}
}

func TestESQLPragmasPreserveNativeControlsWithinOperatorCeilings(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		t.Run(version, func(t *testing.T) {
			f := newESFixture(t)
			f.version = version
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Version = version
			cfg.Elasticsearch.ESQLPragmas = esqlPragmaFixtureProfile()
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
				raw, _ := io.ReadAll(r.Body)
				body, err := decodeObject(raw)
				if err != nil || body["accept_pragma_risks"] != true || body["query"] != "ROW x=1" {
					t.Errorf("native ES|QL pragma envelope changed: %s", raw)
				}
				if r.Method != http.MethodPost || r.URL.Query().Get("allow_partial_results") != "false" {
					t.Error("pragma execution profile changed")
				}
				fmt.Fprint(w, esqlRows)
				return true
			})
			pragma := map[string]any{
				"exchange_buffer_size": 16, "exchange_concurrent_clients": 3,
				"enrich_max_workers": 2, "task_concurrency": 4,
				"data_partitioning": "SHARD", "page_size": 1024,
				"status_interval": "250ms", "max_concurrent_nodes_per_cluster": -1,
				"max_concurrent_shards_per_node":        10,
				"unavailable_shard_resolution_attempts": 3,
				"node_level_reduction":                  false, "fold_limit": "32mb",
			}
			if version == "9.1.10" {
				pragma["field_extract_preference"] = "NONE"
			}
			for _, value := range []map[string]any{pragma, {"fold_limit": "5%", "page_size": 0, "unavailable_shard_resolution_attempts": 0}} {
				body := mustJSON(t, map[string]any{"query": "ROW x=1", "pragma": value})
				out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: body})
				if err != nil || out == nil {
					t.Fatal("scoped ES|QL pragma request failed", err)
				}
			}
			if calls.Load() != 2 || controller.Stats().Active != 0 || target.Info().Capabilities["esql_pragmas"] != "implemented_operator_resource_profile" {
				t.Fatal("ES|QL pragma capability, calls or admission changed")
			}
		})
	}
}

func TestESQLPragmasRejectUnprovedResourceSettingsBeforeDispatch(t *testing.T) {
	f := newESFixture(t)
	cfg, limits, controller := fixtureConfig(t, f)
	cfg.Elasticsearch.ESQLPragmas = esqlPragmaFixtureProfile()
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
	for _, pragma := range []map[string]any{
		{"task_concurrency": 9}, {"page_size": 2049},
		{"exchange_buffer_size": -1}, {"exchange_concurrent_clients": 5},
		{"enrich_max_workers": 3}, {"max_concurrent_nodes_per_cluster": 5},
		{"max_concurrent_shards_per_node": 21},
		{"unavailable_shard_resolution_attempts": -1},
		{"fold_limit": "65mb"}, {"fold_limit": "11%"},
		{"status_interval": "50ms"}, {"status_interval": "0ms"},
		{"node_level_reduction": "false"}, {"unknown_native_setting": true},
		{"field_extract_preference": "NONE"},
	} {
		body := mustJSON(t, map[string]any{"query": "ROW x=1", "pragma": pragma})
		if out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: body}); err == nil || out != nil {
			t.Fatal("unproved ES|QL pragma dispatched", pragma)
		}
	}
	if out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "esql.query", Body: json.RawMessage(`{"query":"ROW x=1","pragma":{"page_size":1},"accept_pragma_risks":true}`)}); err == nil || out != nil {
		t.Fatal("caller controlled native risk flag")
	}
	if calls.Load() != 0 || controller.Stats().Active != 0 {
		t.Fatal("rejected pragmas reached ES or leaked admission")
	}
}

func TestESQLPragmasRequireOperatorOptIn(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	if target.Info().Capabilities["esql_pragmas"] != "awaiting_resource_profile" {
		t.Fatal("experimental ES|QL pragmas were advertised without a profile")
	}
	request := core.ElasticsearchQueryRequest{Operation: "esql.query", Body: json.RawMessage(`{"query":"ROW x=1","pragma":{"node_level_reduction":true}}`)}
	if out, err := target.ElasticsearchQuery(context.Background(), request); err == nil || out != nil {
		t.Fatal("experimental pragma ran without an operator profile")
	}
	if controller.Stats().Active != 0 {
		t.Fatal("rejected pragma leaked admission")
	}
}
