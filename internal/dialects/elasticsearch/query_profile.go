package elasticsearch

import (
	"encoding/json"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

// Build identity does not attest installed plugins. An analyzer or field type
// supplied by a plugin can execute outside the stock query/script sandbox even
// when the submitted query only names built-in query types. Metadata remains
// usable; query profiles for external plugins must carry their own effect proof.
func (p *queryProof) stockQueryProfile() error {
	if p.stockProven {
		return nil
	}
	if err := p.t.ready(); err != nil {
		return err
	}
	raw, err := p.t.wire.get(p.ctx, p.endpoint, "/_nodes/plugins", nil, p.t.limits.MaxResultBytes)
	if err != nil {
		return err
	}
	var result struct {
		Outcome struct {
			Total      int `json:"total"`
			Successful int `json:"successful"`
			Failed     int `json:"failed"`
		} `json:"_nodes"`
		Nodes map[string]struct {
			Version   string            `json:"version"`
			BuildHash string            `json:"build_hash"`
			Plugins   []json.RawMessage `json:"plugins"`
		} `json:"nodes"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Nodes) == 0 || result.Outcome.Total != len(result.Nodes) || result.Outcome.Successful != result.Outcome.Total || result.Outcome.Failed != 0 {
		return failure("capability_unavailable", "query node/plugin inventory is incomplete")
	}
	for _, node := range result.Nodes {
		if node.Version != p.t.cfg.Elasticsearch.Version || node.BuildHash != config.ElasticsearchBuilds[node.Version] {
			return failure("profile_mismatch", "query node differs from pinned build")
		}
		if node.Plugins == nil || len(node.Plugins) != 0 {
			return failure("capability_unavailable", "installed plugins require an explicit query effect profile")
		}
	}
	p.stockProven = true
	return nil
}
