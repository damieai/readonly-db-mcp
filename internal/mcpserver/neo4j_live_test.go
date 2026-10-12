package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/registry"
)

func TestNeo4jDisposableMCPRoundTrip(t *testing.T) {
	path := os.Getenv("NEO4J_DISPOSABLE_CONFIG")
	if path == "" {
		t.Skip("set NEO4J_DISPOSABLE_CONFIG for the disposable Enterprise gate")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatal("Neo4j MCP gate requires exactly one target")
	}
	var alias string
	for alias = range cfg.Targets {
	}
	targetCfg := cfg.Targets[alias]
	if targetCfg.Engine != config.EngineNeo4j || targetCfg.Neo4j == nil || targetCfg.Neo4j.Attestor == nil || targetCfg.Environment != "local" || targetCfg.Host != "127.0.0.1" || targetCfg.Database != "knowledge" || targetCfg.Username != "agent_graph_ro" || targetCfg.Neo4j.Attestor.Username != "agent_graph_attestor" {
		t.Fatal("Neo4j MCP gate requires the local examples/neo4j fixture and dedicated identities")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	targets, err := registry.Open(ctx, cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer targets.Close()
	server := New(targets, slog.Default(), "test")
	a, b := mcp.NewInMemoryTransports()
	ss, err := server.mcp.Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "neo4j-gate", Version: "1"}, nil)
	cs, err := client.Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	call := func(name string, args map[string]any, output any) {
		t.Helper()
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("Neo4j MCP %s failed: %v", name, err)
		}
		if result.IsError {
			t.Fatalf("Neo4j MCP %s failed: err=%v content=%v", name, err, result.Content)
		}
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, output); err != nil {
			t.Fatalf("Neo4j MCP %s output is malformed: %v", name, err)
		}
	}
	var read core.Neo4jReadResult
	call("neo4j_read", map[string]any{
		"target":     alias,
		"cypher":     "CALL { MATCH (d:Neo4jMCPDocument {id: $id})-[:NEO4J_MCP_CITES]->(s) RETURN d.id AS id, s.id AS source_id, elementId(s) AS element_id } RETURN id, source_id, element_id",
		"parameters": map[string]any{"id": "neo4j-fixture-doc"},
		"max_rows":   5,
	}, &read)
	if read.RowCount != 1 || read.Rows[0]["source_id"] != "neo4j-fixture-source" || read.Rows[0]["element_id"] == "" {
		t.Fatalf("Neo4j MCP traversal lost source citation: %+v", read)
	}
	var vector core.Neo4jReadResult
	call("neo4j_read", map[string]any{
		"target":     alias,
		"cypher":     "CYPHER 25 MATCH (d:Neo4jMCPDocument) SEARCH d IN (VECTOR INDEX neo4j_mcp_doc_vector FOR $embedding WHERE d.tenant = $tenant LIMIT 5) SCORE AS score RETURN d.id AS id, score",
		"parameters": map[string]any{"embedding": []any{1.0, 0.0}, "tenant": "public"},
		"max_rows":   5,
	}, &vector)
	if vector.RowCount == 0 {
		t.Fatalf("Neo4j MCP vector SEARCH returned no rows: %+v", vector)
	}
	for _, row := range vector.Rows {
		if row["id"] != "neo4j-fixture-doc" {
			t.Fatalf("Neo4j MCP vector filter leaked another tenant: %+v", vector)
		}
	}
	var truncated core.Neo4jReadResult
	call("neo4j_read", map[string]any{"target": alias, "cypher": "UNWIND [1, 2, 3] AS n RETURN n", "max_rows": 1}, &truncated)
	if truncated.RowCount != 1 || !truncated.Truncated {
		t.Fatalf("Neo4j MCP row cap did not mark truncation: %+v", truncated)
	}
	var metadata core.Neo4jMetadataResult
	call("neo4j_metadata", map[string]any{"target": alias, "kind": "indexes"}, &metadata)
	found := false
	for _, row := range metadata.Rows {
		if row["name"] == "neo4j_mcp_doc_vector" && row["state"] == "ONLINE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Neo4j MCP metadata omitted fixture vector index: %+v", metadata)
	}
}
