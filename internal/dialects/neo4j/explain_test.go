package neo4j

import (
	"strings"
	"testing"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type testPlan struct {
	operator    string
	arguments   map[string]any
	identifiers []string
	children    []driver.Plan
}

func (p testPlan) Operator() string          { return p.operator }
func (p testPlan) Arguments() map[string]any { return p.arguments }
func (p testPlan) Identifiers() []string     { return p.identifiers }
func (p testPlan) Children() []driver.Plan   { return p.children }

func TestNeo4jExplainPlanPreservesHierarchyAndBounds(t *testing.T) {
	target := &Target{limits: config.Limits{MaxRows: 3, MaxCellBytes: 100, MaxResultBytes: 2048}}
	plan := testPlan{operator: "ProduceResults", children: []driver.Plan{
		testPlan{operator: "Projection", arguments: map[string]any{"EstimatedRows": 1.5}, identifiers: []string{"source_id"}, children: []driver.Plan{
			testPlan{operator: "NodeIndexSeek", arguments: map[string]any{"Index": "document_id"}},
		}},
	}}
	operators := []core.Neo4jPlanOperator{}
	bytes := 0
	if err := target.appendPlanOperators(plan, -1, 0, &operators, &bytes); err != nil {
		t.Fatal(err)
	}
	if len(operators) != 3 || operators[0].ParentID != -1 || operators[1].ParentID != 0 || operators[2].ParentID != 1 || operators[2].Arguments["Index"] != "document_id" {
		t.Fatalf("plan hierarchy or arguments lost: %+v", operators)
	}
	for _, tc := range []struct {
		name   string
		plan   driver.Plan
		rows   int
		cell   int
		result int
	}{
		{"node_count", plan, 2, 100, 2048},
		{"argument_cell", testPlan{operator: "Scan", arguments: map[string]any{"detail": strings.Repeat("x", 101)}}, 3, 100, 2048},
		{"result_bytes", plan, 3, 100, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target.limits = config.Limits{MaxRows: tc.rows, MaxCellBytes: tc.cell, MaxResultBytes: tc.result}
			operators := []core.Neo4jPlanOperator{}
			bytes := 0
			if err := target.appendPlanOperators(tc.plan, -1, 0, &operators, &bytes); err == nil {
				t.Fatal("oversized plan was accepted")
			}
		})
	}
}
