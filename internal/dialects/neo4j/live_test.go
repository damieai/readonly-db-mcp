package neo4j

import (
	"context"
	"os"
	"testing"
	"time"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// This gate is opt-in because it requires a disposable Neo4j Enterprise server
// with two independently provisioned native users. The config must contain
// exactly one target; a failed native denial can create a test node.
func TestNeo4jDisposableEnterpriseGate(t *testing.T) {
	path := os.Getenv("NEO4J_DISPOSABLE_CONFIG")
	if path == "" {
		t.Skip("set NEO4J_DISPOSABLE_CONFIG for the disposable Enterprise gate")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatal("disposable gate requires exactly one target")
	}
	var targetCfg *config.TargetConfig
	for _, targetCfg = range cfg.Targets {
	}
	if targetCfg.Engine != config.EngineNeo4j {
		t.Fatal("disposable gate requires a Neo4j target")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	read, err := target.Read(ctx, core.Neo4jReadRequest{Cypher: "RETURN $value AS value", Parameters: map[string]any{"value": int64(7)}, MaxRows: 1})
	if err != nil || read.RowCount != 1 || read.Rows[0]["value"] != int64(7) {
		t.Fatalf("bounded parameterized read failed: result=%+v err=%v", read, err)
	}
	indexes, err := target.Metadata(ctx, "indexes")
	if err != nil {
		t.Fatalf("native index metadata failed: %v", err)
	}
	online := map[string]bool{}
	for _, row := range indexes.Rows {
		if name, ok := row["name"].(string); ok && row["state"] == "ONLINE" {
			online[name] = true
		}
	}
	if !online["neo4j_mcp_doc_vector"] || !online["neo4j_mcp_doc_text"] {
		t.Fatal("apply examples/neo4j/fixture.cypher and wait for both indexes to become ONLINE")
	}
	traversal, err := target.Read(ctx, core.Neo4jReadRequest{
		Cypher:     "MATCH (d:Neo4jMCPDocument {id: $id})-[:NEO4J_MCP_CITES]->(s:Neo4jMCPSource) RETURN d.id AS id, s.id AS source_id, elementId(s) AS element_id",
		Parameters: map[string]any{"id": "neo4j-fixture-doc"}, MaxRows: 5,
	})
	if err != nil || traversal.RowCount != 1 || traversal.Rows[0]["source_id"] != "neo4j-fixture-source" || traversal.Rows[0]["element_id"] == "" {
		t.Fatalf("cited traversal failed: result=%+v err=%v", traversal, err)
	}
	vector, err := target.Read(ctx, core.Neo4jReadRequest{
		Cypher:     "CYPHER 25 MATCH (d:Neo4jMCPDocument) SEARCH d IN (VECTOR INDEX neo4j_mcp_doc_vector FOR $embedding WHERE d.tenant = $tenant LIMIT 5) SCORE AS score RETURN d.id AS id, score",
		Parameters: map[string]any{"embedding": []any{1.0, 0.0}, "tenant": "public"}, MaxRows: 5,
	})
	if err != nil || !hasNeo4jFixtureID(vector, "neo4j-fixture-doc") || hasNeo4jFixtureID(vector, "neo4j-fixture-private") {
		t.Fatalf("filtered vector SEARCH failed: result=%+v err=%v", vector, err)
	}
	if targetCfg.Neo4j.Version >= "2026.09.0" {
		fullText, err := target.Read(ctx, core.Neo4jReadRequest{
			Cypher:     "CYPHER 25 MATCH (d:Neo4jMCPDocument) SEARCH d IN (FULLTEXT INDEX neo4j_mcp_doc_text FOR $term LIMIT 5) SCORE AS score RETURN d.id AS id, score",
			Parameters: map[string]any{"term": "cited"}, MaxRows: 5,
		})
		if err != nil || !hasNeo4jFixtureID(fullText, "neo4j-fixture-doc") {
			t.Fatalf("full-text SEARCH failed: result=%+v err=%v", fullText, err)
		}
	}
	if _, err := target.Read(ctx, core.Neo4jReadRequest{Cypher: "CREATE (:Neo4jMCPWriteProbe)"}); err == nil {
		t.Fatal("connector admitted a write")
	}
	// Driver read mode is routing, not an authorization boundary. Exercise the
	// query account directly to prove that server-side grants reject mutation.
	session := target.query.NewSession(ctx, driver.SessionConfig{DatabaseName: targetCfg.Database, AccessMode: driver.AccessModeWrite})
	defer session.Close(ctx)
	result, err := session.Run(ctx, "CREATE (:Neo4jMCPWriteProbe)", nil)
	if err == nil {
		_, err = result.Consume(ctx)
	}
	if err == nil {
		t.Fatal("Neo4j query account has native write authority")
	}
}

func hasNeo4jFixtureID(result *core.Neo4jReadResult, id string) bool {
	if result == nil {
		return false
	}
	for _, row := range result.Rows {
		if row["id"] == id {
			return true
		}
	}
	return false
}
