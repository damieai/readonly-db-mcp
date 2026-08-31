package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func (t *Target) BatchQuery(ctx context.Context, request core.BatchRequest) (*core.BatchResult, error) {
	if len(request.Queries) == 0 {
		return nil, fmt.Errorf("batch must contain at least one query")
	}
	if len(request.Queries) > t.limits.MaxBatchQueries {
		return nil, fmt.Errorf("batch contains too many queries")
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout > t.limits.MaxTimeout {
		return nil, fmt.Errorf("requested timeout exceeds configured maximum")
	}

	validations := make([]*core.Validation, len(request.Queries))
	for i, query := range request.Queries {
		validation, err := t.policy.Validate(query.SQL)
		if err != nil {
			return nil, fmt.Errorf("batch query %d: %w", i+1, err)
		}
		if len(query.Parameters) > t.limits.MaxParameters {
			return nil, fmt.Errorf("batch query %d has too many parameters", i+1)
		}
		if err := validateParameters(query.Parameters); err != nil {
			return nil, fmt.Errorf("batch query %d: %w", i+1, err)
		}
		validations[i] = validation
	}

	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := t.acquire(queryCtx); err != nil {
		return nil, fmt.Errorf("query concurrency limit: %w", err)
	}
	defer t.release()

	started := time.Now()
	batchID := uuid.NewString()
	tx, err := t.db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, sanitizeDBError(err)
	}
	defer tx.Rollback()

	result := &core.BatchResult{
		BatchID:     batchID,
		Target:      t.config.Name,
		Engine:      t.config.Engine,
		Environment: t.config.Environment,
		Consistency: t.config.Consistency,
		Database:    t.config.Database,
		Results:     make([]*core.QueryResult, 0, len(request.Queries)),
	}
	batchBytes := 0
	for i, query := range request.Queries {
		maxRows := query.MaxRows
		if maxRows <= 0 {
			maxRows = t.limits.MaxRows
		}
		if maxRows > t.limits.MaxRows {
			return nil, fmt.Errorf("batch query %d row limit exceeds configured maximum", i+1)
		}
		rows, err := tx.QueryContext(queryCtx, query.SQL, query.Parameters...)
		if err != nil {
			t.record(queryCtx, audit.Event{QueryID: batchID, Target: t.config.Name, Operation: "query_batch", Fingerprint: validations[i].Fingerprint, Tables: validations[i].Tables, Decision: "failed", Reason: errorClass(err), Duration: time.Since(started)})
			return nil, sanitizeDBError(err)
		}
		queryResult, collectErr := t.collectRows(rows, maxRows)
		_ = rows.Close()
		if collectErr != nil {
			return nil, collectErr
		}
		queryResult.QueryID = fmt.Sprintf("%s/%d", batchID, i+1)
		queryResult.Target = t.config.Name
		queryResult.Engine = t.config.Engine
		queryResult.Environment = t.config.Environment
		queryResult.Consistency = t.config.Consistency
		queryResult.Database = t.config.Database
		queryResult.DurationMS = time.Since(started).Milliseconds()
		encoded, err := json.Marshal(queryResult)
		if err != nil {
			return nil, fmt.Errorf("encode batch result")
		}
		if batchBytes+len(encoded) > t.limits.MaxResultBytes {
			return nil, fmt.Errorf("batch result exceeds configured result-byte limit")
		}
		batchBytes += len(encoded)
		result.Results = append(result.Results, queryResult)
		t.record(queryCtx, audit.Event{QueryID: queryResult.QueryID, Target: t.config.Name, Operation: "query_batch_item", Fingerprint: validations[i].Fingerprint, Tables: validations[i].Tables, Decision: "allowed", Rows: len(queryResult.Rows), Truncated: queryResult.Truncated, Duration: time.Since(started)})
	}
	result.DurationMS = time.Since(started).Milliseconds()
	t.record(queryCtx, audit.Event{QueryID: batchID, Target: t.config.Name, Operation: "query_batch", Decision: "allowed", Rows: len(result.Results), Duration: time.Since(started)})
	return result, nil
}
