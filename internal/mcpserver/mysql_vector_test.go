package mcpserver

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMySQLVectorToolIsRegisteredReadOnly(t *testing.T) {
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
	for _, item := range listed.Tools {
		if item.Name == "mysql_vector_search" {
			if item.Annotations == nil || !item.Annotations.ReadOnlyHint || !item.Annotations.IdempotentHint {
				t.Fatal("MySQL vector tool lacks read-only annotations")
			}
			return
		}
	}
	t.Fatal("mysql_vector_search tool is missing")
}
