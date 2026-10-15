package core

import (
	"context"
	"time"
)

// MySQLVectorRequest ranks a bounded, complete SQL candidate result. The SQL
// must project one JSON array column named EmbeddingColumn.
type MySQLVectorRequest struct {
	SQL             string
	Parameters      []any
	EmbeddingColumn string
	Vector          []float64
	Metric          string
	TopK            int
	MaxCandidates   int
	Timeout         time.Duration
}

type MySQLVectorHit struct {
	Score float64        `json:"score"`
	Row   map[string]any `json:"row"`
}

type MySQLVectorResult struct {
	QueryID        string           `json:"query_id"`
	Target         string           `json:"target"`
	Metric         string           `json:"metric"`
	ScoreOrder     string           `json:"score_order"`
	CandidateCount int              `json:"candidate_count"`
	Hits           []MySQLVectorHit `json:"hits"`
	DurationMS     int64            `json:"duration_ms"`
}

type MySQLVectorTarget interface {
	Target
	VectorSearch(context.Context, MySQLVectorRequest) (*MySQLVectorResult, error)
}
