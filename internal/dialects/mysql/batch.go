package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func (t *Target) BatchQuery(ctx context.Context, request core.BatchRequest) (*core.BatchResult, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
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
	batchParameterBytes := 0
	for i, query := range request.Queries {
		validation, err := t.policy.Validate(query.SQL)
		if err != nil {
			t.record(ctx, audit.Event{Target: t.config.Name, Operation: "query_batch", Decision: "rejected", Reason: "policy"})
			return nil, fmt.Errorf("batch query %d: %w", i+1, err)
		}
		if len(query.Parameters) > t.limits.MaxParameters {
			return nil, fmt.Errorf("batch query %d has too many parameters", i+1)
		}
		if err := validateParameters(query.Parameters, t.limits.MaxParameterBytes, t.limits.MaxParameterValueBytes); err != nil {
			return nil, fmt.Errorf("batch query %d: %w", i+1, err)
		}
		batchParameterBytes += parameterBytes(query.Parameters)
		if batchParameterBytes > t.limits.MaxParameterBytes {
			return nil, fmt.Errorf("batch SQL parameters exceed the total byte limit")
		}
		if query.MaxRows < 0 || query.MaxRows > t.limits.MaxRows {
			return nil, fmt.Errorf("batch query %d row limit exceeds configured maximum", i+1)
		}
		validations[i] = validation
	}

	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	permit, err := t.admission.Acquire(queryCtx, t.config.Name, admission.Batch)
	if err != nil {
		t.record(queryCtx, audit.Event{Target: t.config.Name, Operation: "query_batch", Decision: "rejected", Reason: "admission"})
		return nil, fmt.Errorf("query concurrency limit: %w", err)
	}
	defer permit.Release()
	t.gate.RLock()
	defer t.gate.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	defer t.recordDBStats()

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
	for i, query := range request.Queries {
		current, _ := json.Marshal(result)
		placeholder, _ := json.Marshal(&core.QueryResult{QueryID: fmt.Sprintf("%s/%d", batchID, i+1), Target: t.config.Name, Engine: t.config.Engine, Environment: t.config.Environment, Consistency: t.config.Consistency, Database: t.config.Database})
		remaining := t.limits.MaxResultBytes - len(current) - len(placeholder) - 1
		if remaining < 1 {
			result.Truncated = true
			result.TruncationReason = "result_byte_limit"
			break
		}
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
		queryResult, collectErr := t.collectRows(rows, maxRows, remaining)
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
		result.Results = append(result.Results, queryResult)
		for {
			encoded, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("encode batch result")
			}
			if len(encoded) <= t.limits.MaxResultBytes {
				break
			}
			if len(queryResult.Rows) == 0 {
				result.Results = result.Results[:len(result.Results)-1]
				result.Truncated = true
				result.TruncationReason = "result_byte_limit"
				break
			}
			queryResult.Rows = queryResult.Rows[:len(queryResult.Rows)-1]
			queryResult.RowCount = len(queryResult.Rows)
			queryResult.Truncated = true
			result.Truncated = true
			result.TruncationReason = "result_byte_limit"
		}
		if len(result.Results) == 0 || result.Results[len(result.Results)-1] != queryResult {
			break
		}
		result.CompletedQueries = len(result.Results)
		t.record(queryCtx, audit.Event{QueryID: queryResult.QueryID, Target: t.config.Name, Operation: "query_batch_item", Fingerprint: validations[i].Fingerprint, Tables: validations[i].Tables, Decision: "allowed", Rows: len(queryResult.Rows), Truncated: queryResult.Truncated, Duration: time.Since(started)})
	}
	result.DurationMS = time.Since(started).Milliseconds()
	if err := enforceBatchBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	encodedResult, _ := json.Marshal(result)
	t.record(queryCtx, audit.Event{QueryID: batchID, Target: t.config.Name, Operation: "query_batch", Decision: "allowed", Rows: len(result.Results), Duration: time.Since(started), ResponseBytes: len(encodedResult), Truncated: result.Truncated, TruncationReason: result.TruncationReason})
	return result, nil
}

func enforceBatchBudget(result *core.BatchResult, maxBytes int) error {
	for {
		encoded, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("encode batch result")
		}
		if len(encoded) <= maxBytes {
			return nil
		}
		if len(result.Results) == 0 {
			return fmt.Errorf("batch result metadata exceeds configured result-byte limit")
		}
		last := result.Results[len(result.Results)-1]
		if len(last.Rows) > 0 {
			last.Rows = last.Rows[:len(last.Rows)-1]
			last.RowCount = len(last.Rows)
			last.Truncated = true
		} else {
			result.Results = result.Results[:len(result.Results)-1]
			result.CompletedQueries = len(result.Results)
		}
		result.Truncated = true
		result.TruncationReason = "result_byte_limit"
	}
}
