package neo4j

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// This opt-in gate requires a single, successful read lasting at least five
// minutes. The operator supplies a query sized for the pinned server rather
// than tying the fixture to a machine-specific amount of CPU work.
func TestNeo4jDisposableSustainedRead(t *testing.T) {
	cypher := os.Getenv("NEO4J_DISPOSABLE_SUSTAINED_CYPHER")
	if cypher == "" {
		t.Skip("set NEO4J_DISPOSABLE_SUSTAINED_CYPHER for the sustained-read gate")
	}
	path := os.Getenv("NEO4J_DISPOSABLE_CONFIG")
	if path == "" {
		t.Fatal("set NEO4J_DISPOSABLE_CONFIG for the sustained-read gate")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatal("sustained-read gate requires exactly one target")
	}
	var targetCfg *config.TargetConfig
	for _, targetCfg = range cfg.Targets {
	}
	requireDisposableFixture(t, targetCfg)
	if cfg.Limits.MaxTimeout < 6*time.Minute || targetCfg.Connection.ReadTimeout < cfg.Limits.MaxTimeout {
		t.Fatal("sustained-read gate requires at least a six-minute query limit and matching connection read timeout")
	}
	controller := admission.New(admission.Config{Global: 1, PerTarget: 1, MaxQueued: 1, QueueTimeout: cfg.Limits.QueueTimeout})
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Limits.MaxTimeout+time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	started := time.Now()
	result, err := target.Read(ctx, core.Neo4jReadRequest{Cypher: cypher, Timeout: cfg.Limits.MaxTimeout, MaxRows: 1})
	elapsed := time.Since(started)
	if err != nil || result == nil || result.RowCount != 1 || result.Truncated {
		t.Fatalf("sustained read did not complete with one row after %s: result=%+v err=%v", elapsed, result, err)
	}
	if elapsed < 5*time.Minute {
		t.Fatalf("read completed in %s; choose a larger read-only workload to establish five-minute behavior", elapsed)
	}
	t.Logf("single read completed in %s", elapsed)
	if _, err := target.Read(ctx, core.Neo4jReadRequest{Cypher: "RETURN 1 AS value", MaxRows: 1}); err != nil {
		t.Fatalf("connection did not recover after sustained read: %v", err)
	}
}
