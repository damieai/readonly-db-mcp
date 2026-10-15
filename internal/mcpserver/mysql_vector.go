package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type MySQLVectorInput struct {
	Target          string    `json:"target" jsonschema:"Exact configured MySQL target alias"`
	SQL             string    `json:"sql" jsonschema:"One read-only MySQL SELECT returning an embedding JSON array column; use question-mark parameters for values"`
	Parameters      []any     `json:"parameters,omitempty" jsonschema:"Positional SQL parameter values"`
	EmbeddingColumn string    `json:"embedding_column" jsonschema:"Exact projected column alias containing each candidate JSON numeric vector"`
	Vector          []float64 `json:"vector" jsonschema:"Finite query vector; its length must equal every candidate embedding dimension"`
	Metric          string    `json:"metric" jsonschema:"cosine, l2 or dot"`
	TopK            int       `json:"top_k" jsonschema:"Maximum ranked hits to return"`
	MaxCandidates   int       `json:"max_candidates" jsonschema:"Maximum complete SQL candidate rows to rank, capped by target max_rows"`
	TimeoutMS       int       `json:"timeout_ms,omitempty" jsonschema:"Whole-call timeout in milliseconds, capped by server configuration"`
}

func (s *Server) mysqlVectorSearch(ctx context.Context, _ *mcp.CallToolRequest, input MySQLVectorInput) (*mcp.CallToolResult, core.MySQLVectorResult, error) {
	target, err := s.registry.GetMySQLVector(input.Target)
	if err != nil {
		return nil, core.MySQLVectorResult{}, err
	}
	timeout, err := durationMilliseconds(input.TimeoutMS)
	if err != nil {
		return nil, core.MySQLVectorResult{}, err
	}
	if input.SQL == "" || input.EmbeddingColumn == "" || len(input.Vector) == 0 || input.Metric == "" {
		return nil, core.MySQLVectorResult{}, fmt.Errorf("sql, embedding_column, vector and metric are required")
	}
	result, err := target.VectorSearch(ctx, core.MySQLVectorRequest{
		SQL: input.SQL, Parameters: input.Parameters, EmbeddingColumn: input.EmbeddingColumn,
		Vector: input.Vector, Metric: input.Metric, TopK: input.TopK,
		MaxCandidates: input.MaxCandidates, Timeout: timeout,
	})
	if err != nil {
		return nil, core.MySQLVectorResult{}, err
	}
	return nil, *result, nil
}
