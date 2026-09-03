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
	t.clientMu.RLock()
	defer t.clientMu.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	type prepared struct {
		validation  *core.RedisValidation
		args        []any
		maxElements int
	}
	commands := make([]prepared, len(request.Commands))
	atomicSlot := -1
	var fingerprints strings.Builder
	for i, command := range request.Commands {
		validation, args, err := t.policy.Load().validate(qctx, t.client, command)
		if err != nil {
			t.record(qctx, audit.Event{Target: t.cfg.Name, Operation: "redis_batch", Decision: "rejected", Reason: err.Error()})
			return nil, fmt.Errorf("Redis batch command %d: %w", i+1, err)
		}
		if request.Atomic && t.cfg.Redis.Mode == "cluster" {
			if err := extendAtomicSlot(&atomicSlot, validation.KeySlots); err != nil {
				return nil, err
			}
		}
		fingerprints.WriteString(validation.Fingerprint)
		fingerprints.WriteByte(0)
		maxElements := command.MaxElements
		if maxElements <= 0 || maxElements > t.cfg.Redis.MaxReplyElements {
			maxElements = t.cfg.Redis.MaxReplyElements
		}
		commands[i] = prepared{validation: validation, args: args, maxElements: maxElements}
	}
	result := &core.RedisBatchResult{BatchID: uuid.NewString(), Target: t.cfg.Name, Engine: t.cfg.Engine, Environment: t.cfg.Environment, Results: make([]*core.RedisResult, 0, len(commands))}
	base, _ := json.Marshal(result)
	estimatedBytes := len(base) - 2 // Replace the empty [] with encoded items.
	const metadataReserve = 128
	appendResult := func(i int, value any) error {
		remainingCommands := len(commands) - i
		share := (t.limits.MaxResultBytes - estimatedBytes - metadataReserve) / remainingCommands
		if share < 256 {
			result.Truncated = true
			return nil
		}
		cell := normalizationCellLimit(share, commands[i].maxElements, t.limits.MaxCellBytes)
		normalized, count, truncated, err := normalizeRedis(value, 0, &normalizer{maxDepth: t.cfg.Redis.MaxReplyDepth, maxElements: commands[i].maxElements, maxCell: cell})
		if err != nil {
			return err
		}
		item := &core.RedisResult{RequestID: fmt.Sprintf("%s/%d", result.BatchID, i+1), Target: t.cfg.Name, Engine: t.cfg.Engine, Environment: t.cfg.Environment, Command: commands[i].validation.Command, Value: normalized, ElementCount: count, Truncated: truncated}
		encodedItem, err := json.Marshal(item)
		if err != nil {
			return errors.New("encode Redis batch item")
		}
		separator := 0
		if len(result.Results) > 0 {
			separator = 1
		}
		if estimatedBytes+separator+len(encodedItem)+metadataReserve > t.limits.MaxResultBytes {
			result.Truncated = true
			return nil
		}
		result.Results = append(result.Results, item)
		result.CompletedCommands++
		estimatedBytes += separator + len(encodedItem)
		return nil
	}
	if request.Atomic {
		queued := make([]*redisdriver.Cmd, 0, len(commands))
		_, err = t.client.TxPipelined(qctx, func(pipe redisdriver.Pipeliner) error {
			for _, command := range commands {
				queued = append(queued, pipe.Do(qctx, command.args...))
			}
			return nil
		})
		if err != nil {
			return nil, sanitizeRedis(err)
		}
		for i, command := range queued {
			value, commandErr := command.Result()
			if commandErr != nil {
				return nil, sanitizeRedis(commandErr)
			}
			if err := appendResult(i, value); err != nil {
				return nil, err
			}
			if result.Truncated && result.CompletedCommands <= i {
				break
			}
		}
	} else {
		// Execute non-atomic batches one command at a time so a pipeline cannot
		// retain every raw reply before response normalization and truncation.
		for i, command := range commands {
			value, commandErr := t.client.Do(qctx, command.args...).Result()
			if commandErr != nil {
				return nil, sanitizeRedis(commandErr)
			}
			if err := appendResult(i, value); err != nil {
				return nil, err
			}
			if result.Truncated && result.CompletedCommands <= i {
				break
			}
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

func extendAtomicSlot(current *int, slots []int) error {
	if len(slots) == 0 {
		return fmt.Errorf("Redis Cluster atomic batches require keyed commands in one slot")
	}
	for _, slot := range slots {
		if *current < 0 {
			*current = slot
		} else if *current != slot {
			return fmt.Errorf("Redis Cluster atomic batch spans multiple hash slots")
		}
	}
	return nil
}
