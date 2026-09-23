package elasticsearch

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

const safeKeyRole = `{"cluster":["monitor"],"indices":[{"names":["reports-*"],"privileges":["read","view_index_metadata"]}],"applications":[],"run_as":[],"global":[]}`
const broadKeyRole = `{"cluster":["all"],"indices":[{"names":["*"],"privileges":["all"]}]}`
const readSecurityProof = `{"cluster":["read_security"],"indices":[],"applications":[],"run_as":[],"global":[]}`

func keyInfo(assigned, limited string) string {
	return fmt.Sprintf(`{"api_keys":[{"id":"fixture-key","username":"fixture_owner","type":"rest","invalidated":false,"expiration":null,"role_descriptors":%s,"limited_by":[%s]}]}`, assigned, limited)
}

func apiKeyFixtureConfig(t *testing.T, f *esFixture) (*config.TargetConfig, config.Limits, *admission.Controller) {
	t.Helper()
	cfg, limits, controller := fixtureConfig(t, f)
	encoded := base64.StdEncoding.EncodeToString([]byte("fixture-key:fixture-secret"))
	t.Setenv("ES_FIXTURE_API_KEY", encoded)
	t.Setenv("ES_FIXTURE_ATTESTOR_PASSWORD", "attestor-secret")
	cfg.Username, cfg.PasswordEnv = "", ""
	cfg.Elasticsearch.APIKey = &config.ElasticsearchAPIKeyConfig{ID: "fixture-key", OwnerUsername: "fixture_owner", KeyEnv: "ES_FIXTURE_API_KEY", Attestor: config.ElasticsearchAttestorConfig{Username: "fixture_attestor", PasswordEnv: "ES_FIXTURE_ATTESTOR_PASSWORD"}}
	f.mu.Lock()
	f.apiKeyHeader = encoded
	f.apiKeyInfo = keyInfo(`{"query":{"cluster":["monitor"],"indices":[{"names":["reports-*"],"privileges":["read","view_index_metadata"]}]}}`, `{"owner":{"cluster":["all"],"indices":[{"names":["*"],"privileges":["all"]}]}}`)
	f.attestorName, f.attestorPassword, f.attestorPrivilege = "fixture_attestor", "attestor-secret", readSecurityProof
	f.mu.Unlock()
	return cfg, limits, controller
}

func TestAPIKeyAttestationAcceptsReadOnlyIntersection(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, tc := range []struct{ name, assigned, limited string }{
			{"assigned-safe", `{"query":` + safeKeyRole + `}`, `{"owner":` + broadKeyRole + `}`},
			{"limited-safe", `{"query":` + broadKeyRole + `}`, `{"owner":` + safeKeyRole + `}`},
			{"split-limiting-roles", `{"query":` + broadKeyRole + `}`, `{"monitor":{"cluster":["monitor"]}},{"read":{"indices":[{"names":["reports-*"],"privileges":["read","view_index_metadata"]}]}}`},
			{"empty-assigned", `{}`, `{"owner":` + safeKeyRole + `}`},
			{"dls-fls", `{"query":{"cluster":["monitor"],"indices":[{"names":["reports-*"],"privileges":["read","view_index_metadata"],"field_security":{"grant":["title"]},"query":"{\"term\":{\"tenant\":\"a\"}}"}]}}`, `{"owner":` + broadKeyRole + `}`},
		} {
			t.Run(fmt.Sprintf("%s/%s", version, tc.name), func(t *testing.T) {
				f := newESFixture(t)
				f.version = version
				cfg, limits, controller := apiKeyFixtureConfig(t, f)
				cfg.Elasticsearch.Version = version
				f.mu.Lock()
				f.apiKeyInfo = keyInfo(tc.assigned, tc.limited)
				f.mu.Unlock()
				target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
				if err != nil {
					t.Fatal("read-only key intersection failed", err)
				}
				defer target.Close()
				out, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "mappings"})
				if err != nil || out == nil || out.Partial || target.Info().Capabilities["api_key_auth"] != "implemented_readonly_attestor" {
					t.Fatal("API key metadata read or capability failed", err)
				}
				if controller.Stats().Active != 0 || len(target.wire.clients) != len(target.wire.attestors) || target.wire.pool.MaxConnsPerHost != cfg.Connection.MaxOpen {
					t.Fatal("API key and attestor did not share target connection budget")
				}
			})
		}
	}
}

func TestAPIKeyAttestationRejectsMissingOrElevatedProof(t *testing.T) {
	for _, tc := range []struct {
		name, info, attestor string
	}{
		{"invalidated", `{"api_keys":[{"id":"fixture-key","username":"fixture_owner","type":"rest","invalidated":true,"expiration":null,"role_descriptors":{"query":` + safeKeyRole + `},"limited_by":[{"owner":` + broadKeyRole + `}]}]}`, readSecurityProof},
		{"expired", `{"api_keys":[{"id":"fixture-key","username":"fixture_owner","type":"rest","invalidated":false,"expiration":1,"role_descriptors":{"query":` + safeKeyRole + `},"limited_by":[{"owner":` + broadKeyRole + `}]}]}`, readSecurityProof},
		{"missing limiting descriptors", `{"api_keys":[{"id":"fixture-key","username":"fixture_owner","type":"rest","invalidated":false,"expiration":null,"role_descriptors":{"query":` + safeKeyRole + `}}]}`, readSecurityProof},
		{"empty assigned with broad owner", keyInfo(`{}`, `{"owner":`+broadKeyRole+`}`), readSecurityProof},
		{"elevated intersection", keyInfo(`{"query":`+broadKeyRole+`}`, `{"owner":`+broadKeyRole+`}`), readSecurityProof},
		{"remote grant on both sides", keyInfo(`{"query":{"cluster":["monitor"],"indices":[{"names":["reports-*"],"privileges":["read"]}],"remote_cluster":[{"clusters":["remote"],"privileges":["monitor_enrich"]}]}}`, `{"owner":{"cluster":["monitor"],"indices":[{"names":["reports-*"],"privileges":["read"]}],"remote_cluster":[{"clusters":["remote"],"privileges":["monitor_enrich"]}]}}`), readSecurityProof},
		{"cross-cluster key", `{"api_keys":[{"id":"fixture-key","username":"fixture_owner","type":"cross_cluster","invalidated":false,"expiration":null,"role_descriptors":{"query":` + safeKeyRole + `},"limited_by":[{"owner":` + broadKeyRole + `}]}]}`, readSecurityProof},
		{"elevated attestor", keyInfo(`{"query":`+safeKeyRole+`}`, `{"owner":`+broadKeyRole+`}`), `{"cluster":["manage_api_key"],"indices":[],"applications":[],"run_as":[],"global":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newESFixture(t)
			cfg, limits, controller := apiKeyFixtureConfig(t, f)
			f.mu.Lock()
			f.apiKeyInfo, f.attestorPrivilege = tc.info, tc.attestor
			f.mu.Unlock()
			if target, err := Open(context.Background(), cfg, limits, controller, nil, nil); err == nil {
				target.Close()
				t.Fatal("unproved API key or attestor started")
			}
		})
	}
}

func TestAPIKeyReattestationDetectsRevocation(t *testing.T) {
	f := newESFixture(t)
	cfg, limits, controller := apiKeyFixtureConfig(t, f)
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	f.mu.Lock()
	f.apiKeyInfo = keyInfo(`{"query":`+safeKeyRole+`}`, `{"owner":`+broadKeyRole+`}`)
	f.mu.Unlock()
	if err := target.attest(context.Background()); err != nil {
		t.Fatal("changed but still read-only descriptor failed", err)
	}
	f.mu.Lock()
	f.apiKeyInfo = `{"api_keys":[{"id":"fixture-key","username":"fixture_owner","type":"rest","invalidated":true,"expiration":null,"role_descriptors":{"query":` + safeKeyRole + `},"limited_by":[{"owner":` + broadKeyRole + `}]}]}`
	f.mu.Unlock()
	if err := target.refresh(context.Background()); err == nil {
		t.Fatal("revoked API key remained attested")
	}
	if target.ready() == nil {
		t.Fatal("revoked API key target remained healthy")
	}
}

func TestAPIKeyCredentialIDMismatchFailsBeforeTransport(t *testing.T) {
	f := newESFixture(t)
	cfg, limits, controller := apiKeyFixtureConfig(t, f)
	t.Setenv("ES_FIXTURE_API_KEY", base64.StdEncoding.EncodeToString([]byte("different-key:secret")))
	if target, err := Open(context.Background(), cfg, limits, controller, nil, nil); err == nil {
		target.Close()
		t.Fatal("API key credential with wrong pinned ID was accepted")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) != 0 {
		t.Fatal("mismatched API key reached Elasticsearch")
	}
}
