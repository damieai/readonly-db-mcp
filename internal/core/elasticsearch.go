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

// Native bodies and option values must not pass through float64 decoding.
type ElasticsearchQueryRequest struct {
	Operation string                     `json:"operation"`
	Indices   []string                   `json:"indices,omitempty"`
	ID        string                     `json:"id,omitempty"`
	Body      json.RawMessage            `json:"body,omitempty"`
	Options   map[string]json.RawMessage `json:"options,omitempty"`
	MaxRows   int                        `json:"max_rows,omitempty"`
	Timeout   time.Duration              `json:"-"`
}

type ElasticsearchBatchRequest struct {
	Requests    []ElasticsearchQueryRequest `json:"requests"`
	Consistency string                      `json:"consistency,omitempty"`
	Timeout     time.Duration               `json:"-"`
}

type ElasticsearchBatchResult struct {
	QueryID     string                 `json:"query_id"`
	Target      string                 `json:"target"`
	Engine      string                 `json:"engine"`
	Consistency string                 `json:"consistency"`
	Results     []*ElasticsearchResult `json:"results"`
	DurationMS  int64                  `json:"duration_ms"`
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
	ElasticsearchQuery(context.Context, ElasticsearchQueryRequest) (*ElasticsearchResult, error)
	ElasticsearchBatch(context.Context, ElasticsearchBatchRequest) (*ElasticsearchBatchResult, error)
}
