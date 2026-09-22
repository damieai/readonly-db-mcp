package elasticsearch

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/your-org/readonly-db-mcp/internal/dialects/elasticsearch/eqlparser"
)

func (p *queryProof) prepareEQL(q *preparedQuery, body map[string]any) error {
	if err := fieldsOnly(body, "query filter timestamp_field tiebreaker_field event_category_field size fetch_size fields runtime_mappings result_position max_samples_per_key keep_alive keep_on_completion wait_for_completion_timeout allow_partial_search_results allow_partial_sequence_results"); err != nil {
		return err
	}
	query, ok := body["query"].(string)
	if !ok || query == "" {
		return failure("invalid_request", "EQL requires a non-empty query string")
	}
	settings := p.t.cfg.Elasticsearch
	analysis, err := eqlparser.Analyze(p.ctx, settings.Version, query, eqlparser.Limits{Bytes: settings.MaxLanguageBytes, Tokens: settings.MaxLanguageTokens, Depth: settings.MaxLanguageDepth})
	if err != nil {
		var e *eqlparser.Error
		if errors.As(err, &e) {
			return failure(e.Code, "EQL analysis failed its grammar or resource contract")
		}
		return failure("invalid_request", "EQL analysis failed")
	}
	if analysis.SourceMode != "request_indices" {
		return failure("capability_unavailable", "unrecognized EQL source contract")
	}
	q.eqlKind = analysis.Kind
	// The pinned transport selects async execution for non-negative wait times.
	// Do not inject the MCP deadline here: that would create persisted results.
	// Omitted wait or exactly -1 chooses the synchronous branch.
	for _, name := range []string{"wait_for_completion_timeout", "keep_on_completion", "allow_partial_search_results", "allow_partial_sequence_results"} {
		value, present := body[name]
		if present {
			if err := eqlControl(name, value); err != nil {
				return err
			}
		}
		if values, exists := q.options[name]; exists {
			if present {
				return failure("invalid_request", "EQL control is specified in both body and options")
			}
			if len(values) != 1 {
				return failure("invalid_request", "invalid EQL option")
			}
			var value any = values[0]
			if name != "wait_for_completion_timeout" {
				b, err := strconv.ParseBool(values[0])
				if err != nil {
					return failure("invalid_request", "EQL completion options must be boolean")
				}
				value = b
			}
			if err := eqlControl(name, value); err != nil {
				return err
			}
		}
	}
	if _, present := body["keep_alive"]; present && q.options.Has("keep_alive") {
		return failure("invalid_request", "EQL keep_alive is specified twice")
	}
	// Always reject shard failures even when the cluster default permits them.
	for _, key := range []string{"allow_partial_search_results", "allow_partial_sequence_results"} {
		if _, ok := body[key]; !ok {
			q.options.Set(key, "false")
		}
	}
	if err := pageSize(body, q.rows, 10); err != nil {
		return err
	}
	if err := eqlInteger(body, "fetch_size", 1000, 2, settings.MaxEQLFetchSize); err != nil {
		return err
	}
	if err := eqlInteger(body, "max_samples_per_key", 1, 1, q.rows); err != nil {
		return err
	}
	if _, err := p.sourceMappings(q.request.Indices); err != nil {
		return err
	}
	if filter, ok := body["filter"]; ok {
		if err := p.query(filter, q.rows, 0); err != nil {
			return err
		}
	}
	if runtime, ok := body["runtime_mappings"]; ok {
		if err := p.runtime(runtime); err != nil {
			return err
		}
	}
	return nil
}

func eqlControl(name string, value any) error {
	if name == "wait_for_completion_timeout" {
		if value == "-1" {
			return nil
		}
		return failure("capability_unavailable", "asynchronous EQL requires a persisted-result lifecycle profile; omit wait_for_completion_timeout or use -1 for synchronous execution")
	}
	b, ok := value.(bool)
	if !ok {
		return failure("invalid_request", "EQL completion controls must be booleans")
	}
	if !b {
		return nil
	}
	if name == "keep_on_completion" {
		return failure("mutation_forbidden", "retaining asynchronous EQL results is outside the read-only profile")
	}
	return failure("capability_unavailable", "partial EQL results are not enabled")
}

func eqlInteger(body map[string]any, key string, fallback, minimum, maximum int) error {
	value := int64(fallback)
	if raw, ok := body[key]; ok {
		n, ok := raw.(json.Number)
		if !ok {
			return failure("invalid_request", "EQL size settings must be integers")
		}
		var err error
		value, err = n.Int64()
		if err != nil {
			return failure("invalid_request", "invalid EQL size setting")
		}
	}
	if value < int64(minimum) || value > int64(maximum) {
		return failure("resource_limit", "EQL size setting exceeds its configured resource limit")
	}
	return nil
}

// EQL sequences/samples retain their native grouping, join keys and missing
// event placeholders. Enforce total event volume without cutting a sequence.
func validateEQLResponse(root map[string]any, q *preparedQuery) error {
	if _, exists := root["id"]; exists {
		return failure("profile_mismatch", "synchronous EQL unexpectedly returned a persisted-result identifier")
	}
	for _, key := range []string{"timed_out", "is_partial", "is_running"} {
		flag, ok := root[key].(bool)
		if !ok {
			return failure("invalid_response", "EQL completion flag is missing or malformed")
		}
		if flag {
			if key == "is_running" {
				return failure("profile_mismatch", "synchronous EQL returned running work")
			}
			return failure("partial_result", "EQL returned incomplete results")
		}
	}
	if failures, ok := root["shard_failures"]; ok {
		items, ok := failures.([]any)
		if !ok {
			return failure("invalid_response", "invalid EQL shard failure collection")
		}
		if len(items) > 0 {
			return failure("partial_result", "EQL returned shard failures")
		}
	}
	hits, err := object(root["hits"])
	if err != nil {
		return failure("invalid_response", "EQL hits are missing")
	}
	count := 0
	validateEvents := func(value any, allowMissing bool) error {
		events, ok := value.([]any)
		if !ok {
			return failure("invalid_response", "EQL event collection is missing")
		}
		for _, value := range events {
			count++
			if count > q.rows {
				return failure("resource_limit", "complete EQL event/sequence result exceeds max_rows")
			}
			event, err := object(value)
			if err != nil {
				return failure("invalid_response", "invalid EQL event")
			}
			missing := false
			if value, ok := event["missing"]; ok {
				var valid bool
				missing, valid = value.(bool)
				if !valid {
					return failure("invalid_response", "invalid missing-event marker")
				}
			}
			if missing {
				// Native missing events contain empty identity/source. A missing
				// marker cannot exempt real payload from index authorization.
				source, ok := event["_source"].(map[string]any)
				if !allowMissing || !ok || len(source) != 0 || event["_index"] != "" || event["_id"] != "" || len(event) != 4 {
					return failure("scope_denied", "invalid EQL missing-event placeholder")
				}
				continue
			}
			index, ok := event["_index"].(string)
			if !ok || !q.physical[index] {
				return failure("scope_denied", "EQL event escaped the resolved index inventory")
			}
			if _, ok := event["_id"].(string); !ok {
				return failure("invalid_response", "EQL event identity is missing")
			}
		}
		return nil
	}
	if q.eqlKind == "event" {
		if _, exists := hits["sequences"]; exists {
			return failure("invalid_response", "event query returned sequence results")
		}
		return validateEvents(hits["events"], false)
	}
	if _, exists := hits["events"]; exists {
		return failure("invalid_response", "sequence query returned ungrouped events")
	}
	groups, ok := hits["sequences"].([]any)
	if !ok {
		return failure("invalid_response", "EQL sequence collection is missing")
	}
	if len(groups) > q.rows {
		return failure("resource_limit", "EQL sequence count exceeds max_rows")
	}
	for _, value := range groups {
		group, err := object(value)
		if err != nil {
			return failure("invalid_response", "invalid EQL sequence")
		}
		if err := validateEvents(group["events"], q.eqlKind == "sequence"); err != nil {
			return err
		}
	}
	return nil
}
