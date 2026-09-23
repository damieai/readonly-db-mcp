package elasticsearch

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/dialects/elasticsearch/esqlparser"
)

type esqlProfile struct{ columnar bool }

func (p *queryProof) prepareESQL(q *preparedQuery, body map[string]any) error {
	// RequestXContent's synchronous parser has no persisted-result controls.
	if err := fieldsOnly(body, "query params columnar filter locale profile include_ccs_metadata pragma"); err != nil {
		return err
	}
	text, ok := body["query"].(string)
	if !ok || text == "" {
		return failure("invalid_request", "ES|QL requires a non-empty query string")
	}
	var params []any
	if raw, exists := body["params"]; exists {
		params, ok = raw.([]any)
		if !ok {
			return failure("invalid_request", "ES|QL params must be a native parameter array")
		}
	}
	if len(params) > p.t.limits.MaxParameters {
		return failure("resource_limit", "ES|QL parameter count exceeds the configured limit")
	}
	cfg := p.t.cfg.Elasticsearch
	if pragma, exists := body["pragma"]; exists {
		if err := validateESQLPragmas(pragma, cfg.Version, cfg.ESQLPragmas); err != nil {
			return err
		}
		// Both pinned release builds require this flag for non-empty pragmas.
		// The caller cannot set it independently of the operator profile.
		body["accept_pragma_risks"] = true
	}
	a, err := esqlparser.Analyze(p.ctx, cfg.Version, text, params, esqlparser.Limits{Bytes: cfg.MaxLanguageBytes, Tokens: cfg.MaxLanguageTokens, Depth: cfg.MaxLanguageDepth})
	if err != nil {
		var e *esqlparser.Error
		if errors.As(err, &e) {
			return failure(e.Code, "ES|QL analysis failed its grammar or resource contract")
		}
		return failure("invalid_request", "ES|QL analysis failed")
	}
	if a.Inference {
		return failure("capability_unavailable", "ES|QL inference commands require an endpoint, egress and cost authority profile")
	}
	q.esql = &esqlProfile{}
	for _, key := range []string{"columnar", "profile", "include_ccs_metadata"} {
		if value, exists := body[key]; exists {
			flag, ok := value.(bool)
			if !ok {
				return failure("invalid_request", "ES|QL boolean control has an invalid type")
			}
			if key == "columnar" {
				q.esql.columnar = flag
			}
		}
	}
	if locale, exists := body["locale"]; exists {
		if _, ok := locale.(string); !ok {
			return failure("invalid_request", "ES|QL locale must be a language tag string")
		}
	}
	if format := q.options.Get("format"); format != "" && format != "json" {
		return failure("invalid_request", "ES|QL uses structured JSON results; format must be json")
	}
	q.options.Set("format", "json")
	if partial := q.options.Get("allow_partial_results"); partial != "" && partial != "false" {
		return failure("capability_unavailable", "ES|QL requires complete results")
	}
	q.options.Set("allow_partial_results", "false")
	if value := q.options.Get("drop_null_columns"); value != "" && value != "true" && value != "false" {
		return failure("invalid_request", "drop_null_columns must be a boolean")
	}
	indices := []string{}
	seen := map[string]bool{}
	for _, group := range a.SourceGroups {
		hasWildcard := false
		for _, source := range group {
			if strings.HasPrefix(source, "-") {
				// Native exclusion syntax can only remove matches from preceding
				// wildcard sources in this relation. Prove the entire positive
				// superset; preserve the exclusion in the original native query.
				if !hasWildcard || !config.ValidElasticsearchPattern(strings.TrimPrefix(source, "-")) {
					return failure("scope_denied", "ES|QL exclusion has no proved local wildcard source")
				}
				continue
			}
			name := strings.TrimSuffix(source, "::data")
			if !dateMathSource(name) && strings.Contains(name, ":") {
				return failure("capability_unavailable", "ES|QL remote or backing-index selector requires a separate authority profile")
			}
			if err := requestSourcePattern(p.ctx, name, cfg); err != nil {
				return err
			}
			if len(q.request.Indices) > 0 {
				allowed, err := p.requestedSource(name, q.request.Indices)
				if err != nil {
					return err
				}
				if !allowed {
					return failure("scope_denied", "ES|QL source is outside the requested index scope")
				}
			}
			hasWildcard = hasWildcard || strings.Contains(name, "*")
			if !seen[name] {
				indices = append(indices, name)
				seen[name] = true
			}
		}
	}
	if len(indices) > cfg.MaxResolvedIndices {
		return failure("resource_limit", "ES|QL source count exceeds its configured limit")
	}
	q.request.Indices = indices
	p.activeIndices = indices
	if len(indices) > 0 {
		root, err := p.sourceMappings(indices)
		if err != nil {
			return err
		}
		if a.FullText {
			// These are ES|QL identifiers, not DSL field^boost expressions.
			// Keep punctuation literal when checking their possible lineage.
			for _, value := range root {
				index, _ := value.(map[string]any)
				mapping, _ := index["mappings"].(map[string]any)
				if err := semanticMappingFields(p, mapping, a.FullTextFields); err != nil {
					return err
				}
			}
		}
	}
	if filter, exists := body["filter"]; exists {
		if err := p.query(filter, q.rows, 0); err != nil {
			return err
		}
	}
	if len(a.EnrichPolicies) != 0 {
		if err := p.prepareEnrich(a.EnrichPolicies); err != nil {
			return err
		}
	}
	return nil
}

func validateESQLResponse(m map[string]any, q *preparedQuery) error {
	for _, key := range []string{"id", "cursor", "is_running"} {
		if _, exists := m[key]; exists {
			return failure("profile_mismatch", "synchronous ES|QL unexpectedly returned asynchronous or cursor state")
		}
	}
	partial, ok := m["is_partial"].(bool)
	if !ok {
		return failure("invalid_response", "ES|QL completion outcome is missing")
	}
	if partial {
		return failure("partial_result", "Elasticsearch returned incomplete ES|QL results")
	}
	if _, exists := m["_clusters"]; exists {
		// Local ES|QL does not expose cross-cluster execution metadata.
		return failure("profile_mismatch", "ES|QL unexpectedly returned cross-cluster execution metadata")
	}
	columns, ok := m["columns"].([]any)
	if !ok {
		return failure("invalid_response", "ES|QL columns are missing")
	}
	checkColumns := func(columns []any) error {
		for _, column := range columns {
			c, ok := column.(map[string]any)
			if !ok {
				return failure("invalid_response", "invalid ES|QL column")
			}
			name, nok := c["name"].(string)
			kind, kok := c["type"].(string)
			if !nok || !kok || name == "" || kind == "" {
				return failure("invalid_response", "invalid ES|QL column metadata")
			}
		}
		return nil
	}
	if err := checkColumns(columns); err != nil {
		return err
	}
	if raw, exists := m["all_columns"]; exists {
		all, ok := raw.([]any)
		if !ok || len(all) < len(columns) {
			return failure("invalid_response", "invalid ES|QL all_columns metadata")
		}
		if err := checkColumns(all); err != nil {
			return err
		}
	}
	values, ok := m["values"].([]any)
	if !ok {
		return failure("invalid_response", "ES|QL values are missing")
	}
	rows := len(values)
	if q.esql.columnar {
		if len(columns) == 0 && q.options.Get("drop_null_columns") == "true" {
			return failure("capability_unavailable", "ES|QL columnar output with every column dropped cannot prove row count; retain null columns or use row output")
		}
		if len(values) != len(columns) {
			return failure("invalid_response", "ES|QL columnar width differs from its metadata")
		}
		rows = 0
		for i, value := range values {
			column, ok := value.([]any)
			if !ok || (i > 0 && len(column) != rows) {
				return failure("invalid_response", "ES|QL columnar heights differ")
			}
			rows = len(column)
		}
	} else {
		for _, value := range values {
			row, ok := value.([]any)
			if !ok || len(row) != len(columns) {
				return failure("invalid_response", "ES|QL row width differs from its metadata")
			}
		}
	}
	if rows > q.rows {
		return failure("resource_limit", "ES|QL results exceed max_rows; use a native LIMIT or aggregation")
	}
	if took, ok := m["took"].(json.Number); !ok {
		return failure("invalid_response", "ES|QL execution time is missing")
	} else if n, err := took.Int64(); err != nil || n < 0 {
		return failure("invalid_response", "invalid ES|QL execution time")
	}
	return nil
}
