package neo4j

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j/dbtype"
	"github.com/your-org/readonly-db-mcp/internal/config"
)

func TestGraphValuePreservesStructureAndPrecision(t *testing.T) {
	node := dbtype.Node{ElementId: "4:node:12", Labels: []string{"Document"}, Props: map[string]any{"source_id": "doc-12", "large": int64(9007199254740993)}}
	rel := dbtype.Relationship{ElementId: "5:rel:8", StartElementId: "4:node:12", EndElementId: "4:node:13", Type: "CITES"}
	value, err := normalize(dbtype.Path{Nodes: []dbtype.Node{node, {ElementId: "4:node:13"}}, Relationships: []dbtype.Relationship{rel}}, 1<<20, 0)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"source_id":"doc-12"`, `"large":"9007199254740993"`, `"start_element_id":"4:node:12"`, `"type":"path"`} {
		if !strings.Contains(string(encoded), expected) {
			t.Fatalf("graph result lost %s: %s", expected, encoded)
		}
	}
	if strconv.IntSize == 64 {
		large := int64(9007199254740993)
		if value, err := normalize(int(large), 1<<20, 0); err != nil || value != "9007199254740993" {
			t.Fatalf("Go int response lost JSON precision: %#v, %v", value, err)
		}
	}
	if _, err := normalize("too long", 3, 0); err == nil {
		t.Fatal("oversized cell accepted")
	}
	for _, value := range []any{
		dbtype.Node{ElementId: "long-element-id"},
		dbtype.Node{ElementId: "id", Labels: []string{"LongLabel"}},
		dbtype.Relationship{ElementId: "id", Type: "LongType"},
	} {
		if _, err := normalize(value, 5, 0); err == nil {
			t.Fatalf("oversized graph identity accepted: %#v", value)
		}
	}
}

func TestNamedParameterBudget(t *testing.T) {
	limits := config.Limits{MaxParameters: 2, MaxParameterBytes: 1024, MaxParameterValueBytes: 256}
	params, err := validateParameters(map[string]any{"id": json.Number("9007199254740993"), "vector": []any{0.1, 0.2}}, limits)
	if err != nil || params["id"] != int64(9007199254740993) {
		t.Fatalf("integer parameter lost precision: %#v, %v", params, err)
	}
	if _, err := validateParameters(map[string]any{"one": 1, "two": 2, "three": 3}, limits); err == nil {
		t.Fatal("parameter count cap ignored")
	}
	if _, err := validateParameters(map[string]any{"id": json.Number("9223372036854775808")}, limits); err == nil {
		t.Fatal("out-of-range integer silently changed to float")
	}
	if _, err := validateParameters(map[string]any{"id": float64(9007199254740992)}, limits); err == nil {
		t.Fatal("unsafe JSON float was accepted as an exact integer")
	}
}
