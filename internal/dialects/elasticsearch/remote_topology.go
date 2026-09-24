package elasticsearch

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

type remoteTopologySnapshot struct {
	Alias           string   `json:"alias"`
	Mode            string   `json:"mode"`
	ProxyAddress    string   `json:"proxy_address,omitempty"`
	Seeds           []string `json:"seeds,omitempty"`
	Connected       bool     `json:"connected"`
	SkipUnavailable bool     `json:"skip_unavailable"`
}

func (t *Target) attestRemoteTopology(ctx context.Context, endpoint int) ([]byte, error) {
	if len(t.cfg.Elasticsearch.RemoteClusters) == 0 {
		return nil, nil
	}
	raw, err := t.wire.get(ctx, endpoint, "/_remote/info", nil, t.limits.MaxResultBytes)
	if err != nil {
		return nil, err
	}
	return validateRemoteTopology(raw, t.cfg.Elasticsearch.RemoteClusters)
}

// _remote/info reports a redacted credential marker, not a key ID or remote
// cluster UUID. This proves only the configured alias/topology and API-key
// security model. Remote query execution still requires a separate key/scope
// proof and is not admitted by this function.
func validateRemoteTopology(raw []byte, profiles []config.ElasticsearchRemoteCluster) ([]byte, error) {
	root, err := decodeObject(raw)
	if err != nil {
		return nil, failure("profile_mismatch", "remote cluster information is malformed")
	}
	snapshots := make([]remoteTopologySnapshot, 0, len(profiles))
	for _, profile := range profiles {
		value, exists := root[profile.Alias]
		if !exists {
			return nil, failure("profile_mismatch", "configured remote alias is missing")
		}
		entry, err := object(value)
		if err != nil {
			return nil, failure("profile_mismatch", "configured remote alias has invalid metadata")
		}
		mode, mok := entry["mode"].(string)
		credentials, cok := entry["cluster_credentials"].(string)
		connected, aok := entry["connected"].(bool)
		skip, sok := entry["skip_unavailable"].(bool)
		if !mok || mode != profile.Mode || !cok || credentials != "::es_redacted::" || !aok || !sok {
			return nil, failure("profile_mismatch", "remote alias mode, API-key model or availability metadata differs")
		}
		snapshot := remoteTopologySnapshot{Alias: profile.Alias, Mode: mode, Connected: connected, SkipUnavailable: skip}
		switch mode {
		case "proxy":
			address, ok := entry["proxy_address"].(string)
			if !ok || address != profile.ProxyAddress {
				return nil, failure("profile_mismatch", "remote proxy address differs from its pinned profile")
			}
			snapshot.ProxyAddress = address
		case "sniff":
			values, ok := entry["seeds"].([]any)
			if !ok || len(values) != len(profile.Seeds) {
				return nil, failure("profile_mismatch", "remote seed inventory differs from its pinned profile")
			}
			seeds := make([]string, 0, len(values))
			for _, value := range values {
				seed, ok := value.(string)
				if !ok {
					return nil, failure("profile_mismatch", "remote seed inventory is malformed")
				}
				seeds = append(seeds, seed)
			}
			sort.Strings(seeds)
			wanted := append([]string(nil), profile.Seeds...)
			sort.Strings(wanted)
			for i := range seeds {
				if seeds[i] != wanted[i] {
					return nil, failure("profile_mismatch", "remote seed address differs from its pinned profile")
				}
			}
			snapshot.Seeds = seeds
		default:
			return nil, failure("profile_mismatch", "remote connection mode is unrecognized")
		}
		snapshots = append(snapshots, snapshot)
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Alias < snapshots[j].Alias })
	return json.Marshal(snapshots)
}
