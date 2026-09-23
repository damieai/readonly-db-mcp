package elasticsearch

import "strings"

// Resolve native date expressions on the same pinned endpoint used for the
// request. Aliases and data streams are logical source names; their backing
// indices are proved separately by validateResolution but do not replace an
// alias/data-stream name when comparing with a caller's requested scope.
func (p *queryProof) dateMathNames(expression string) ([]string, error) {
	raw, _, err := p.t.resolve(p.ctx, p.endpoint, []string{expression})
	if err != nil {
		return nil, err
	}
	var inventory resolution
	if err := decodeProof(raw, &inventory); err != nil {
		return nil, err
	}
	backing := map[string]bool{}
	names := map[string]bool{}
	for _, alias := range inventory.Aliases {
		names[alias.Name] = true
		for _, index := range alias.Indices {
			backing[index] = true
		}
	}
	for _, stream := range inventory.DataStreams {
		names[stream.Name] = true
		for _, index := range stream.BackingIndices {
			backing[index] = true
		}
	}
	for _, index := range inventory.Indices {
		if !backing[index.Name] {
			names[index.Name] = true
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result, nil
}

func (p *queryProof) requestedSource(source string, requested []string) (bool, error) {
	if len(requested) == 0 {
		return true, nil
	}
	if !dateMathSource(source) {
		ordinary := true
		for _, pattern := range requested {
			ordinary = ordinary && !dateMathSource(pattern)
		}
		if ordinary {
			return globRelation(p.ctx, []string{source}, requested, false)
		}
	}
	wanted := make([]string, 0, len(requested))
	for _, pattern := range requested {
		if dateMathSource(pattern) {
			names, err := p.dateMathNames(pattern)
			if err != nil {
				return false, err
			}
			wanted = append(wanted, names...)
		} else {
			wanted = append(wanted, pattern)
		}
	}
	if len(wanted) == 0 {
		return false, nil
	}
	if dateMathSource(source) {
		names, err := p.dateMathNames(source)
		if err != nil {
			return false, err
		}
		if len(names) == 0 {
			return false, nil
		}
		for _, name := range names {
			contained, err := globRelation(p.ctx, []string{name}, wanted, false)
			if err != nil || !contained {
				return contained, err
			}
		}
		return true, nil
	}
	// A non-date wildcard must be contained by the whole requested scope, even
	// if today's date expression happens to resolve to one matching index.
	return globRelation(p.ctx, []string{strings.TrimSuffix(source, "::data")}, wanted, false)
}
