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

func TestNativeFieldMappingsByNameAndWildcard(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		t.Run(version, func(t *testing.T) {
			f := newESFixture(t)
			f.version = version
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Version = version
			target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			var reads atomic.Int64
			var fullReads atomic.Int64
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/reports-*/_mapping" {
					fullReads.Add(1)
					if r.URL.Query().Get("allow_no_indices") != "false" {
						t.Error("native full mapping option was not forwarded")
					}
					fmt.Fprint(w, f.mapping)
					return true
				}
				if r.URL.Path == "/reports-*/_mapping/field/user name,custom%value,tag#value" {
					reads.Add(1)
					if !strings.Contains(r.RequestURI, "user%20name") || !strings.Contains(r.RequestURI, "custom%25value") || !strings.Contains(r.RequestURI, "tag%23value") || r.URL.Fragment != "" {
						t.Error("literal field names were not kept in the path")
					}
					fmt.Fprint(w, `{"reports-2026":{"mappings":{"user name":{"full_name":"user name","mapping":{"user name":{"type":"keyword"}}}}}}`)
					return true
				}
				if r.URL.Path != "/reports-*/_mapping/field/title*,event.?d" {
					return false
				}
				reads.Add(1)
				if r.Method != http.MethodGet || !strings.Contains(r.RequestURI, "%3F") || r.URL.Query().Get("include_defaults") != "true" {
					t.Error("field mapping route, wildcard encoding or native options changed")
				}
				fmt.Fprint(w, `{"reports-2026":{"mappings":{"title":{"full_name":"title","mapping":{"title":{"type":"text","fields":{"keyword":{"type":"keyword"}}}}},"event.id":{"full_name":"event.id","mapping":{"id":{"type":"keyword"}}}}}}`)
				return true
			})
			out, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "field_mappings", Fields: []string{"title*", "event.?d"}, Options: map[string]json.RawMessage{"include_defaults": json.RawMessage(`true`)}})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out.Data), `"event.id"`) || reads.Load() != 1 || controller.Stats().Active != 0 || target.Info().Capabilities["field_mappings"] != "implemented" {
				t.Fatal("native field mapping result, admission or capability changed")
			}
			if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "field_mappings", Fields: []string{"user name", "custom%value", "tag#value"}}); err != nil || reads.Load() != 2 {
				t.Fatal("literal field selectors were rejected or changed", err)
			}
			if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "mappings", Options: map[string]json.RawMessage{"allow_no_indices": json.RawMessage(`false`)}}); err != nil || fullReads.Load() != 1 {
				t.Fatal("native full mapping options were lost", err)
			}
			if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "field_mappings"}); err == nil {
				t.Fatal("metadata operation was routed through es_query")
			}
		})
	}
}

func TestFieldMappingsRejectEscapesMalformedResultsAndDrift(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	response := `{"reports-2026":{"mappings":{"title":{"full_name":"title","mapping":{"title":{"type":"text"}}}}}}`
	drift := false
	var reads atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.Contains(r.URL.Path, "/_mapping/field/") {
			return false
		}
		reads.Add(1)
		if drift {
			f.mu.Lock()
			f.resolution = `{"indices":[{"name":"reports-2027","attributes":["open"]}],"aliases":[],"data_streams":[]}`
			f.mu.Unlock()
		}
		fmt.Fprint(w, response)
		return true
	})
	request := core.ElasticsearchMetadataRequest{Operation: "field_mappings", Fields: []string{"title*"}}
	for _, raw := range []string{
		`{"private-2026":{"mappings":{}}}`,
		`{"reports-2026":{"mappings":null}}`,
		`{"reports-2026":{"mappings":{"title":{"full_name":"title"}}}}`,
		`{"reports-2026":{"mappings":{"title":{"full_name":null,"mapping":{}}}}}`,
		`{"reports-2026":{"mappings":{}},"reports-2026":{"mappings":{"title":{"full_name":"title","mapping":{}}}}}`,
	} {
		response = raw
		if result, err := target.ElasticsearchMetadata(context.Background(), request); err == nil || result != nil {
			t.Fatal("unsafe field mappings response returned", raw)
		}
	}
	response = `{"reports-2026":{"mappings":{}}}`
	drift = true
	if result, err := target.ElasticsearchMetadata(context.Background(), request); err == nil || result != nil {
		t.Fatal("field mappings inventory drift returned a result")
	}
	if reads.Load() != 6 || controller.Stats().Active != 0 {
		t.Fatal("field mapping rejection leaked admission")
	}
	for _, invalid := range []core.ElasticsearchMetadataRequest{
		{Operation: "field_mappings"},
		{Operation: "field_mappings", Fields: []string{"title/private"}},
		{Operation: "field_mappings", Fields: []string{"title,private"}},
		{Operation: "mappings", Fields: []string{"title"}},
		{Operation: "resolve", Fields: []string{"title"}},
	} {
		if result, err := target.ElasticsearchMetadata(context.Background(), invalid); err == nil || result != nil {
			t.Fatal("invalid field mappings request was accepted", invalid)
		}
	}
}
