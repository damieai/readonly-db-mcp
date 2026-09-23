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

// Reuses the operator-provisioned two-document EQL fixture; never writes data.
func TestElasticsearchLiveESQL(t *testing.T) {
	path, index := os.Getenv("READONLY_DB_MCP_ES_CONFIG"), os.Getenv("READONLY_DB_MCP_ES_ESQL_INDEX")
	if path == "" || index == "" {
		t.Skip("READONLY_DB_MCP_ES_CONFIG and READONLY_DB_MCP_ES_ESQL_INDEX are not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load ES qualification config")
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" || !strings.HasPrefix(index, "mcp-es-acceptance-") || !concreteName(index) {
		t.Fatal("select a test target and mcp-es-acceptance-* ES|QL fixture")
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
		"FROM " + strconv.Quote(index) + " | STATS n=COUNT(*)",
		"FROM " + strconv.Quote(index) + " | SORT @timestamp | KEEP @timestamp | LIMIT 2",
	} {
		body, _ := json.Marshal(map[string]any{"query": query})
		if _, err := target.ElasticsearchQuery(ctx, core.ElasticsearchQueryRequest{Operation: "esql.query", Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	literal, err := target.ElasticsearchQuery(ctx, core.ElasticsearchQueryRequest{Operation: "esql.query", Body: json.RawMessage(`{"query":"ROW n=?","params":[9007199254740993]}`)})
	if err != nil || !strings.Contains(string(literal.Data), "9007199254740993") {
		t.Fatal("native ES|QL parameter precision failed", err)
	}
}

// Exercises the release-build opt-in on an operator-configured read-only target.
func TestElasticsearchLiveESQLPragmas(t *testing.T) {
	path := os.Getenv("READONLY_DB_MCP_ES_CONFIG")
	if path == "" {
		t.Skip("live Elasticsearch config is not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load ES qualification config:", err)
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" {
		t.Fatal("select an environment:test Elasticsearch target")
	}
	if targetCfg.Elasticsearch.ESQLPragmas == nil {
		t.Skip("test target has no ES|QL pragma resource profile")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if _, err := target.ElasticsearchQuery(ctx, core.ElasticsearchQueryRequest{Operation: "esql.query", Body: json.RawMessage(`{"query":"ROW x=1","pragma":{"node_level_reduction":true}}`)}); err != nil {
		t.Fatal("native ES|QL pragma request failed:", err)
	}
}
