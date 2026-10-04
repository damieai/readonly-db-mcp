package duckdb

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
	"github.com/your-org/readonly-db-mcp/internal/duckdbprotocol"
)

func (t *Target) BatchQuery(parent context.Context, request core.BatchRequest) (result *core.BatchResult, err error) {
	start := time.Now()
	var id [12]byte
	if _, err = rand.Read(id[:]); err != nil {
		return nil, err
	}
	batchID := hex.EncodeToString(id[:])
	hash := sha256.New()
	defer func() {
		decision, reason, rows, size := "allowed", "", 0, 0
		if err != nil {
			decision, reason = "denied", err.Error()
		}
		if result != nil {
			for _, q := range result.Results {
				rows += q.RowCount
			}
			b, _ := json.Marshal(result)
			size = len(b)
		}
		if t.auditor != nil {
			t.auditor.Record(parent, audit.Event{QueryID: batchID, Target: t.cfg.Name, Operation: "duckdb_batch", Fingerprint: hex.EncodeToString(hash.Sum(nil)[:12]), Decision: decision, Reason: reason, Rows: rows, ResponseBytes: size, Duration: time.Since(start)})
		}
		if t.recorder != nil {
			t.recorder.Add("query", 1, "duckdb_batch", decision)
			t.recorder.Observe("query_duration", time.Since(start), "duckdb_batch")
		}
	}()
	if request.PostgreSQLOptions != nil {
		return nil, errors.New("invalid_request: PostgreSQL options are unavailable for DuckDB")
	}
	if len(request.Queries) == 0 || len(request.Queries) > t.limits.MaxBatchQueries {
		return nil, errors.New("resource_limit: DuckDB batch count is invalid")
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, errors.New("resource_limit: DuckDB batch timeout exceeds maximum")
	}
	queries := make([]duckdbprotocol.Query, 0, len(request.Queries))
	for i, q := range request.Queries {
		hash.Write([]byte(q.SQL))
		hash.Write([]byte{0})
		if _, e := t.ValidateQuery(q.SQL); e != nil {
			return nil, fmt.Errorf("query %d: %w", i+1, e)
		}
		if q.PostgreSQLOptions != nil || q.Timeout != 0 || q.MaxRows < 0 || q.MaxRows > t.limits.MaxRows {
			return nil, errors.New("invalid_request: DuckDB batch query options are invalid")
		}
		if len(q.Parameters) > t.limits.MaxParameters {
			return nil, errors.New("resource_limit: too many DuckDB parameters")
		}
		b, e := json.Marshal(q.Parameters)
		if e != nil || len(b) > t.limits.MaxParameterBytes {
			return nil, errors.New("resource_limit: DuckDB batch parameters exceed limit")
		}
		for _, p := range q.Parameters {
			b, e := json.Marshal(p)
			if e != nil || len(b) > t.limits.MaxParameterValueBytes {
				return nil, errors.New("resource_limit: DuckDB batch parameter exceeds value limit")
			}
		}
		queries = append(queries, duckdbprotocol.Query{SQL: q.SQL, Parameters: q.Parameters, MaxRows: q.MaxRows})
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	permit, e := t.controller.Acquire(ctx, t.cfg.Name, admission.Batch)
	if e != nil {
		return nil, e
	}
	defer permit.Release()
	response, e := t.call(ctx, duckdbprotocol.Request{Operation: "batch", Queries: queries})
	if e != nil {
		return nil, e
	}
	if response.Batch == nil || len(response.Batch.Results) != len(queries) {
		return nil, errors.New("invalid_response: DuckDB batch result is incomplete")
	}
	result = response.Batch
	result.BatchID = batchID
	result.Target = t.cfg.Name
	result.Engine = config.EngineDuckDB
	result.Environment = t.cfg.Environment
	result.Consistency = t.cfg.Consistency
	result.Database = "main"
	result.DurationMS = time.Since(start).Milliseconds()
	for i, item := range result.Results {
		item.QueryID = batchID + "-" + fmt.Sprint(i+1)
		item.Target = t.cfg.Name
		item.Engine = config.EngineDuckDB
		item.Environment = t.cfg.Environment
		item.Consistency = t.cfg.Consistency
		item.Database = "main"
		item.DurationMS = result.DurationMS
		cap := request.Queries[i].MaxRows
		if cap == 0 {
			cap = t.limits.MaxRows
		}
		if item.RowCount > cap {
			return nil, errors.New("invalid_response: DuckDB batch exceeded row cap")
		}
	}
	b, e := json.Marshal(result)
	if e != nil || len(b) > t.limits.MaxResultBytes {
		return nil, errors.New("resource_limit: DuckDB batch exceeds max_result_bytes")
	}
	return result, nil
}
