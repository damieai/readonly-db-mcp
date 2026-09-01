package mysql

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
)

// TestMySQLReadOnlyIntegration runs only when a disposable MySQL 8 SELECT-only
// account is supplied. The DSN is never included in failures or logs.
func TestMySQLReadOnlyIntegration(t *testing.T) {
	dsn := os.Getenv("READONLY_DB_MCP_TEST_DSN")
	if dsn == "" {
		t.Skip("READONLY_DB_MCP_TEST_DSN is not configured")
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("open integration database")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	identity, grants, err := inspectIdentity(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Database != cfg.DBName {
		t.Fatal("connected database identity mismatch")
	}
	if err := ValidateGrants(grants, []string{cfg.DBName}); err != nil {
		t.Fatalf("integration account is not SELECT-only: %v", err)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal("begin read-only transaction")
	}
	defer tx.Rollback()
	var one int
	if err := tx.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatal("read-only SELECT failed")
	}
	if _, err := tx.ExecContext(ctx, "CREATE TEMPORARY TABLE readonly_db_mcp_must_fail (id INT)"); err == nil {
		t.Fatal("write unexpectedly succeeded")
	}
}

func TestMySQLCancellationIntegration(t *testing.T) {
	dsn := os.Getenv("READONLY_DB_MCP_TEST_DSN")
	if dsn == "" {
		t.Skip("READONLY_DB_MCP_TEST_DSN is not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("open integration database")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := db.QueryContext(ctx, "SELECT SLEEP(5)"); err == nil {
		t.Fatal("sleep query was not canceled")
	}
	if time.Since(started) > time.Second {
		t.Fatal("driver cancellation was not prompt")
	}
}
