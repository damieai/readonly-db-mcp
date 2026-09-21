package core

import (
	"context"
	"encoding/json"
	"time"
)

type ElasticsearchMetadataRequest struct {
	Operation string        `json:"operation"`
	Indices   []string      `json:"indices,omitempty"`
	Timeout   time.Duration `json:"-"`
}

type ElasticsearchResult struct {
	QueryID     string          `json:"query_id"`
	Target      string          `json:"target"`
	Engine      string          `json:"engine"`
	Consistency string          `json:"consistency"`
	Operation   string          `json:"operation"`
	Data        json.RawMessage `json:"data"`
	DurationMS  int64           `json:"duration_ms"`
	Partial     bool            `json:"partial"`
	Truncated   bool            `json:"truncated"`
}

type ElasticsearchTarget interface {
	Target
	ElasticsearchMetadata(context.Context, ElasticsearchMetadataRequest) (*ElasticsearchResult, error)
}
