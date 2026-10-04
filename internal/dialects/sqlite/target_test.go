package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func fixture(t *testing.T) (*Target, *sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.db")
	writer, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE docs (id INTEGER PRIMARY KEY, title TEXT, value INTEGER)`,
		`CREATE INDEX docs_title ON docs(title)`,
		`INSERT INTO docs(title,value) VALUES ('alpha',9007199254740992),('beta',2)`,
		`CREATE VIEW doc_titles AS SELECT title FROM docs`,
		`CREATE VIRTUAL TABLE docs_fts USING fts4(title)`,
		`INSERT INTO docs_fts(title) VALUES ('agent retrieval'),('database connector')`,
	} {
		if _, err := writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.TargetConfig{Name: "local", Engine: config.EngineSQLite, Environment: "test", Consistency: config.ConsistencyCurrent, SQLite: &config.SQLiteConfig{Root: dir, File: path}, Connection: config.ConnectionConfig{MaxOpen: 2, MaxIdle: 1, ConnectTimeout: time.Second}}
	limits := config.Limits{DefaultTimeout: time.Second, MaxTimeout: 2 * time.Second, MaxRows: 10, MaxResultBytes: 1 << 20, MaxCellBytes: 1 << 16, MaxSQLBytes: 1 << 16, MaxBatchQueries: 10, MaxParameters: 50, MaxParameterBytes: 1 << 20, MaxParameterValueBytes: 1 << 16}
	controller := admission.New(admission.Config{Global: 2, PerTarget: 2, BatchMax: 1, MaxQueued: 10, QueueTimeout: time.Second})
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.Close(); writer.Close() })
	return target, writer, path
}

func TestSQLiteReadAndMetadata(t *testing.T) {
	target, _, _ := fixture(t)
	ctx := context.Background()
	queries := []string{
		`SELECT title FROM docs WHERE id=?`,
		`WITH ranked AS (SELECT title,row_number() OVER (ORDER BY id) AS rank FROM docs) SELECT * FROM ranked`,
		`SELECT json_extract('{"a":3}', '$.a') AS value`,
		`SELECT title FROM doc_titles`,
		`SELECT title FROM docs_fts WHERE docs_fts MATCH 'agent'`,
		`SELECT a.title,b.title FROM docs a JOIN docs b ON a.id=b.id WHERE a.id=1`,
	}
	for _, query := range queries {
		params := []any{}
		if strings.Contains(query, "?") {
			params = []any{1}
		}
		if _, err := target.Query(ctx, core.QueryRequest{SQL: query, Parameters: params}); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	result, err := target.Query(ctx, core.QueryRequest{SQL: "SELECT value FROM docs WHERE id=1"})
	if err != nil || result.Rows[0]["value"] != "9007199254740992" {
		t.Fatalf("large integer lost: %#v, %v", result, err)
	}
	joined, err := target.Query(ctx, core.QueryRequest{SQL: "SELECT a.title,b.title FROM docs a JOIN docs b ON a.id=b.id WHERE a.id=1"})
	if err != nil || joined.Rows[0]["title"] != "alpha" || joined.Rows[0]["title_2"] != "alpha" {
		t.Fatalf("duplicate columns lost: %#v, %v", joined, err)
	}
	tables, err := target.ListTables(ctx, "", false)
	if err != nil || len(tables) < 3 {
		t.Fatalf("tables: %#v, %v", tables, err)
	}
	desc, err := target.DescribeTable(ctx, "main", "docs", false)
	if err != nil || len(desc.Columns) != 3 || len(desc.Indexes) == 0 {
		t.Fatalf("description: %#v, %v", desc, err)
	}
	if _, err := target.Explain(ctx, core.QueryRequest{SQL: "SELECT title FROM docs WHERE id=?", Parameters: []any{1}}); err != nil {
		t.Fatalf("explain: %v", err)
	}
	limited, err := target.Query(ctx, core.QueryRequest{SQL: "SELECT * FROM docs ORDER BY id", MaxRows: 1})
	if err != nil || !limited.Truncated || limited.RowCount != 1 {
		t.Fatalf("row cap: %#v, %v", limited, err)
	}
}

func TestSQLiteRejectsMutationsAndEscape(t *testing.T) {
	target, writer, path := fixture(t)
	if _, err := target.db.Exec("UPDATE docs SET title='changed'"); err == nil {
		t.Fatal("native connection allowed UPDATE")
	}
	if _, err := target.db.Exec("ATTACH DATABASE ':memory:' AS other"); err == nil {
		t.Fatal("native connection allowed ATTACH")
	}
	bad := []string{
		`UPDATE docs SET title='changed'`,
		`WITH x AS (SELECT 1) UPDATE docs SET title='changed'`,
		`SELECT 1; DELETE FROM docs`,
		`SELECT 1; -- comment
DELETE FROM docs`,
		`ATTACH DATABASE '/tmp/other.db' AS other`,
		`SELECT load_extension('/tmp/x.so')`,
		`SELECT * FROM pragma_database_list`,
		`PRAGMA writable_schema=ON`,
		`SELECT 1; SELECT 2`,
	}
	for _, query := range bad {
		if _, err := target.Query(context.Background(), core.QueryRequest{SQL: query}); err == nil {
			t.Fatalf("accepted %q", query)
		}
	}
	var title string
	if err := writer.QueryRow("SELECT title FROM docs WHERE id=1").Scan(&title); err != nil || title != "alpha" {
		t.Fatalf("database changed: %q, %v", title, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.db")
	if err := os.WriteFile(outside, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err := target.Query(context.Background(), core.QueryRequest{SQL: "SELECT 1"}); err == nil {
		t.Fatal("accepted symlink escape")
	}
}

func TestSQLiteSQLScanner(t *testing.T) {
	good := []string{`SELECT ';' AS value`, "-- header\nSELECT 1; -- tail", `WITH c AS (VALUES(1)) SELECT * FROM c`}
	for _, query := range good {
		if _, err := validateSQL(query, 1024); err != nil {
			t.Fatalf("rejected %q: %v", query, err)
		}
	}
	bad := []string{`SELECT 1; SELECT 2`, `SELECT 1; /* comment */ DELETE FROM x`, `SELECT 'unterminated`, `/* unterminated`, "SELECT 1\x00"}
	for _, query := range bad {
		if _, err := validateSQL(query, 1024); err == nil {
			t.Fatalf("accepted %q", query)
		}
	}
}

func TestSQLiteBatchSnapshotAndRollback(t *testing.T) {
	target, writer, _ := fixture(t)
	result, err := target.BatchQuery(context.Background(), core.BatchRequest{Queries: []core.QueryRequest{{SQL: "SELECT count(*) AS n FROM docs"}, {SQL: "SELECT title FROM docs ORDER BY id", MaxRows: 1}}})
	if err != nil || result.CompletedQueries != 2 || !result.Truncated {
		t.Fatalf("batch: %#v, %v", result, err)
	}
	if _, err := target.BatchQuery(context.Background(), core.BatchRequest{Queries: []core.QueryRequest{{SQL: "SELECT 1"}, {SQL: "WITH x AS (SELECT 1) DELETE FROM docs"}}}); err == nil {
		t.Fatal("batch accepted write")
	}
	var count int
	if err := writer.QueryRow("SELECT count(*) FROM docs").Scan(&count); err != nil || count != 2 {
		t.Fatalf("batch changed data: %d, %v", count, err)
	}
}

func TestSQLiteCancellation(t *testing.T) {
	target, _, _ := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := target.Query(ctx, core.QueryRequest{SQL: "WITH RECURSIVE x(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM x WHERE n<1000000000) SELECT max(n) FROM x"})
	if err == nil {
		t.Fatal("long-running SQLite query was not cancelled")
	}
	if ctx.Err() == nil {
		t.Fatalf("query failed before cancellation: %v", err)
	}
}
