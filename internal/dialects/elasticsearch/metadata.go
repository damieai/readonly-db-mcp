package elasticsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

type resolvedIndex struct {
	Name       string   `json:"name"`
	Aliases    []string `json:"aliases,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
	DataStream string   `json:"data_stream,omitempty"`
}
type resolvedAlias struct {
	Name    string   `json:"name"`
	Indices []string `json:"indices"`
}
type resolvedStream struct {
	Name           string   `json:"name"`
	BackingIndices []string `json:"backing_indices"`
	TimestampField string   `json:"timestamp_field"`
}
type resolution struct {
	Indices     []resolvedIndex  `json:"indices"`
	Aliases     []resolvedAlias  `json:"aliases"`
	DataStreams []resolvedStream `json:"data_streams"`
}

func validateResolution(ctx context.Context, raw []byte, cfg *config.ElasticsearchConfig) (map[string]bool, error) {
	var r resolution
	if err := decodeProof(raw, &r); err != nil {
		return nil, err
	}
	if r.Indices == nil || r.Aliases == nil || r.DataStreams == nil {
		return nil, failure("scope_denied", "index resolution is incomplete")
	}
	physical := map[string]bool{}
	backings := map[string]bool{}
	add := func(name string) error {
		if !concreteName(name) {
			return failure("scope_denied", "invalid resolved index name")
		}
		if physical[name] {
			return nil
		}
		if len(physical) >= cfg.MaxResolvedIndices {
			return failure("resource_limit", "resolved index inventory exceeds limit")
		}
		physical[name] = true
		return nil
	}
	for _, s := range r.DataStreams {
		if !concreteName(s.Name) || len(s.BackingIndices) == 0 {
			return nil, failure("scope_denied", "invalid data stream resolution")
		}
		if err := scopePattern(ctx, s.Name, cfg); err != nil {
			return nil, err
		}
		for _, name := range s.BackingIndices {
			if !strings.HasPrefix(name, ".ds-"+s.Name+"-") {
				// Manually attached ordinary backing indices remain usable when
				// their own physical name also satisfies the configured scope.
				if err := scopePattern(ctx, name, cfg); err != nil {
					return nil, err
				}
			}
			// Explicit denies still apply to hidden backing index names.
			denied, err := globRelation(ctx, []string{name}, cfg.DeniedIndices, true)
			if err != nil {
				return nil, err
			}
			if denied {
				return nil, failure("scope_denied", "data stream includes a denied backing index")
			}
			if err := add(name); err != nil {
				return nil, err
			}
			backings[name] = true
		}
	}
	for _, i := range r.Indices {
		if !backings[i.Name] {
			if err := scopePattern(ctx, i.Name, cfg); err != nil {
				return nil, err
			}
		}
		if err := add(i.Name); err != nil {
			return nil, err
		}
	}
	for _, a := range r.Aliases {
		if !concreteName(a.Name) || len(a.Indices) == 0 {
			return nil, failure("scope_denied", "invalid alias resolution")
		}
		if err := scopePattern(ctx, a.Name, cfg); err != nil {
			return nil, err
		}
		for _, name := range a.Indices {
			if !backings[name] {
				if err := scopePattern(ctx, name, cfg); err != nil {
					return nil, err
				}
			}
			if err := add(name); err != nil {
				return nil, err
			}
		}
	}
	return physical, nil
}

func (t *Target) resolve(ctx context.Context, endpoint int, indices []string) ([]byte, map[string]bool, error) {
	for _, name := range indices {
		if err := requestSourcePattern(ctx, name, t.cfg.Elasticsearch); err != nil {
			return nil, nil, err
		}
	}
	data, err := t.wire.request(ctx, endpoint, http.MethodGet, "/_resolve/index/"+escapedIndexTargets(indices), url.Values{"expand_wildcards": {"all"}}, nil, "application/json", t.limits.MaxResultBytes, false)
	if err != nil {
		return nil, nil, err
	}
	physical, err := validateResolution(ctx, data, t.cfg.Elasticsearch)
	return data, physical, err
}

func validateMappings(raw []byte, physical map[string]bool) error {
	var result map[string]json.RawMessage
	if json.Unmarshal(raw, &result) != nil || result == nil {
		return failure("invalid_response", "expected a native mapping object")
	}
	for name := range result {
		if !physical[name] {
			return failure("scope_denied", "mapping response escaped the attested index inventory")
		}
	}
	return nil
}
