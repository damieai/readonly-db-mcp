package elasticsearch

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Operators provision a match policy with id/label and execute it separately.
// This test reads an existing snapshot; it never creates or executes a policy.
func TestElasticsearchLiveEnrich(t *testing.T) {
	path, policy := os.Getenv("READONLY_DB_MCP_ES_CONFIG"), os.Getenv("READONLY_DB_MCP_ES_ENRICH_POLICY")
	key, label := os.Getenv("READONLY_DB_MCP_ES_ENRICH_KEY"), os.Getenv("READONLY_DB_MCP_ES_ENRICH_LABEL")
	if path == "" || policy == "" || key == "" || label == "" {
		t.Skip("live ENRICH config, policy, key and expected label are not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load ENRICH qualification config")
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" || !targetCfg.Elasticsearch.EnrichEnabled() || !strings.HasPrefix(policy, "mcp-es-acceptance-") || !validEnrichPolicy(policy) {
		t.Fatal("select an explicitly snapshot-authorized test target and mcp-es-acceptance-* policy")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	for _, mode := range []string{"", "_any:", "_coordinator:", "_remote:"} {
		body, _ := json.Marshal(map[string]any{"query": "ROW id=? | ENRICH " + strconv.Quote(mode+policy) + " ON id WITH label | KEEP label", "params": []any{key}})
		out, err := target.ElasticsearchQuery(ctx, core.ElasticsearchQueryRequest{Operation: "esql.query", Body: body})
		if err != nil {
			t.Fatal(mode, err)
		}
		var result struct {
			Values [][]any `json:"values"`
		}
		if json.Unmarshal(out.Data, &result) != nil || len(result.Values) != 1 || len(result.Values[0]) != 1 || result.Values[0][0] != label {
			t.Fatal("native ENRICH did not return the expected snapshot label")
		}
	}
}
