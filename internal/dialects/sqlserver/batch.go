package sqlserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func (t *Target) BatchQuery(ctx context.Context, request core.BatchRequest) (*core.BatchResult, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if len(request.Queries) == 0 {
		return nil, errors.New("batch must contain at least one query")
	}
	if len(request.Queries) > t.limits.MaxBatchQueries {
		return nil, errors.New("batch contains too many queries")
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout > t.limits.MaxTimeout {
		return nil, errors.New("requested timeout exceeds configured maximum")
	}
	batchParameterBytes := 0
	for i, query := range request.Queries {
		_, err := t.policy.Load().Validate(query.SQL, len(query.Parameters))
		if err != nil {
			return nil, fmt.Errorf("batch query %d: %w", i+1, err)
		}
		if query.MaxRows < 0 || query.MaxRows > t.limits.MaxRows {
			return nil, fmt.Errorf("batch query %d row limit exceeds configured maximum", i+1)
		}
		if len(query.Parameters) > t.limits.MaxParameters {
			return nil, fmt.Errorf("batch query %d has too many parameters", i+1)
		}
		if err := validateParameters(query.Parameters, t.limits.MaxParameterBytes, t.limits.MaxParameterValueBytes); err != nil {
			return nil, fmt.Errorf("batch query %d: %w", i+1, err)
		}
		batchParameterBytes += parameterBytes(query.Parameters)
		if batchParameterBytes > t.limits.MaxParameterBytes {
			return nil, errors.New("batch SQL parameters exceed the total byte limit")
		}
	}

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	permit, err := t.admission.Acquire(qctx, t.cfg.Name, admission.Batch)
	if err != nil {
		return nil, fmt.Errorf("query concurrency limit: %w", err)
	}
	defer permit.Release()
	t.gate.RLock()
	defer t.gate.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	for i, query := range request.Queries {
		if _, err := t.showPlan(qctx, query.SQL, namedParameters(query.Parameters)); err != nil {
			return nil, fmt.Errorf("batch query %d SHOWPLAN: %w", i+1, err)
		}
	}
	tx, err := t.db.BeginTx(qctx, &sql.TxOptions{Isolation: sql.LevelSnapshot})
	if err != nil {
		return nil, sanitize(err)
	}
	defer tx.Rollback()

	started := time.Now()
	id := uuid.NewString()
	result := &core.BatchResult{
		BatchID:     id,
		Target:      t.cfg.Name,
		Engine:      t.cfg.Engine,
		Environment: t.cfg.Environment,
		Consistency: t.cfg.Consistency,
		Database:    t.cfg.Database,
		Results:     make([]*core.QueryResult, 0, len(request.Queries)),
	}
	for i, query := range request.Queries {
		current, _ := json.Marshal(result)
		placeholder, _ := json.Marshal(&core.QueryResult{QueryID: fmt.Sprintf("%s/%d", id, i+1), Target: t.cfg.Name, Engine: t.cfg.Engine, Environment: t.cfg.Environment, Consistency: t.cfg.Consistency, Database: t.cfg.Database})
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
		rows, err := tx.QueryContext(qctx, query.SQL, namedParameters(query.Parameters)...)
		if err != nil {
			return nil, sanitize(err)
		}
		queryResult, err := t.collect(rows, maxRows, remaining)
		rows.Close()
		if err != nil {
			return nil, err
		}
		queryResult.QueryID = fmt.Sprintf("%s/%d", id, i+1)
		queryResult.Target = t.cfg.Name
		queryResult.Engine = t.cfg.Engine
		queryResult.Environment = t.cfg.Environment
		queryResult.Consistency = t.cfg.Consistency
		queryResult.Database = t.cfg.Database
		result.Results = append(result.Results, queryResult)
		result.CompletedQueries = len(result.Results)
	}
	result.DurationMS = time.Since(started).Milliseconds()
	for {
		encoded, _ := json.Marshal(result)
		if len(encoded) <= t.limits.MaxResultBytes {
			break
		}
		if len(result.Results) == 0 {
			return nil, errors.New("batch result metadata exceeds configured result-byte limit")
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
	return result, nil
}
