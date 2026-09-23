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

// Reads an operator-provisioned date index. It never creates or changes data.
func TestElasticsearchLiveDateMath(t *testing.T) {
	path, source, index := os.Getenv("READONLY_DB_MCP_ES_CONFIG"), os.Getenv("READONLY_DB_MCP_ES_DATE_SOURCE"), os.Getenv("READONLY_DB_MCP_ES_DATE_INDEX")
	if path == "" || source == "" || index == "" {
		t.Skip("live Elasticsearch date-math config, source and index are not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load Elasticsearch qualification config:", err)
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" || !dateMathSource(source) || !strings.HasPrefix(index, "mcp-es-acceptance-") || !concreteName(index) {
		t.Fatal("select a test target, native date source and existing mcp-es-acceptance-* index")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	resolved, err := target.ElasticsearchMetadata(ctx, core.ElasticsearchMetadataRequest{Operation: "resolve", Indices: []string{source}})
	if err != nil {
		t.Fatal("date source resolution:", err)
	}
	if !strings.Contains(string(resolved.Data), `"`+index+`"`) {
		t.Fatal("date source did not resolve to the acceptance index")
	}
	if _, err := target.ElasticsearchMetadata(ctx, core.ElasticsearchMetadataRequest{Operation: "mappings", Indices: []string{source}}); err != nil {
		t.Fatal("date source mappings:", err)
	}
	for _, request := range []core.ElasticsearchQueryRequest{
		{Operation: "search", Indices: []string{source}, Body: json.RawMessage(`{"size":1,"query":{"match_all":{}}}`)},
		{Operation: "count", Indices: []string{source}, Body: json.RawMessage(`{"query":{"match_all":{}}}`)},
	} {
		if _, err := target.ElasticsearchQuery(ctx, request); err != nil {
			t.Fatal(request.Operation, err)
		}
	}
}
