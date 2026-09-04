package sqlserver

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/your-org/readonly-db-mcp/internal/config"
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
