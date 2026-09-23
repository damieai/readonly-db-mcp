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

// Reads an operator-provisioned acceptance index and alias without modifying them.
func TestElasticsearchLiveAliasSettingsMetadata(t *testing.T) {
	path, index, alias := os.Getenv("READONLY_DB_MCP_ES_CONFIG"), os.Getenv("READONLY_DB_MCP_ES_METADATA_INDEX"), os.Getenv("READONLY_DB_MCP_ES_METADATA_ALIAS")
	if path == "" || index == "" || alias == "" {
		t.Skip("live Elasticsearch metadata config, index and alias are not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load Elasticsearch qualification config:", err)
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" || !strings.HasPrefix(index, "mcp-es-acceptance-") || !concreteName(index) || !strings.HasPrefix(alias, "mcp-es-acceptance-") || !concreteName(alias) {
		t.Fatal("select a test target, acceptance index and acceptance alias")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	for _, request := range []core.ElasticsearchMetadataRequest{
		{Operation: "aliases", Indices: []string{index}, Names: []string{alias}},
		{Operation: "settings", Indices: []string{index}, Names: []string{"index.number_of_*"}, Options: map[string]json.RawMessage{"flat_settings": json.RawMessage(`true`), "include_defaults": json.RawMessage(`true`)}},
	} {
		result, err := target.ElasticsearchMetadata(ctx, request)
		if err != nil {
			t.Fatal(request.Operation, err)
		}
		if !strings.Contains(string(result.Data), `"`+index+`"`) {
			t.Fatal("native index metadata omitted the acceptance index")
		}
		if request.Operation == "aliases" && !strings.Contains(string(result.Data), `"`+alias+`"`) {
			t.Fatal("native alias metadata omitted the acceptance alias")
		}
	}
}
