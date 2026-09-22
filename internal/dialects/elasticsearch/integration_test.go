package elasticsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Run only with an operator-provisioned disposable index/account. The test owns
// its randomized probe document. No administrator credentials enter this process.
func TestElasticsearchLiveReadOnlyMetadata(t *testing.T) {
	path := os.Getenv("READONLY_DB_MCP_ES_CONFIG")
	if path == "" {
		t.Skip("READONLY_DB_MCP_ES_CONFIG is not configured")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load ES integration configuration")
	}
	targetCfg := cfg.Targets[os.Getenv("READONLY_DB_MCP_ES_TARGET")]
	index := os.Getenv("READONLY_DB_MCP_ES_WRITE_PROBE_INDEX")
	if targetCfg == nil || targetCfg.Engine != config.EngineElasticsearch || targetCfg.Environment != "test" || !strings.HasPrefix(index, "mcp-es-acceptance-") || !concreteName(index) {
		t.Fatal("select an environment:test ES target and a disposable mcp-es-acceptance-* probe index")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout, MaintenanceMax: cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency, MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved, BatchMax: cfg.Limits.WorkloadClasses.BatchMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	for _, operation := range []string{"resolve", "mappings"} {
		if _, err := target.ElasticsearchMetadata(ctx, core.ElasticsearchMetadataRequest{Operation: operation, Indices: []string{index}}); err != nil {
			t.Fatal(err)
		}
	}
	// Searches only depend on the operator-owned index existing, not on a
	// particular document schema or a writable identity inside this process.
	for _, body := range []string{
		`{"size":0,"query":{"match_all":{}},"aggs":{"n":{"filter":{"match_all":{}}}}}`,
		`{"size":0,"query":{"script_score":{"query":{"match_all":{}},"script":{"source":"1.0"}}}}`,
		`{"size":0,"runtime_mappings":{"mcp_value":{"type":"long","script":{"source":"emit(1)"}}},"aggs":{"n":{"sum":{"field":"mcp_value"}}}}`,
	} {
		if _, err := target.ElasticsearchQuery(ctx, core.ElasticsearchQueryRequest{Operation: "search", Indices: []string{index}, Body: json.RawMessage(body)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := target.ElasticsearchBatch(ctx, core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{{Operation: "search", Indices: []string{index}, Body: json.RawMessage(`{"size":0}`)}, {Operation: "search", Indices: []string{index}, Body: json.RawMessage(`{"size":0}`)}}}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"pit", "scroll"} {
		page, err := target.ElasticsearchCursor(ctx, core.ElasticsearchCursorRequest{Action: "open", Kind: kind, Owner: "live-qualification", OwnerContext: ctx, Indices: []string{index}, Body: json.RawMessage(`{"size":1,"sort":["_doc"]}`)})
		if err != nil {
			t.Fatal(kind, err)
		}
		if page.Handle != "" {
			page, err = target.ElasticsearchCursor(ctx, core.ElasticsearchCursorRequest{Action: "next", Owner: "live-qualification", Handle: page.Handle})
			if err != nil {
				t.Fatal(kind, err)
			}
			if page.Handle != "" {
				if _, err := target.ElasticsearchCursor(ctx, core.ElasticsearchCursorRequest{Action: "close", Owner: "live-qualification", Handle: page.Handle}); err != nil {
					t.Fatal(kind, err)
				}
			}
		}
	}
	if _, err := target.ElasticsearchBatch(ctx, core.ElasticsearchBatchRequest{Consistency: "pit", Requests: []core.ElasticsearchQueryRequest{{Operation: "search", Indices: []string{index}, Body: json.RawMessage(`{"size":0}`)}, {Operation: "search", Indices: []string{index}, Body: json.RawMessage(`{"size":0}`)}}}); err != nil {
		t.Fatal(err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPut, "/"+index+"/_doc/readonly-must-fail-"+uuid.NewString(), strings.NewReader(`{"fixture":"native_write_must_fail"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := target.wire.clients[0].Perform(request)
	if err != nil {
		t.Fatal("native write-denial probe failed at transport")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("native identity must deny document writes with 403, got %d", response.StatusCode)
	}
}
