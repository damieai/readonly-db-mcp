package registry

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestSQLiteThroughRegistry(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "items.db")
	writer, err := sql.Open("sqlite3", file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec("CREATE TABLE items(id INTEGER PRIMARY KEY, name TEXT); INSERT INTO items(name) VALUES ('one')"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	path := filepath.Join(dir, "config.yaml")
	yaml := "server:\n  transport: stdio\n  strict_startup: true\ntargets:\n  local:\n    engine: sqlite\n    environment: local\n    sqlite:\n      root: .\n      file: items.db\n"
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
	target, err := r.GetSQL("local")
	if err != nil {
		t.Fatal(err)
	}
	result, err := target.Query(context.Background(), core.QueryRequest{SQL: "SELECT name FROM items"})
	if err != nil || result.RowCount != 1 || result.Rows[0]["name"] != "one" {
		t.Fatalf("query: %#v, %v", result, err)
	}
}
