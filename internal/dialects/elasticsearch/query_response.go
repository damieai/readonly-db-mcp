package elasticsearch

import "encoding/json"

func validateQueryResponse(raw []byte, q *preparedQuery) error {
	m, err := decodeObject(raw)
	if err != nil {
		return failure("invalid_response", "expected native response object")
	}
	if _, ok := m["error"]; ok {
		return failure("upstream_error", "Elasticsearch returned an item error")
	}
	if err := completeResponse(m); err != nil {
		return err
	}
	checkIndex := func(m map[string]any) error {
		index, ok := m["_index"].(string)
		if !ok || !q.physical[index] {
			return failure("scope_denied", "document response escaped the resolved index inventory")
		}
		return nil
	}
	count := 0
	var hits func(any) error
	hits = func(v any) error {
		h, err := object(v)
		if err != nil {
			return failure("invalid_response", "invalid native hit collection")
		}
		items, ok := h["hits"].([]any)
		if !ok {
			return failure("invalid_response", "native hits array is missing")
		}
		for _, v := range items {
			hit, err := object(v)
			if err != nil {
				return err
			}
			if err := checkIndex(hit); err != nil {
				return err
			}
			count++
			if count > q.rows {
				return failure("resource_limit", "combined returned hits exceed max_rows; request smaller outer/inner/top_hits pages")
			}
			if inner, ok := hit["inner_hits"].(map[string]any); ok {
				for _, v := range inner {
					child, err := object(v)
					if err != nil {
						return err
					}
					if err := hits(child["hits"]); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	buckets := 0
	var aggregation func(map[string]any) error
	aggregation = func(m map[string]any) error {
		_, isBucket := m["doc_count"]
		for key, v := range m {
			switch key {
			case "meta", "value", "values", "key", "key_as_string", "after_key":
				if !isBucket || key == "key" || key == "key_as_string" {
					continue
				} // metric/script data are opaque; bucket child names are not control fields
			case "hits":
				if h, ok := v.(map[string]any); ok {
					if _, ok := h["hits"].([]any); ok {
						if err := hits(h); err != nil {
							return err
						}
						continue
					}
				}
			case "buckets":
				switch b := v.(type) {
				case []any:
					buckets += len(b)
					for _, v := range b {
						if child, ok := v.(map[string]any); ok {
							if err := aggregation(child); err != nil {
								return err
							}
						}
					}
				case map[string]any:
					buckets += len(b)
					for _, v := range b {
						if child, ok := v.(map[string]any); ok {
							if err := aggregation(child); err != nil {
								return err
							}
						}
					}
				}
				if buckets > q.buckets {
					return failure("resource_limit", "returned aggregation buckets exceed profile limit")
				}
				continue
			}
			if child, ok := v.(map[string]any); ok {
				if err := aggregation(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	switch q.request.Operation {
	case "search", "search_template":
		if _, ok := m["_shards"]; !ok {
			return failure("invalid_response", "native shard outcome is missing")
		}
		if _, ok := m["timed_out"].(bool); !ok {
			return failure("invalid_response", "native timeout outcome is missing")
		}
		if err := hits(m["hits"]); err != nil {
			return err
		}
		if aggs, ok := m["aggregations"].(map[string]any); ok {
			for _, v := range aggs {
				if node, ok := v.(map[string]any); ok {
					if err := aggregation(node); err != nil {
						return err
					}
				}
			}
		}
		if suggest, ok := m["suggest"].(map[string]any); ok {
			for _, v := range suggest {
				for _, v := range list(v) {
					s, ok := v.(map[string]any)
					if !ok {
						continue
					}
					if options, ok := s["options"].([]any); ok {
						for _, v := range options {
							hit, ok := v.(map[string]any)
							if !ok {
								continue
							}
							if _, ok := hit["_index"]; ok {
								if err := checkIndex(hit); err != nil {
									return err
								}
								count++
								if count > q.rows {
									return failure("resource_limit", "returned suggestions exceed max_rows")
								}
							}
						}
					}
				}
			}
		}
	case "get", "termvectors", "explain":
		if err := checkIndex(m); err != nil {
			return err
		}
		if q.request.Operation == "get" {
			if _, ok := m["found"].(bool); !ok {
				return failure("invalid_response", "document found status missing")
			}
		}
	case "mget", "mtermvectors":
		docs, ok := m["docs"].([]any)
		if !ok {
			return failure("invalid_response", "document array missing")
		}
		if len(docs) > q.rows {
			return failure("resource_limit", "returned document count exceeds max_rows")
		}
		for _, v := range docs {
			doc, err := object(v)
			if err != nil {
				return err
			}
			if _, ok := doc["error"]; ok {
				return failure("upstream_error", "Elasticsearch document member failed")
			}
			if err := checkIndex(doc); err != nil {
				return err
			}
		}
	case "field_caps":
		if indices, ok := m["indices"].([]any); ok {
			for _, v := range indices {
				name, ok := v.(string)
				if !ok || !q.physical[name] {
					return failure("scope_denied", "field capabilities escaped resolved inventory")
				}
			}
		}
	case "search_shards":
		if groups, ok := m["shards"].([]any); ok {
			for _, group := range groups {
				for _, v := range list(group) {
					shard, err := object(v)
					if err != nil {
						return err
					}
					name, ok := shard["index"].(string)
					if !ok || !q.physical[name] {
						return failure("scope_denied", "shard response escaped resolved inventory")
					}
				}
			}
		}
	}
	return nil
}

func completeResponse(m map[string]any) error {
	for _, key := range []string{"timed_out", "terminated_early"} {
		if v, ok := m[key]; ok {
			b, ok := v.(bool)
			if !ok {
				return failure("invalid_response", "invalid search completion flag")
			}
			if b {
				return failure("partial_result", "Elasticsearch returned incomplete results")
			}
		}
	}
	if v, ok := m["_shards"]; ok {
		shards, err := object(v)
		if err != nil {
			return err
		}
		failed, ok := shards["failed"].(json.Number)
		if !ok {
			return failure("invalid_response", "shard failure count missing")
		}
		n, err := failed.Int64()
		if err != nil || n < 0 {
			return failure("invalid_response", "invalid shard failure count")
		}
		if n > 0 {
			return failure("partial_result", "Elasticsearch returned failed shards")
		}
	}
	return nil
}
