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

// Reads an existing operator-owned acceptance index; no stored-template writes.
func TestElasticsearchLiveTemplates(t *testing.T) {
	path, index := os.Getenv("READONLY_DB_MCP_ES_CONFIG"), os.Getenv("READONLY_DB_MCP_ES_INDEX")
	if path == "" || index == "" {
		t.Skip("live Elasticsearch template config and index are not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load template qualification config")
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" || !strings.HasPrefix(index, "mcp-es-acceptance-") || !concreteName(index) {
		t.Fatal("select a test target and an existing mcp-es-acceptance-* index")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	requests := make([]core.ElasticsearchQueryRequest, 2)
	for i := range requests {
		body, _ := json.Marshal(map[string]any{"source": `{"size":{{size}},"query":{"match_all":{}},"profile":true,"explain":true}`, "params": map[string]any{"size": 1}, "profile": i == 0, "explain": i == 0})
		requests[i] = core.ElasticsearchQueryRequest{Operation: "search_template", Indices: []string{index}, Body: body}
	}
	check := func(data json.RawMessage, enabled bool) {
		t.Helper()
		body, err := decodeObject(data)
		if err != nil {
			t.Fatal(err)
		}
		_, profile := body["profile"]
		if profile != enabled {
			t.Fatal("template profile control did not match native output")
		}
		hits, _ := object(body["hits"])
		rows, _ := hits["hits"].([]any)
		if len(rows) == 0 {
			t.Fatal("template qualification requires a matching document to check explanations")
		}
		for _, row := range rows {
			hit, _ := object(row)
			_, explanation := hit["_explanation"]
			if explanation != enabled {
				t.Fatal("template explain control did not match native output")
			}
		}
	}
	for i, request := range requests {
		out, err := target.ElasticsearchQuery(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		check(out.Data, i == 0)
	}
	batch, err := target.ElasticsearchBatch(ctx, core.ElasticsearchBatchRequest{Requests: requests})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Results) != 2 {
		t.Fatal("native template batch result count differs")
	}
	for i, result := range batch.Results {
		check(result.Data, i == 0)
	}
}
