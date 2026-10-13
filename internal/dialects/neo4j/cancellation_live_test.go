package neo4j

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// The query deliberately consumes CPU without modifying data. The admin
// observer proves that cancellation reaches a query already running on the
// server, then checks its transaction disappears and the pool remains usable.
func TestNeo4jDisposableCancellationRecovery(t *testing.T) {
	path := os.Getenv("NEO4J_DISPOSABLE_CONFIG")
	if path == "" {
		t.Skip("set NEO4J_DISPOSABLE_CONFIG for the disposable Enterprise gate")
	}
	adminUser := os.Getenv("NEO4J_DISPOSABLE_ADMIN_USER")
	adminPassword := os.Getenv("NEO4J_DISPOSABLE_ADMIN_PASSWORD")
	if adminUser == "" || adminPassword == "" {
		t.Fatal("set NEO4J_DISPOSABLE_ADMIN_USER and NEO4J_DISPOSABLE_ADMIN_PASSWORD for cancellation verification")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatal("cancellation gate requires exactly one target")
	}
	var targetCfg *config.TargetConfig
	for _, targetCfg = range cfg.Targets {
	}
	requireDisposableFixture(t, targetCfg)
	if cfg.Limits.MaxTimeout < 10*time.Second || targetCfg.Connection.MaxOpen < 2 {
		t.Fatal("cancellation gate requires a 10s query allowance and at least two pooled connections")
	}
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout})
	ctx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	adminAuth := driver.BasicAuth(adminUser, adminPassword, "")
	showTransactions := func() ([]map[string]any, error) {
		showCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		session := target.query.NewSession(showCtx, driver.SessionConfig{DatabaseName: "system", AccessMode: driver.AccessModeRead, Auth: &adminAuth})
		defer session.Close(showCtx)
		return readFixed(showCtx, session, "SHOW TRANSACTIONS YIELD username, database, currentQuery RETURN username, database, currentQuery", 1001)
	}
	const marker = "neo4j_mcp_cancel_probe"
	const expensive = "UNWIND range(1, 100000) AS i UNWIND range(1, 100000) AS j RETURN sum(toFloat(i * j)) AS " + marker
	active := func(rows []map[string]any) bool {
		for _, row := range rows {
			query, _ := row["currentQuery"].(string)
			if row["username"] == targetCfg.Username && row["database"] == targetCfg.Database && strings.Contains(query, marker) {
				return true
			}
		}
		return false
	}
	queryCtx, cancelQuery := context.WithCancel(ctx)
	defer cancelQuery()
	done := make(chan error, 1)
	go func() {
		_, err := target.Read(queryCtx, core.Neo4jReadRequest{Cypher: expensive, Timeout: 10 * time.Second, MaxRows: 1})
		done <- err
	}()
	poll := func(wantActive bool, limit time.Duration) {
		t.Helper()
		deadline := time.Now().Add(limit)
		for {
			rows, err := showTransactions()
			if err != nil {
				t.Fatalf("cannot inspect server transactions: %v", err)
			}
			if active(rows) == wantActive {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("server query active=%t, wanted %t", active(rows), wantActive)
			}
			if wantActive {
				select {
				case err := <-done:
					t.Fatalf("expensive query completed before server observation: %v", err)
				default:
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	poll(true, 5*time.Second)
	cancelledAt := time.Now()
	cancelQuery()
	select {
	case err := <-done:
		if err == nil || !strings.HasPrefix(err.Error(), "deadline_exceeded:") {
			t.Fatalf("cancelled active query did not report interruption: %v", err)
		}
		t.Logf("client cancellation returned in %s", time.Since(cancelledAt))
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled query did not return within five seconds")
	}
	poll(false, 5*time.Second)
	t.Logf("server transaction disappeared in %s", time.Since(cancelledAt))
	if _, err := target.Read(ctx, core.Neo4jReadRequest{Cypher: "RETURN 1 AS value", MaxRows: 1}); err != nil {
		t.Fatalf("pool did not recover after cancellation: %v", err)
	}
	if _, err := target.Read(ctx, core.Neo4jReadRequest{Cypher: expensive, Timeout: 100 * time.Millisecond, MaxRows: 1}); err == nil || !strings.HasPrefix(err.Error(), "deadline_exceeded:") {
		t.Fatalf("short query deadline did not interrupt expensive query: %v", err)
	}
	poll(false, 5*time.Second)
	if _, err := target.Read(ctx, core.Neo4jReadRequest{Cypher: "RETURN 2 AS value", MaxRows: 1}); err != nil {
		t.Fatalf("pool did not recover after query deadline: %v", err)
	}
}
