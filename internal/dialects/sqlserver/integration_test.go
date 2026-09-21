package sqlserver

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// TestSQLServerReadOnlyIntegration runs only against an operator-supplied,
// disposable SQL Server database. The DSN is never logged. Besides exercising
// SHOWPLAN, it verifies that the database itself rejects a transactional DDL
// probe; rollback keeps the probe recoverable if an unsafe account is supplied.
func TestSQLServerReadOnlyIntegration(t *testing.T) {
	dsn := os.Getenv("READONLY_DB_MCP_SQLSERVER_DSN")
	if dsn == "" {
		t.Skip("READONLY_DB_MCP_SQLSERVER_DSN is not configured")
	}
	driverConfig, err := msdsn.Parse(dsn)
	if err != nil {
		t.Fatal("invalid SQL Server integration DSN")
	}
	schema := os.Getenv("READONLY_DB_MCP_SQLSERVER_SCHEMA")
	if schema == "" {
		schema = "reporting"
	}
	requireSnapshot := true
	db := sql.OpenDB(mssql.NewConnectorConfig(driverConfig))
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal("SQL Server integration database is unreachable")
	}
	cfg := &config.TargetConfig{
		Name:           "sqlserver-integration",
		Engine:         config.EngineSQLServer,
		Database:       driverConfig.Database,
		Username:       driverConfig.User,
		AllowedSchemas: []string{schema},
		Connection:     config.ConnectionConfig{WriteTimeout: 3 * time.Second},
		SQLServer:      config.SQLServerConfig{RequireSnapshot: &requireSnapshot},
	}
	if _, err := verifyIdentityAndPrivileges(ctx, db, cfg); err != nil {
		t.Fatalf("integration account failed read-only attestation: %v", err)
	}
	target := &Target{cfg: cfg, db: db, denied: map[string]struct{}{}}
	if _, err := target.showPlan(ctx, `SELECT SUM(CONVERT(int,[value])) OVER () FROM OPENJSON(N'[1,2]')`, nil); err != nil {
		t.Fatalf("advanced read-only SHOWPLAN failed: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("begin SQL Server privilege probe")
	}
	defer tx.Rollback()
	probeName := "readonly_db_mcp_must_fail_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	probe := "CREATE TABLE " + quoteIntegrationIdentifier(schema) + "." + quoteIntegrationIdentifier(probeName) + " ([id] int NULL)"
	if _, err := tx.ExecContext(ctx, probe); err == nil {
		t.Fatal("SQL Server account unexpectedly created a table")
	}
}

func quoteIntegrationIdentifier(value string) string {
	return "[" + strings.ReplaceAll(value, "]", "]]") + "]"
}

// This gate exercises production Open, ScriptDom, module closure, SHOWPLAN,
// execution, snapshot batches and connection recovery with the same config used
// by MCP. Provision tools/sqlserver-parser/acceptance-fixture.sql first, using an
// administrator only in a disposable database. No administrator DSN is read here.
func TestSQLServerConfiguredTargetIntegration(t *testing.T) {
	path := os.Getenv("READONLY_DB_MCP_SQLSERVER_CONFIG")
	if path == "" {
		t.Skip("READONLY_DB_MCP_SQLSERVER_CONFIG is not configured")
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal("load SQL Server integration configuration")
	}
	cfg := loaded.Targets[os.Getenv("READONLY_DB_MCP_SQLSERVER_TARGET")]
	if cfg == nil || cfg.Engine != config.EngineSQLServer || cfg.Environment != "test" {
		t.Fatal("select a disposable SQL Server integration target with environment: test")
	}
	controller := admission.New(admission.Config{Global: loaded.Limits.GlobalConcurrency, PerTarget: loaded.Limits.PerTargetConcurrency, MaxQueued: loaded.Limits.MaxQueuedRequests, QueueTimeout: loaded.Limits.QueueTimeout, BatchMax: loaded.Limits.WorkloadClasses.BatchMaxConcurrency, MaintenanceMax: loaded.Limits.WorkloadClasses.MaintenanceMaxConcurrency})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	target, err := Open(ctx, cfg, loaded.Limits, controller, nil, nil)
	if err != nil {
		t.Fatalf("production target startup: %v", err)
	}
	defer target.Close()
	request := core.QueryRequest{SQL: `WITH c AS (SELECT id,amount,payload FROM reporting.mcp_acceptance_view)
SELECT c.id, SUM(c.amount) OVER () AS total, reporting.mcp_acceptance_scalar(c.amount) AS adjusted,j.value
FROM c CROSS APPLY OPENJSON(c.payload) j WHERE c.id>=@p1 ORDER BY c.id OPTION(RECOMPILE)`, Parameters: []any{1}}
	result, err := target.Query(ctx, request)
	if err != nil || result.RowCount != 2 {
		t.Fatalf("advanced query through curated view and scalar function: error=%v", err)
	}
	if _, err := target.Explain(ctx, request); err != nil {
		t.Fatalf("explain: %v", err)
	}
	batch, err := target.BatchQuery(ctx, core.BatchRequest{Queries: []core.QueryRequest{request, {SQL: `SELECT * FROM reporting.mcp_acceptance_rows(@p1)`, Parameters: []any{1}}}})
	if err != nil || batch.CompletedQueries != 2 {
		t.Fatalf("snapshot batch with table-valued function: error=%v", err)
	}
	for _, query := range []string{
		`DELETE FROM reporting.mcp_acceptance_view`,
		`UPDATE reporting.mcp_acceptance_view SET amount=0`,
		`SELECT * INTO reporting.must_not_create FROM reporting.mcp_acceptance_view`,
		`SELECT * FROM mcp_private.mcp_acceptance_items`,
	} {
		if _, err := target.Query(ctx, core.QueryRequest{SQL: query}); err == nil {
			t.Fatal("write or direct private entry passed MCP policy")
		}
	}
	// Probe the final database boundary without going through application policy.
	for _, query := range []string{
		`UPDATE reporting.mcp_acceptance_view SET amount=0 WHERE id=1`,
		`DELETE FROM reporting.mcp_acceptance_view WHERE id=1`,
	} {
		tx, err := target.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal("begin native denial probe")
		}
		_, writeErr := tx.ExecContext(ctx, query)
		_ = tx.Rollback()
		if writeErr == nil {
			t.Fatal("runtime identity can modify fixture through a view")
		}
		var serverError mssql.Error
		if !errors.As(writeErr, &serverError) || serverError.Number != 229 {
			t.Fatal("native write probe did not return SQL Server permission-denied error 229")
		}
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if _, err := target.Query(cancelled, request); err == nil {
		t.Fatal("cancelled request succeeded")
	}
	after, err := target.Query(ctx, request)
	if err != nil || !reflect.DeepEqual(after.Rows, result.Rows) {
		t.Fatalf("connection recovery: %v", err)
	}
}
