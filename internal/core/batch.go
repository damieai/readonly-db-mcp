package core

import (
	"context"
	"time"
)

type BatchRequest struct {
	Queries []QueryRequest
	Timeout time.Duration
}

type BatchResult struct {
	BatchID     string         `json:"batch_id"`
	Target      string         `json:"target"`
	Engine      string         `json:"engine"`
	Environment string         `json:"environment"`
	Consistency string         `json:"consistency"`
	Database    string         `json:"database"`
	Results     []*QueryResult `json:"results"`
	DurationMS  int64          `json:"duration_ms"`
}

type BatchTarget interface {
	BatchQuery(context.Context, BatchRequest) (*BatchResult, error)
}
