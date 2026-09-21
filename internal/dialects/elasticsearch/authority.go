package elasticsearch

import (
	"context"
	"encoding/json"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

type indexPrivilege struct {
	Names           []string          `json:"names"`
	Privileges      []string          `json:"privileges"`
	FieldSecurity   []json.RawMessage `json:"field_security,omitempty"`
	Query           []json.RawMessage `json:"query,omitempty"`
	AllowRestricted *bool             `json:"allow_restricted_indices"`
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
	for _, name := range p.Cluster {
		if name != "monitor" && name != "none" {
			return failure("authority_unproven", "cluster privilege exceeds the read-only profile")
		}
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
	}
	return nil
}
