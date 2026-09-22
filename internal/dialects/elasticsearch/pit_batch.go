package elasticsearch

import (
	"context"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

// All searches share the index/routing snapshot fixed at PIT creation. Other
// native options and DSL remain per-member, with no hidden fallback to eventual.
func (t *Target) pitBatch(ctx context.Context, p *queryProof, queries []*preparedQuery, id string, started time.Time) (*core.ElasticsearchBatchResult, error) {
	first := queries[0]
	for _, q := range queries {
		if q.request.Operation != "search" && q.request.Operation != "search_template" {
			return nil, failure("invalid_request", "PIT batches require search or search_template members")
		}
		if scopeKey(q.request.Indices) != scopeKey(first.request.Indices) {
			return nil, failure("invalid_request", "PIT batch members must share an index scope")
		}
		for _, key := range []string{"routing", "preference", "expand_wildcards", "ignore_unavailable", "allow_no_indices"} {
			if q.options.Get(key) != first.options.Get(key) {
				return nil, failure("invalid_request", "PIT batch members require compatible index and routing options")
			}
		}
	}
	c, err := t.reserveCursor(id, ctx, "pit", p, first)
	if err != nil {
		return nil, err
	}
	defer c.unlock()
	cleanupAttempted := false
	defer func() {
		if !cleanupAttempted {
			t.recoverCursor(ctx, c)
		}
	}()
	ctx, cancel := context.WithDeadline(ctx, c.absolute)
	defer cancel()
	if err := t.updateCursorQuery(c, p, first); err != nil {
		return nil, err
	}
	if err := t.openPIT(ctx, c, first, t.cfg.Elasticsearch.ContextKeepAlive); err != nil {
		return nil, err
	}
	result := &core.ElasticsearchBatchResult{QueryID: id, Target: t.cfg.Name, Engine: "elasticsearch", Consistency: "pit", Results: make([]*core.ElasticsearchResult, 0, len(queries))}
	for _, q := range queries {
		data, err := t.cursorPage(ctx, c, q, t.cfg.Elasticsearch.ContextKeepAlive, false)
		if err != nil {
			return nil, err
		}
		member := t.queryResult(id, q, data, started)
		member.Consistency = "pit"
		result.Results = append(result.Results, member)
		if err := t.encodedLimit(result); err != nil {
			return nil, err
		}
	}
	if err := p.verifySources(); err != nil {
		return nil, err
	}
	if err := t.usableCursor(c); err != nil {
		return nil, err
	}
	cleanup, cancelCleanup := context.WithTimeout(ctx, cursorCleanupTimeout)
	defer cancelCleanup()
	cleanupAttempted = true
	if err := t.cleanupCursor(cleanup, c); err != nil {
		return nil, err
	}
	result.DurationMS = time.Since(started).Milliseconds()
	if err := t.encodedLimit(result); err != nil {
		return nil, err
	}
	return result, nil
}
