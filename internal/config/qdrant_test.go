package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQdrantConfigurationScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qdrant.yaml")
	base := `server:
  transport: stdio
  strict_startup: true
targets:
  vector:
    engine: qdrant
    environment: test
    tls:
      mode: disabled
    qdrant:
      endpoint: http://127.0.0.1:6333
      version: 1.19.1
      collections: [docs]
      key_env: QDRANT_TEST_JWT
`
	if err := os.WriteFile(path, []byte(base), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("valid loopback Qdrant config: %v", err)
	}
	for name, content := range map[string]string{
		"remote_http":    strings.Replace(base, "127.0.0.1", "example.com", 1),
		"wildcard_scope": strings.Replace(base, "[docs]", "['*']", 1),
		"wrong_version":  strings.Replace(base, "1.19.1", "1.18.0", 1),
		"admin_identity": strings.Replace(base, "    qdrant:", "    username: admin\n    qdrant:", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}
