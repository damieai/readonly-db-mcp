package elasticsearch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Checks a provisioned key and separate read-only attestor without creating keys.
func TestElasticsearchLiveAPIKeyAttestation(t *testing.T) {
	path := os.Getenv("READONLY_DB_MCP_ES_CONFIG")
	if path == "" {
		t.Skip("live Elasticsearch config is not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load ES API key qualification config:", err)
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" {
		t.Fatal("select an environment:test Elasticsearch target")
	}
	if targetCfg.Elasticsearch.APIKey == nil {
		t.Skip("test target does not use an API key")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal("API key and attestor startup proof failed:", err)
	}
	defer target.Close()
	if _, err := target.ElasticsearchMetadata(ctx, core.ElasticsearchMetadataRequest{Operation: "resolve"}); err != nil {
		t.Fatal("API key scoped read failed:", err)
	}
}
