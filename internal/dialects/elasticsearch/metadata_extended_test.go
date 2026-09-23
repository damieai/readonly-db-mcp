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

func TestNativeAliasAndSettingsMetadata(t *testing.T) {
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
			for _, operation := range []string{"aliases", "settings"} {
				if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: operation}); err == nil {
					t.Fatal("metadata operation was routed through es_query", operation)
				}
			}
			var reads atomic.Int64
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				switch r.URL.Path {
				case "/reports-*/_alias":
					reads.Add(1)
					if r.Method != http.MethodGet || r.URL.Query().Get("ignore_unavailable") != "false" {
						t.Error("native alias route/options changed")
					}
					fmt.Fprint(w, `{"reports-2026":{"aliases":{"reports-date":{"filter":{"term":{"tenant":"a"}},"search_routing":"tenant-a"}}}}`)
				case "/reports-*/_settings":
					reads.Add(1)
					if r.Method != http.MethodGet || r.URL.Query().Get("flat_settings") != "true" || r.URL.Query().Get("include_defaults") != "true" {
						t.Error("native settings route/options changed")
					}
					fmt.Fprint(w, `{"reports-2026":{"settings":{"index.number_of_docs":"9007199254740993"},"defaults":{"index.number_of_shards":"1"}}}`)
				case "/" + daySource + "/_settings":
					reads.Add(1)
					if !strings.Contains(r.RequestURI, "%2F") || !strings.Contains(r.RequestURI, "%2B08%3A00") {
						t.Error("date-math metadata path was not encoded")
					}
					fmt.Fprint(w, `{"reports-2026":{"settings":{"index.number_of_shards":"1"}}}`)
				default:
					return false
				}
				return true
			})
			for _, request := range []core.ElasticsearchMetadataRequest{
				{Operation: "aliases", Options: map[string]json.RawMessage{"ignore_unavailable": json.RawMessage(`false`)}},
				{Operation: "settings", Options: map[string]json.RawMessage{"flat_settings": json.RawMessage(`true`), "include_defaults": json.RawMessage(`true`)}},
				{Operation: "settings", Indices: []string{daySource}},
			} {
				out, err := target.ElasticsearchMetadata(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				if out.Partial || out.Truncated || !strings.Contains(string(out.Data), "reports-2026") {
					t.Fatal("native metadata result was changed")
				}
			}
			if reads.Load() != 3 || controller.Stats().Active != 0 || target.Info().Capabilities["aliases"] != "implemented" || target.Info().Capabilities["settings"] != "implemented" {
				t.Fatal("native metadata capability, execution or admission mismatch")
			}
		})
	}
}

func TestAliasSettingsMetadataScopeAndDrift(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	var reads atomic.Int64
	response := `{"reports-2026":{"aliases":{"reports-date":{}}}}`
	drift := false
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasSuffix(r.URL.Path, "/_alias") && !strings.HasSuffix(r.URL.Path, "/_settings") {
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
	for _, tc := range []struct{ operation, raw string }{
		{"aliases", `{"reports-2026":{"aliases":{"private-hidden":{}}}}`},
		{"aliases", `{"private-2026":{"aliases":{"reports-date":{}}}}`},
		{"aliases", `{"reports-2026":{"aliases":null}}`},
		{"aliases", `{"reports-2026":{"aliases":{}},"reports-2026":{"aliases":{"private-hidden":{}}}}`},
		{"settings", `{"private-2026":{"settings":{}}}`},
		{"settings", `{"reports-2026":{"settings":null}}`},
	} {
		response = tc.raw
		if out, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: tc.operation}); err == nil || out != nil {
			t.Fatal("unsafe metadata response returned", tc)
		}
	}
	response = `{"reports-2026":{"settings":{}}}`
	drift = true
	if out, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "settings"}); err == nil || out != nil {
		t.Fatal("metadata inventory drift returned a result")
	}
	if reads.Load() != 7 || controller.Stats().Active != 0 {
		t.Fatal("metadata rejection leaked admission")
	}
	for _, options := range []map[string]json.RawMessage{{"unknown": json.RawMessage(`true`)}, {"flat_settings": json.RawMessage(`{"nested":"bad"`)}} {
		if out, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "settings", Options: options}); err == nil || out != nil {
			t.Fatal("invalid settings option was accepted")
		}
	}
}

func TestDataStreamAliasMetadataKeepsLogicalName(t *testing.T) {
	f := newESFixture(t)
	backing := ".ds-reports-stream-2026.09.23-000001"
	f.resolution = `{"indices":[],"aliases":[],"data_streams":[{"name":"reports-stream","backing_indices":["` + backing + `"],"timestamp_field":"@timestamp"}]}`
	target, _ := openFixture(t, f)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/reports-stream/_alias":
			fmt.Fprint(w, `{"reports-stream":{"aliases":{"reports-feed":{}}}}`)
		case "/reports-stream/_settings":
			fmt.Fprintf(w, `{"%s":{"settings":{"index.number_of_shards":"1"}}}`, backing)
		default:
			return false
		}
		return true
	})
	for _, operation := range []string{"aliases", "settings"} {
		if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: operation, Indices: []string{"reports-stream"}}); err != nil {
			t.Fatal("data stream metadata lost its logical source or backing index", operation, err)
		}
	}
}
