package elasticsearch

import (
	"bytes"
	"encoding/json"
)

// Native MultiTermVectorsRequest applies parameters to subsequent docs, then
// applies the final template to ids. Ordinary map re-encoding would sort docs
// before parameters and silently change that behavior. Materialize each member
// in native order, so source proofs see every effective _index before dispatch.
func normalizeTermVectors(raw []byte, body map[string]any) (map[string]any, error) {
	if err := fieldsOnly(body, "docs ids parameters"); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	_, _ = d.Token()
	defaults := map[string]any{}
	var docs []any
	for d.More() {
		name, err := d.Token()
		if err != nil {
			return nil, failure("invalid_request", "invalid multi-term-vector body")
		}
		var ignored json.RawMessage
		if d.Decode(&ignored) != nil {
			return nil, failure("invalid_request", "invalid multi-term-vector body")
		}
		switch name {
		case "parameters":
			defaults, err = object(body["parameters"])
			if err != nil {
				return nil, err
			}
			if err := fieldsOnly(defaults, "_index _id doc fields offsets positions payloads term_statistics field_statistics per_field_analyzer filter routing version version_type termStatistics fieldStatistics perFieldAnalyzer"); err != nil {
				return nil, err
			}
		case "docs":
			items, ok := body["docs"].([]any)
			if !ok {
				return nil, failure("invalid_request", "docs must be an array")
			}
			for _, v := range items {
				member, err := object(v)
				if err != nil {
					return nil, err
				}
				merged := map[string]any{}
				for k, v := range defaults {
					merged[k] = v
				}
				for k, v := range member {
					merged[k] = v
				}
				docs = append(docs, merged)
			}
		}
	}
	if value, ok := body["ids"]; ok {
		ids, ok := value.([]any)
		if !ok {
			return nil, failure("invalid_request", "ids must be an array")
		}
		for _, id := range ids {
			if _, ok := id.(string); !ok {
				return nil, failure("invalid_request", "ids must contain strings")
			}
			member := map[string]any{}
			for k, v := range defaults {
				member[k] = v
			}
			member["_id"] = id
			docs = append(docs, member)
		}
	}
	if len(docs) == 0 {
		return nil, failure("invalid_request", "multi-term-vector read requires documents")
	}
	return map[string]any{"docs": docs}, nil
}
