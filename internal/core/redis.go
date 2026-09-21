package core

import (
	"context"
	"time"
)

type RedisArgument struct {
	String *string `json:"string,omitempty"`
	Base64 *string `json:"base64,omitempty"`
}

type RedisRequest struct {
	Command     string
	Arguments   []RedisArgument
	Timeout     time.Duration
	MaxElements int
	Purpose     string
}

type RedisValidation struct {
	Command         string
	Fingerprint     string
	KeyFingerprints []string
	KeyCount        int
	ArgumentBytes   int
	KeySlots        []int `json:"-"`
}

type RedisResult struct {
	RequestID    string            `json:"request_id"`
	Target       string            `json:"target"`
	Engine       string            `json:"engine"`
	Environment  string            `json:"environment"`
	Command      string            `json:"command"`
	Value        any               `json:"value"`
	ElementCount int               `json:"element_count"`
	Truncated    bool              `json:"truncated"`
	DurationMS   int64             `json:"duration_ms"`
	SearchIndex  *RedisSearchIndex `json:"search_index,omitempty"`
}

// RedisSearchIndex describes the live FT.INFO response, not a certification of
// a build or a promise that every native query option has been qualified.
type RedisSearchIndex struct {
	Name    string             `json:"name"`
	Storage string             `json:"storage"`
	Vectors []RedisVectorField `json:"vectors"`
}

type RedisVectorField struct {
	Identifier     string `json:"identifier"`
	Attribute      string `json:"attribute"`
	Algorithm      string `json:"algorithm"`
	DataType       string `json:"data_type"`
	Dimensions     int    `json:"dimensions"`
	DistanceMetric string `json:"distance_metric"`
}

type RedisBatchRequest struct {
	Commands []RedisRequest
	Atomic   bool
	Timeout  time.Duration
}

type RedisBatchResult struct {
	BatchID           string         `json:"batch_id"`
	Target            string         `json:"target"`
	Engine            string         `json:"engine"`
	Environment       string         `json:"environment"`
	Results           []*RedisResult `json:"results"`
	CompletedCommands int            `json:"completed_commands"`
	Truncated         bool           `json:"truncated"`
	DurationMS        int64          `json:"duration_ms"`
}

type RedisTarget interface {
	Target
	ValidateRedis(context.Context, RedisRequest) (*RedisValidation, error)
	RedisCommand(context.Context, RedisRequest) (*RedisResult, error)
	RedisBatch(context.Context, RedisBatchRequest) (*RedisBatchResult, error)
}
