package registry

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestDuckDBThroughRegistry(t *testing.T) {
	worker := os.Getenv("READONLY_DB_MCP_DUCKDB_WORKER")
	if worker == "" {
		t.Skip("build cmd/readonly-duckdb-worker and set READONLY_DB_MCP_DUCKDB_WORKER")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "items.duckdb")
	writer, err := sql.Open("duckdb", file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec("CREATE TABLE items(id INTEGER, name VARCHAR); INSERT INTO items VALUES (1, 'one')"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	yaml := fmt.Sprintf("server:\n  transport: stdio\n  strict_startup: true\nlimits:\n  global_concurrency: 2\n  per_target_concurrency: 2\ntargets:\n  analytics:\n    engine: duckdb\n    environment: local\n    duckdb:\n      root: .\n      file: items.duckdb\n      worker_path: %q\n      version: 1.5.6\n    connection:\n      max_open: 2\n      max_idle: 1\n", worker)
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Open(context.Background(), cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	target, err := r.GetSQL("analytics")
	if err != nil {
		t.Fatal(err)
	}
	result, err := target.Query(context.Background(), core.QueryRequest{SQL: "SELECT name FROM items WHERE id=?", Parameters: []any{1}})
	if err != nil || result.RowCount != 1 || result.Rows[0]["name"] != "one" {
		t.Fatalf("query: %#v, %v", result, err)
	}
	if _, err := target.Query(context.Background(), core.QueryRequest{SQL: "DELETE FROM items"}); err == nil {
		t.Fatal("registry query accepted mutation")
	}
}
