package config

import (
	"strings"
	"testing"
	"time"
)

func TestSQLServerDefaults(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineSQLServer
	target.Port = 0
	target.MySQL = MySQLConfig{}
	applyDefaults(cfg)

	if target.Port != 1433 {
		t.Fatalf("default port = %d, want 1433", target.Port)
	}
	if target.SQLServer.ApplicationName != "readonly-db-mcp" || target.SQLServer.ApplicationIntent != "read-only" {
		t.Fatalf("unexpected SQL Server defaults: %#v", target.SQLServer)
	}
	if target.SQLServer.LockTimeout != 15*time.Second || target.SQLServer.BatchIsolation != "snapshot" {
		t.Fatalf("unexpected SQL Server execution defaults: %#v", target.SQLServer)
	}
	if target.SQLServer.RequireSnapshot == nil || !*target.SQLServer.RequireSnapshot {
		t.Fatal("snapshot isolation must default to required")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLServerAttestorConfigAndRelativePaths(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineSQLServer
	target.MySQL = MySQLConfig{}
	applyDefaults(cfg)
	target.SQLServer.ParserPath = "../bin/sqlserver-parser/readonly-sqlserver-parser"
	target.SQLServer.Attestor = &SQLServerAttestorConfig{Username: "catalog_reader", PasswordFile: "../secrets/catalog.password"}
	resolveRelativePaths(target, "/srv/mcp/configs")
	if target.SQLServer.ParserPath != "/srv/mcp/bin/sqlserver-parser/readonly-sqlserver-parser" || target.SQLServer.Attestor.PasswordFile != "/srv/mcp/secrets/catalog.password" {
		t.Fatal("SQL Server paths did not resolve")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	target.SQLServer.Attestor.PasswordEnv = "ALSO_A_PASSWORD"
	if err := cfg.Validate(); err == nil {
		t.Fatal("ambiguous attestor secrets accepted")
	}
	target.SQLServer.Attestor.PasswordEnv = ""
	target.Connection.MaxOpen = 1
	target.Connection.MaxIdle = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("unbudgeted attestor connection accepted")
	}
}

func TestSQLServerRequiresEventualConsistencyForReplica(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineSQLServer
	target.MySQL = MySQLConfig{}
	target.SQLServer = SQLServerConfig{
		ApplicationName:        "readonly-db-mcp",
		ApplicationIntent:      "read-only",
		RequireReadOnlyReplica: true,
		LockTimeout:            time.Second,
		BatchIsolation:         "snapshot",
		PrivilegeRecheck:       5 * time.Minute,
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "eventual consistency") {
		t.Fatalf("expected replica consistency error, got %v", err)
	}
}

func TestOtherEnginesRejectSQLServerSettings(t *testing.T) {
	cfg := validConfig()
	cfg.Targets["test"].SQLServer = SQLServerConfig{ApplicationName: "x"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "sqlserver settings") {
		t.Fatalf("expected cross-engine settings rejection, got %v", err)
	}
}

func TestSQLServerRejectsExplicitSnapshotOptOut(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineSQLServer
	target.MySQL = MySQLConfig{}
	disabled := false
	target.SQLServer = SQLServerConfig{
		ApplicationName:   "readonly-db-mcp",
		ApplicationIntent: "read-only",
		LockTimeout:       time.Second,
		BatchIsolation:    "snapshot",
		RequireSnapshot:   &disabled,
		PrivilegeRecheck:  5 * time.Minute,
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "require_snapshot_isolation") {
		t.Fatalf("expected snapshot requirement error, got %v", err)
	}
}
