package neo4j

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type preparedNeo4jRead struct {
	cypher  string
	params  map[string]any
	maxRows int
}

// BatchRead executes sequentially within one explicit transaction. Neo4j's
// read-committed isolation does not promise repeatable reads across members.
func (t *Target) BatchRead(parent context.Context, request core.Neo4jBatchRequest) (out *core.Neo4jBatchResult, err error) {
	started := time.Now()
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, err
	}
	batchID := hex.EncodeToString(random[:])
	fingerprint := sha256.New()
	for _, query := range request.Queries {
		_, _ = fingerprint.Write([]byte(strconv.Itoa(len(query.Cypher)) + ":" + query.Cypher))
	}
	defer func() {
		decision, reason, rows, size, truncated := "allowed", "", 0, 0, false
		if err != nil {
			decision, reason = "denied", err.Error()
		}
		if out != nil {
			truncated = out.Truncated
			for _, result := range out.Results {
				rows += result.RowCount
			}
			encoded, _ := json.Marshal(out)
			size = len(encoded)
		}
		if t.auditor != nil {
			t.auditor.Record(parent, audit.Event{QueryID: batchID, Target: t.cfg.Name, Operation: "neo4j_batch_read", Fingerprint: hex.EncodeToString(fingerprint.Sum(nil)[:12]), Decision: decision, Reason: reason, Rows: rows, ResponseBytes: size, Truncated: truncated, Duration: time.Since(started)})
		}
		if t.recorder != nil {
			t.recorder.Add("query", 1, "neo4j_batch_read", decision)
			t.recorder.Observe("query_duration", time.Since(started), "neo4j_batch_read")
		}
	}()
	if len(request.Queries) == 0 || len(request.Queries) > t.limits.MaxBatchQueries {
		return nil, errors.New("resource_limit: Neo4j batch query count is invalid")
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, errors.New("resource_limit: Neo4j batch timeout exceeds maximum")
	}
	prepared := make([]preparedNeo4jRead, len(request.Queries))
	parameterBytes := 0
	for i, query := range request.Queries {
		if query.Timeout != 0 {
			return nil, fmt.Errorf("invalid_request: Neo4j batch query %d has a per-query timeout", i+1)
		}
		cypher, params, maxRows, err := t.validateReadInput(query)
		if err != nil {
			return nil, fmt.Errorf("Neo4j batch query %d: %w", i+1, err)
		}
		encoded, _ := json.Marshal(query.Parameters) // validateReadInput already checked JSON encoding.
		parameterBytes += len(encoded)
		if parameterBytes > t.limits.MaxParameterBytes {
			return nil, errors.New("resource_limit: Neo4j batch parameters exceed total byte limit")
		}
		prepared[i] = preparedNeo4jRead{cypher: cypher, params: params, maxRows: maxRows}
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	if err := t.ensureProof(ctx); err != nil {
		return nil, err
	}
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Batch)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	session := t.query.NewSession(ctx, driver.SessionConfig{DatabaseName: t.cfg.Database, AccessMode: driver.AccessModeRead, FetchSize: min(t.limits.MaxRows+1, 256)})
	defer closeSession(session)
	tx, err := session.BeginTransaction(ctx, driver.WithTxTimeout(remaining(ctx)))
	if err != nil {
		if neo4jReadInterrupted(ctx, err) {
			return nil, errors.New("deadline_exceeded: Neo4j batch interrupted")
		}
		return nil, errors.New("transport_error: Neo4j batch transaction failed")
	}
	defer closeTransaction(tx)
	out = &core.Neo4jBatchResult{BatchID: batchID, Target: t.cfg.Name, Database: t.cfg.Database, Results: make([]core.Neo4jReadResult, 0, len(prepared))}
	for i, query := range prepared {
		if ctx.Err() != nil {
			return nil, errors.New("deadline_exceeded: Neo4j batch interrupted")
		}
		queryID := fmt.Sprintf("%s/%d", batchID, i+1)
		memberLimit, err := batchMemberByteLimit(out, queryID, t.limits.MaxResultBytes)
		if err != nil {
			return nil, err
		}
		result, err := t.runReadInTransaction(ctx, tx, query.cypher, query.params, query.maxRows, queryID, time.Now(), memberLimit)
		if err != nil {
			return nil, fmt.Errorf("Neo4j batch query %d: %w", i+1, err)
		}
		out.Results = append(out.Results, *result)
		out.CompletedQueries++
		if result.Truncated {
			out.Truncated = true
			out.TruncationReason = "row_limit"
		}
		out.DurationMS = time.Since(started).Milliseconds()
		if tooLarge(out, t.limits.MaxResultBytes) {
			return nil, errors.New("resource_limit: Neo4j batch result exceeds max_result_bytes")
		}
		if out.Truncated {
			break // Do not buffer the unconsumed result to execute another statement.
		}
	}
	return out, nil
}

func batchMemberByteLimit(batch *core.Neo4jBatchResult, queryID string, maximum int) (int, error) {
	member := core.Neo4jReadResult{QueryID: queryID, Target: batch.Target, Database: batch.Database, Keys: []string{}, Rows: []map[string]any{}}
	batch.Results = append(batch.Results, member)
	withMember, err := json.Marshal(batch)
	batch.Results = batch.Results[:len(batch.Results)-1]
	if err != nil || len(withMember) > maximum {
		return 0, errors.New("resource_limit: Neo4j batch result exceeds max_result_bytes")
	}
	memberJSON, _ := json.Marshal(member)
	return maximum - len(withMember) + len(memberJSON), nil
}
