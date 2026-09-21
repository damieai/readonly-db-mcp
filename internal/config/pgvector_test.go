package config

import (
	"strings"
	"testing"
)

func TestPGVectorRequiresExplicitDeployment(t *testing.T) {
	valid := PGVectorConfig{Profile: "pg16-vector0.8.2", Schema: "vectors", HelperSchema: "mcp_proof", Owner: "extension_owner", HelperLibrary: "/opt/mcp/pgvector-proof.so", DeploymentID: "release-1", VectorLibrarySHA256: strings.Repeat("a", 64), HelperLibrarySHA256: strings.Repeat("b", 64)}
	if p := validatePGVector(&valid, "reader"); len(p) != 0 {
		t.Fatal(p)
	}
	for _, mutate := range []func(*PGVectorConfig){
		func(v *PGVectorConfig) { v.Profile = "pg17-vector0.8.2" },
		func(v *PGVectorConfig) { v.Owner = "reader" },
		func(v *PGVectorConfig) { v.HelperSchema = v.Schema },
		func(v *PGVectorConfig) { v.Schema = "pg_catalog" },
		func(v *PGVectorConfig) { v.HelperLibrary = "pgvector-proof.so" },
		func(v *PGVectorConfig) { v.VectorLibrarySHA256 = "" },
		func(v *PGVectorConfig) { v.HelperLibrarySHA256 = strings.Repeat("0", 64) },
		func(v *PGVectorConfig) { v.MaxScanMemoryBytes = 2 << 30 },
	} {
		v := valid
		mutate(&v)
		if len(validatePGVector(&v, "reader")) == 0 {
			t.Fatal("invalid deployment admitted", v)
		}
	}
	cfg := validConfig()
	cfg.Targets["test"].PostgreSQL.PGVector = &valid
	if cfg.Validate() == nil {
		t.Fatal("cross-engine vector settings accepted")
	}
}
