package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func (t *Target) RedisBatch(ctx context.Context, request core.RedisBatchRequest) (*core.RedisBatchResult, error) {
	started := time.Now()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if len(request.Commands) == 0 || len(request.Commands) > t.limits.MaxBatchQueries {
		return nil, errors.New("Redis batch command count is outside configured limits")
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout > t.limits.MaxTimeout {
		return nil, errors.New("requested timeout exceeds configured maximum")
	}
	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	permit, err := t.admission.Acquire(qctx, t.cfg.Name, admission.Batch)
	if err != nil {
		return nil, fmt.Errorf("Redis concurrency limit: %w", err)
	}
	defer permit.Release()
	type prepared struct {
		validation  *core.RedisValidation
		args        []any
		maxElements int
	}
	commands := make([]prepared, len(request.Commands))
	var fingerprints strings.Builder
	for i, command := range request.Commands {
		validation, args, err := t.policy.Load().validate(qctx, t.client, command)
		if err != nil {
			t.record(qctx, audit.Event{Target: t.cfg.Name, Operation: "redis_batch", Decision: "rejected", Reason: err.Error()})
			return nil, fmt.Errorf("Redis batch command %d: %w", i+1, err)
		}
		fingerprints.WriteString(validation.Fingerprint)
		fingerprints.WriteByte(0)
		maxElements := command.MaxElements
		if maxElements <= 0 || maxElements > t.cfg.Redis.MaxReplyElements {
			maxElements = t.cfg.Redis.MaxReplyElements
		}
		commands[i] = prepared{validation: validation, args: args, maxElements: maxElements}
	}
	queued := make([]*redisdriver.Cmd, 0, len(commands))
	fn := func(pipe redisdriver.Pipeliner) error {
		for _, command := range commands {
			queued = append(queued, pipe.Do(qctx, command.args...))
		}
		return nil
	}
	if request.Atomic {
		_, err = t.client.TxPipelined(qctx, fn)
	} else {
		_, err = t.client.Pipelined(qctx, fn)
	}
	if err != nil {
		return nil, sanitizeRedis(err)
	}
	result := &core.RedisBatchResult{BatchID: uuid.NewString(), Target: t.cfg.Name, Engine: t.cfg.Engine, Environment: t.cfg.Environment, Results: make([]*core.RedisResult, 0, len(queued))}
	for i, command := range queued {
		value, err := command.Result()
		if err != nil {
			return nil, sanitizeRedis(err)
		}
		normalized, count, truncated, err := normalizeRedis(value, 0, &normalizer{maxDepth: t.cfg.Redis.MaxReplyDepth, maxElements: commands[i].maxElements, maxCell: t.limits.MaxCellBytes})
		if err != nil {
			return nil, err
		}
		item := &core.RedisResult{RequestID: fmt.Sprintf("%s/%d", result.BatchID, i+1), Target: t.cfg.Name, Engine: t.cfg.Engine, Environment: t.cfg.Environment, Command: commands[i].validation.Command, Value: normalized, ElementCount: count, Truncated: truncated}
		result.Results = append(result.Results, item)
		result.CompletedCommands++
		encoded, _ := json.Marshal(result)
		if len(encoded) > t.limits.MaxResultBytes {
			result.Results = result.Results[:len(result.Results)-1]
			result.CompletedCommands--
			result.Truncated = true
			break
		}
	}
	result.DurationMS = time.Since(started).Milliseconds()
	encoded, _ := json.Marshal(result)
	if len(encoded) > t.limits.MaxResultBytes {
		return nil, errors.New("Redis batch metadata exceeds configured byte limit")
	}
	sum := sha256.Sum256([]byte(fingerprints.String()))
	t.record(qctx, audit.Event{QueryID: result.BatchID, Target: t.cfg.Name, Operation: "redis_batch", Fingerprint: hex.EncodeToString(sum[:12]), Decision: "allowed", Rows: result.CompletedCommands, Truncated: result.Truncated, Duration: time.Since(started), ResponseBytes: len(encoded)})
	return result, nil
}
