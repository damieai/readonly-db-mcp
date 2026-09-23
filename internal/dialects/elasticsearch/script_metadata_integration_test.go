package elasticsearch

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Reads an existing, explicitly allowlisted script without creating one.
func TestElasticsearchLiveStoredScriptMetadata(t *testing.T) {
	path, id := os.Getenv("READONLY_DB_MCP_ES_CONFIG"), os.Getenv("READONLY_DB_MCP_ES_SCRIPT_ID")
	if path == "" || id == "" {
		t.Skip("live Elasticsearch config and stored script ID are not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load Elasticsearch qualification config:", err)
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" {
		t.Fatal("select an environment:test Elasticsearch target")
	}
	if err := validateScriptMetadataRequest(core.ElasticsearchMetadataRequest{Operation: "get_script", Names: []string{id}}, targetCfg.Elasticsearch); err != nil {
		t.Fatal("acceptance script ID must be operator-allowlisted:", err)
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	result, err := target.ElasticsearchMetadata(ctx, core.ElasticsearchMetadataRequest{Operation: "get_script", Names: []string{id}})
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		ID string `json:"_id"`
	}
	if json.Unmarshal(result.Data, &response) != nil || response.ID != id {
		t.Fatal("native stored script response changed identity")
	}
}
