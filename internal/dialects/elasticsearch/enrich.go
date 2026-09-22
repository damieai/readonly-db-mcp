package elasticsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

const enrichStatePath = "/_cluster/state/metadata,version/.enrich-*"

type enrichSnapshot struct {
	name, uuid, policy, kind, match string
}
type enrichInventory struct {
	state   string
	version json.Number
	digest  [32]byte
	active  map[string]enrichSnapshot
	bytes   int
	count   int
}
type enrichProof struct {
	inventory *enrichInventory
	policies  map[string][32]byte
}

// This scope grants all historical/current/future native enrichment snapshots
// in the pinned local cluster. Inventory reads are effect/consistency checks,
// not evidence of source-index DLS/FLS or policy-level confidentiality.
func (t *Target) readEnrichInventory(ctx context.Context, endpoint int) (*enrichInventory, error) {
	if !t.cfg.Elasticsearch.EnrichEnabled() {
		return nil, failure("capability_unavailable", "ENRICH requires explicit all_cluster_snapshots data scope")
	}
	left := time.Until(deadline(ctx)).Milliseconds()
	if left <= 0 {
		return nil, failure("deadline_exceeded", "ENRICH proof deadline expired")
	}
	options := url.Values{"flat_settings": {"true"}, "expand_wildcards": {"all"}, "allow_no_indices": {"true"}, "ignore_unavailable": {"false"}, "master_timeout": {strconv.FormatInt(left, 10) + "ms"}}
	raw, err := t.wire.get(ctx, endpoint, enrichStatePath, options, t.limits.MaxResultBytes)
	if err != nil {
		return nil, err
	}
	var state struct {
		ClusterUUID string      `json:"cluster_uuid"`
		StateUUID   string      `json:"state_uuid"`
		Version     json.Number `json:"version"`
		Metadata    struct {
			ClusterUUID string                     `json:"cluster_uuid"`
			Indices     map[string]json.RawMessage `json:"indices"`
			Projects    json.RawMessage            `json:"projects"`
		} `json:"metadata"`
	}
	if json.Unmarshal(raw, &state) != nil || state.StateUUID == "" || state.StateUUID == "_na_" || state.Metadata.ClusterUUID != t.cfg.Elasticsearch.ClusterUUID || (state.ClusterUUID != "" && state.ClusterUUID != state.Metadata.ClusterUUID) || state.Metadata.Indices == nil || state.Metadata.Projects != nil {
		return nil, failure("profile_mismatch", "ENRICH cluster-state identity or snapshot inventory is incomplete")
	}
	if n, err := state.Version.Int64(); err != nil || n < 0 {
		return nil, failure("invalid_response", "ENRICH cluster-state version is invalid")
	}
	if len(state.Metadata.Indices) > t.cfg.Elasticsearch.MaxResolvedIndices {
		return nil, failure("resource_limit", "ENRICH snapshot inventory exceeds the configured source limit")
	}
	result := &enrichInventory{state: state.StateUUID, version: state.Version, active: map[string]enrichSnapshot{}, bytes: len(raw), count: len(state.Metadata.Indices)}
	uuids := map[string]bool{}
	for name, raw := range state.Metadata.Indices {
		if ctx.Err() != nil {
			return nil, failure("deadline_exceeded", "ENRICH inventory analysis exceeded the request deadline")
		}
		snapshot, aliases, err := validateEnrichSnapshot(ctx, name, raw, t.limits.MaxResultBytes)
		if err != nil {
			return nil, err
		}
		if uuids[snapshot.uuid] {
			return nil, failure("invalid_response", "ENRICH inventory repeats a snapshot UUID")
		}
		uuids[snapshot.uuid] = true
		for _, alias := range aliases {
			if alias != ".enrich-"+snapshot.policy {
				return nil, failure("capability_unavailable", "ENRICH alias differs from the native snapshot policy")
			}
			if _, exists := result.active[snapshot.policy]; exists {
				return nil, failure("capability_unavailable", "ENRICH policy alias resolves to multiple snapshots")
			}
			result.active[snapshot.policy] = snapshot
		}
	}
	// Preserve numbers while canonicalizing metadata keys. The digest observes
	// snapshot/alias/mapping changes independently of unrelated cluster activity.
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&value) != nil {
		return nil, failure("invalid_response", "invalid ENRICH state metadata")
	}
	indices := value.(map[string]any)["metadata"].(map[string]any)["indices"]
	canonical, _ := json.Marshal(indices)
	result.digest = sha256.Sum256(canonical)
	return result, nil
}

func validEnrichPolicy(name string) bool {
	return name != "" && len(name) <= 240 && !strings.ContainsAny(name, "*?") && config.ValidElasticsearchPattern(".enrich-"+name)
}

func validateEnrichSnapshot(ctx context.Context, name string, raw []byte, workLimit int) (enrichSnapshot, []string, error) {
	var index struct {
		State    string                    `json:"state"`
		Settings map[string]string         `json:"settings"`
		Mappings map[string]map[string]any `json:"mappings"`
		Aliases  []string                  `json:"aliases"`
	}
	deny := func() (enrichSnapshot, []string, error) {
		return enrichSnapshot{}, nil, failure("capability_unavailable", "ENRICH requires native, write-blocked snapshot mappings")
	}
	if !strings.HasPrefix(name, ".enrich-") || !concreteName(name) || json.Unmarshal(raw, &index) != nil || index.State != "open" || index.Aliases == nil || index.Settings["index.blocks.write"] != "true" || index.Settings["index.number_of_shards"] != "1" || len(index.Settings["index.uuid"]) < 16 || len(index.Settings["index.uuid"]) > 64 || len(index.Mappings) != 1 {
		return deny()
	}
	mapping, ok := index.Mappings["_doc"]
	if !ok || (mapping["dynamic"] != false && mapping["dynamic"] != "false") {
		return deny()
	}
	if err := fieldsOnly(mapping, "dynamic _source properties _meta"); err != nil {
		return deny()
	}
	if sourceMode := index.Settings["index.mapping.source.mode"]; sourceMode != "" && sourceMode != "stored" {
		return deny()
	}
	if raw, exists := mapping["_source"]; exists {
		source, ok := raw.(map[string]any)
		if !ok || len(source) != 1 || source["enabled"] != true {
			return deny()
		}
	}
	meta, ok := mapping["_meta"].(map[string]any)
	if !ok {
		return deny()
	}
	policy, pok := meta["enrich_policy_name"].(string)
	kind, kok := meta["enrich_policy_type"].(string)
	match, mok := meta["enrich_match_field"].(string)
	if !pok || !kok || !mok || !validEnrichPolicy(policy) || match == "" || !strings.HasPrefix(name, ".enrich-"+policy+"-") {
		return deny()
	}
	if kind != "match" && kind != "range" && kind != "geo_match" {
		return deny()
	}
	properties, ok := mapping["properties"].(map[string]any)
	if !ok || len(properties) == 0 {
		return deny()
	}
	var matchMapping map[string]any
	work := 0
	var traversalError error
	var inspect func(map[string]any, string) bool
	inspect = func(properties map[string]any, prefix string) bool {
		for name, value := range properties {
			if ctx.Err() != nil {
				traversalError = failure("deadline_exceeded", "ENRICH mapping analysis exceeded the request deadline")
				return false
			}
			// Bound expanded nested paths without retaining a second flattened
			// mapping inventory whose size could multiply the wire byte budget.
			work += len(prefix) + len(name) + 1
			if work > workLimit {
				traversalError = failure("resource_limit", "ENRICH mapping path work exceeds the proof byte budget")
				return false
			}
			field, ok := value.(map[string]any)
			if !ok {
				return false
			}
			full := prefix + name
			if full == match {
				if matchMapping != nil {
					return false
				}
				matchMapping = field
			}
			if child, exists := field["properties"]; exists {
				nested, ok := child.(map[string]any)
				if !ok || fieldsOnly(field, "type properties") != nil || (field["type"] != nil && field["type"] != "object") || !inspect(nested, full+".") {
					return false
				}
				continue
			}
			// EnrichPolicyRunner only copies type/format and index/doc_values.
			// Runtime/lookup/alias/script mappings are not native snapshots.
			if fieldsOnly(field, "type format index doc_values") != nil {
				return false
			}
			kind, ok := field["type"].(string)
			if !ok || kind == "" || kind == "semantic_text" || kind == "alias" || kind == "lookup" {
				return false
			}
		}
		return true
	}
	if !inspect(properties, "") {
		if traversalError != nil {
			return enrichSnapshot{}, nil, traversalError
		}
		return deny()
	}
	field := matchMapping
	if field == nil {
		return deny()
	}
	if kind == "match" && field["type"] != "keyword" || kind == "geo_match" && field["type"] != "geo_shape" {
		return deny()
	}
	if kind == "range" {
		switch field["type"] {
		case "integer_range", "float_range", "long_range", "double_range", "date_range", "ip_range":
		default:
			return deny()
		}
	}
	return enrichSnapshot{name: name, uuid: index.Settings["index.uuid"], policy: policy, kind: kind, match: match}, index.Aliases, nil
}

func enrichPolicyName(reference string) (string, error) {
	name := reference
	if before, after, ok := strings.Cut(reference, ":"); ok {
		// With FROM/LOOKUP sources proved local, all three native modes resolve
		// locally (including _remote). Never reinterpret a mode as a cluster.
		before = strings.ToLower(before)
		if before != "_any" && before != "_coordinator" && before != "_remote" {
			return "", failure("invalid_request", "invalid native ENRICH policy mode")
		}
		name = after
	}
	if !validEnrichPolicy(name) {
		return "", failure("invalid_request", "invalid native ENRICH policy name")
	}
	return name, nil
}

func (p *queryProof) prepareEnrich(references []string) error {
	if !p.t.cfg.Elasticsearch.EnrichEnabled() {
		return failure("capability_unavailable", "ENRICH requires explicit all_cluster_snapshots data scope")
	}
	if p.enrich == nil {
		inventory, err := p.t.readEnrichInventory(p.ctx, p.endpoint)
		if err != nil {
			return err
		}
		if inventory.count+len(p.physical) > p.t.cfg.Elasticsearch.MaxResolvedIndices {
			return failure("resource_limit", "combined ENRICH and index source inventory exceeds its limit")
		}
		if err := p.retainProof(inventory.bytes); err != nil {
			return err
		}
		p.enrich = &enrichProof{inventory: inventory, policies: map[string][32]byte{}}
	}
	for _, reference := range references {
		name, err := enrichPolicyName(reference)
		if err != nil {
			return err
		}
		if _, exists := p.enrich.policies[name]; exists {
			continue
		}
		if len(p.enrich.policies) >= p.t.cfg.Elasticsearch.MaxResolvedIndices {
			return failure("resource_limit", "ENRICH policy proof count exceeds its configured limit")
		}
		snapshot, exists := p.enrich.inventory.active[name]
		if !exists {
			return failure("capability_unavailable", "ENRICH policy has no proved active native snapshot")
		}
		digest, size, err := p.enrichPolicy(name, snapshot)
		if err != nil {
			return err
		}
		if err := p.retainProof(size); err != nil {
			return err
		}
		p.enrich.policies[name] = digest
	}
	return nil
}

func (p *queryProof) enrichPolicy(name string, snapshot enrichSnapshot) ([32]byte, int, error) {
	var zero [32]byte
	raw, err := p.t.wire.get(p.ctx, p.endpoint, "/_enrich/policy/"+url.PathEscape(name), nil, p.t.limits.MaxResultBytes)
	if err != nil {
		return zero, 0, err
	}
	body, err := decodeObject(raw)
	if err != nil {
		return zero, 0, err
	}
	policies, ok := body["policies"].([]any)
	if !ok || len(policies) != 1 {
		return zero, 0, failure("invalid_response", "ENRICH policy inventory is incomplete")
	}
	entry, ok := policies[0].(map[string]any)
	if !ok {
		return zero, 0, failure("invalid_response", "invalid ENRICH policy entry")
	}
	configuration, ok := entry["config"].(map[string]any)
	if !ok || len(configuration) != 1 {
		return zero, 0, failure("invalid_response", "invalid ENRICH policy configuration")
	}
	definition, ok := configuration[snapshot.kind].(map[string]any)
	if !ok || definition["name"] != name || definition["match_field"] != snapshot.match {
		return zero, 0, failure("capability_unavailable", "ENRICH policy differs from the native active snapshot")
	}
	if fieldsOnly(definition, "name query indices match_field enrich_fields") != nil {
		return zero, 0, failure("invalid_response", "unknown native ENRICH policy field")
	}
	for _, key := range []string{"indices", "enrich_fields"} {
		values, ok := definition[key].([]any)
		if !ok || len(values) == 0 {
			return zero, 0, failure("invalid_response", "ENRICH policy source/field metadata is missing")
		}
		for _, value := range values {
			if s, ok := value.(string); !ok || s == "" {
				return zero, 0, failure("invalid_response", "invalid ENRICH policy source/field metadata")
			}
		}
	}
	// Policy indices/query describe the earlier snapshot build. ENRICH does not
	// execute that query or read those live indices. Their DLS/FLS is not reused.
	canonical, _ := json.Marshal(body)
	return sha256.Sum256(canonical), len(raw), nil
}

func (p *queryProof) verifyEnrich() error {
	for name, before := range p.enrich.policies {
		after, _, err := p.enrichPolicy(name, p.enrich.inventory.active[name])
		if err != nil {
			return err
		}
		if before != after {
			return failure("scope_denied", "ENRICH policy changed during the request; retry explicitly")
		}
	}
	current, err := p.t.readEnrichInventory(p.ctx, p.endpoint)
	if err != nil {
		return err
	}
	previous := p.enrich.inventory
	if current.state != previous.state || current.version != previous.version || current.digest != previous.digest {
		return failure("scope_denied", "ENRICH cluster metadata changed during the request; retry explicitly")
	}
	return nil
}
