package core

import (
	"context"
	"encoding/json"
	"time"
)

type QdrantRequest struct {
	Collection string
	Operation  string
	Body       json.RawMessage
	Timeout    time.Duration
}

type QdrantResult struct {
	Target     string          `json:"target"`
	Engine     string          `json:"engine"`
	Collection string          `json:"collection"`
	Operation  string          `json:"operation"`
	Data       json.RawMessage `json:"data"`
	DurationMS int64           `json:"duration_ms"`
}

type QdrantTarget interface {
	Target
	QdrantRead(context.Context, QdrantRequest) (*QdrantResult, error)
}
