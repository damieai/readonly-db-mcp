package elasticsearch

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

// A phrase candidate is only known to Elasticsearch after the search starts.
// Render every caller-supplied parameter now, prove that query, and leave only
// the candidate as a JSON-encoded scalar in a frozen inline Mustache template.
// This prevents a mutable stored template or a candidate from changing the
// query's source graph after the preflight proof.
func (p *queryProof) prepareCollate(phrase map[string]any, value any, rows, depth int) error {
	collate, err := object(value)
	if err != nil {
		return err
	}
	if err := fieldsOnly(collate, "query params prune"); err != nil {
		return err
	}
	if prune, exists := collate["prune"]; exists {
		if _, ok := prune.(bool); !ok {
			return failure("invalid_request", "collate prune must be boolean")
		}
	}
	query, err := object(collate["query"])
	if err != nil {
		return err
	}
	if err := fieldsOnly(query, "source id"); err != nil {
		return err
	}
	if id, exists := query["id"]; exists {
		if _, both := query["source"]; both {
			return failure("invalid_request", "collate template id and source are mutually exclusive")
		}
		name, ok := id.(string)
		if !ok || name == "" || len(name) > 1024 {
			return failure("invalid_request", "invalid collate template id")
		}
		stored, err := p.storedScript(name)
		if err != nil {
			return err
		}
		if stored["lang"] != "mustache" {
			return failure("capability_unavailable", "stored collate template must use Mustache")
		}
		query = map[string]any{"source": stored["source"]}
	}
	source, exists := query["source"]
	if !exists {
		return failure("invalid_request", "collate query requires source or a readable stored id")
	}
	params := map[string]any{}
	if raw, exists := collate["params"]; exists {
		given, err := object(raw)
		if err != nil {
			return err
		}
		if len(given) > p.t.limits.MaxParameters {
			return failure("resource_limit", "collate parameter count exceeds limit")
		}
		for key, value := range given {
			if key != "suggestion" { // Native phrase suggester overwrites this parameter.
				params[key] = value
			}
		}
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return failure("invalid_request", "invalid collate template source")
	}
	// A candidate-dependent section, lambda or unescaped variable could change
	// the query shape. Ordinary caller parameters retain their full Mustache
	// expressiveness because they are substituted before the source proof.
	for _, tag := range mustacheTags(encoded) {
		if strings.Contains(tag, "suggestion") && tag != "{{suggestion}}" {
			return failure("capability_unavailable", "collate candidate must occupy a JSON scalar value")
		}
	}
	marker := ":collate-" + uuid.NewString() + ":"
	params["suggestion"] = marker
	rendered, err := p.render(map[string]any{"source": source, "params": params})
	if err != nil {
		return err
	}
	if err := collateCandidateScalars(rendered, marker, ""); err != nil {
		return err
	}
	if err := p.query(rendered, rows, depth+1); err != nil {
		return err
	}
	encoded, err = json.Marshal(rendered)
	if err != nil {
		return failure("invalid_request", "invalid rendered collate query")
	}
	// toJson escapes the actual candidate as a JSON literal, including quotes
	// and backslashes. The source graph cannot be changed by suggestion text.
	frozen := strings.ReplaceAll(string(encoded), `"`+marker+`"`, "{{#toJson}}suggestion{{/toJson}}")
	collate["query"] = map[string]any{"source": frozen}
	delete(collate, "params")
	phrase["collate"] = collate
	return nil
}

func mustacheTags(raw []byte) []string {
	var tags []string
	s := string(raw)
	for offset := 0; offset < len(s); {
		start := strings.Index(s[offset:], "{{")
		if start < 0 {
			break
		}
		start += offset
		end := strings.Index(s[start+2:], "}}")
		if end < 0 {
			break
		}
		end += start + 4
		tags = append(tags, s[start:end])
		offset = end
	}
	return tags
}

func collateCandidateScalars(value any, marker, parent string) error {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if strings.Contains(key, marker) {
				return failure("capability_unavailable", "collate candidate cannot name a query field or operator")
			}
			if parent == "script" && (key == "source" || key == "id") && collateContainsMarker(child, marker) {
				return failure("capability_unavailable", "collate candidate cannot supply executable code or stored resource IDs")
			}
			if err := collateCandidateScalars(child, marker, key); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := collateCandidateScalars(child, marker, parent); err != nil {
				return err
			}
		}
	case string:
		if strings.Contains(v, marker) && v != marker {
			return failure("capability_unavailable", "collate candidate must occupy a complete JSON scalar")
		}
	}
	return nil
}

func collateContainsMarker(value any, marker string) bool {
	switch v := value.(type) {
	case string:
		return strings.Contains(v, marker)
	case map[string]any:
		for key, child := range v {
			if strings.Contains(key, marker) || collateContainsMarker(child, marker) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if collateContainsMarker(child, marker) {
				return true
			}
		}
	}
	return false
}
