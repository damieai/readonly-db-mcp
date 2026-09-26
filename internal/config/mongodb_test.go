package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMongoDBConfigurationScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mongodb.yaml")
	base := `server:
  transport: stdio
  strict_startup: true
targets:
  documents:
    engine: mongodb
    environment: test
    host: 127.0.0.1
    database: agent_data
    username: agent_ro
    password_env: MONGODB_TEST_PASSWORD
    consistency: eventual
    tls:
      mode: disabled
    mongodb:
      version: 8.0.32
      auth_source: admin
      collections: [docs]
`
	if err := os.WriteFile(path, []byte(base), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("valid MongoDB config: %v", err)
	}
	for name, content := range map[string]string{
		"remote_plaintext":    strings.Replace(base, "127.0.0.1", "example.com", 1),
		"wildcard_collection": strings.Replace(base, "[docs]", "['*']", 1),
		"unpatched_build":     strings.Replace(base, "8.0.32", "8.0.29", 1),
		"other_engine":        strings.Replace(base, "    mongodb:", "    qdrant: {}\n    mongodb:", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("unsafe MongoDB config accepted")
			}
		})
	}
}
