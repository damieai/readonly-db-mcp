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
	if _, err := Load("../../configs/example.yaml"); err != nil {
		t.Fatalf("example configuration is invalid: %v", err)
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
			GlobalConcurrency:      4,
			PerTargetConcurrency:   2,
			DefaultTimeout:         3 * time.Second,
			MaxTimeout:             10 * time.Second,
			MaxRows:                500,
			MaxResultBytes:         1 << 20,
			MaxCellBytes:           64 << 10,
			MaxSQLBytes:            32 << 10,
			MaxBatchQueries:        10,
			MaxParameters:          100,
			MaxParameterBytes:      1 << 20,
			MaxParameterValueBytes: 256 << 10,
			MaxQueuedRequests:      32,
			QueueTimeout:           500 * time.Millisecond,
			WorkloadClasses:        WorkloadClasses{MetadataReserved: 1, BatchMaxConcurrency: 1, MaintenanceMaxConcurrency: 1},
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
					ConnectTimeout: 3 * time.Second,
					ReadTimeout:    12 * time.Second,
					WriteTimeout:   3 * time.Second,
					MaxOpen:        2,
					MaxIdle:        1,
					MaxLifetime:    3 * time.Minute,
					MaxIdleTime:    time.Minute,
				},
				TLS:   TLSConfig{Mode: TLSDisabled},
				MySQL: MySQLConfig{PrivilegeRecheck: 5 * time.Minute},
			},
		},
	}
}
