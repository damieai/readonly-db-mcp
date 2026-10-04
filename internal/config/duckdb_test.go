package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDuckDBFileScope(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "sample.duckdb")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(t.TempDir(), "worker")
	if err := os.WriteFile(worker, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	target := &TargetConfig{DuckDB: &DuckDBConfig{Root: root, File: file, WorkerPath: worker}}
	if _, _, _, err := target.DuckDBPath(); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.duckdb")
	if err := os.WriteFile(outside, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, file); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := target.DuckDBPath(); err == nil || !strings.Contains(err.Error(), "inside duckdb.root") {
		t.Fatalf("symlink escape accepted: %v", err)
	}
	target.DuckDB.Root = "/"
	target.DuckDB.File = outside
	if _, _, _, err := target.DuckDBPath(); err == nil || !strings.Contains(err.Error(), "dedicated data directory") {
		t.Fatalf("root filesystem accepted: %v", err)
	}
}

func TestDuckDBLoadRelativePaths(t *testing.T) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bubblewrap is unavailable")
	}
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "sample.duckdb"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "worker"), []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.yaml")
	content := "server:\n  transport: stdio\n  strict_startup: true\nlimits:\n  global_concurrency: 2\n  per_target_concurrency: 2\ntargets:\n  analytics:\n    engine: duckdb\n    environment: local\n    duckdb:\n      root: data\n      file: data/sample.duckdb\n      worker_path: worker\n      version: 1.5.6\n    connection:\n      max_open: 2\n      max_idle: 1\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets["analytics"].DuckDB.File != filepath.Join(data, "sample.duckdb") {
		t.Fatalf("relative paths were not resolved: %#v", cfg.Targets["analytics"].DuckDB)
	}
}
