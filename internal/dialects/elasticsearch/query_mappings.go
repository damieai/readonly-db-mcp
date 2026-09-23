package elasticsearch

import (
	"net/http"
	"strings"
)

func (p *queryProof) retainProof(size int) error {
	p.proofBytes += size
	if p.proofBytes > p.t.limits.MaxResultBytes {
		return failure("resource_limit", "combined mapping/script proof exceeds memory budget")
	}
	return nil
}

func (p *queryProof) sourceMappings(indices []string) (map[string]any, error) {
	key := strings.Join(indices, ",")
	if m, ok := p.mappings[key]; ok {
		return m, nil
	}
	if p.snapshot != nil && key == scopeKey(p.snapshot.query.Indices) {
		root := p.snapshot.mappings
		// Runtime lookup fields still address live indices, even when the main
		// reader is a PIT snapshot. Reprove those references on every new page.
		for _, v := range root {
			index, err := object(v)
			if err != nil {
				return nil, err
			}
			mapping, err := object(index["mappings"])
			if err != nil {
				return nil, err
			}
			if runtime, ok := mapping["runtime"]; ok {
				if err := p.runtime(runtime); err != nil {
					return nil, err
				}
			}
		}
		p.mappings[key] = root
		return root, nil
	}
	physical, err := p.sources(indices)
	if err != nil {
		return nil, err
	}
	if err := p.t.ready(); err != nil {
		return nil, err
	}
	raw, err := p.t.wire.request(p.ctx, p.endpoint, http.MethodGet, "/"+escapedIndexTargets(indices)+"/_mapping", nil, nil, "application/json", p.t.limits.MaxResultBytes, false)
	if err != nil {
		return nil, err
	}
	if err := validateMappings(raw, physical); err != nil {
		return nil, err
	}
	if err := p.retainProof(len(raw)); err != nil {
		return nil, err
	}
	root, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	for _, v := range root {
		index, err := object(v)
		if err != nil {
			return nil, err
		}
		m, err := object(index["mappings"])
		if err != nil {
			return nil, err
		}
		if runtime, ok := m["runtime"]; ok {
			if err := p.runtime(runtime); err != nil {
				return nil, err
			}
		}
	}
	p.mappings[key] = root
	return root, nil
}

// Collect aliases as well as concrete fields: a field alias pointing to
// semantic_text must not bypass inference checks on an otherwise ordinary match.
func semanticMappingFields(p *queryProof, mapping map[string]any, patterns []string) error {
	fields := map[string]map[string]any{}
	var collect func(any, string)
	collect = func(v any, prefix string) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		for name, v := range m {
			field, ok := v.(map[string]any)
			if !ok {
				continue
			}
			full := prefix + name
			fields[full] = field
			collect(field["properties"], full+".")
			collect(field["fields"], full+".")
		}
	}
	collect(mapping["properties"], "")
	for name, field := range fields {
		seen := map[string]bool{}
		for field["type"] == "alias" {
			path, ok := field["path"].(string)
			if !ok || seen[path] {
				return failure("capability_unavailable", "field alias source cannot be proved")
			}
			seen[path] = true
			field = fields[path]
			if field == nil {
				return failure("capability_unavailable", "field alias target is missing")
			}
		}
		if field["type"] == "semantic_text" {
			for _, pattern := range patterns {
				if err := p.step(0); err != nil {
					return err
				}
				matches, err := p.fieldMatches(pattern, name)
				if err != nil {
					return err
				}
				if matches {
					return failure("capability_unavailable", "matching semantic_text requires an inference profile")
				}
			}
		}
	}
	return nil
}

// Field selectors have a different alphabet from index authorization patterns;
// field names may contain punctuation that is forbidden in an index path.
func (proof *queryProof) fieldMatches(pattern, name string) (bool, error) {
	p, s := []rune(pattern), []rune(name)
	i, j, star, matched := 0, 0, -1, 0
	for j < len(s) {
		if err := proof.step(0); err != nil {
			return false, err
		}
		if i < len(p) && (p[i] == '?' || p[i] == s[j]) {
			i++
			j++
		} else if i < len(p) && p[i] == '*' {
			star = i
			i++
			matched = j
		} else if star >= 0 {
			matched++
			j = matched
			i = star + 1
		} else {
			return false, nil
		}
	}
	for i < len(p) && p[i] == '*' {
		i++
	}
	return i == len(p), nil
}
