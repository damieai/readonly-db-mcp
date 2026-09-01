package config

import (
	"testing"
	"time"
)

func TestPostgreSQLDefaultsAndValidation(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EnginePostgreSQL
	target.Port = 5432
	target.PostgreSQL = PostgreSQLConfig{ApplicationName: "readonly-db-mcp", StatementTimeoutMargin: 250 * time.Millisecond, BatchIsolation: "repeatable-read", PrivilegeRecheck: 5 * time.Minute}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestMySQLRejectsPostgreSQLSettings(t *testing.T) {
	cfg := validConfig()
	cfg.Targets["test"].PostgreSQL = PostgreSQLConfig{ApplicationName: "x"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected cross-engine settings rejection")
	}
}
