package elasticsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

func validateMetadataNames(names []string) error {
	if len(names) > 10000 {
		return failure("resource_limit", "too many metadata names")
	}
	for _, name := range names {
		if name == "" || len(name) > 4096 || name == "." || name == ".." {
			return failure("invalid_request", "invalid metadata name selector")
		}
		for _, r := range name {
			if unicode.IsControl(r) || strings.ContainsRune(`/\\,`, r) {
				return failure("invalid_request", "invalid metadata name selector")
			}
		}
	}
	return nil
}

func escapedMetadataNames(names []string) string {
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = url.PathEscape(name)
	}
	return strings.Join(parts, ",")
}

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

func validateFieldMappings(raw []byte, physical map[string]bool) error {
	result, err := decodeObject(raw)
	if err != nil {
		return failure("invalid_response", "expected native field mappings object")
	}
	for index, value := range result {
		if !physical[index] {
			return failure("scope_denied", "field mappings escaped the attested index inventory")
		}
		entry, err := object(value)
		if err != nil {
			return failure("invalid_response", "invalid field mappings index entry")
		}
		mappings, err := object(entry["mappings"])
		if err != nil {
			return failure("invalid_response", "field mappings are missing")
		}
		for _, value := range mappings {
			field, err := object(value)
			if err != nil {
				return failure("invalid_response", "invalid field mapping entry")
			}
			name, ok := field["full_name"].(string)
			if !ok || name == "" {
				return failure("invalid_response", "field mapping name is missing")
			}
			if _, err := object(field["mapping"]); err != nil {
				return failure("invalid_response", "field mapping definition is missing")
			}
		}
	}
	return nil
}

func validateIndexMetadata(ctx context.Context, raw, resolved []byte, physical map[string]bool, operation string, cfg *config.ElasticsearchConfig) error {
	if err := strictJSONContext(ctx, raw, cfg.MaxJSONDepth, cfg.MaxJSONNodes, 0); err != nil {
		return err
	}
	allowed := make(map[string]bool, len(physical))
	for name := range physical {
		allowed[name] = true
	}
	if operation == "aliases" {
		var inventory resolution
		if err := decodeProof(resolved, &inventory); err != nil {
			return err
		}
		// Native data-stream alias metadata is keyed by the logical stream,
		// while settings for a stream are keyed by its backing indices.
		for _, stream := range inventory.DataStreams {
			allowed[stream.Name] = true
		}
	}
	result, err := decodeObject(raw)
	if err != nil {
		return failure("invalid_response", "expected native index metadata object")
	}
	for index, value := range result {
		if !allowed[index] {
			return failure("scope_denied", "metadata response escaped the resolved index inventory")
		}
		entry, err := object(value)
		if err != nil {
			return failure("invalid_response", "invalid index metadata entry")
		}
		if operation == "settings" {
			if _, err := object(entry["settings"]); err != nil {
				return failure("invalid_response", "index settings are missing")
			}
			continue
		}
		aliases, err := object(entry["aliases"])
		if err != nil {
			return failure("invalid_response", "index aliases are missing")
		}
		for name, value := range aliases {
			if !concreteName(name) {
				return failure("scope_denied", "invalid resolved alias name")
			}
			if err := scopePattern(ctx, name, cfg); err != nil {
				return err
			}
			if _, err := object(value); err != nil {
				return failure("invalid_response", "invalid alias definition")
			}
		}
	}
	return nil
}

func sameResolvedInventory(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for name := range a {
		if !b[name] {
			return false
		}
	}
	return true
}
