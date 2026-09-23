package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestAllowlistedStoredScriptMetadataPreservesNativeDefinition(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		t.Run(version, func(t *testing.T) {
			f := newESFixture(t)
			f.version = version
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Version = version
			cfg.Elasticsearch.ReadableScriptIDs = []string{"report/template"}
			target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			var reads atomic.Int64
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if !strings.HasPrefix(r.URL.Path, "/_scripts/") {
					return false
				}
				reads.Add(1)
				if r.Method != http.MethodGet || r.URL.Query().Get("master_timeout") != "100ms" || strings.Contains(r.RequestURI, "?id=") || !strings.Contains(r.RequestURI, "%2F") {
					t.Error("stored script route or deadline changed", r.RequestURI)
				}
				fmt.Fprint(w, `{"_id":"report/template","found":true,"script":{"lang":"mustache","source":{"query":{"match":{"title":"{{term}}"}},"size":9007199254740993}}}`)
				return true
			})
			out, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "get_script", Names: []string{"report/template"}, Options: map[string]json.RawMessage{"master_timeout": json.RawMessage(`"100ms"`)}})
			if err != nil || out == nil || !strings.Contains(string(out.Data), "9007199254740993") {
				t.Fatal("native stored script metadata was not preserved", err)
			}
			if reads.Load() != 1 || target.Info().Capabilities["get_script"] != "implemented_allowlisted_metadata" || controller.Stats().Active != 0 {
				t.Fatal("stored script capability, dispatch or admission changed")
			}
		})
	}
}

func TestStoredScriptMetadataRequiresOperatorDisclosureGrant(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	if target.Info().Capabilities["get_script"] != "internal_proof_requires_script_get_privilege" {
		t.Fatal("stored script metadata was advertised without an operator grant")
	}
	if out, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "get_script", Names: []string{"report"}}); err == nil || out != nil {
		t.Fatal("stored script definition disclosed without an operator grant")
	}
	if controller.Stats().Active != 0 {
		t.Fatal("denied stored script metadata leaked admission")
	}
}

func TestStoredScriptMetadataFailsClosedOnScopeAndResponse(t *testing.T) {
	f := newESFixture(t)
	cfg, limits, controller := fixtureConfig(t, f)
	cfg.Elasticsearch.ReadableScriptIDs = []string{"report"}
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	var reads atomic.Int64
	response := `{"_id":"private","found":true,"script":{"lang":"mustache","source":"{}"}}`
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasPrefix(r.URL.Path, "/_scripts/") {
			return false
		}
		reads.Add(1)
		fmt.Fprint(w, response)
		return true
	})
	for _, request := range []core.ElasticsearchMetadataRequest{
		{Operation: "get_script", Names: []string{"private"}},
		{Operation: "get_script", Names: []string{"report", "private"}},
		{Operation: "get_script", Names: []string{"report"}, Indices: []string{"reports-*"}},
		{Operation: "get_script", Names: []string{"report"}, Options: map[string]json.RawMessage{"master_timeout": json.RawMessage(`"nonsense"`)}},
	} {
		if out, err := target.ElasticsearchMetadata(context.Background(), request); err == nil || out != nil {
			t.Fatal("unsafe stored script request returned data", request)
		}
	}
	if reads.Load() != 0 {
		t.Fatal("invalid stored script request reached Elasticsearch")
	}
	for _, invalid := range []string{
		`{"_id":"private","found":true,"script":{"lang":"mustache","source":"{}"}}`,
		`{"_id":"report","found":false,"script":{"lang":"mustache","source":"{}"}}`,
		`{"_id":"report","found":true,"script":null}`,
		`{"_id":"report","found":true,"script":{"lang":"mustache"}}`,
		`{"_id":"report","found":true,"script":{"lang":"mustache","source":"{}"},"_id":"private"}`,
	} {
		response = invalid
		if out, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "get_script", Names: []string{"report"}}); err == nil || out != nil {
			t.Fatal("untrusted script response returned data", invalid)
		}
	}
	if reads.Load() != 5 || controller.Stats().Active != 0 {
		t.Fatal("stored script rejection leaked admission")
	}
}
