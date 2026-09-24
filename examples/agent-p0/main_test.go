package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/your-org/readonly-db-mcp/internal/dialects/postgresql"
)

type fixtureCaller struct {
	missingES bool
	accepted  bool
}

func (f fixtureCaller) CallTool(ctx context.Context, request *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	args, _ := json.Marshal(request.Arguments)
	if request.Name == "query_select" && strings.Contains(string(args), "generate_series") {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if strings.Contains(string(args), "DELETE FROM") || strings.Contains(string(args), `"command":"DEL"`) || strings.Contains(string(args), `"operation":"index"`) {
		return &mcp.CallToolResult{IsError: !f.accepted}, nil
	}
	var payload any
	switch request.Name {
	case "list_targets":
		payload = map[string]any{"targets": []any{
			map[string]any{"name": "agent-pg", "engine": "postgresql", "server_version": "16.15", "healthy": true, "read_only_user": true},
			map[string]any{"name": "agent-redis", "engine": "redis", "server_version": "7.4.0", "healthy": true, "read_only_user": true},
			map[string]any{"name": "agent-es", "engine": "elasticsearch", "server_version": "8.19.21", "healthy": true, "read_only_user": true},
		}}
	case "schema_list_tables":
		payload = map[string]any{"tables": []any{map[string]any{"schema": "reporting", "name": "incidents"}}}
	case "schema_describe_table":
		payload = map[string]any{"columns": []any{map[string]any{"name": "id"}, map[string]any{"name": "service"}, map[string]any{"name": "summary"}, map[string]any{"name": "severity"}}}
	case "query_select":
		if strings.Contains(string(args), "incident_embeddings") {
			payload = map[string]any{"rows": []any{map[string]any{"incident_id": 101, "distance": 0.0}}}
		} else {
			payload = map[string]any{"rows": []any{map[string]any{"id": 101, "service": "checkout", "summary": "Checkout timeout", "severity": "high"}}}
		}
	case "redis_command":
		payload = map[string]any{"value": []any{1, "tenant:agent:incident:101", []any{"distance", "0"}}}
	case "query_explain":
		payload = map[string]any{"rows": []any{map[string]any{"plan": "fixture"}}}
	case "es_query":
		id := 101
		if f.missingES {
			id = 999
		}
		payload = map[string]any{"data": map[string]any{"hits": map[string]any{"hits": []any{map[string]any{"_index": "agent-incidents", "_id": "101", "_source": map[string]any{"incident_id": id, "summary": "Checkout timeout during payment authorization"}}}}}}
	default:
		return nil, errors.New("unexpected tool")
	}
	return &mcp.CallToolResult{StructuredContent: payload}, nil
}

func TestRunGroundsBusinessFactAcrossThreeTargets(t *testing.T) {
	o := options{postgres: "agent-pg", redis: "agent-redis", elastic: "agent-es", index: "agent-incidents", redisIndex: "search-agent-incidents", incidentID: 101}
	got, err := run(context.Background(), fixtureCaller{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 3 || len(got.WriteDenial) != 3 || got.Limits["max_rows"] != 3 || !got.CancelRecovery {
		t.Fatalf("incomplete P0 evidence: %+v", got)
	}
	if got.ServerVersions["agent-es"] != "8.19.21" {
		t.Fatal("server version omitted")
	}
	if got.Sources[0].Score == nil || *got.Sources[0].Score != 0 || got.Sources[1].Score == nil || *got.Sources[1].Score != 0 {
		t.Fatal("zero vector distance was omitted")
	}
}

func TestRunRejectsUnrelatedOrWritableResults(t *testing.T) {
	o := options{postgres: "agent-pg", redis: "agent-redis", elastic: "agent-es", index: "agent-incidents", redisIndex: "search-agent-incidents", incidentID: 101}
	if _, err := run(context.Background(), fixtureCaller{missingES: true}, o); err == nil || !strings.Contains(err.Error(), "did not ground") {
		t.Fatalf("unrelated ES hit passed: %v", err)
	}
	if _, err := run(context.Background(), fixtureCaller{accepted: true}, o); err == nil || !strings.Contains(err.Error(), "accepted a write") {
		t.Fatalf("write acceptance passed: %v", err)
	}
}

func TestCancellationProbePassesPostgreSQLReadPolicy(t *testing.T) {
	policy := postgresql.NewPolicy([]string{"reporting"}, nil, 1<<20)
	if _, err := policy.Validate("SELECT COUNT(*) FROM reporting.incidents CROSS JOIN pg_catalog.generate_series(1,1000000000) AS g", 0); err != nil {
		t.Fatalf("cancellation probe is not a valid read: %v", err)
	}
}
