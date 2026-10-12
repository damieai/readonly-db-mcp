package neo4j

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// This opt-in test changes a dedicated fixture role. It must only run against
// the disposable users/databases provisioned by examples/neo4j/provision.cypher.
func TestNeo4jDisposablePrivilegeDrift(t *testing.T) {
	path := os.Getenv("NEO4J_DISPOSABLE_CONFIG")
	if path == "" {
		t.Skip("set NEO4J_DISPOSABLE_CONFIG for the disposable Enterprise gate")
	}
	adminUser := os.Getenv("NEO4J_DISPOSABLE_ADMIN_USER")
	adminPassword := os.Getenv("NEO4J_DISPOSABLE_ADMIN_PASSWORD")
	if adminUser == "" || adminPassword == "" {
		t.Fatal("set NEO4J_DISPOSABLE_ADMIN_USER and NEO4J_DISPOSABLE_ADMIN_PASSWORD for privilege-drift verification")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatal("privilege-drift gate requires exactly one target")
	}
	var targetCfg *config.TargetConfig
	for _, targetCfg = range cfg.Targets {
	}
	requireDisposableFixture(t, targetCfg)
	// The fixture profile keeps the test short while exercising actual expiry.
	targetCfg.Neo4j.PrivilegeRecheck = time.Second
	controller := admission.New(admission.Config{Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency, MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	target, err := Open(ctx, targetCfg, cfg.Limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	adminAuth := driver.BasicAuth(adminUser, adminPassword, "")
	admin := func(query string) ([]map[string]any, error) {
		session := target.query.NewSession(ctx, driver.SessionConfig{DatabaseName: "system", AccessMode: driver.AccessModeWrite, Auth: &adminAuth})
		defer session.Close(ctx)
		return readFixed(ctx, session, query, 10)
	}
	privateRows, err := admin("SHOW DATABASE `private` YIELD name, currentStatus RETURN name, currentStatus")
	if err != nil || len(privateRows) != 1 || privateRows[0]["name"] != "private" || privateRows[0]["currentStatus"] != "online" {
		t.Fatalf("administrator must verify an online private database: rows=%v err=%v", privateRows, err)
	}
	privateSession := target.query.NewSession(ctx, driver.SessionConfig{DatabaseName: "private", AccessMode: driver.AccessModeRead})
	privateResult, privateErr := privateSession.Run(ctx, "MATCH (n) RETURN count(n) AS count", nil)
	if privateErr == nil {
		_, privateErr = privateResult.Consume(ctx)
	}
	_ = privateSession.Close(ctx)
	var neo4jErr *driver.Neo4jError
	if !errors.As(privateErr, &neo4jErr) || (neo4jErr.Code != "Neo.ClientError.Security.Forbidden" && neo4jErr.Code != "Neo.ClientError.Database.DatabaseNotFound") {
		t.Fatalf("query identity did not receive native cross-database denial: %v", privateErr)
	}
	read := func() error {
		_, err := target.Read(ctx, core.Neo4jReadRequest{Cypher: "RETURN 1 AS value", MaxRows: 1})
		return err
	}
	if err := read(); err != nil {
		t.Fatalf("baseline graph read failed: %v", err)
	}
	cleanupAdmin := func(query string) error {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		session := target.query.NewSession(cleanupCtx, driver.SessionConfig{DatabaseName: "system", AccessMode: driver.AccessModeWrite, Auth: &adminAuth})
		defer session.Close(cleanupCtx)
		result, err := session.Run(cleanupCtx, query, nil)
		if err == nil {
			_, err = result.Consume(cleanupCtx)
		}
		return err
	}
	roleRevoked := false
	defer func() {
		if roleRevoked {
			if err := cleanupAdmin("GRANT ROLE agent_graph_reader TO USER agent_graph_ro"); err != nil {
				t.Errorf("disposable fixture read-role restoration failed: %v", err)
			}
		}
	}()
	if _, err := admin("REVOKE ROLE agent_graph_reader FROM USER agent_graph_ro"); err != nil {
		t.Fatalf("cannot revoke fixture read role: %v", err)
	}
	roleRevoked = true
	time.Sleep(1100 * time.Millisecond)
	if err := read(); err == nil || target.Info().Healthy {
		t.Fatalf("revoked read role did not close the proof gate: err=%v info=%+v", err, target.Info())
	}
	if err := cleanupAdmin("GRANT ROLE agent_graph_reader TO USER agent_graph_ro"); err != nil {
		t.Fatalf("disposable fixture read-role restoration failed: %v", err)
	}
	roleRevoked = false
	if err := read(); err != nil {
		t.Fatalf("read role restoration did not recover target: %v", err)
	}
	writeGranted := false
	defer func() {
		if writeGranted {
			if err := cleanupAdmin("REVOKE GRANT WRITE ON GRAPH knowledge FROM agent_graph_reader"); err != nil {
				t.Errorf("disposable fixture write-grant restoration failed: %v", err)
			}
		}
	}()
	if _, err := admin("GRANT WRITE ON GRAPH knowledge TO agent_graph_reader"); err != nil {
		t.Fatalf("cannot inject fixture write grant: %v", err)
	}
	writeGranted = true
	time.Sleep(1100 * time.Millisecond)
	if err := read(); err == nil || target.Info().Healthy {
		t.Fatalf("write grant did not close the proof gate: err=%v info=%+v", err, target.Info())
	}
	if err := cleanupAdmin("REVOKE GRANT WRITE ON GRAPH knowledge FROM agent_graph_reader"); err != nil {
		t.Fatalf("disposable fixture write-grant restoration failed: %v", err)
	}
	writeGranted = false
	if err := read(); err != nil {
		t.Fatalf("write grant removal did not recover target: %v", err)
	}
}
