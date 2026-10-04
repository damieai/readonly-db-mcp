package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLiteConfigScope(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "app.db")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineSQLite
	target.Host = ""
	target.Port = 0
	target.Database = ""
	target.Username = ""
	target.PasswordFile = ""
	target.AllowedSchemas = nil
	target.MySQL = MySQLConfig{}
	target.SQLite = &SQLiteConfig{Root: root, File: file}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid SQLite config: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.db")
	if err := os.WriteFile(outside, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, file); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "inside sqlite.root") {
		t.Fatalf("symlink escape accepted: %v", err)
	}
}

func TestSQLiteRejectsNetworkFields(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "app.db")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineSQLite
	target.SQLite = &SQLiteConfig{Root: root, File: file}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "network and credential") {
		t.Fatalf("network target accepted: %v", err)
	}
}

func TestSQLiteLoadRelativePaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "app.db"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	content := "server:\n  transport: stdio\n  strict_startup: true\ntargets:\n  local:\n    engine: sqlite\n    environment: test\n    sqlite:\n      root: data\n      file: data/app.db\n"
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets["local"].SQLite.File != filepath.Join(dir, "data", "app.db") {
		t.Fatalf("relative file not resolved: %q", cfg.Targets["local"].SQLite.File)
	}
}
