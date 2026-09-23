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

func TestNativeDocumentExistenceAndSourceReads(t *testing.T) {
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
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				if !strings.Contains(r.URL.Path, "/_doc/") && !strings.Contains(r.URL.Path, "/_source/") {
					return false
				}
				reads.Add(1)
				if !strings.Contains(r.RequestURI, "a%2Fb%3F%23%25") || r.URL.Query().Get("routing") != "tenant-a" || r.URL.Query().Get("realtime") != "false" {
					t.Error("native document route, ID escaping or options changed", r.RequestURI)
				}
				if r.Method == http.MethodHead {
					// HEAD may report the size of its GET representation, but has no body.
					w.Header().Set("Content-Length", "999999")
					w.WriteHeader(http.StatusOK)
					return true
				}
				if r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/_source/") {
					t.Error("unexpected document read method")
				}
				fmt.Fprint(w, `{"error":"ordinary document value","pit_id":"also ordinary","_scroll_id":"also ordinary","precise":9007199254740993}`)
				return true
			})
			base := core.ElasticsearchQueryRequest{Indices: []string{"reports-2026"}, ID: "a/b?#%", Options: map[string]json.RawMessage{"routing": json.RawMessage(`"tenant-a"`), "realtime": json.RawMessage(`false`)}}
			for _, operation := range []string{"exists", "exists_source", "get_source"} {
				request := base
				request.Operation = operation
				out, err := target.ElasticsearchQuery(context.Background(), request)
				if err != nil {
					t.Fatal(operation, err)
				}
				if out.Consistency != "eventual" || out.Partial || out.Truncated || target.Info().Capabilities[operation] != "implemented" {
					t.Fatal("document read capability or consistency changed", operation)
				}
				if operation == "get_source" {
					if string(out.Data) != `{"error":"ordinary document value","pit_id":"also ordinary","_scroll_id":"also ordinary","precise":9007199254740993}` {
						t.Fatal("native source data changed", string(out.Data))
					}
				} else if string(out.Data) != `{"exists":true}` {
					t.Fatal("HEAD existence result changed", operation, string(out.Data))
				}
			}
			exists, source := base, base
			exists.Operation, source.Operation = "exists", "get_source"
			batch, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{exists, source}})
			if err != nil || len(batch.Results) != 2 || string(batch.Results[0].Data) != `{"exists":true}` || !strings.Contains(string(batch.Results[1].Data), `"precise":9007199254740993`) {
				t.Fatal("independent document batch changed", err)
			}
			if reads.Load() != 5 || controller.Stats().Active != 0 {
				t.Fatal("document reads or admission did not complete")
			}
		})
	}
}

func TestDocumentReadMissingScopeAndSideEffects(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	var reads atomic.Int64
	status := http.StatusNotFound
	source := `{"error":{"type":"resource_not_found_exception"}}`
	drift := false
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.Contains(r.URL.Path, "/_doc/") && !strings.Contains(r.URL.Path, "/_source/") {
			return false
		}
		reads.Add(1)
		if drift {
			f.mu.Lock()
			f.resolution = `{"indices":[{"name":"reports-2027","attributes":["open"]}],"aliases":[],"data_streams":[]}`
			f.mu.Unlock()
		}
		w.WriteHeader(status)
		if r.Method == http.MethodGet {
			fmt.Fprint(w, source)
		}
		return true
	})
	for _, operation := range []string{"exists", "exists_source"} {
		out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: operation, Indices: []string{"reports-2026"}, ID: "missing"})
		if err != nil || string(out.Data) != `{"exists":false}` || out.Consistency != "realtime" {
			t.Fatal("missing document/source existence changed", operation, err)
		}
	}
	if out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "get_source", Indices: []string{"reports-2026"}, ID: "missing"}); err == nil || out != nil {
		t.Fatal("missing source returned a document")
	}
	status = http.StatusNoContent
	if out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "exists", Indices: []string{"reports-2026"}, ID: "missing"}); err == nil || out != nil {
		t.Fatal("unexpected HEAD status returned an existence result")
	}
	status = http.StatusOK
	source = `{"value":1}`
	drift = true
	if out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "get_source", Indices: []string{"reports-2026"}, ID: "a"}); err == nil || out != nil {
		t.Fatal("source inventory drift returned a result")
	}
	if reads.Load() != 5 || controller.Stats().Active != 0 {
		t.Fatal("failed document reads leaked admission")
	}
	for _, request := range []core.ElasticsearchQueryRequest{
		{Operation: "exists", Indices: []string{"private-2026"}, ID: "a"},
		{Operation: "get_source", Indices: []string{"reports-2026"}},
		{Operation: "exists_source", Indices: []string{"reports-2026"}, ID: "a", Body: json.RawMessage(`{"unexpected":true}`)},
		{Operation: "get_source", Indices: []string{"reports-2026"}, ID: "a", Options: map[string]json.RawMessage{"refresh": json.RawMessage(`true`)}},
	} {
		if out, err := target.ElasticsearchQuery(context.Background(), request); err == nil || out != nil {
			t.Fatal("unsafe document request was accepted", request.Operation)
		}
	}
}
