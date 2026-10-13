package neo4j

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func (t *Target) Read(parent context.Context, request core.Neo4jReadRequest) (out *core.Neo4jReadResult, err error) {
	start := time.Now()
	var id [12]byte
	if _, err = rand.Read(id[:]); err != nil {
		return nil, err
	}
	queryID := hex.EncodeToString(id[:])
	hash := sha256.Sum256([]byte(request.Cypher))
	defer func() {
		decision, reason, rows, size, truncated := "allowed", "", 0, 0, false
		if err != nil {
			decision, reason = "denied", err.Error()
		}
		if out != nil {
			rows, truncated = out.RowCount, out.Truncated
			b, _ := json.Marshal(out)
			size = len(b)
		}
		if t.auditor != nil {
			t.auditor.Record(parent, audit.Event{QueryID: queryID, Target: t.cfg.Name, Operation: "neo4j_read", Fingerprint: hex.EncodeToString(hash[:12]), Decision: decision, Reason: reason, Rows: rows, ResponseBytes: size, Truncated: truncated, Duration: time.Since(start)})
		}
		if t.recorder != nil {
			t.recorder.Add("query", 1, "neo4j_read", decision)
			t.recorder.Observe("query_duration", time.Since(start), "neo4j_read")
		}
	}()
	cypher := strings.TrimSpace(request.Cypher)
	if cypher == "" || len(cypher) > t.limits.MaxSQLBytes || !utf8.ValidString(cypher) || strings.IndexByte(cypher, 0) >= 0 {
		return nil, errors.New("invalid_request: Neo4j Cypher is empty or exceeds limit")
	}
	params, err := validateParameters(request.Parameters, t.limits)
	if err != nil {
		return nil, err
	}
	maxRows := request.MaxRows
	if maxRows == 0 {
		maxRows = t.limits.MaxRows
	}
	if maxRows < 1 || maxRows > t.limits.MaxRows {
		return nil, errors.New("resource_limit: Neo4j row cap exceeds maximum")
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, errors.New("resource_limit: Neo4j timeout exceeds maximum")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	if err := t.ensureProof(ctx); err != nil {
		return nil, err
	}
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Interactive)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	session := t.query.NewSession(ctx, driver.SessionConfig{DatabaseName: t.cfg.Database, AccessMode: driver.AccessModeRead, FetchSize: min(maxRows+1, 256)})
	defer closeSession(session)
	// The attested native grants are the mutation boundary. EXPLAIN changes
	// a write query into a plan-only query, so its summary cannot prove the
	// execution query is read-only; it can also reject valid Cypher prefixes.
	tx, err := session.BeginTransaction(ctx, driver.WithTxTimeout(remaining(ctx)))
	if err != nil {
		if neo4jReadInterrupted(ctx, err) {
			return nil, errors.New("deadline_exceeded: Neo4j query interrupted")
		}
		return nil, errors.New("transport_error: Neo4j read transaction failed")
	}
	defer closeTransaction(tx)
	result, err := tx.Run(ctx, cypher, params)
	if err != nil {
		if neo4jReadInterrupted(ctx, err) {
			return nil, errors.New("deadline_exceeded: Neo4j query interrupted")
		}
		return nil, errors.New("query_error: Neo4j query failed")
	}
	keys, err := result.Keys()
	if err != nil {
		if neo4jReadInterrupted(ctx, err) {
			return nil, errors.New("deadline_exceeded: Neo4j query interrupted")
		}
		return nil, errors.New("query_error: Neo4j result keys failed")
	}
	for _, key := range keys {
		if len(key) > t.limits.MaxCellBytes {
			return nil, errors.New("resource_limit: Neo4j column name exceeds max_cell_bytes")
		}
	}
	out = &core.Neo4jReadResult{QueryID: queryID, Target: t.cfg.Name, Database: t.cfg.Database, Keys: keys, Rows: []map[string]any{}}
	budget, err := newResponseBudget(out, t.limits.MaxResultBytes)
	if err != nil {
		return nil, err
	}
	for result.Next(ctx) {
		if len(out.Rows) >= maxRows {
			out.Truncated = true
			break
		}
		record := result.Record()
		row := make(map[string]any, len(record.Keys))
		for i, key := range record.Keys {
			value, e := normalize(record.Values[i], t.limits.MaxCellBytes, 0)
			if e != nil {
				return nil, e
			}
			row[key] = value
		}
		if err := budget.add(row); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, row)
	}
	iterationErr := result.Err()
	if neo4jReadInterrupted(ctx, iterationErr) {
		return nil, errors.New("deadline_exceeded: Neo4j query interrupted")
	}
	if iterationErr != nil {
		return nil, errors.New("query_error: Neo4j result iteration failed")
	}
	out.RowCount = len(out.Rows)
	out.DurationMS = time.Since(start).Milliseconds()
	if tooLarge(out, t.limits.MaxResultBytes) {
		return nil, errors.New("resource_limit: Neo4j result exceeds max_result_bytes")
	}
	return out, nil
}

func neo4jReadInterrupted(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	var neo4jErr *driver.Neo4jError
	if errors.As(err, &neo4jErr) {
		switch neo4jErr.Code {
		case "Neo.ClientError.Transaction.TransactionTimedOut", "Neo.ClientError.Transaction.TransactionTimedOutClientConfiguration":
			return true
		}
	}
	return false
}

func (t *Target) Metadata(parent context.Context, kind string) (*core.Neo4jMetadataResult, error) {
	queries := map[string]string{
		"indexes":            "SHOW INDEXES YIELD name, type, state, labelsOrTypes, properties, options RETURN name, type, state, labelsOrTypes, properties, options",
		"constraints":        "SHOW CONSTRAINTS YIELD name, type, labelsOrTypes, properties RETURN name, type, labelsOrTypes, properties",
		"labels":             "MATCH (n) UNWIND labels(n) AS name RETURN DISTINCT name",
		"relationship_types": "MATCH ()-[r]->() RETURN DISTINCT type(r) AS name",
	}
	query, ok := queries[kind]
	if !ok {
		return nil, errors.New("invalid_request: unsupported Neo4j metadata kind")
	}
	ctx, cancel := context.WithTimeout(parent, t.limits.DefaultTimeout)
	defer cancel()
	if err := t.ensureProof(ctx); err != nil {
		return nil, err
	}
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Metadata)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	query += fmt.Sprintf(" LIMIT %d", t.limits.MaxRows+1)
	session := t.query.NewSession(ctx, driver.SessionConfig{DatabaseName: t.cfg.Database, AccessMode: driver.AccessModeRead, FetchSize: min(t.limits.MaxRows+1, 256)})
	defer closeSession(session)
	result, err := session.Run(ctx, query, nil, driver.WithTxTimeout(remaining(ctx)))
	if err != nil {
		return nil, errors.New("query_error: Neo4j metadata query failed")
	}
	out := &core.Neo4jMetadataResult{Target: t.cfg.Name, Database: t.cfg.Database, Kind: kind, Rows: []map[string]any{}}
	budget, err := newResponseBudget(out, t.limits.MaxResultBytes)
	if err != nil {
		return nil, err
	}
	for result.Next(ctx) {
		if len(out.Rows) >= t.limits.MaxRows {
			return nil, errors.New("resource_limit: Neo4j metadata row cap exceeded")
		}
		row := map[string]any{}
		for i, key := range result.Record().Keys {
			value, e := normalize(result.Record().Values[i], t.limits.MaxCellBytes, 0)
			if e != nil {
				return nil, e
			}
			row[key] = value
		}
		if err := budget.add(row); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, row)
	}
	if result.Err() != nil || ctx.Err() != nil {
		return nil, errors.New("query_error: Neo4j metadata interrupted")
	}
	if tooLarge(out, t.limits.MaxResultBytes) {
		return nil, errors.New("resource_limit: Neo4j metadata exceeds max_result_bytes")
	}
	return out, nil
}

func remaining(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Minute
	}
	return max(time.Millisecond, time.Until(deadline))
}

func closeSession(session driver.Session) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = session.Close(ctx)
}

func closeTransaction(tx driver.ExplicitTransaction) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

func tooLarge(value any, limit int) bool {
	encoded, err := json.Marshal(value)
	return err != nil || len(encoded) > limit
}
