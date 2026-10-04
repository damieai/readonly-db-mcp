package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeo4jConfigAndAttestorScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := "server:\n  transport: stdio\n  strict_startup: true\nlimits:\n  global_concurrency: 2\n  per_target_concurrency: 2\ntargets:\n  graph:\n    engine: neo4j\n    environment: local\n    host: 127.0.0.1\n    database: knowledge\n    username: graph_ro\n    password_env: NEO4J_QUERY_PASSWORD\n    tls:\n      mode: disabled\n    neo4j:\n      version: 2026.09.0\n      attestor:\n        username: graph_attestor\n        password_file: attestor.secret\n    connection:\n      max_open: 2\n      max_idle: 1\n"
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["graph"]
	if target.Port != 7687 || target.Neo4j.Attestor.PasswordFile != filepath.Join(dir, "attestor.secret") {
		t.Fatalf("Neo4j defaults or path resolution failed: %#v", target)
	}
	target.Neo4j.Attestor.Username = target.Username
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "distinct system metadata attestor") {
		t.Fatalf("shared query and attestor identity accepted: %v", err)
	}
	target.Neo4j.Attestor.Username = "graph_attestor"
	target.Neo4j.Version = "2026.02.0"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "2026.03") {
		t.Fatalf("pre-auth-rule server version accepted: %v", err)
	}
}
