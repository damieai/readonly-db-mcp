package postgresql

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestVectorSyntaxRequiresNativeAdmission(t *testing.T) {
	c := &config.TargetConfig{AllowedSchemas: []string{"reporting"}}
	q := `SELECT vectors.l2_distance(embedding,$1::vectors.vector) FROM reporting.items`
	if _, err := targetPolicy(c, 1024, map[string]struct{}{}).Validate(q, 1); err == nil {
		t.Fatal("extension syntax enabled without opt-in")
	}
	c.PostgreSQL.PGVector = &config.PGVectorConfig{}
	if _, err := targetPolicy(c, 1024, map[string]struct{}{}).Validate(q, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := targetPolicy(c, 1024, nil).Validate(`WITH changed AS (DELETE FROM reporting.items RETURNING *) SELECT * FROM changed`, 0); err == nil {
		t.Fatal("native binding mode weakened statement safety")
	}
}

func TestDenseVectorOutputBudgetAndInvalidValues(t *testing.T) {
	for _, score := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		v, tr := normalize(score, 100)
		if _, err := json.Marshal(v); err != nil || tr {
			t.Fatal("native non-finite score cannot be represented", v, err)
		}
	}
	v, tr, err := normalizeVector("[1,0.5,-2]", "vector", 100)
	if err != nil || tr || len(v.([]float64)) != 3 {
		t.Fatal(v, tr, err)
	}
	v, tr, err = normalizeVector("[1,0.5,-2]", "halfvec", 4)
	if err != nil || !tr || v.(map[string]any)["truncated"] != true {
		t.Fatal("vector cell was silently sliced", v, tr, err)
	}
	if _, err := json.Marshal(v); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"[null]", "[NaN]", "[1e999]", "[]", "{}", "[1,]"} {
		if _, _, err := normalizeVector(raw, "vector", 100); err == nil {
			t.Fatal("invalid vector result accepted", raw)
		}
	}
}

func TestVectorOptionsVersionBounds(t *testing.T) {
	ef := 1000
	scan := 2147483647
	probes := 32768
	mult := 1000.0
	relaxed := "relaxed_order"
	o := &core.PostgreSQLQueryOptions{HNSWEfSearch: &ef, HNSWMaxScanTuples: &scan, HNSWScanMemMultiplier: &mult, IVFFlatProbes: &probes, IVFFlatMaxProbes: &probes, IVFFlatIterativeScan: &relaxed}
	if err := validateVectorOptions(o, true); err != nil {
		t.Fatal("native advanced limits narrowed", err)
	}
	if validateVectorOptions(o, false) == nil {
		t.Fatal("options accepted without a profile")
	}
	mult = math.NaN()
	if validateVectorOptions(o, true) == nil {
		t.Fatal("NaN memory multiplier accepted")
	}
	mult = 1
	relaxed = "strict_order"
	if validateVectorOptions(o, true) == nil {
		t.Fatal("unsupported IVFFlat mode accepted")
	}
}
