package elasticsearch

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type preparedQuery struct {
	request      core.ElasticsearchQueryRequest
	path, method string
	options      url.Values
	body         []byte
	physical     map[string]bool
	rows         int
	buckets      int
	eqlKind      string
	sql          *sqlProfile
	esql         *esqlProfile
}

// One request owns admission, the shared deadline, and memory throughout source
// proof, rendering, execution and response encoding. A batch never reacquires
// permits while holding another permit.
func (t *Target) queryLease(ctx context.Context, timeout time.Duration, raw []byte, class admission.Class) (context.Context, func(), error) {
	return t.requestLease(ctx, timeout, raw, class, true)
}

func (t *Target) requestLease(ctx context.Context, timeout time.Duration, raw []byte, class admission.Class, requireReady bool) (context.Context, func(), error) {
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, nil, failure("resource_limit", "request timeout exceeds configured maximum")
	}
	if len(raw) > t.cfg.Elasticsearch.MaxRequestBytes {
		return nil, nil, failure("resource_limit", "Elasticsearch request exceeds byte limit")
	}
	if requireReady {
		if err := t.ready(); err != nil {
			return nil, nil, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	unlink := context.AfterFunc(t.ctx, cancel)
	cleanup := func() { unlink(); cancel() }
	charge := int64(len(raw))
	if !queuedMemory.TryAcquire(charge) {
		cleanup()
		return nil, nil, failure("resource_limit", "queued Elasticsearch request budget is exhausted")
	}
	permit, err := t.admission.Acquire(ctx, t.cfg.Name, class)
	if err != nil {
		queuedMemory.Release(charge)
		cleanup()
		return nil, nil, err
	}
	memory := t.memoryCost() + 6*charge
	if !responseMemory.TryAcquire(memory) {
		permit.Release()
		queuedMemory.Release(charge)
		cleanup()
		return nil, nil, failure("resource_limit", "Elasticsearch response memory budget is exhausted")
	}
	release := func() { responseMemory.Release(memory); permit.Release(); queuedMemory.Release(charge); cleanup() }
	if err := strictJSONContext(ctx, raw, t.cfg.Elasticsearch.MaxJSONDepth, t.cfg.Elasticsearch.MaxJSONNodes, 0); err != nil {
		release()
		return nil, nil, err
	}
	return ctx, release, nil
}

func (t *Target) recordQuery(ctx context.Context, id, operation string, raw []byte, started time.Time, err error) {
	outcome, code := "allowed", ""
	if err != nil {
		outcome = "rejected"
		var e *Error
		if errors.As(err, &e) {
			code = e.Code
		}
	}
	if code == "permission_denied" || code == "profile_mismatch" {
		t.invalidateAuthority()
	}
	if t.metrics != nil {
		t.metrics.Observe("elasticsearch_request", time.Since(started), operation)
		t.metrics.Add("elasticsearch_request", 1, operation, outcome)
	}
	if t.auditor != nil {
		mac := hmac.New(sha256.New, t.auditKey[:])
		_, _ = mac.Write(raw)
		t.auditor.Record(ctx, audit.Event{QueryID: id, Target: t.cfg.Name, Operation: operation, Fingerprint: hex.EncodeToString(mac.Sum(nil)[:12]), Decision: outcome, Reason: code, Duration: time.Since(started)})
	}
}

func (t *Target) ElasticsearchQuery(ctx context.Context, request core.ElasticsearchQueryRequest) (result *core.ElasticsearchResult, err error) {
	started, id := time.Now(), uuid.NewString()
	raw, encodeErr := json.Marshal(request)
	defer func() { t.recordQuery(ctx, id, "es_query", raw, started, err) }()
	if encodeErr != nil {
		return nil, failure("invalid_request", "invalid query envelope")
	}
	ctx, release, err := t.queryLease(ctx, request.Timeout, raw, admission.Interactive)
	if err != nil {
		return nil, err
	}
	defer release()
	p := newQueryProof(t, ctx, int(t.next.Add(1)-1)%len(t.wire.clients))
	query, err := p.prepare(request)
	if err != nil {
		return nil, err
	}
	data, err := t.execute(ctx, p.endpoint, query)
	if err != nil {
		return nil, err
	}
	if err := p.verifySources(); err != nil {
		return nil, err
	}
	result = t.queryResult(id, query, data, started)
	if err := t.encodedLimit(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (t *Target) ElasticsearchBatch(ctx context.Context, request core.ElasticsearchBatchRequest) (result *core.ElasticsearchBatchResult, err error) {
	started, id := time.Now(), uuid.NewString()
	raw, encodeErr := json.Marshal(request)
	defer func() { t.recordQuery(ctx, id, "es_batch", raw, started, err) }()
	if encodeErr != nil {
		return nil, failure("invalid_request", "invalid batch envelope")
	}
	if request.Consistency != "" && request.Consistency != "independent" && request.Consistency != "pit" {
		return nil, failure("invalid_request", "batch consistency must be independent or pit")
	}
	if len(request.Requests) == 0 || len(request.Requests) > t.limits.MaxBatchQueries {
		return nil, failure("resource_limit", "batch member count exceeds configured limit")
	}
	ctx, release, err := t.queryLease(ctx, request.Timeout, raw, admission.Batch)
	if err != nil {
		return nil, err
	}
	defer release()
	p := newQueryProof(t, ctx, int(t.next.Add(1)-1)%len(t.wire.clients))
	queries := make([]*preparedQuery, 0, len(request.Requests))
	preparedBytes := 0
	for _, member := range request.Requests {
		if member.Timeout != 0 {
			return nil, failure("invalid_request", "batch members share the batch deadline")
		}
		q, err := p.prepare(member)
		if err != nil {
			return nil, err
		}
		queries = append(queries, q)
		preparedBytes += len(q.body)
		if preparedBytes > t.cfg.Elasticsearch.MaxRequestBytes {
			return nil, failure("resource_limit", "materialized batch exceeds request byte limit")
		}
	}
	if request.Consistency == "pit" {
		return t.pitBatch(ctx, p, queries, id, started)
	}
	// Preflight EVERY member before dispatching any executable query. Rendering,
	// script inspection and source resolution above are bounded read-only proofs.
	result = &core.ElasticsearchBatchResult{QueryID: id, Target: t.cfg.Name, Engine: config.EngineElasticsearch, Consistency: "independent", Results: make([]*core.ElasticsearchResult, 0, len(queries))}
	if data, used, err := t.multiSearch(ctx, p.endpoint, queries); used {
		if err != nil {
			return nil, err
		}
		if err := p.verifySources(); err != nil {
			return nil, err
		}
		for i, raw := range data {
			result.Results = append(result.Results, t.queryResult(id, queries[i], raw, started))
		}
		result.DurationMS = time.Since(started).Milliseconds()
		if err := t.encodedLimit(result); err != nil {
			return nil, err
		}
		return result, nil
	}
	for _, q := range queries {
		data, err := t.execute(ctx, p.endpoint, q)
		if err != nil {
			return nil, err
		}
		result.Results = append(result.Results, t.queryResult(id, q, data, started))
		// Bound the combined encoded result while accumulating, not N full budgets.
		if err := t.encodedLimit(result); err != nil {
			return nil, err
		}
	}
	result.DurationMS = time.Since(started).Milliseconds()
	if err := p.verifySources(); err != nil {
		return nil, err
	}
	if err := t.encodedLimit(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (t *Target) queryResult(id string, q *preparedQuery, raw []byte, started time.Time) *core.ElasticsearchResult {
	consistency := config.ConsistencyEventual
	if q.request.Operation == "get" || q.request.Operation == "mget" || q.request.Operation == "termvectors" || q.request.Operation == "mtermvectors" {
		if q.options.Get("realtime") != "false" {
			consistency = "realtime"
		}
	}
	return &core.ElasticsearchResult{QueryID: id, Target: t.cfg.Name, Engine: config.EngineElasticsearch, Consistency: consistency, Operation: q.request.Operation, Data: json.RawMessage(raw), DurationMS: time.Since(started).Milliseconds()}
}

func (t *Target) encodedLimit(value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return failure("invalid_response", "cannot encode Elasticsearch result")
	}
	textCopy, _ := json.Marshal(string(raw))
	if len(raw)+len(textCopy)+512 > t.limits.MaxResultBytes {
		return failure("resource_limit", "encoded MCP result exceeds response byte limit")
	}
	return nil
}

func (t *Target) execute(ctx context.Context, endpoint int, q *preparedQuery) ([]byte, error) {
	if err := t.ready(); err != nil {
		return nil, err
	}
	if q.request.Operation == "render_search_template" {
		return q.body, nil
	}
	if q.request.Operation == "sql.query" {
		return t.executeSQL(ctx, q)
	}
	// Bound native search execution with the remaining end-to-end budget. Never
	// inject terminate_after, tracking approximations or smaller aggregation sizes.
	if strings.HasSuffix(q.path, "/_search") {
		left := time.Until(deadline(ctx)).Milliseconds()
		if left <= 0 {
			return nil, failure("deadline_exceeded", "query deadline expired before dispatch")
		}
		if err := searchTimeout(q, left); err != nil {
			return nil, err
		}
	}
	if len(q.body) > t.cfg.Elasticsearch.MaxRequestBytes {
		return nil, failure("resource_limit", "materialized query exceeds request byte limit")
	}
	raw, err := t.wire.request(ctx, endpoint, q.method, q.path, q.options, q.body, "application/json", t.limits.MaxResultBytes, q.request.Operation == "get")
	if err != nil {
		return nil, err
	}
	if err := validateQueryResponse(raw, q); err != nil {
		return nil, err
	}
	return raw, nil
}

func deadline(ctx context.Context) time.Time { d, _ := ctx.Deadline(); return d }

type queryProof struct {
	t             *Target
	ctx           context.Context
	endpoint      int
	resolved      map[string]map[string]bool
	physical      map[string]bool
	scripts       map[string]map[string]any
	activeIndices []string
	mappings      map[string]map[string]any
	proofBytes    int
	stockProven   bool
	embedded      map[string]map[string]bool
	snapshot      *ownedCursor
	enrich        *enrichProof
	// Shared traversal work includes decoded wrapper queries and rendered DSL.
	work int
}

func newQueryProof(t *Target, ctx context.Context, endpoint int) *queryProof {
	return &queryProof{t: t, ctx: ctx, endpoint: endpoint, resolved: map[string]map[string]bool{}, physical: map[string]bool{}, scripts: map[string]map[string]any{}, mappings: map[string]map[string]any{}, embedded: map[string]map[string]bool{}}
}

func (p *queryProof) sources(indices []string) (map[string]bool, error) {
	if len(indices) == 0 || len(indices) > p.t.cfg.Elasticsearch.MaxResolvedIndices {
		return nil, failure("resource_limit", "invalid number of index expressions")
	}
	for _, name := range indices {
		if err := scopePattern(p.ctx, name, p.t.cfg.Elasticsearch); err != nil {
			return nil, err
		}
	}
	key := strings.Join(indices, ",")
	if physical, ok := p.resolved[key]; ok {
		return physical, nil
	}
	if len(p.resolved) >= p.t.cfg.Elasticsearch.MaxResolvedIndices {
		return nil, failure("resource_limit", "source proof count exceeds limit")
	}
	if err := p.t.ready(); err != nil {
		return nil, err
	}
	_, physical, err := p.t.resolve(p.ctx, p.endpoint, indices)
	if err != nil {
		return nil, err
	}
	for name := range physical {
		p.physical[name] = true
	}
	enrichCount := 0
	if p.enrich != nil {
		enrichCount = p.enrich.inventory.count
	}
	if len(p.physical)+enrichCount > p.t.cfg.Elasticsearch.MaxResolvedIndices {
		return nil, failure("resource_limit", "combined source inventory exceeds limit")
	}
	retained := len(key) + 128
	for name := range physical {
		retained += len(name) + 128
	}
	if err := p.retainProof(retained); err != nil {
		return nil, err
	}
	p.resolved[key] = physical
	return physical, nil
}

// Aggregate responses do not necessarily carry hit index names. Detect alias
// and rollover inventory drift before releasing data as well as validating hit
// sources. This detects observed drift, not administrator ABA changes; native
// grants and controlled provisioning remain part of the deployment contract.
func (p *queryProof) verifySources() error {
	if err := p.t.ready(); err != nil {
		return err
	}
	for expression, before := range p.resolved {
		_, after, err := p.t.resolve(p.ctx, p.endpoint, strings.Split(expression, ","))
		if err != nil {
			return err
		}
		if len(before) != len(after) {
			return failure("scope_denied", "source inventory changed during request; retry explicitly")
		}
		for name := range before {
			if !after[name] {
				return failure("scope_denied", "source inventory changed during request; retry explicitly")
			}
		}
	}
	if p.enrich != nil {
		if err := p.verifyEnrich(); err != nil {
			return err
		}
	}
	return nil
}

func decodeObject(raw []byte) (map[string]any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var obj map[string]any
	if err := d.Decode(&obj); err != nil || obj == nil {
		return nil, failure("invalid_request", "native body must be a JSON object")
	}
	return obj, nil
}

func (p *queryProof) prepare(request core.ElasticsearchQueryRequest) (*preparedQuery, error) {
	if err := queryOperation(p.t.cfg.Elasticsearch.Version, request.Operation); err != nil {
		return nil, err
	}
	if err := p.stockQueryProfile(); err != nil {
		return nil, err
	}
	indices := request.Indices
	if len(indices) == 0 {
		indices = p.t.cfg.Elasticsearch.AllowedIndices
	}
	p.activeIndices = indices
	// Validate syntactic scope before any body-driven upstream proof.
	for _, name := range indices {
		if err := scopePattern(p.ctx, name, p.t.cfg.Elasticsearch); err != nil {
			return nil, err
		}
	}
	rows := request.MaxRows
	if rows == 0 {
		rows = p.t.limits.MaxRows
	}
	if rows < 1 || rows > p.t.limits.MaxRows {
		return nil, failure("resource_limit", "max_rows exceeds configured page limit")
	}
	options, err := queryOptions(p.t.cfg.Elasticsearch.Version, request.Operation, request.Options)
	if err != nil {
		return nil, err
	}
	raw := request.Body
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	if err := strictJSONContext(p.ctx, raw, p.t.cfg.Elasticsearch.MaxJSONDepth, p.t.cfg.Elasticsearch.MaxJSONNodes, 0); err != nil {
		return nil, err
	}
	body, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	base := "/" + url.PathEscape(strings.Join(indices, ","))
	q := &preparedQuery{request: request, options: options, method: http.MethodPost, rows: rows, buckets: p.t.cfg.Elasticsearch.MaxAggregationBuckets}
	q.request.Indices = append([]string(nil), indices...)
	switch request.Operation {
	case "esql.query":
		if err := p.prepareESQL(q, body); err != nil {
			return nil, err
		}
		indices = q.request.Indices
		q.path = "/_query"
	case "sql.query", "sql.translate":
		if err := p.prepareSQL(q, body); err != nil {
			return nil, err
		}
		indices = q.request.Indices
		q.path = "/_sql"
		if request.Operation == "sql.translate" {
			q.path += "/translate"
		}
	case "eql.search":
		if err := p.prepareEQL(q, body); err != nil {
			return nil, err
		}
		q.path = base + "/_eql/search"
	case "search", "search_template", "render_search_template":
		if request.Operation != "search" {
			body, err = p.render(body)
			if err != nil {
				return nil, err
			}
		}
		for _, key := range []string{"size", "from", "timeout"} {
			if value := options.Get(key); value != "" {
				if _, exists := body[key]; exists {
					return nil, failure("invalid_request", "native body and options both define a pagination/deadline field")
				}
				if key == "timeout" {
					body[key] = value
				} else {
					n, err := strconv.ParseInt(value, 10, 64)
					if err != nil || n < 0 {
						return nil, failure("invalid_request", "pagination option must be a non-negative integer")
					}
					body[key] = json.Number(strconv.FormatInt(n, 10))
				}
				options.Del(key)
			}
		}
		if _, err := p.sourceMappings(indices); err != nil {
			return nil, err
		}
		if err := p.search(body, rows, 0); err != nil {
			return nil, err
		}
		q.path = base + "/_search"
	case "count", "indices.validate_query", "explain":
		if _, err := p.sourceMappings(indices); err != nil {
			return nil, err
		}
		if err := fieldsOnly(body, "query"); err != nil {
			return nil, err
		}
		if v, ok := body["query"]; ok {
			if err := p.query(v, rows, 0); err != nil {
				return nil, err
			}
		}
		q.path = base + "/_count"
		if request.Operation == "indices.validate_query" {
			q.path = base + "/_validate/query"
		}
		if request.Operation == "explain" {
			q.path = base + "/_explain/" + url.PathEscape(request.ID)
		}
	case "get":
		if len(body) != 0 {
			return nil, failure("invalid_request", "get accepts document id and options, not a body")
		}
		q.path, q.method = base+"/_doc/"+url.PathEscape(request.ID), http.MethodGet
	case "mget", "mtermvectors":
		if request.Operation == "mtermvectors" {
			body, err = normalizeTermVectors(raw, body)
			if err != nil {
				return nil, err
			}
		}
		if err := p.documents(body, indices, rows, request.Operation); err != nil {
			return nil, err
		}
		q.path = base + "/_" + request.Operation
	case "termvectors":
		if err := fieldsOnly(body, "_index _id doc fields offsets positions payloads term_statistics field_statistics per_field_analyzer filter routing version version_type termStatistics fieldStatistics perFieldAnalyzer"); err != nil {
			return nil, err
		}
		if _, ok := body["_index"]; ok {
			if err := p.reference(body, "_index", ""); err != nil {
				return nil, err
			}
		}
		q.path = base + "/_termvectors"
		if request.ID != "" {
			q.path += "/" + url.PathEscape(request.ID)
		}
	case "field_caps":
		if err := fieldsOnly(body, "fields index_filter runtime_mappings"); err != nil {
			return nil, err
		}
		if v, ok := body["index_filter"]; ok {
			if err := p.query(v, rows, 0); err != nil {
				return nil, err
			}
		}
		if v, ok := body["runtime_mappings"]; ok {
			if err := p.runtime(v); err != nil {
				return nil, err
			}
		}
		q.path = base + "/_field_caps"
	case "search_shards":
		if len(body) != 0 {
			return nil, failure("invalid_request", "search_shards does not accept a body")
		}
		q.path, q.method = base+"/_search_shards", http.MethodGet
	default:
		return nil, failure("capability_unavailable", "query handler is not implemented")
	}
	needsID := request.Operation == "get" || request.Operation == "explain"
	if needsID || request.Operation == "termvectors" {
		if len(indices) != 1 || !concreteName(indices[0]) {
			return nil, failure("invalid_request", "document operation requires one concrete index or alias")
		}
		if (needsID && request.ID == "") || len(request.ID) > 1024 {
			return nil, failure("invalid_request", "document id is missing or exceeds limit")
		}
	} else if request.ID != "" {
		return nil, failure("invalid_request", "id is not valid for this operation")
	}
	if (q.sql != nil || q.esql != nil) && len(indices) == 0 {
		q.physical = map[string]bool{}
	} else if p.snapshot != nil && scopeKey(indices) == scopeKey(p.snapshot.query.Indices) {
		q.physical = p.snapshot.physical
	} else {
		q.physical, err = p.sources(indices)
	}
	if err != nil {
		return nil, err
	}
	// Multi-document responses can contain member-specific sources.
	if request.Operation == "mget" || request.Operation == "mtermvectors" || request.Operation == "termvectors" {
		q.physical = p.physical
	}
	if options.Get("q") != "" {
		if err := p.localTextFields("query_string", map[string]any{}); err != nil {
			return nil, err
		}
	}
	if request.Operation == "render_search_template" {
		body = map[string]any{"template_output": body}
	}
	q.body, err = json.Marshal(body)
	if err != nil {
		return nil, failure("invalid_request", "cannot encode native body")
	}
	if len(q.body) > p.t.cfg.Elasticsearch.MaxRequestBytes {
		return nil, failure("resource_limit", "materialized query exceeds request byte limit")
	}
	if request.Operation == "search" || request.Operation == "search_template" {
		if err := searchTimeout(q, time.Until(deadline(p.ctx)).Milliseconds()); err != nil {
			return nil, err
		}
	}
	if q.method == http.MethodGet {
		q.body = nil
	}
	return q, nil
}

func fieldsOnly(m map[string]any, allowed string) error {
	set := map[string]bool{}
	for _, key := range strings.Fields(allowed) {
		set[key] = true
	}
	for key := range m {
		if !set[key] {
			return failure("capability_unavailable", "native field has no effect/source visitor in this profile")
		}
	}
	return nil
}

func queryOptions(version, operation string, options map[string]json.RawMessage) (url.Values, error) {
	api := operation
	if api == "search_template" || api == "render_search_template" {
		api = "search"
	}
	allowed := map[string]bool{}
	for _, name := range catalog[version][api].Parameters {
		allowed[name] = true
	}
	result := url.Values{}
	for name, raw := range options {
		if !allowed[name] {
			return nil, failure("invalid_request", "unknown option for the pinned operation")
		}
		if name == "scroll" {
			return nil, failure("capability_unavailable", "scroll requires a service-owned context")
		}
		if name == "source" || name == "source_content_type" {
			return nil, failure("invalid_request", "native body must use the body field")
		}
		if err := strictJSON(raw, 8, 1000); err != nil {
			return nil, err
		}
		var value any
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if d.Decode(&value) != nil {
			return nil, failure("invalid_request", "invalid option value")
		}
		var text string
		switch v := value.(type) {
		case string:
			text = v
		case json.Number:
			text = string(v)
		case bool:
			text = strconv.FormatBool(v)
		case []any:
			var parts []string
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					return nil, failure("invalid_request", "option arrays must contain strings")
				}
				parts = append(parts, s)
			}
			text = strings.Join(parts, ",")
		default:
			return nil, failure("invalid_request", "options must contain scalar values or string arrays")
		}
		if name == "refresh" && text != "false" {
			return nil, failure("mutation_forbidden", "refresh changes index visibility and is not a document read")
		}
		if name == "allow_partial_search_results" && text != "false" {
			return nil, failure("capability_unavailable", "partial search results are not enabled")
		}
		result.Set(name, text)
	}
	if api == "search" {
		result.Set("allow_partial_search_results", "false")
	}
	return result, nil
}
