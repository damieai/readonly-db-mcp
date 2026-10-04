package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type Neo4jReadInput struct {
	Target     string         `json:"target" jsonschema:"Exact configured Neo4j target alias"`
	Cypher     string         `json:"cypher" jsonschema:"One native read-only Cypher query against the configured graph"`
	Parameters map[string]any `json:"parameters,omitempty" jsonschema:"Named Cypher parameters, bound by the driver"`
	TimeoutMS  int            `json:"timeout_ms,omitempty" jsonschema:"Optional whole-query deadline in milliseconds"`
	MaxRows    int            `json:"max_rows,omitempty" jsonschema:"Optional result row cap"`
}

type Neo4jMetadataInput struct {
	Target string `json:"target" jsonschema:"Exact configured Neo4j target alias"`
	Kind   string `json:"kind" jsonschema:"One of indexes, constraints, labels, relationship_types"`
}

func (s *Server) registerNeo4jTools() {
	mcp.AddTool(s.mcp, tool("neo4j_read", "Run bounded read-only Cypher against an attested Neo4j graph target."), s.neo4jRead)
	mcp.AddTool(s.mcp, tool("neo4j_metadata", "Inspect bounded Neo4j index, constraint, label or relationship metadata."), s.neo4jMetadata)
}

func (s *Server) neo4jRead(ctx context.Context, _ *mcp.CallToolRequest, input Neo4jReadInput) (*mcp.CallToolResult, core.Neo4jReadResult, error) {
	target, err := s.registry.GetNeo4j(input.Target)
	if err != nil {
		return nil, core.Neo4jReadResult{}, err
	}
	timeout, err := durationMilliseconds(input.TimeoutMS)
	if err != nil {
		return nil, core.Neo4jReadResult{}, err
	}
	result, err := target.Read(ctx, core.Neo4jReadRequest{Cypher: input.Cypher, Parameters: input.Parameters, Timeout: timeout, MaxRows: input.MaxRows})
	if err != nil {
		return nil, core.Neo4jReadResult{}, err
	}
	return nil, *result, nil
}

func (s *Server) neo4jMetadata(ctx context.Context, _ *mcp.CallToolRequest, input Neo4jMetadataInput) (*mcp.CallToolResult, core.Neo4jMetadataResult, error) {
	target, err := s.registry.GetNeo4j(input.Target)
	if err != nil {
		return nil, core.Neo4jMetadataResult{}, err
	}
	result, err := target.Metadata(ctx, input.Kind)
	if err != nil {
		return nil, core.Neo4jMetadataResult{}, err
	}
	return nil, *result, nil
}
