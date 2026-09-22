package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func searchTimeout(q *preparedQuery, left int64) error {
	if left <= 0 {
		return failure("deadline_exceeded", "query deadline expired before dispatch")
	}
	m, err := decodeObject(q.body)
	if err != nil {
		return err
	}
	if value, ok := m["timeout"]; ok {
		text, ok := value.(string)
		if !ok {
			return failure("invalid_request", "native timeout must be a duration string")
		}
		d, err := time.ParseDuration(text)
		if err != nil || d < 0 {
			return failure("invalid_request", "native timeout must be a non-negative duration")
		}
		if ms := d.Milliseconds(); ms < left {
			left = ms
		}
	}
	m["timeout"] = strconv.FormatInt(left, 10) + "ms"
	q.body, _ = json.Marshal(m)
	return nil
}

// Homogeneous compatible searches use service-constructed NDJSON. Options
// without an msearch-header equivalent keep their semantics via sequential
// execution under the same batch lease; they are never silently discarded.
func (t *Target) multiSearch(ctx context.Context, endpoint int, queries []*preparedQuery) ([][]byte, bool, error) {
	headers := map[string]bool{"routing": true, "preference": true, "search_type": true, "request_cache": true, "allow_no_indices": true, "ignore_unavailable": true, "expand_wildcards": true, "allow_partial_search_results": true}
	typed := ""
	for i, q := range queries {
		if q.request.Operation != "search" && q.request.Operation != "search_template" {
			return nil, false, nil
		}
		for key := range q.options {
			if key != "typed_keys" && !headers[key] {
				return nil, false, nil
			}
		}
		if i == 0 {
			typed = q.options.Get("typed_keys")
		} else if typed != q.options.Get("typed_keys") {
			return nil, false, nil
		}
	}
	if err := t.ready(); err != nil {
		return nil, true, err
	}
	var body bytes.Buffer
	for _, q := range queries {
		if err := searchTimeout(q, time.Until(deadline(ctx)).Milliseconds()); err != nil {
			return nil, true, err
		}
		header := map[string]any{"index": q.request.Indices}
		for key, values := range q.options {
			if key == "typed_keys" {
				continue
			}
			value := values[0]
			switch key {
			case "allow_partial_search_results", "request_cache", "allow_no_indices", "ignore_unavailable":
				b, err := strconv.ParseBool(value)
				if err != nil {
					return nil, true, failure("invalid_request", "boolean search option is malformed")
				}
				header[key] = b
			default:
				header[key] = value
			}
		}
		raw, _ := json.Marshal(header)
		body.Write(raw)
		body.WriteByte('\n')
		body.Write(q.body)
		body.WriteByte('\n')
		if body.Len() > t.cfg.Elasticsearch.MaxRequestBytes {
			return nil, true, failure("resource_limit", "framed batch exceeds request byte limit")
		}
	}
	options := url.Values{}
	if typed != "" {
		options.Set("typed_keys", typed)
	}
	// Limit server-side fan-out independently of the number of members. One
	// public admission permit must not imply unbounded concurrent shard work.
	options.Set("max_concurrent_searches", "1")
	raw, err := t.wire.request(ctx, endpoint, http.MethodPost, "/_msearch", options, body.Bytes(), "application/x-ndjson", t.limits.MaxResultBytes, false)
	if err != nil {
		return nil, true, err
	}
	var result struct {
		Responses []json.RawMessage `json:"responses"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Responses) != len(queries) {
		return nil, true, failure("invalid_response", "msearch response count differs from request")
	}
	out := make([][]byte, len(queries))
	for i, data := range result.Responses {
		if err := validateQueryResponse(data, queries[i]); err != nil {
			return nil, true, err
		}
		out[i] = data
	}
	return out, true, nil
}
