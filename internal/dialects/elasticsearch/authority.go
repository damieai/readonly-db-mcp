package elasticsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

type indexPrivilege struct {
	Names           []string        `json:"names"`
	Privileges      []string        `json:"privileges"`
	FieldSecurity   json.RawMessage `json:"field_security,omitempty"`
	Query           json.RawMessage `json:"query,omitempty"`
	AllowRestricted *bool           `json:"allow_restricted_indices"`
}
type privileges struct {
	Cluster       []string          `json:"cluster"`
	Indices       []indexPrivilege  `json:"indices"`
	Applications  []json.RawMessage `json:"applications"`
	RunAs         []string          `json:"run_as"`
	Global        []json.RawMessage `json:"global"`
	RemoteIndices []json.RawMessage `json:"remote_indices,omitempty"`
	RemoteCluster []json.RawMessage `json:"remote_cluster,omitempty"`
}

func validatePrivileges(ctx context.Context, raw []byte, cfg *config.ElasticsearchConfig) error {
	var p privileges
	if err := decodeProof(raw, &p); err != nil {
		return err
	}
	if p.Cluster == nil || p.Indices == nil || p.Applications == nil || p.RunAs == nil || p.Global == nil {
		return failure("authority_unproven", "effective privilege response is incomplete")
	}
	if len(p.Applications)+len(p.RunAs)+len(p.Global)+len(p.RemoteIndices)+len(p.RemoteCluster) > 0 {
		return failure("authority_unproven", "application, delegation, global or remote authority is outside this profile")
	}
	hasEnrich := false
	for _, name := range p.Cluster {
		// The pinned resolver expands an action name to its action-prefix grant.
		// script/get has only read handlers in these builds; never permit the
		// broader script/* or manage privilege to inspect stored scripts.
		if name == "monitor_enrich" && cfg.EnrichEnabled() {
			hasEnrich = true
			continue
		}
		if name != "monitor" && name != "none" && name != "cluster:admin/script/get" {
			return failure("authority_unproven", "cluster privilege exceeds the read-only profile")
		}
	}
	if cfg.EnrichEnabled() && !hasEnrich {
		return failure("authority_unproven", "ENRICH requires the read-only monitor_enrich privilege and explicit cluster snapshot scope")
	}
	if len(p.Indices) == 0 || len(p.Indices) > 128 {
		return failure("authority_unproven", "missing or excessive index authority")
	}
	for _, entry := range p.Indices {
		if entry.AllowRestricted == nil || *entry.AllowRestricted || len(entry.Names) == 0 || len(entry.Names) > 64 || len(entry.Privileges) == 0 {
			return failure("authority_unproven", "incomplete or restricted-index authority")
		}
		for _, name := range entry.Privileges {
			if name != "read" && name != "view_index_metadata" && name != "none" {
				return failure("authority_unproven", "index privilege exceeds the read-only profile")
			}
		}
		for _, pattern := range entry.Names {
			if err := scopePattern(ctx, pattern, cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Target) attest(ctx context.Context) error {
	digest := sha256.New()
	for i := range t.wire.clients {
		data, err := t.wire.get(ctx, i, "/", nil, t.limits.MaxResultBytes)
		if err != nil {
			return err
		}
		var root struct {
			ClusterUUID string `json:"cluster_uuid"`
			Version     struct {
				Number      string `json:"number"`
				BuildHash   string `json:"build_hash"`
				BuildFlavor string `json:"build_flavor"`
				Snapshot    *bool  `json:"build_snapshot"`
			} `json:"version"`
		}
		if json.Unmarshal(data, &root) != nil || root.ClusterUUID != t.cfg.Elasticsearch.ClusterUUID || root.Version.Number != t.cfg.Elasticsearch.Version || root.Version.BuildHash != config.ElasticsearchBuilds[t.cfg.Elasticsearch.Version] || root.Version.BuildFlavor != "default" || root.Version.Snapshot == nil || *root.Version.Snapshot {
			return failure("profile_mismatch", "Elasticsearch cluster or build differs from the configured pinned profile")
		}
		data, err = t.wire.get(ctx, i, "/_security/_authenticate", nil, t.limits.MaxResultBytes)
		if err != nil {
			return err
		}
		if t.cfg.Elasticsearch.APIKey != nil {
			proof, err := t.attestAPIKey(ctx, i, data)
			if err != nil {
				return err
			}
			_, _ = digest.Write(proof)
			_, _ = digest.Write([]byte{0})
			if t.cfg.Elasticsearch.EnrichEnabled() {
				inventory, err := t.readEnrichInventory(ctx, i)
				if err != nil {
					return err
				}
				_, _ = digest.Write([]byte(config.ElasticsearchEnrichScope))
				_, _ = digest.Write(inventory.digest[:])
			}
			continue
		}
		var user struct {
			Username           string `json:"username"`
			Enabled            *bool  `json:"enabled"`
			AuthenticationType string `json:"authentication_type"`
		}
		if json.Unmarshal(data, &user) != nil || user.Username != t.cfg.Username || user.Enabled == nil || !*user.Enabled || user.AuthenticationType != "realm" {
			return failure("authority_unproven", "expected an enabled dedicated realm user; API-key proof is not implemented")
		}
		data, err = t.wire.get(ctx, i, "/_security/user/_privileges", nil, t.limits.MaxResultBytes)
		if err != nil {
			return err
		}
		if err := validatePrivileges(ctx, data, t.cfg.Elasticsearch); err != nil {
			return err
		}
		// Retain DLS/FLS values without float conversion and canonicalize keys.
		var proof any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&proof); err != nil {
			return failure("authority_unproven", "invalid privilege proof")
		}
		canonical, _ := json.Marshal(proof)
		_, _ = digest.Write(canonical)
		_, _ = digest.Write([]byte{0})
		if t.cfg.Elasticsearch.EnrichEnabled() {
			inventory, err := t.readEnrichInventory(ctx, i)
			if err != nil {
				return err
			}
			_, _ = digest.Write([]byte(config.ElasticsearchEnrichScope))
			_, _ = digest.Write(inventory.digest[:])
		}
	}
	var hash [32]byte
	copy(hash[:], digest.Sum(nil))
	t.authorityMu.Lock()
	if hash != t.authorityHash {
		t.authorityHash = hash
		t.authorityEpoch.Add(1)
	}
	t.authorityMu.Unlock()
	return nil
}

func (t *Target) invalidateAuthority() {
	if t.healthy.Swap(false) {
		t.authorityEpoch.Add(1)
	}
}
