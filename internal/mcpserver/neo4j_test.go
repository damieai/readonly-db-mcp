package mcpserver

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNeo4jToolsAreRegisteredReadOnly(t *testing.T) {
	server := New(nil, slog.Default(), "test")
	a, b := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ss, err := server.mcp.Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"neo4j_read": false, "neo4j_metadata": false}
	for _, tool := range listed.Tools {
		if _, ok := want[tool.Name]; !ok {
			continue
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || !tool.Annotations.IdempotentHint {
			t.Fatalf("Neo4j tool %q lacks read-only annotations", tool.Name)
		}
		want[tool.Name] = true
	}
	for name, found := range want {
		if !found {
			t.Fatalf("Neo4j MCP tool %q is missing", name)
		}
	}
}
