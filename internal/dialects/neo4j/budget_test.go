package neo4j

import (
	"encoding/json"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestResponseBudgetMatchesEncodedRows(t *testing.T) {
	result := &core.Neo4jReadResult{QueryID: "id", Target: "graph", Database: "knowledge", Keys: []string{"value"}, Rows: []map[string]any{}}
	budget, err := newResponseBudget(result, 1024)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []map[string]any{{"value": "one"}, {"value": "two\nthree"}, {"value": map[string]any{"nested": []any{1, 2}}}} {
		if err := budget.add(row); err != nil {
			t.Fatal(err)
		}
		result.Rows = append(result.Rows, row)
		encoded, err := json.Marshal(result)
		if err != nil || budget.used != len(encoded) {
			t.Fatalf("incremental size %d differs from JSON size %d: %v", budget.used, len(encoded), err)
		}
	}
	if _, err := newResponseBudget(result, budget.used-1); err == nil {
		t.Fatal("oversized result envelope accepted")
	}
	limited, err := newResponseBudget(&core.Neo4jMetadataResult{Rows: []map[string]any{}}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := limited.add(map[string]any{"value": "a very long row that should exceed the tiny remaining response budget"}); err == nil {
		t.Fatal("oversized row accepted")
	}
}
