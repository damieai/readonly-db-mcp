package redis

import (
	"encoding/json"
	"math"
	"testing"
)

func TestVectorMetadataAndScope(t *testing.T) {
	p := searchPolicy()
	p.cfg.ModuleObjectPatterns["FT.INFO"] = []string{"tenant-*"}
	vector := []any{"identifier", "$.embedding", "attribute", "embedding", "type", "VECTOR", "algorithm", "HNSW", "data_type", "FLOAT32", "dim", int64(1536), "distance_metric", "COSINE"}
	info := append(searchInfo("tenant-index", "tenant:").([]any), "attributes", []any{[]any{"identifier", "title", "attribute", "title", "type", "TEXT", "SORTABLE", "UNF"}, vector})
	result, err := p.searchSummary("FT.INFO", info, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "tenant-index" || len(result.Vectors) != 1 || result.Vectors[0].Dimensions != 1536 || result.Vectors[0].Identifier != "$.embedding" {
		t.Fatalf("metadata=%+v", result)
	}
	for _, bad := range []any{
		append(searchInfo("tenant-index", "private:").([]any), "attributes", []any{vector}),
		append(searchInfo("private-index", "tenant:").([]any), "attributes", []any{vector}),
		append(searchInfo("tenant-index", "tenant:").([]any), "attributes", []any{append(append([]any{}, vector...), "dim", int64(3))}),
	} {
		if _, err := p.searchSummary("FT.INFO", bad, 1024); err == nil {
			t.Fatal("untrusted vector metadata accepted")
		}
	}
	vector[11] = int64(0)
	if _, err := p.searchSummary("FT.INFO", info, 1024); err == nil {
		t.Fatal("zero dimensions accepted")
	}
}

func TestRESP3NonFiniteMetadataSurvivesJSON(t *testing.T) {
	for i, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		result, _, truncated, err := normalizeRedis(value, 0, &normalizer{maxCell: 1024})
		if err != nil || truncated {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		want := []string{`{"float":"nan"}`, `{"float":"+inf"}`, `{"float":"-inf"}`}[i]
		if err != nil || string(encoded) != want {
			t.Fatalf("got %s, err=%v", encoded, err)
		}
	}
}
