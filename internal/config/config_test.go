package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidateRejectsPrivilegedUsername(t *testing.T) {
	cfg := validConfig()
	cfg.Targets["test"].Username = "root"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "privileged-looking") {
		t.Fatalf("expected privileged username error, got %v", err)
	}
}

func TestValidateRequiresExplicitSecretSource(t *testing.T) {
	cfg := validConfig()
	cfg.Targets["test"].PasswordFile = ""
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected secret source error, got %v", err)
	}
}

func TestPasswordRejectsLoosePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permissions are not available on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "password")
	if err := os.WriteFile(path, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := &TargetConfig{PasswordFile: path}
	if _, err := target.Password(); err == nil || !strings.Contains(err.Error(), "group or others") {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestPasswordReadsProtectedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "password")
	if err := os.WriteFile(path, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := &TargetConfig{PasswordFile: path}
	got, err := target.Password()
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret" {
		t.Fatalf("got %q", got)
	}
}

func TestExampleConfigLoads(t *testing.T) {
	cfg, err := Load("../../configs/example.yaml")
	if err != nil {
		t.Fatalf("example configuration is invalid: %v", err)
	}
	if cfg.Limits.DefaultTimeout != 5*time.Minute || cfg.Limits.MaxTimeout != 10*time.Minute || cfg.Limits.QueueTimeout != time.Minute {
		t.Fatalf("unexpected agent-friendly timeout profile: %#v", cfg.Limits)
	}
	if cfg.Limits.PerTargetConcurrency != 8 || cfg.Limits.WorkloadClasses.BatchMaxConcurrency != 4 {
		t.Fatalf("unexpected agent-friendly concurrency profile: %#v", cfg.Limits)
	}
	for name, target := range cfg.Targets {
		if target.Connection.MaxOpen != 8 || target.Connection.ReadTimeout != 11*time.Minute {
			t.Fatalf("target %q has unexpected connection profile: %#v", name, target.Connection)
		}
	}
}

func TestValidateAllowsTenMinuteQueriesAndOneMinuteQueue(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("agent-friendly limits should validate: %v", err)
	}
}

func TestValidateRetainsBoundedTimeoutCeilings(t *testing.T) {
	cfg := validConfig()
	cfg.Limits.MaxTimeout = 16 * time.Minute
	cfg.Targets["test"].Connection.ReadTimeout = 17 * time.Minute
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "fifteen-minute") {
		t.Fatalf("expected maximum timeout ceiling error, got %v", err)
	}

	cfg = validConfig()
	cfg.Limits.QueueTimeout = 6 * time.Minute
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "between 1ms and 5m") {
		t.Fatalf("expected queue timeout ceiling error, got %v", err)
	}
}

func TestLoadRejectsMultipleYAMLDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  transport: stdio\n---\nserver:\n  transport: stdio\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected multiple YAML documents to be rejected")
	}
}

func validConfig() *Config {
	return &Config{
		Server: ServerConfig{Transport: TransportStdio, StrictStartup: true},
		Limits: Limits{
			GlobalConcurrency:      16,
			PerTargetConcurrency:   8,
			DefaultTimeout:         5 * time.Minute,
			MaxTimeout:             10 * time.Minute,
			MaxRows:                5_000,
			MaxResultBytes:         8 << 20,
			MaxCellBytes:           1 << 20,
			MaxSQLBytes:            256 << 10,
			MaxBatchQueries:        50,
			MaxParameters:          1_000,
			MaxParameterBytes:      8 << 20,
			MaxParameterValueBytes: 2 << 20,
			MaxQueuedRequests:      256,
			QueueTimeout:           time.Minute,
			WorkloadClasses:        WorkloadClasses{MetadataReserved: 2, BatchMaxConcurrency: 4, MaintenanceMaxConcurrency: 2},
		},
		Targets: map[string]*TargetConfig{
			"test": {
				Name:           "test",
				Engine:         EngineMySQL,
				Environment:    "test",
				Consistency:    ConsistencyCurrent,
				Host:           "127.0.0.1",
				Port:           3306,
				Database:       "sample",
				Username:       "sample_ro",
				PasswordFile:   "/tmp/sample-password",
				AllowedSchemas: []string{"sample"},
				MetadataCache:  MetadataCacheConfig{TableListTTL: 20 * time.Minute, TableDescriptionTTL: 20 * time.Minute, NegativeTTL: 5 * time.Second, FreshCooldown: time.Second, MaxEntries: 256, MaxBytes: 8 << 20},
				Connection: ConnectionConfig{
					ConnectTimeout: 15 * time.Second,
					ReadTimeout:    11 * time.Minute,
					WriteTimeout:   15 * time.Second,
					MaxOpen:        8,
					MaxIdle:        4,
					MaxLifetime:    30 * time.Minute,
					MaxIdleTime:    10 * time.Minute,
				},
				TLS:   TLSConfig{Mode: TLSDisabled},
				MySQL: MySQLConfig{PrivilegeRecheck: 5 * time.Minute},
			},
		},
	}
}
