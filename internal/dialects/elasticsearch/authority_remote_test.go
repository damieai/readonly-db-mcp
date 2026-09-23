package elasticsearch

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func remoteAuthorityProof(index, cluster string) []byte {
	raw := string(readonlyProof())
	var fields []string
	if index != "" {
		fields = append(fields, `"remote_indices":[`+index+`]`)
	}
	if cluster != "" {
		fields = append(fields, `"remote_cluster":[`+cluster+`]`)
	}
	return []byte(strings.TrimSuffix(raw, "}") + "," + strings.Join(fields, ",") + "}")
}

func TestRemoteReadGrantsDoNotBlockLocalElasticsearchReads(t *testing.T) {
	index := `{"clusters":["_archive*"],"names":["logs-*"],"privileges":["read","read_cross_cluster","view_index_metadata"],"allow_restricted_indices":false,"field_security":{"grant":["message"]}}`
	cluster := `{"clusters":"_archive","privileges":["monitor_enrich","monitor_stats"]}`
	for _, version := range []string{"8.19.21", "9.1.10"} {
		t.Run(version, func(t *testing.T) {
			f := newESFixture(t)
			f.version = version
			f.privilege = string(remoteAuthorityProof(index, cluster))
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Version = version
			target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
			if err != nil {
				t.Fatal("read-only remote role blocked local target", err)
			}
			defer target.Close()
			if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "mappings"}); err != nil {
				t.Fatal("local metadata read failed", err)
			}
			if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Indices: []string{"archive:logs-*"}, Body: json.RawMessage(`{"query":{"match_all":{}}}`)}); err == nil {
				t.Fatal("remote query dispatched without remote-side authority proof")
			}
			f.mu.Lock()
			for _, path := range f.requests {
				if strings.Contains(path, "archive:") || strings.HasSuffix(path, "/_search") {
					f.mu.Unlock()
					t.Fatal("remote query reached Elasticsearch without remote-side proof")
				}
			}
			f.mu.Unlock()
			if controller.Stats().Active != 0 {
				t.Fatal("remote source rejection leaked admission")
			}
		})
	}
}

func TestRemoteAuthorityRejectsMutationAndUnknownGrantShapes(t *testing.T) {
	cfg := &config.ElasticsearchConfig{AllowedIndices: []string{"reports-*"}}
	for _, tc := range []struct {
		name, index, cluster string
	}{
		{"remote-write", `{"clusters":["archive"],"names":["logs-*"],"privileges":["write"]}`, ""},
		{"replication", `{"clusters":["archive"],"names":["logs-*"],"privileges":["cross_cluster_replication"]}`, ""},
		{"restricted", `{"clusters":["archive"],"names":["logs-*"],"privileges":["read"],"allow_restricted_indices":true}`, ""},
		{"unknown-index-field", `{"clusters":["archive"],"names":["logs-*"],"privileges":["read"],"future_authority":true}`, ""},
		{"bad-alias", `{"clusters":["archive:other"],"names":["logs-*"],"privileges":["read"]}`, ""},
		{"missing-index-name", `{"clusters":["archive"],"privileges":["read"]}`, ""},
		{"unknown-cluster-privilege", "", `{"clusters":["archive"],"privileges":["manage"]}`},
		{"unknown-cluster-field", "", `{"clusters":["archive"],"privileges":["monitor_stats"],"future_authority":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validatePrivileges(context.Background(), remoteAuthorityProof(tc.index, tc.cluster), cfg); err == nil {
				t.Fatal("remote authority outside the read-only profile accepted")
			}
		})
	}
}
