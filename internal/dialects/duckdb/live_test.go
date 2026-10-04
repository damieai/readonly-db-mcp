package duckdb

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestLocalDuckDBSandbox(t *testing.T) {
	worker := os.Getenv("READONLY_DB_MCP_DUCKDB_WORKER")
	if worker == "" {
		t.Skip("build cmd/readonly-duckdb-worker and set READONLY_DB_MCP_DUCKDB_WORKER")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.duckdb")
	parquet := filepath.Join(dir, "items.parquet")
	writer, err := sql.Open("duckdb", file)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"CREATE TABLE items(id INTEGER, name VARCHAR)", "INSERT INTO items VALUES (1,'alpha'),(2,'beta')", "CREATE UNIQUE INDEX items_id ON items(id)", "COPY items TO '" + parquet + "' (FORMAT PARQUET)"} {
		if _, err := writer.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	writer.Close()
	cfg := &config.TargetConfig{Name: "analytical", Engine: config.EngineDuckDB, Environment: "test", Consistency: config.ConsistencyCurrent, DuckDB: &config.DuckDBConfig{Root: dir, File: file, WorkerPath: worker, Version: "1.5.6", MemoryMB: 256, Threads: 2}, Connection: config.ConnectionConfig{ConnectTimeout: 10 * time.Second}}
	limits := config.Limits{DefaultTimeout: 30 * time.Second, MaxTimeout: time.Minute, MaxRows: 10, MaxResultBytes: 1 << 20, MaxCellBytes: 1 << 16, MaxSQLBytes: 1 << 16, MaxBatchQueries: 5, MaxParameters: 100, MaxParameterBytes: 1 << 20, MaxParameterValueBytes: 1 << 16}
	controller := admission.New(admission.Config{Global: 2, PerTarget: 2, BatchMax: 1, MaxQueued: 10, QueueTimeout: time.Second})
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	ctx := context.Background()
	for _, query := range []string{"SELECT * FROM items ORDER BY id", "SELECT count(*) AS n FROM read_parquet('/data/items.parquet')", "WITH x AS (SELECT id FROM items) SELECT sum(id) AS n FROM x", "SELECT $$a;b$$ AS value"} {
		result, err := target.Query(ctx, core.QueryRequest{SQL: query})
		if err != nil || result.RowCount == 0 {
			t.Fatalf("query %q: %#v, %v", query, result, err)
		}
	}
	paramResult, err := target.Query(ctx, core.QueryRequest{SQL: "SELECT name FROM items WHERE id=?", Parameters: []any{1}})
	if err != nil || paramResult.RowCount != 1 || paramResult.Rows[0]["name"] != "alpha" {
		t.Fatalf("parameters: %#v, %v", paramResult, err)
	}
	if _, err := target.Explain(ctx, core.QueryRequest{SQL: "SELECT * FROM items"}); err != nil {
		t.Fatalf("explain: %v", err)
	}
	tables, err := target.ListTables(ctx, "", false)
	if err != nil || len(tables) == 0 {
		t.Fatalf("tables: %#v, %v", tables, err)
	}
	description, err := target.DescribeTable(ctx, "main", "items", false)
	if err != nil || len(description.Indexes) != 1 || description.Indexes[0].Name != "items_id" {
		t.Fatalf("describe: %#v, %v", description, err)
	}
	batch, err := target.BatchQuery(ctx, core.BatchRequest{Queries: []core.QueryRequest{{SQL: "SELECT count(*) AS n FROM items"}, {SQL: "SELECT name FROM items WHERE id=?", Parameters: []any{2}}}})
	if err != nil || batch.CompletedQueries != 2 || batch.Results[1].Rows[0]["name"] != "beta" {
		t.Fatalf("batch: %#v, %v", batch, err)
	}
	if _, err := target.BatchQuery(ctx, core.BatchRequest{Queries: []core.QueryRequest{{SQL: "SELECT 1"}, {SQL: "DELETE FROM items"}}}); err == nil {
		t.Fatal("batch accepted mutation")
	}
	for _, query := range []string{"DELETE FROM items", "WITH x AS (SELECT 1) DELETE FROM items", "SELECT 1; DELETE FROM items", "COPY items TO '/data/stolen.csv' (FORMAT CSV)", "SELECT * FROM query('DELETE FROM items')", "SELECT * FROM query('COPY items TO ''/data/stolen.csv'' (FORMAT CSV)')", "SELECT read_text('/etc/passwd')", "SELECT read_text('/usr/lib/x86_64-linux-gnu/libc.so.6')", "SELECT * FROM read_parquet('https://example.com/remote.parquet')"} {
		if _, err := target.Query(ctx, core.QueryRequest{SQL: query, Timeout: 3 * time.Second}); err == nil {
			t.Fatalf("accepted hostile query %q", query)
		}
	}
	if _, err := target.Query(ctx, core.QueryRequest{SQL: "SELECT sum(i) FROM range(10000000000) t(i)", Timeout: 10 * time.Millisecond}); err == nil {
		t.Fatal("long query ignored deadline")
	}
	if _, err := target.Query(ctx, core.QueryRequest{SQL: "SELECT count(*) AS n FROM items"}); err != nil {
		t.Fatalf("query after cancellation: %v", err)
	}
	check, err := sql.Open("duckdb", file)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var count int
	if err := check.QueryRow("SELECT count(*) FROM items").Scan(&count); err != nil || count != 2 {
		t.Fatalf("persistent file changed: %d, %v", count, err)
	}
}
