package elasticsearch

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestRemoteTopologyAttestsPinnedAliasesOnEveryEndpoint(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, mode := range []string{"proxy", "sniff"} {
			t.Run(version+"/"+mode, func(t *testing.T) {
				f := newESFixture(t)
				f.version = version
				cfg, limits, controller := fixtureConfig(t, f)
				cfg.Elasticsearch.Version = version
				profile := config.ElasticsearchRemoteCluster{Alias: "archive", Mode: mode, AllowedIndices: []string{"logs-*"}}
				info := `{"archive":{"mode":"proxy","connected":true,"skip_unavailable":false,"cluster_credentials":"::es_redacted::","proxy_address":"proxy.example.invalid:9443","num_proxy_sockets_connected":2}}`
				if mode == "proxy" {
					profile.ProxyAddress = "proxy.example.invalid:9443"
				} else {
					profile.Seeds = []string{"seed-a.example.invalid:9443", "seed-b.example.invalid:9443"}
					info = `{"archive":{"mode":"sniff","connected":true,"skip_unavailable":false,"cluster_credentials":"::es_redacted::","seeds":["seed-b.example.invalid:9443","seed-a.example.invalid:9443"],"num_nodes_connected":2}}`
				}
				cfg.Elasticsearch.RemoteClusters = []config.ElasticsearchRemoteCluster{profile}
				var probes atomic.Int64
				interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
					if r.URL.Path != "/_remote/info" {
						return false
					}
					probes.Add(1)
					fmt.Fprint(w, info)
					return true
				})
				target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
				if err != nil {
					t.Fatal("configured remote topology failed proof", err)
				}
				defer target.Close()
				if probes.Load() != int64(len(cfg.Elasticsearch.Endpoints)) || target.Info().Capabilities["remote_topology"] != "configured_aliases_api_key_model_attested" {
					t.Fatal("remote topology was not attested on configured endpoints")
				}
				if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "mappings"}); err != nil {
					t.Fatal("local metadata was blocked by a valid remote profile", err)
				}
				if controller.Stats().Active != 0 {
					t.Fatal("remote proof leaked admission")
				}
			})
		}
	}
}

func TestRemoteTopologyRejectsDriftAndMissingAPIKeyModel(t *testing.T) {
	profile := config.ElasticsearchRemoteCluster{Alias: "archive", Mode: "proxy", ProxyAddress: "proxy.example.invalid:9443", AllowedIndices: []string{"logs-*"}}
	if _, err := validateRemoteTopology([]byte(`{"archive":{"mode":"proxy","connected":false,"skip_unavailable":true,"cluster_credentials":"::es_redacted::","proxy_address":"proxy.example.invalid:9443"}}`), []config.ElasticsearchRemoteCluster{profile}); err != nil {
		t.Fatal("remote availability state should not block local reads", err)
	}
	for _, tc := range []struct{ name, response string }{
		{"missing-alias", `{}`},
		{"certificate-model", `{"archive":{"mode":"proxy","connected":true,"skip_unavailable":false,"proxy_address":"proxy.example.invalid:9443"}}`},
		{"wrong-marker", `{"archive":{"mode":"proxy","connected":true,"skip_unavailable":false,"cluster_credentials":"other","proxy_address":"proxy.example.invalid:9443"}}`},
		{"wrong-address", `{"archive":{"mode":"proxy","connected":true,"skip_unavailable":false,"cluster_credentials":"::es_redacted::","proxy_address":"other.example.invalid:9443"}}`},
		{"wrong-mode", `{"archive":{"mode":"sniff","connected":true,"skip_unavailable":false,"cluster_credentials":"::es_redacted::","seeds":["seed.example.invalid:9443"]}}`},
		{"missing-availability", `{"archive":{"mode":"proxy","connected":true,"cluster_credentials":"::es_redacted::","proxy_address":"proxy.example.invalid:9443"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := validateRemoteTopology([]byte(tc.response), []config.ElasticsearchRemoteCluster{profile}); err == nil {
				t.Fatal("unproved remote topology accepted")
			}
		})
	}
	f := newESFixture(t)
	cfg, limits, controller := fixtureConfig(t, f)
	cfg.Elasticsearch.RemoteClusters = []config.ElasticsearchRemoteCluster{profile}
	var drift atomic.Bool
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/_remote/info" {
			return false
		}
		if drift.Load() {
			fmt.Fprint(w, `{"archive":{"mode":"proxy","connected":true,"skip_unavailable":false,"cluster_credentials":"::es_redacted::","proxy_address":"other.example.invalid:9443"}}`)
		} else {
			fmt.Fprint(w, `{"archive":{"mode":"proxy","connected":true,"skip_unavailable":false,"cluster_credentials":"::es_redacted::","proxy_address":"proxy.example.invalid:9443"}}`)
		}
		return true
	})
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	drift.Store(true)
	if err := target.refresh(context.Background()); err == nil || target.Info().Healthy {
		t.Fatal("remote topology drift did not invalidate target proof")
	}
}

func TestRemoteTopologyUsesQueryIdentityWithAPIKeyAttestor(t *testing.T) {
	f := newESFixture(t)
	cfg, limits, controller := apiKeyFixtureConfig(t, f)
	cfg.Elasticsearch.RemoteClusters = []config.ElasticsearchRemoteCluster{{Alias: "archive", Mode: "proxy", ProxyAddress: "proxy.example.invalid:9443", AllowedIndices: []string{"logs-*"}}}
	var probes atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/_remote/info" {
			return false
		}
		if r.Header.Get("Authorization") != "APIKey "+f.apiKeyHeader {
			t.Error("attestor credential was used for remote topology")
		}
		probes.Add(1)
		fmt.Fprint(w, `{"archive":{"mode":"proxy","connected":true,"skip_unavailable":false,"cluster_credentials":"::es_redacted::","proxy_address":"proxy.example.invalid:9443"}}`)
		return true
	})
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if probes.Load() != 1 || controller.Stats().Active != 0 {
		t.Fatal("API-key remote topology proof or admission failed")
	}
}
