package elasticsearch

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Operators seed the two-event fixture described in the EQL qualification
// record. Runtime tests use only the dedicated read identity; no fixture writes.
func TestElasticsearchLiveEQL(t *testing.T) {
	path, index := os.Getenv("READONLY_DB_MCP_ES_CONFIG"), os.Getenv("READONLY_DB_MCP_ES_EQL_INDEX")
	if path == "" || index == "" {
		t.Skip("READONLY_DB_MCP_ES_CONFIG and READONLY_DB_MCP_ES_EQL_INDEX are not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load ES qualification config")
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" || !strings.HasPrefix(index, "mcp-es-acceptance-") || !concreteName(index) {
		t.Fatal("select a test ES target and disposable mcp-es-acceptance-* EQL fixture")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	for _, query := range []string{
		`any where true | head 1`,
		`sequence by host.id with maxspan=1m [process where event.type == "start"] [network where true]`,
		`sequence by host.id with maxspan=1m [process where event.type == "start"] ![file where true] [network where true]`,
		`sample by host.id [process where event.type == "start"] [network where true]`,
	} {
		body, _ := json.Marshal(map[string]any{"query": query, "size": 1})
		result, err := target.ElasticsearchQuery(ctx, core.ElasticsearchQueryRequest{Operation: "eql.search", Indices: []string{index}, Body: body})
		if err != nil {
			t.Fatal(err)
		}
		type event struct {
			Missing bool `json:"missing"`
		}
		var response struct {
			Hits struct {
				Events    []event `json:"events"`
				Sequences []struct {
					Events []event `json:"events"`
				} `json:"sequences"`
			} `json:"hits"`
		}
		if json.Unmarshal(result.Data, &response) != nil || len(response.Hits.Events)+len(response.Hits.Sequences) == 0 {
			t.Fatal("provisioned EQL fixture did not produce the expected event/sequence")
		}
		missing := false
		for _, sequence := range response.Hits.Sequences {
			for _, event := range sequence.Events {
				missing = missing || event.Missing
			}
		}
		if strings.Contains(query, "![") && !missing {
			t.Fatal("native missing-event placeholder was lost")
		}
	}
}
