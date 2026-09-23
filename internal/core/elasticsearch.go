package core

import (
	"context"
	"encoding/json"
	"time"
)

type ElasticsearchMetadataRequest struct {
	Operation string                     `json:"operation"`
	Indices   []string                   `json:"indices,omitempty"`
	Names     []string                   `json:"names,omitempty"`
	Options   map[string]json.RawMessage `json:"options,omitempty"`
	Timeout   time.Duration              `json:"-"`
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

type ElasticsearchCursorRequest struct {
	Action    string                     `json:"action"`
	Kind      string                     `json:"kind,omitempty"`
	Handle    string                     `json:"handle,omitempty"`
	Indices   []string                   `json:"indices,omitempty"`
	Body      json.RawMessage            `json:"body,omitempty"`
	Options   map[string]json.RawMessage `json:"options,omitempty"`
	MaxRows   int                        `json:"max_rows,omitempty"`
	KeepAlive time.Duration              `json:"-"`
	Timeout   time.Duration              `json:"-"`
	// Supplied by the transport, never bound from caller JSON.
	Owner        string          `json:"-"`
	OwnerContext context.Context `json:"-"`
}

type ElasticsearchCursorResult struct {
	ElasticsearchResult
	Kind           string `json:"kind"`
	Handle         string `json:"handle,omitempty"`
	ExpiresAt      string `json:"expires_at,omitempty"`
	Exhausted      bool   `json:"exhausted"`
	Closed         bool   `json:"closed"`
	CleanupPending bool   `json:"cleanup_pending,omitempty"`
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
	ElasticsearchCursor(context.Context, ElasticsearchCursorRequest) (*ElasticsearchCursorResult, error)
	CloseElasticsearchSession(context.Context, string) error
}
