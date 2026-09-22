package elasticsearch

import (
	_ "embed"
	"encoding/json"
)

//go:embed operations.generated.json
var catalogJSON []byte

type operation struct {
	API         string `json:"api"`
	Effect      string `json:"effect"`
	Implemented bool   `json:"implemented"`
	Source      string `json:"source"`
	SHA256      string `json:"sha256"`
	Routes      []struct {
		Path    string   `json:"path"`
		Methods []string `json:"methods"`
	} `json:"routes"`
	Parameters []string `json:"parameters"`
}

var catalog = func() map[string]map[string]operation {
	var result map[string]map[string]operation
	if err := json.Unmarshal(catalogJSON, &result); err != nil {
		panic("invalid compiled Elasticsearch operation catalog")
	}
	return result
}()

func metadataOperation(version, name string) error {
	op, exists := catalog[version][name]
	if !exists {
		return failure("capability_unavailable", "operation has no implemented metadata handler; inspect target capabilities")
	}
	if op.Effect == "mutation" {
		return failure("mutation_forbidden", "operation modifies persistent data or administrative state")
	}
	if !op.Implemented || (name != "resolve" && name != "mappings") {
		return failure("capability_unavailable", "read or owned-context operation awaits its query/lifecycle implementation")
	}
	return nil
}

func queryOperation(version, name string) error {
	op, exists := catalog[version][name]
	if exists && op.Effect == "mutation" {
		return failure("mutation_forbidden", "operation modifies persistent data or administrative state")
	}
	if !exists || !op.Implemented || name == "resolve" || name == "mappings" || name == "msearch" || name == "get_script" {
		return failure("capability_unavailable", "operation has no public query handler; inspect target capabilities")
	}
	return nil
}
func capabilities(version string) map[string]string {
	result := map[string]string{"query_dsl": "implemented_native_visitors", "scripts_runtime_fields": "implemented_painless_expression_lookup", "aggregations": "implemented_native_visitors", "vector_hybrid": "implemented_supplied_vectors_local_fusion", "es_batch": "implemented_independent", "inference": "awaiting_authority_profile", "plugins": "stock_distribution_only", "suggest_collate": "awaiting_template_source_proof", "sql": "awaiting_implementation", "esql": "awaiting_implementation", "eql": "awaiting_implementation", "api_key_auth": "awaiting_authority_profile", "cross_cluster": "awaiting_authority_profile", "qualification": "fixture_tested_not_server_certified"}
	for name, op := range catalog[version] {
		if op.Effect == "mutation" {
			continue
		}
		status := "awaiting_implementation"
		if op.Implemented {
			status = "implemented"
		}
		result[name] = status
	}
	result["msearch"] = "implemented_via_es_batch"
	result["get_script"] = "internal_proof_requires_script_get_privilege"
	return result
}
