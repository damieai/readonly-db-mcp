package core

import (
	"context"
	"encoding/json"
	"time"
)

type MongoDBRequest struct {
	Collection string
	Operation  string
	Body       json.RawMessage
	Timeout    time.Duration
}

type MongoDBResult struct {
	Target     string          `json:"target"`
	Engine     string          `json:"engine"`
	Database   string          `json:"database"`
	Collection string          `json:"collection"`
	Operation  string          `json:"operation"`
	Data       json.RawMessage `json:"data"`
	Count      int             `json:"count,omitempty"`
	Truncated  bool            `json:"truncated"`
	DurationMS int64           `json:"duration_ms"`
}

type MongoDBTarget interface {
	Target
	MongoDBRead(context.Context, MongoDBRequest) (*MongoDBResult, error)
}
