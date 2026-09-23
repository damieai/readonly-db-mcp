package elasticsearch

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

func (t *Target) attestAPIKey(ctx context.Context, endpoint int, authRaw []byte) ([]byte, error) {
	k := t.cfg.Elasticsearch.APIKey
	var identity struct {
		Username           string `json:"username"`
		Enabled            *bool  `json:"enabled"`
		AuthenticationType string `json:"authentication_type"`
		APIKey             struct {
			ID string `json:"id"`
		} `json:"api_key"`
	}
	if json.Unmarshal(authRaw, &identity) != nil || identity.Username != k.OwnerUsername || identity.Enabled == nil || !*identity.Enabled || identity.AuthenticationType != "api_key" || identity.APIKey.ID != k.ID {
		return nil, failure("authority_unproven", "configured API key identity does not match pinned owner and ID")
	}
	attestorRaw, err := t.wire.attestorGet(ctx, endpoint, "/_security/_authenticate", nil, t.limits.MaxResultBytes)
	if err != nil {
		return nil, err
	}
	var attestor struct {
		Username           string `json:"username"`
		Enabled            *bool  `json:"enabled"`
		AuthenticationType string `json:"authentication_type"`
	}
	if json.Unmarshal(attestorRaw, &attestor) != nil || attestor.Username != k.Attestor.Username || attestor.Enabled == nil || !*attestor.Enabled || attestor.AuthenticationType != "realm" {
		return nil, failure("authority_unproven", "expected a separate enabled realm attestor")
	}
	privilegeRaw, err := t.wire.attestorGet(ctx, endpoint, "/_security/user/_privileges", nil, t.limits.MaxResultBytes)
	if err != nil {
		return nil, err
	}
	if err := validateAttestorPrivileges(privilegeRaw); err != nil {
		return nil, err
	}
	keyRaw, err := t.wire.attestorGet(ctx, endpoint, "/_security/api_key", url.Values{"id": {k.ID}, "with_limited_by": {"true"}}, t.limits.MaxResultBytes)
	if err != nil {
		return nil, err
	}
	root, err := decodeObject(keyRaw)
	if err != nil {
		return nil, failure("authority_unproven", "API key descriptor response is malformed")
	}
	list, ok := root["api_keys"].([]any)
	if !ok || len(list) != 1 {
		return nil, failure("authority_unproven", "API key descriptor response must contain exactly one key")
	}
	entry, err := object(list[0])
	if err != nil || entry["id"] != k.ID || entry["username"] != k.OwnerUsername || entry["invalidated"] != false {
		return nil, failure("authority_unproven", "API key descriptor identity or active state differs")
	}
	if kind, exists := entry["type"]; exists && kind != "rest" {
		return nil, failure("authority_unproven", "cross-cluster API key is outside the local query profile")
	}
	if expiry, exists := entry["expiration"]; !exists {
		return nil, failure("authority_unproven", "API key expiration state is missing")
	} else if expiry != nil {
		value, ok := expiry.(json.Number)
		if !ok {
			return nil, failure("authority_unproven", "API key expiration is malformed")
		}
		ms, err := value.Int64()
		if err != nil || ms <= time.Now().UnixMilli() {
			return nil, failure("authority_unproven", "API key is expired")
		}
	}
	assigned, exists := entry["role_descriptors"]
	if !exists || !descriptorShape(assigned) {
		return nil, failure("authority_unproven", "API key assigned descriptors are missing")
	}
	limited, ok := entry["limited_by"].([]any)
	if !ok || len(limited) == 0 || len(limited) > 128 {
		return nil, failure("authority_unproven", "API key limiting descriptors are missing")
	}
	for _, set := range limited {
		if !descriptorShape(set) {
			return nil, failure("authority_unproven", "API key limiting descriptor is malformed")
		}
	}
	assignedSafe := safeDescriptorSet(ctx, assigned, t.cfg.Elasticsearch)
	limitedSafe := safeLimitedDescriptors(ctx, limited, t.cfg.Elasticsearch)
	// Native API-key authority is the intersection: one fully proved side
	// bounds the result even when the other side has broader grants.
	if ctx.Err() != nil || (!assignedSafe && !limitedSafe) {
		return nil, failure("authority_unproven", "API key assigned/limited intersection has no proven read-only side")
	}
	return json.Marshal([]json.RawMessage{authRaw, attestorRaw, privilegeRaw, keyRaw})
}

func safeLimitedDescriptors(ctx context.Context, limited []any, cfg *config.ElasticsearchConfig) bool {
	all := map[string]any{}
	for i, set := range limited {
		roles := set.(map[string]any) // descriptorShape checked above.
		for name, value := range roles {
			all[strconv.Itoa(i)+":"+name] = value
		}
	}
	return safeDescriptorSet(ctx, all, cfg)
}

func validateAttestorPrivileges(raw []byte) error {
	var p privileges
	if err := decodeProof(raw, &p); err != nil {
		return err
	}
	if p.Cluster == nil || p.Indices == nil || p.Applications == nil || p.RunAs == nil || p.Global == nil || len(p.Indices)+len(p.Applications)+len(p.RunAs)+len(p.Global)+len(p.RemoteIndices)+len(p.RemoteCluster) != 0 || len(p.Cluster) != 1 || p.Cluster[0] != "read_security" {
		return failure("authority_unproven", "attestor requires only read_security and no query or mutation grants")
	}
	return nil
}

func descriptorShape(value any) bool {
	roles, ok := value.(map[string]any)
	if !ok || len(roles) > 128 {
		return false
	}
	for _, descriptor := range roles {
		if _, ok := descriptor.(map[string]any); !ok {
			return false
		}
	}
	return true
}

func safeDescriptorSet(ctx context.Context, value any, cfg *config.ElasticsearchConfig) bool {
	roles, ok := value.(map[string]any)
	if !ok || len(roles) == 0 || len(roles) > 128 {
		return false
	}
	aggregate := privileges{Cluster: []string{}, Indices: []indexPrivilege{}, Applications: []json.RawMessage{}, RunAs: []string{}, Global: []json.RawMessage{}}
	for _, value := range roles {
		role, ok := value.(map[string]any)
		if !ok || fieldsOnly(role, "cluster indices applications run_as global remote_indices remote_cluster restriction metadata transient_metadata description") != nil {
			return false
		}
		if entries, exists := role["indices"]; exists {
			list, ok := entries.([]any)
			if !ok {
				return false
			}
			for _, entry := range list {
				m, ok := entry.(map[string]any)
				if !ok || fieldsOnly(m, "names privileges field_security query allow_restricted_indices") != nil {
					return false
				}
			}
		}
		encoded, err := json.Marshal(role)
		if err != nil {
			return false
		}
		var p privileges
		if json.Unmarshal(encoded, &p) != nil {
			return false
		}
		for i := range p.Indices {
			if p.Indices[i].AllowRestricted == nil {
				no := false
				p.Indices[i].AllowRestricted = &no
			}
		}
		aggregate.Cluster = append(aggregate.Cluster, p.Cluster...)
		aggregate.Indices = append(aggregate.Indices, p.Indices...)
		aggregate.Applications = append(aggregate.Applications, p.Applications...)
		aggregate.RunAs = append(aggregate.RunAs, p.RunAs...)
		aggregate.Global = append(aggregate.Global, p.Global...)
		aggregate.RemoteIndices = append(aggregate.RemoteIndices, p.RemoteIndices...)
		aggregate.RemoteCluster = append(aggregate.RemoteCluster, p.RemoteCluster...)
	}
	encoded, err := json.Marshal(aggregate)
	return err == nil && validatePrivileges(ctx, encoded, cfg) == nil
}
