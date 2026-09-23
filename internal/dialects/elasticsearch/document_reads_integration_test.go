package elasticsearch

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Reads an operator-provisioned document whose source is enabled.
func TestElasticsearchLiveDocumentReads(t *testing.T) {
	path, index, id := os.Getenv("READONLY_DB_MCP_ES_CONFIG"), os.Getenv("READONLY_DB_MCP_ES_DOCUMENT_INDEX"), os.Getenv("READONLY_DB_MCP_ES_DOCUMENT_ID")
	if path == "" || index == "" || id == "" {
		t.Skip("live Elasticsearch document config, index and ID are not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load Elasticsearch qualification config:", err)
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" || !strings.HasPrefix(index, "mcp-es-acceptance-") || !concreteName(index) || len(id) > 1024 {
		t.Fatal("select a test target and existing acceptance document")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	for _, operation := range []string{"exists", "exists_source", "get_source"} {
		out, err := target.ElasticsearchQuery(ctx, core.ElasticsearchQueryRequest{Operation: operation, Indices: []string{index}, ID: id})
		if err != nil {
			t.Fatal(operation, err)
		}
		if operation == "get_source" {
			if len(out.Data) == 0 || out.Data[0] != '{' {
				t.Fatal("native document source is not an object")
			}
		} else if string(out.Data) != `{"exists":true}` {
			t.Fatal("operator-provisioned document is missing", operation)
		}
	}
}
