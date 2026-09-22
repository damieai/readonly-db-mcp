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
func TestElasticsearchLiveSQL(t *testing.T) {
	path, index := os.Getenv("READONLY_DB_MCP_ES_CONFIG"), os.Getenv("READONLY_DB_MCP_ES_SQL_INDEX")
	if path == "" || index == "" {
		t.Skip("READONLY_DB_MCP_ES_CONFIG and READONLY_DB_MCP_ES_SQL_INDEX are not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load ES qualification config")
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" || !strings.HasPrefix(index, "mcp-es-acceptance-") || !concreteName(index) {
		t.Fatal("select a test target and mcp-es-acceptance-* SQL fixture")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	body, _ := json.Marshal(map[string]any{"query": "SELECT COUNT(*) FROM " + strconv.Quote(index)})
	for _, op := range []string{"sql.query", "sql.translate"} {
		if _, err := target.ElasticsearchQuery(ctx, core.ElasticsearchQueryRequest{Operation: op, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	literal, err := target.ElasticsearchQuery(ctx, core.ElasticsearchQueryRequest{Operation: "sql.query", Body: json.RawMessage(`{"query":"SELECT ?","params":[9007199254740993]}`)})
	if err != nil || !strings.Contains(string(literal.Data), "9007199254740993") {
		t.Fatal("native parameter precision failed", err)
	}
	body, _ = json.Marshal(map[string]any{"query": "SELECT \"@timestamp\" FROM " + strconv.Quote(index) + " ORDER BY \"@timestamp\" LIMIT 2", "fetch_size": 1, "columnar": true})
	page, err := target.ElasticsearchCursor(ctx, core.ElasticsearchCursorRequest{Action: "open", Kind: "sql", Owner: "live-sql", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	defer target.CloseElasticsearchSession(context.Background(), "live-sql")
	rows := 0
	for n := 0; n < 4; n++ {
		var value struct {
			Values [][]json.RawMessage `json:"values"`
		}
		if json.Unmarshal(page.Data, &value) != nil {
			t.Fatal("invalid native columnar result")
		}
		if len(value.Values) > 0 {
			rows += len(value.Values[0])
		}
		if page.Exhausted {
			break
		}
		page, err = target.ElasticsearchCursor(ctx, core.ElasticsearchCursorRequest{Action: "next", Owner: "live-sql", Handle: page.Handle})
		if err != nil {
			t.Fatal(err)
		}
	}
	if rows != 2 || !page.Exhausted || !page.Closed {
		t.Fatal("SQL fixture needs at least two dated documents and complete cursor exhaustion")
	}
}
