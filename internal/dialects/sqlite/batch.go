package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func (t *Target) BatchQuery(parent context.Context, request core.BatchRequest) (result *core.BatchResult, err error) {
	started := time.Now()
	var rawID [12]byte
	if _, err = rand.Read(rawID[:]); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(rawID[:])
	batchHash := sha256.New()
	defer func() {
		decision, reason, rows, size := "allowed", "", 0, 0
		if err != nil {
			decision, reason = "denied", err.Error()
		}
		if result != nil {
			for _, item := range result.Results {
				rows += item.RowCount
			}
			encoded, _ := json.Marshal(result)
			size = len(encoded)
		}
		if t.auditor != nil {
			t.auditor.Record(parent, audit.Event{QueryID: id, Target: t.cfg.Name, Operation: "sqlite_batch", Fingerprint: hex.EncodeToString(batchHash.Sum(nil)[:12]), Decision: decision, Reason: reason, Rows: rows, ResponseBytes: size, Duration: time.Since(started)})
		}
		if t.recorder != nil {
			t.recorder.Add("query", 1, "sqlite_batch", decision)
			t.recorder.Observe("query_duration", time.Since(started), "sqlite_batch")
		}
	}()
	if len(request.Queries) == 0 || len(request.Queries) > t.limits.MaxBatchQueries {
		return nil, errors.New("resource_limit: SQLite batch count is invalid")
	}
	if request.PostgreSQLOptions != nil {
		return nil, errors.New("invalid_request: PostgreSQL options are unavailable for SQLite")
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, errors.New("resource_limit: SQLite batch timeout exceeds maximum")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	for i, q := range request.Queries {
		batchHash.Write([]byte(q.SQL))
		batchHash.Write([]byte{0})
		if _, err := t.ValidateQuery(q.SQL); err != nil {
			return nil, fmt.Errorf("query %d: %w", i+1, err)
		}
		if q.PostgreSQLOptions != nil || q.Timeout != 0 {
			return nil, errors.New("invalid_request: SQLite batch query has unsupported options")
		}
		if q.MaxRows < 0 || q.MaxRows > t.limits.MaxRows || len(q.Parameters) > t.limits.MaxParameters {
			return nil, errors.New("resource_limit: SQLite batch query exceeds limits")
		}
		b, e := json.Marshal(q.Parameters)
		if e != nil || len(b) > t.limits.MaxParameterBytes {
			return nil, errors.New("resource_limit: SQLite batch parameters exceed limit")
		}
		for _, p := range q.Parameters {
			b, e := json.Marshal(p)
			if e != nil || len(b) > t.limits.MaxParameterValueBytes {
				return nil, errors.New("resource_limit: SQLite batch parameter exceeds cell limit")
			}
		}
	}
	if err := t.ready(); err != nil {
		return nil, err
	}
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Batch)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result = &core.BatchResult{BatchID: id, Target: t.cfg.Name, Engine: config.EngineSQLite, Environment: t.cfg.Environment, Consistency: t.cfg.Consistency, Database: "main", Results: make([]*core.QueryResult, 0, len(request.Queries))}
	usedBytes := 256
	for i, q := range request.Queries {
		rows, err := tx.QueryContext(ctx, q.SQL, q.Parameters...)
		if err != nil {
			return nil, fmt.Errorf("query %d: %w", i+1, err)
		}
		cap := q.MaxRows
		if cap == 0 {
			cap = t.limits.MaxRows
		}
		item, e := t.collect(ctx, rows, cap, id+"-"+fmt.Sprint(i+1), started)
		rows.Close()
		if e != nil {
			return nil, fmt.Errorf("query %d: %w", i+1, e)
		}
		encoded, _ := json.Marshal(item)
		usedBytes += len(encoded)
		if usedBytes > t.limits.MaxResultBytes {
			return nil, errors.New("resource_limit: SQLite batch exceeds max_result_bytes")
		}
		result.Results = append(result.Results, item)
		if item.Truncated {
			result.Truncated = true
		}
	}
	result.CompletedQueries = len(result.Results)
	result.DurationMS = time.Since(started).Milliseconds()
	final, err := json.Marshal(result)
	if err != nil || len(final) > t.limits.MaxResultBytes {
		return nil, errors.New("resource_limit: SQLite batch exceeds max_result_bytes")
	}
	return result, nil
}
