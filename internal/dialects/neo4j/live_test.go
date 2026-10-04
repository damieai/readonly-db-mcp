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
	if _, err := target.Metadata(ctx, "indexes"); err != nil {
		t.Fatalf("native index metadata failed: %v", err)
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
