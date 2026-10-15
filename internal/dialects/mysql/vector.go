package mysql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

const maxVectorDimensions = 4096

// VectorSearch calculates exact distances over a complete, bounded SELECT
// result. It is deliberately not described as an indexed nearest-neighbor
// search: stock MySQL 8 has no general-purpose built-in distance function.
func (t *Target) VectorSearch(ctx context.Context, request core.MySQLVectorRequest) (*core.MySQLVectorResult, error) {
	started := time.Now()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if request.Timeout < 0 || request.Timeout > t.limits.MaxTimeout {
		return nil, errors.New("requested timeout exceeds configured maximum")
	}
	if err := validateVectorRequest(request, t.limits.MaxRows, t.limits.MaxParameterValueBytes); err != nil {
		return nil, err
	}
	validation, err := t.policy.Validate(request.SQL)
	if err != nil {
		t.record(ctx, audit.Event{Target: t.config.Name, Operation: "mysql_vector_search", Decision: "rejected", Reason: err.Error()})
		return nil, err
	}
	completed := false
	candidateID := ""
	defer func() {
		if !completed {
			t.record(ctx, audit.Event{
				QueryID: candidateID, Target: t.config.Name, Operation: "mysql_vector_search",
				Fingerprint: validation.Fingerprint, Tables: validation.Tables,
				Decision: "failed", Reason: "candidate_or_scoring", Duration: time.Since(started),
			})
		}
	}()
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	searchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	candidates, err := t.execute(searchCtx, "mysql_vector_candidates", request.SQL, core.QueryRequest{
		SQL: request.SQL, Parameters: request.Parameters, Timeout: request.Timeout, MaxRows: request.MaxCandidates,
	}, validation)
	if err != nil {
		return nil, err
	}
	candidateID = candidates.QueryID
	// collectRows uses one extra row to detect truncation. Never rank an
	// incomplete candidate set or a clipped embedding cell.
	if candidates.Truncated || candidates.TruncatedCells != 0 {
		return nil, errors.New("vector candidate set exceeds row, cell or byte limit; narrow the SELECT or raise configured limits")
	}
	found := false
	for _, column := range candidates.Columns {
		if column.Name == request.EmbeddingColumn {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("vector candidate SELECT must project the named embedding column")
	}
	result := &core.MySQLVectorResult{
		QueryID: candidates.QueryID, Target: t.config.Name, Metric: request.Metric,
		CandidateCount: len(candidates.Rows), Hits: make([]core.MySQLVectorHit, 0, len(candidates.Rows)),
		ScoreOrder: "ascending",
	}
	if request.Metric == "dot" {
		result.ScoreOrder = "descending"
	}
	for _, row := range candidates.Rows {
		if err := searchCtx.Err(); err != nil {
			return nil, err
		}
		embedding, err := decodeJSONVector(row[request.EmbeddingColumn], len(request.Vector))
		if err != nil {
			return nil, fmt.Errorf("candidate embedding is invalid: %w", err)
		}
		score, err := vectorScore(request.Vector, embedding, request.Metric)
		if err != nil {
			return nil, fmt.Errorf("candidate embedding is invalid: %w", err)
		}
		visible := make(map[string]any, len(row)-1)
		for key, value := range row {
			if key != request.EmbeddingColumn {
				visible[key] = value
			}
		}
		result.Hits = append(result.Hits, core.MySQLVectorHit{Score: score, Row: visible})
	}
	sort.SliceStable(result.Hits, func(i, j int) bool {
		if request.Metric == "dot" {
			return result.Hits[i].Score > result.Hits[j].Score
		}
		return result.Hits[i].Score < result.Hits[j].Score
	})
	if len(result.Hits) > request.TopK {
		result.Hits = result.Hits[:request.TopK]
	}
	if err := searchCtx.Err(); err != nil {
		return nil, err
	}
	result.DurationMS = time.Since(started).Milliseconds()
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > t.limits.MaxResultBytes {
		return nil, errors.New("vector result exceeds configured result-byte limit")
	}
	t.record(searchCtx, audit.Event{
		QueryID: result.QueryID, Target: t.config.Name, Operation: "mysql_vector_search",
		Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "allowed",
		Rows: len(result.Hits), Duration: time.Since(started), ResponseBytes: len(raw),
	})
	completed = true
	return result, nil
}

func validateVectorRequest(request core.MySQLVectorRequest, maxRows, maxParameterValueBytes int) error {
	if request.Metric != "cosine" && request.Metric != "l2" && request.Metric != "dot" {
		return errors.New("vector metric must be cosine, l2 or dot")
	}
	if request.MaxCandidates < 1 || request.MaxCandidates > maxRows {
		return errors.New("max_candidates must be positive and at most the configured row limit")
	}
	if request.TopK < 1 || request.TopK > request.MaxCandidates {
		return errors.New("top_k must be positive and at most max_candidates")
	}
	if !safeIdentifier(request.EmbeddingColumn) {
		return errors.New("embedding_column must be a plain column alias")
	}
	if len(request.Vector) < 1 || len(request.Vector) > maxVectorDimensions || len(request.Vector)*8 > maxParameterValueBytes {
		return errors.New("query vector exceeds the configured dimension or byte limit")
	}
	var norm float64
	for _, value := range request.Vector {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("query vector must contain finite numbers")
		}
		norm = math.Hypot(norm, value)
	}
	if request.Metric == "cosine" && (norm == 0 || math.IsInf(norm, 0)) {
		return errors.New("cosine query vector must have a finite, nonzero norm")
	}
	return nil
}

func decodeJSONVector(value any, dimensions int) ([]float64, error) {
	var source []byte
	switch typed := value.(type) {
	case string:
		source = []byte(typed)
	case []byte:
		source = typed
	default:
		return nil, errors.New("embedding must be a JSON numeric array")
	}
	decoder := json.NewDecoder(bytes.NewReader(source))
	var values []float64
	if err := decoder.Decode(&values); err != nil {
		return nil, errors.New("embedding must be a JSON numeric array")
	}
	if err := decoder.Decode(new(any)); err != io.EOF || len(values) != dimensions {
		return nil, errors.New("embedding dimension or JSON shape is invalid")
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.New("embedding must contain finite numbers")
		}
	}
	return values, nil
}

func vectorScore(query, candidate []float64, metric string) (float64, error) {
	var dot, queryNorm, candidateNorm, distance float64
	for i, value := range query {
		other := candidate[i]
		switch metric {
		case "dot":
			dot += value * other
		case "l2":
			distance = math.Hypot(distance, value-other)
		case "cosine":
			queryNorm = math.Hypot(queryNorm, value)
			candidateNorm = math.Hypot(candidateNorm, other)
		}
	}
	var score float64
	switch metric {
	case "l2":
		score = distance
	case "dot":
		score = dot
	case "cosine":
		if candidateNorm == 0 || queryNorm == 0 || math.IsInf(candidateNorm, 0) || math.IsInf(queryNorm, 0) {
			return 0, errors.New("cosine vectors must have finite, nonzero norms")
		}
		for i, value := range query {
			dot += (value / queryNorm) * (candidate[i] / candidateNorm)
		}
		score = 1 - math.Max(-1, math.Min(1, dot))
	}
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return 0, errors.New("vector score is not finite")
	}
	return score, nil
}
