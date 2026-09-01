package config

import (
	"strings"
	"testing"
)

func TestValidateRejectsSystemSchema(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Database = "mysql"
	target.AllowedSchemas = []string{"mysql"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "system schema") {
		t.Fatalf("expected system schema rejection, got %v", err)
	}
}

func TestValidateRequiresDefaultDatabaseInSchemaScope(t *testing.T) {
	cfg := validConfig()
	cfg.Targets["test"].AllowedSchemas = []string{"reporting"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "database must be included") {
		t.Fatalf("expected default database scope rejection, got %v", err)
	}
}

func TestValidateRejectsInvalidDeniedTableSelector(t *testing.T) {
	cfg := validConfig()
	cfg.Targets["test"].DeniedTables = []string{"inventory.secret.extra"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "schema.table") {
		t.Fatalf("expected denied-table selector rejection, got %v", err)
	}
}

func TestValidateRecognizesProductionEnvironmentVariants(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Environment = "prod-eu"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "production") {
		t.Fatalf("expected production TLS rejection, got %v", err)
	}
}

func TestValidateRejectsCleartextRemoteDatabase(t *testing.T) {
	cfg := validConfig()
	cfg.Targets["test"].Host = "database.internal.example"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "allow_insecure_remote") {
		t.Fatalf("expected cleartext remote database rejection, got %v", err)
	}
}

func TestValidateAllowsExplicitCleartextRemoteDatabaseForNonProduction(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Host = "database.internal.example"
	target.TLS.AllowInsecureRemote = true

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected explicit cleartext remote opt-in to pass, got %v", err)
	}
}

func TestValidateRejectsExplicitCleartextRemoteDatabaseForProduction(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Host = "database.internal.example"
	target.Environment = "production"
	target.TLS.AllowInsecureRemote = true

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "TLS cannot be disabled for production") {
		t.Fatalf("expected production cleartext rejection, got %v", err)
	}
}

func TestValidateRequiresStrictStartup(t *testing.T) {
	cfg := validConfig()
	cfg.Server.StrictStartup = false

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "strict_startup") {
		t.Fatalf("expected strict-startup rejection, got %v", err)
	}
}
