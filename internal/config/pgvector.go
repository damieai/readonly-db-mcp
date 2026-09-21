package config

import (
	"encoding/hex"
	"path"
	"regexp"
	"strings"
)

// Library digests are operator assertions about the server deployment. SQL
// catalogs cannot authenticate the bytes of a remote shared library.
type PGVectorConfig struct {
	Profile             string `yaml:"profile"`
	Schema              string `yaml:"schema"`
	Owner               string `yaml:"owner"`
	HelperSchema        string `yaml:"helper_schema"`
	HelperLibrary       string `yaml:"helper_library"`
	DeploymentID        string `yaml:"deployment_id"`
	VectorLibrarySHA256 string `yaml:"vector_library_sha256"`
	HelperLibrarySHA256 string `yaml:"helper_library_sha256"`
	MaxScanMemoryBytes  int64  `yaml:"max_scan_memory_bytes"`
}

func validatePGVector(v *PGVectorConfig, runtimeRole string) []string {
	if v == nil {
		return nil
	}
	var problems []string
	name := regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)
	if v.Profile != "pg16-vector0.8.2" {
		problems = append(problems, "postgresql.pgvector.profile must be pg16-vector0.8.2")
	}
	for _, s := range []string{v.Schema, v.HelperSchema} {
		if !name.MatchString(s) || strings.HasPrefix(s, "pg_") || s == "information_schema" || s == "public" {
			problems = append(problems, "pgvector schemas must be dedicated lower-case identifiers")
		}
	}
	if v.Schema == v.HelperSchema || !safeIdentifier.MatchString(v.Owner) || v.Owner == runtimeRole {
		problems = append(problems, "pgvector needs separate extension/helper schemas and a distinct trusted owner")
	}
	if !safeName.MatchString(v.DeploymentID) {
		problems = append(problems, "pgvector.deployment_id must identify the operator-reviewed deployment")
	}
	if !path.IsAbs(v.HelperLibrary) || path.Clean(v.HelperLibrary) != v.HelperLibrary || strings.ContainsAny(v.HelperLibrary, "\x00\r\n") {
		problems = append(problems, "pgvector.helper_library must be an absolute server-side library path")
	}
	for _, s := range []string{v.VectorLibrarySHA256, v.HelperLibrarySHA256} {
		b, err := hex.DecodeString(s)
		if err != nil || len(b) != 32 || strings.Trim(s, "0") == "" {
			problems = append(problems, "pgvector library SHA-256 assertions are required")
		}
	}
	if v.MaxScanMemoryBytes == 0 {
		v.MaxScanMemoryBytes = 64 << 20
	}
	if v.MaxScanMemoryBytes < 1<<20 || v.MaxScanMemoryBytes > 1<<30 {
		problems = append(problems, "pgvector.max_scan_memory_bytes must be between 1 MiB and 1 GiB")
	}
	return problems
}
