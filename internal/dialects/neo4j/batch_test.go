package neo4j

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestNeo4jBatchRejectsInvalidInputBeforeConnection(t *testing.T) {
	target := &Target{cfg: &config.TargetConfig{Name: "graph", Database: "knowledge"}, limits: config.Limits{
		MaxBatchQueries: 2, DefaultTimeout: time.Second, MaxTimeout: 2 * time.Second,
		MaxRows: 5, MaxSQLBytes: 100, MaxParameters: 2,
		MaxParameterBytes: 40, MaxParameterValueBytes: 30,
	}}
	for _, tc := range []struct {
		name string
		r    core.Neo4jBatchRequest
		want string
	}{
		{"empty", core.Neo4jBatchRequest{}, "count"},
		{"too_many", core.Neo4jBatchRequest{Queries: []core.Neo4jReadRequest{{Cypher: "RETURN 1"}, {Cypher: "RETURN 2"}, {Cypher: "RETURN 3"}}}, "count"},
		{"member_timeout", core.Neo4jBatchRequest{Queries: []core.Neo4jReadRequest{{Cypher: "RETURN 1", Timeout: time.Second}}}, "per-query timeout"},
		{"invalid_row_cap", core.Neo4jBatchRequest{Queries: []core.Neo4jReadRequest{{Cypher: "RETURN 1", MaxRows: 6}}}, "row cap"},
		{"aggregate_parameters", core.Neo4jBatchRequest{Queries: []core.Neo4jReadRequest{
			{Cypher: "RETURN $value", Parameters: map[string]any{"value": "1234567890"}},
			{Cypher: "RETURN $value", Parameters: map[string]any{"value": "1234567890"}},
		}}, "total byte limit"},
		{"timeout", core.Neo4jBatchRequest{Queries: []core.Neo4jReadRequest{{Cypher: "RETURN 1"}}, Timeout: 3 * time.Second}, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := target.BatchRead(t.Context(), tc.r)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q rejection, got %v", tc.want, err)
			}
		})
	}
}

func TestNeo4jBatchMemberBudgetIncludesBatchEnvelope(t *testing.T) {
	batch := &core.Neo4jBatchResult{BatchID: "batch", Target: "graph", Database: "knowledge", Results: []core.Neo4jReadResult{}}
	member := core.Neo4jReadResult{QueryID: "batch/1", Target: "graph", Database: "knowledge", Keys: []string{}, Rows: []map[string]any{}}
	batch.Results = append(batch.Results, member)
	encoded, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	batch.Results = batch.Results[:0]
	limit, err := batchMemberByteLimit(batch, "batch/1", len(encoded)+20)
	if err != nil {
		t.Fatal(err)
	}
	memberJSON, _ := json.Marshal(member)
	if limit != len(memberJSON)+20 || len(batch.Results) != 0 {
		t.Fatalf("incorrect member allowance or mutated batch: limit=%d results=%d", limit, len(batch.Results))
	}
	if _, err := batchMemberByteLimit(batch, "batch/1", len(encoded)-1); err == nil {
		t.Fatal("batch envelope exceeded byte limit without rejection")
	}
}
