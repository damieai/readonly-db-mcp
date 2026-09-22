package elasticsearch

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/dialects/elasticsearch/sqlparser"
)

type sqlProfile struct {
	columnar      bool
	keep, timeout time.Duration
	proof         *queryProof
}

func (p *queryProof) prepareSQL(q *preparedQuery, body map[string]any) error {
	fields := "query params time_zone catalog fetch_size request_timeout page_timeout filter mode client_id version runtime_mappings"
	query := q.request.Operation == "sql.query"
	if query {
		fields += " columnar field_multi_value_leniency index_include_frozen binary_format wait_for_completion_timeout keep_on_completion keep_alive allow_partial_search_results"
	}
	if err := fieldsOnly(body, fields); err != nil {
		return err
	}
	text, ok := body["query"].(string)
	if !ok || text == "" {
		return failure("invalid_request", "SQL requires a non-empty query string")
	}
	params, err := sqlParams(body)
	if err != nil {
		return err
	}
	if len(params) > p.t.limits.MaxParameters {
		return failure("resource_limit", "SQL parameter count exceeds the configured limit")
	}
	cfg := p.t.cfg.Elasticsearch
	a, err := sqlparser.Analyze(p.ctx, cfg.Version, text, params, sqlparser.Limits{Bytes: cfg.MaxLanguageBytes, Tokens: cfg.MaxLanguageTokens, Depth: cfg.MaxLanguageDepth})
	if err != nil {
		var e *sqlparser.Error
		if errors.As(err, &e) {
			return failure(e.Code, "SQL analysis failed its grammar or resource contract")
		}
		return failure("invalid_request", "SQL analysis failed")
	}
	q.sql = &sqlProfile{keep: cfg.ContextKeepAlive, proof: p}
	if value, exists := body["catalog"]; exists {
		s, ok := value.(string)
		if !ok {
			return failure("invalid_request", "SQL catalog must be a string")
		}
		a.Catalogs = append(a.Catalogs, sqlparser.Catalog{Value: s})
	}
	for _, c := range a.Catalogs {
		if err := p.sqlCatalog(c.Value); err != nil {
			return err
		}
	}
	var indices []string
	seen := map[string]bool{}
	for _, source := range a.Sources {
		if err := p.sqlCatalog(source.Catalog); err != nil {
			return err
		}
		name := source.Index
		// The default data selector denotes the same local source. Other selectors
		// require their own backing-index authority proof, not a silent rewrite.
		name = strings.TrimSuffix(name, "::data")
		if strings.Contains(name, "::") {
			return failure("capability_unavailable", "SQL source selector needs a backing-index authority profile")
		}
		if len(q.request.Indices) > 0 {
			allowed, err := globRelation(p.ctx, []string{name}, q.request.Indices, false)
			if err != nil {
				return err
			}
			if !allowed {
				return failure("scope_denied", "SQL source is outside the requested index scope")
			}
		}
		if !seen[name] {
			indices = append(indices, name)
			seen[name] = true
		}
	}
	if len(indices) > cfg.MaxResolvedIndices {
		return failure("resource_limit", "SQL source count exceeds its configured limit")
	}
	q.request.Indices = indices
	p.activeIndices = indices
	if len(indices) > 0 {
		if _, err := p.sourceMappings(indices); err != nil {
			return err
		}
		if len(a.FullTextFields) > 0 {
			fields := make([]any, len(a.FullTextFields))
			for i, field := range a.FullTextFields {
				fields[i] = field
			}
			if err := p.localTextFields("multi_match", map[string]any{"fields": fields}); err != nil {
				return err
			}
		}
	}
	if value, ok := body["filter"]; ok {
		if err := p.query(value, q.rows, 0); err != nil {
			return err
		}
	}
	if value, ok := body["runtime_mappings"]; ok {
		if err := p.runtime(value); err != nil {
			return err
		}
	}
	if _, exists := body["fetch_size"]; !exists {
		body["fetch_size"] = json.Number(strconv.Itoa(min(1000, q.rows)))
	}
	if err := languageInteger(body, "fetch_size", 1000, 1, q.rows); err != nil {
		return err
	}
	if value, exists := body["page_timeout"]; exists {
		d, err := sqlDuration(value)
		if err != nil {
			return err
		}
		if d < time.Millisecond || d > 15*time.Minute {
			return failure("resource_limit", "SQL page_timeout must be between 1ms and 15m")
		}
		q.sql.keep = d
	}
	if value, exists := body["request_timeout"]; exists {
		if value != "-1" {
			d, err := sqlDuration(value)
			if err != nil {
				return err
			}
			if d <= 0 {
				return failure("invalid_request", "SQL request_timeout must be positive or -1")
			}
			q.sql.timeout = d
		}
	}
	if query {
		if format := q.options.Get("format"); format != "" && format != "json" {
			return failure("invalid_request", "SQL uses structured JSON results; format must be json")
		}
		q.options.Set("format", "json")
		for _, name := range []string{"columnar", "field_multi_value_leniency", "index_include_frozen", "binary_format", "keep_on_completion", "allow_partial_search_results"} {
			if value, exists := body[name]; exists {
				b, ok := value.(bool)
				if !ok {
					return failure("invalid_request", "SQL boolean control has an invalid type")
				}
				if name == "columnar" {
					q.sql.columnar = b
				}
				if b && (name == "binary_format" || name == "allow_partial_search_results") {
					return failure("capability_unavailable", "SQL requires complete structured JSON responses")
				}
				if b && name == "keep_on_completion" {
					return failure("mutation_forbidden", "persisting asynchronous SQL results is outside the read-only profile")
				}
			}
		}
		if value, exists := body["wait_for_completion_timeout"]; exists && value != "-1" && value != json.Number("-1") {
			return failure("capability_unavailable", "asynchronous SQL requires a persisted-result lifecycle profile")
		}
		body["allow_partial_search_results"] = false
		body["binary_format"] = false
	}
	return nil
}

func sqlParams(body map[string]any) ([]any, error) {
	raw, exists := body["params"]
	if !exists {
		return nil, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, failure("invalid_request", "SQL params must be an array")
	}
	params := make([]any, len(values))
	for i, v := range values {
		if m, ok := v.(map[string]any); ok {
			if err := fieldsOnly(m, "type value"); err != nil {
				return nil, err
			}
			if _, ok := m["type"].(string); !ok {
				return nil, failure("invalid_request", "typed SQL parameter needs a type")
			}
			var exists bool
			v, exists = m["value"]
			if !exists {
				return nil, failure("invalid_request", "typed SQL parameter needs a value")
			}
		}
		switch v.(type) {
		case nil, string, bool, json.Number:
			params[i] = v
		default:
			return nil, failure("invalid_request", "SQL parameters must be native scalar values")
		}
	}
	return params, nil
}
func sqlDuration(value any) (time.Duration, error) {
	s, ok := value.(string)
	if !ok {
		return 0, failure("invalid_request", "SQL duration must be a native duration string")
	}
	// ES uses d, h, m, s, ms, micros and nanos; normalize only for accounting.
	s = strings.ReplaceAll(strings.ReplaceAll(s, "micros", "us"), "nanos", "ns")
	if strings.HasSuffix(s, "d") {
		n, e := strconv.ParseInt(strings.TrimSuffix(s, "d"), 10, 32)
		if e != nil || n < 0 || n > 100000 {
			return 0, failure("invalid_request", "invalid SQL duration")
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, e := time.ParseDuration(s)
	if e != nil {
		return 0, failure("invalid_request", "invalid SQL duration")
	}
	return d, nil
}
func (p *queryProof) sqlCatalog(name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	raw, err := p.t.wire.get(p.ctx, p.endpoint, "/", nil, p.t.limits.MaxResultBytes)
	if err != nil {
		return err
	}
	var root struct {
		ClusterName string `json:"cluster_name"`
		ClusterUUID string `json:"cluster_uuid"`
	}
	if json.Unmarshal(raw, &root) != nil || root.ClusterName == "" || root.ClusterUUID != p.t.cfg.Elasticsearch.ClusterUUID {
		return failure("profile_mismatch", "local SQL catalog identity is unavailable")
	}
	if name != root.ClusterName {
		return failure("capability_unavailable", "remote SQL catalogs require a cross-cluster authority profile")
	}
	return nil
}
func sqlTimeout(ctx context.Context, body map[string]any, requested time.Duration) error {
	left := time.Until(deadline(ctx))
	if left < time.Millisecond {
		return failure("deadline_exceeded", "SQL deadline expired before dispatch")
	}
	if requested > 0 {
		left = min(left, requested)
	}
	body["request_timeout"] = strconv.FormatInt(max(1, left.Milliseconds()), 10) + "ms"
	return nil
}
