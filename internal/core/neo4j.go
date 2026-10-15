package core

import (
	"context"
	"time"
)

type Neo4jReadRequest struct {
	Cypher     string         `json:"cypher"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Timeout    time.Duration  `json:"-"`
	MaxRows    int            `json:"max_rows,omitempty"`
}

type Neo4jReadResult struct {
	QueryID    string           `json:"query_id"`
	Target     string           `json:"target"`
	Database   string           `json:"database"`
	Keys       []string         `json:"keys"`
	Rows       []map[string]any `json:"rows"`
	RowCount   int              `json:"row_count"`
	Truncated  bool             `json:"truncated"`
	DurationMS int64            `json:"duration_ms"`
}

type Neo4jBatchRequest struct {
	Queries []Neo4jReadRequest
	Timeout time.Duration
}

type Neo4jBatchResult struct {
	BatchID          string            `json:"batch_id"`
	Target           string            `json:"target"`
	Database         string            `json:"database"`
	Results          []Neo4jReadResult `json:"results"`
	CompletedQueries int               `json:"completed_queries"`
	Truncated        bool              `json:"truncated"`
	TruncationReason string            `json:"truncation_reason,omitempty"`
	DurationMS       int64             `json:"duration_ms"`
}

type Neo4jMetadataResult struct {
	Target   string           `json:"target"`
	Database string           `json:"database"`
	Kind     string           `json:"kind"`
	Rows     []map[string]any `json:"rows"`
}

type Neo4jTarget interface {
	Target
	Read(context.Context, Neo4jReadRequest) (*Neo4jReadResult, error)
	BatchRead(context.Context, Neo4jBatchRequest) (*Neo4jBatchResult, error)
	Metadata(context.Context, string) (*Neo4jMetadataResult, error)
}
