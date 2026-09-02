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
}

type RedisResult struct {
	RequestID    string `json:"request_id"`
	Target       string `json:"target"`
	Engine       string `json:"engine"`
	Environment  string `json:"environment"`
	Command      string `json:"command"`
	Value        any    `json:"value"`
	ElementCount int    `json:"element_count"`
	Truncated    bool   `json:"truncated"`
	DurationMS   int64  `json:"duration_ms"`
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
