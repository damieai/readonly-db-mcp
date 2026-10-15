package neo4j

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Explain uses the same native identity and fixed database as Read. EXPLAIN
// plans without executing; PROFILE is never added by this tool.
func (t *Target) Explain(parent context.Context, request core.Neo4jExplainRequest) (out *core.Neo4jExplainResult, err error) {
	started := time.Now()
	var id [12]byte
	if _, err = rand.Read(id[:]); err != nil {
		return nil, err
	}
	queryID := hex.EncodeToString(id[:])
	fingerprint := sha256.Sum256([]byte(request.Cypher))
	defer func() {
		decision, reason, size := "allowed", "", 0
		if err != nil {
			decision, reason = "denied", err.Error()
		}
		if out != nil {
			encoded, _ := json.Marshal(out)
			size = len(encoded)
		}
		if t.auditor != nil {
			t.auditor.Record(parent, audit.Event{QueryID: queryID, Target: t.cfg.Name, Operation: "neo4j_explain", Fingerprint: hex.EncodeToString(fingerprint[:12]), Decision: decision, Reason: reason, ResponseBytes: size, Duration: time.Since(started)})
		}
		if t.recorder != nil {
			t.recorder.Add("query", 1, "neo4j_explain", decision)
			t.recorder.Observe("query_duration", time.Since(started), "neo4j_explain")
		}
	}()
	cypher, params, _, err := t.validateReadInput(core.Neo4jReadRequest{Cypher: request.Cypher, Parameters: request.Parameters})
	if err != nil {
		return nil, err
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, errors.New("resource_limit: Neo4j explain timeout exceeds maximum")
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
	session := t.query.NewSession(ctx, driver.SessionConfig{DatabaseName: t.cfg.Database, AccessMode: driver.AccessModeRead, FetchSize: 1})
	defer closeSession(session)
	tx, err := session.BeginTransaction(ctx, driver.WithTxTimeout(remaining(ctx)))
	if err != nil {
		if neo4jReadInterrupted(ctx, err) {
			return nil, errors.New("deadline_exceeded: Neo4j explain interrupted")
		}
		return nil, errors.New("transport_error: Neo4j explain transaction failed")
	}
	defer closeTransaction(tx)
	result, err := tx.Run(ctx, "EXPLAIN "+cypher, params)
	if err != nil {
		if neo4jReadInterrupted(ctx, err) {
			return nil, errors.New("deadline_exceeded: Neo4j explain interrupted")
		}
		return nil, errors.New("query_error: Neo4j explain failed")
	}
	summary, err := result.Consume(ctx)
	if err != nil || summary == nil {
		if neo4jReadInterrupted(ctx, err) {
			return nil, errors.New("deadline_exceeded: Neo4j explain interrupted")
		}
		return nil, errors.New("query_error: Neo4j explain summary failed")
	}
	if summary.Plan() == nil || summary.QueryProfile() != nil {
		return nil, errors.New("profile_mismatch: Neo4j non-executing plan is unavailable")
	}
	if counters := summary.Counters(); counters != nil && (counters.ContainsUpdates() || counters.ContainsSystemUpdates()) {
		return nil, errors.New("profile_mismatch: Neo4j explain unexpectedly reported updates")
	}
	operators := make([]core.Neo4jPlanOperator, 0)
	bytes := 0
	if err := t.appendPlanOperators(summary.Plan(), -1, 0, &operators, &bytes); err != nil {
		return nil, err
	}
	out = &core.Neo4jExplainResult{QueryID: queryID, Target: t.cfg.Name, Database: t.cfg.Database, Operators: operators, DurationMS: time.Since(started).Milliseconds()}
	if tooLarge(out, t.limits.MaxResultBytes) {
		return nil, errors.New("resource_limit: Neo4j plan exceeds max_result_bytes")
	}
	return out, nil
}

func (t *Target) appendPlanOperators(plan driver.Plan, parentID, depth int, out *[]core.Neo4jPlanOperator, bytes *int) error {
	if plan == nil || depth > 64 || len(*out) >= t.limits.MaxRows {
		return errors.New("resource_limit: Neo4j plan exceeds node or depth limit")
	}
	operator := plan.Operator()
	if operator == "" || len(operator) > t.limits.MaxCellBytes {
		return errors.New("resource_limit: Neo4j plan operator exceeds cell limit")
	}
	arguments, err := normalize(plan.Arguments(), t.limits.MaxCellBytes, 0)
	if err != nil {
		return err
	}
	argumentMap, ok := arguments.(map[string]any)
	if arguments == nil {
		argumentMap = map[string]any{}
	} else if !ok {
		return errors.New("invalid_response: Neo4j plan arguments are malformed")
	}
	identifiers := plan.Identifiers()
	if len(identifiers) > t.limits.MaxRows {
		return errors.New("resource_limit: Neo4j plan has too many identifiers")
	}
	for _, identifier := range identifiers {
		if len(identifier) > t.limits.MaxCellBytes {
			return errors.New("resource_limit: Neo4j plan identifier exceeds cell limit")
		}
	}
	id := len(*out)
	current := core.Neo4jPlanOperator{ID: id, ParentID: parentID, Operator: operator, Arguments: argumentMap, Identifiers: append([]string{}, identifiers...)}
	local, err := json.Marshal(current)
	if err != nil || len(local) > t.limits.MaxResultBytes-*bytes {
		return errors.New("resource_limit: Neo4j plan exceeds max_result_bytes")
	}
	*bytes += len(local)
	*out = append(*out, current)
	children := plan.Children()
	if len(children) > t.limits.MaxRows-len(*out) {
		return errors.New("resource_limit: Neo4j plan has too many operators")
	}
	for _, child := range children {
		if err := t.appendPlanOperators(child, id, depth+1, out, bytes); err != nil {
			return err
		}
	}
	return nil
}
