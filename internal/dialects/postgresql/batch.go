package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"strconv"
	"time"
)

func (t *Target) BatchQuery(ctx context.Context, r core.BatchRequest) (*core.BatchResult, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if len(r.Queries) == 0 {
		return nil, fmt.Errorf("batch must contain at least one query")
	}
	if len(r.Queries) > t.limits.MaxBatchQueries {
		return nil, fmt.Errorf("batch contains too many queries")
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout > t.limits.MaxTimeout {
		return nil, fmt.Errorf("requested timeout exceeds configured maximum")
	}
	valid := make([]*core.Validation, len(r.Queries))
	for i, q := range r.Queries {
		v, err := t.policy.Validate(q.SQL, len(q.Parameters))
		if err != nil {
			return nil, fmt.Errorf("batch query %d: %w", i+1, err)
		}
		if q.MaxRows < 0 || q.MaxRows > t.limits.MaxRows {
			return nil, fmt.Errorf("batch query %d row limit exceeds configured maximum", i+1)
		}
		valid[i] = v
	}
	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	permit, err := t.admission.Acquire(qctx, t.cfg.Name, admission.Batch)
	if err != nil {
		return nil, fmt.Errorf("query concurrency limit: %w", err)
	}
	defer permit.Release()
	tx, err := t.db.BeginTx(qctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, sanitize(err)
	}
	defer tx.Rollback()
	serverTimeout := timeout - t.cfg.PostgreSQL.StatementTimeoutMargin
	if serverTimeout <= 0 {
		serverTimeout = timeout
	}
	if _, err = tx.ExecContext(qctx, `SELECT pg_catalog.set_config('statement_timeout',$1,true)`, strconv.FormatInt(serverTimeout.Milliseconds(), 10)); err != nil {
		return nil, sanitize(err)
	}
	started := time.Now()
	id := uuid.NewString()
	out := &core.BatchResult{BatchID: id, Target: t.cfg.Name, Engine: t.cfg.Engine, Environment: t.cfg.Environment, Consistency: t.cfg.Consistency, Database: t.cfg.Database, Results: make([]*core.QueryResult, 0, len(r.Queries))}
	for i, q := range r.Queries {
		minimal := *out
		minimal.Results = append(append([]*core.QueryResult(nil), out.Results...), &core.QueryResult{Target: t.cfg.Name})
		b, _ := json.Marshal(&minimal)
		if len(b) > t.limits.MaxResultBytes {
			out.Truncated = true
			out.TruncationReason = "result_byte_limit"
			break
		}
		maxRows := q.MaxRows
		if maxRows <= 0 {
			maxRows = t.limits.MaxRows
		}
		rows, err := tx.QueryContext(qctx, q.SQL, q.Parameters...)
		if err != nil {
			return nil, sanitize(err)
		}
		qr, err := t.collect(rows, maxRows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		qr.QueryID = fmt.Sprintf("%s/%d", id, i+1)
		qr.Target = t.cfg.Name
		qr.Engine = t.cfg.Engine
		qr.Environment = t.cfg.Environment
		qr.Consistency = t.cfg.Consistency
		qr.Database = t.cfg.Database
		out.Results = append(out.Results, qr)
		out.CompletedQueries = len(out.Results)
		_ = valid[i]
	}
	out.DurationMS = time.Since(started).Milliseconds()
	for {
		b, _ := json.Marshal(out)
		if len(b) <= t.limits.MaxResultBytes {
			break
		}
		if len(out.Results) == 0 {
			return nil, fmt.Errorf("batch result metadata exceeds configured result-byte limit")
		}
		last := out.Results[len(out.Results)-1]
		if len(last.Rows) > 0 {
			last.Rows = last.Rows[:len(last.Rows)-1]
			last.RowCount = len(last.Rows)
			last.Truncated = true
		} else {
			out.Results = out.Results[:len(out.Results)-1]
			out.CompletedQueries = len(out.Results)
		}
		out.Truncated = true
		out.TruncationReason = "result_byte_limit"
	}
	return out, nil
}
