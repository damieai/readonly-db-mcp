package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestPhraseCollateFreezesAndProvesNativeTemplate(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, stored := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stored=%t", version, stored), func(t *testing.T) {
				f := newESFixture(t)
				f.version = version
				cfg, limits, controller := fixtureConfig(t, f)
				cfg.Elasticsearch.Version = version
				target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer target.Close()
				var searches, renders atomic.Int64
				interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
					switch r.URL.Path {
					case "/_scripts/collate-template":
						fmt.Fprint(w, `{"script":{"lang":"mustache","source":{"match":{"{{field_name}}":"{{suggestion}}"}}}}`)
					case "/_render/template":
						renders.Add(1)
						raw, _ := io.ReadAll(r.Body)
						body, _ := decodeObject(raw)
						params, _ := object(body["params"])
						if params["field_name"] != "title" || !strings.HasPrefix(params["suggestion"].(string), ":collate-") {
							t.Error("collate template parameters changed")
						}
						json.NewEncoder(w).Encode(map[string]any{"template_output": map[string]any{"match": map[string]any{"title": params["suggestion"]}}})
					case "/reports-*/_search":
						searches.Add(1)
						raw, _ := io.ReadAll(r.Body)
						body, _ := decodeObject(raw)
						suggest, _ := object(body["suggest"])
						entry, _ := object(suggest["spell"])
						phrase, _ := object(entry["phrase"])
						collate, _ := object(phrase["collate"])
						query, _ := object(collate["query"])
						source, _ := query["source"].(string)
						if _, exists := collate["params"]; exists || collate["prune"] != true || source != `{"match":{"title":{{#toJson}}suggestion{{/toJson}}}}` {
							t.Errorf("unfrozen or unproved collate: %s", raw)
						}
						fmt.Fprint(w, nativeSearchResult)
					default:
						return false
					}
					return true
				})
				query := map[string]any{"source": map[string]any{"match": map[string]any{"{{field_name}}": "{{suggestion}}"}}}
				if stored {
					query = map[string]any{"id": "collate-template"}
				}
				body := map[string]any{"size": 0, "suggest": map[string]any{"spell": map[string]any{"text": "nobel prise", "phrase": map[string]any{"field": "title", "collate": map[string]any{"query": query, "params": map[string]any{"field_name": "title"}, "prune": true}}}}}
				if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Body: mustJSON(t, body)}); err != nil {
					t.Fatal(err)
				}
				if searches.Load() != 1 || renders.Load() != 1 || controller.Stats().Active != 0 {
					t.Fatal("collate was not rendered and executed exactly once")
				}
			})
		}
	}
}

func TestPhraseCollateRejectsCandidateControlledSources(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var searches atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/_render/template":
			raw, _ := io.ReadAll(r.Body)
			body, _ := decodeObject(raw)
			params, _ := object(body["params"])
			marker := params["suggestion"]
			source, _ := object(body["source"])
			switch {
			case source["terms"] != nil:
				json.NewEncoder(w).Encode(map[string]any{"template_output": map[string]any{"terms": map[string]any{"title": map[string]any{"index": "reports-" + marker.(string), "id": "1", "path": "title"}}}})
			case source["script"] != nil:
				json.NewEncoder(w).Encode(map[string]any{"template_output": map[string]any{"script": map[string]any{"script": map[string]any{"source": marker}}}})
			default:
				json.NewEncoder(w).Encode(map[string]any{"template_output": map[string]any{"match": map[string]any{marker.(string): "x"}}})
			}
		case "/reports-*/_search":
			searches.Add(1)
			fmt.Fprint(w, nativeSearchResult)
		default:
			return false
		}
		return true
	})
	for _, source := range []any{
		map[string]any{"terms": map[string]any{"title": map[string]any{"index": "reports-{{suggestion}}", "id": "1", "path": "title"}}},
		map[string]any{"script": map[string]any{"script": map[string]any{"source": "{{suggestion}}"}}},
		map[string]any{"match": map[string]any{"{{suggestion}}": "x"}},
		map[string]any{"match": map[string]any{"title": "{{#suggestion}}x{{/suggestion}}"}},
	} {
		body := map[string]any{"size": 0, "suggest": map[string]any{"spell": map[string]any{"phrase": map[string]any{"field": "title", "collate": map[string]any{"query": map[string]any{"source": source}}}}}}
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "search", Body: mustJSON(t, body)}); err == nil {
			t.Fatal("candidate-controlled query shape or source was accepted")
		}
	}
	if searches.Load() != 0 {
		t.Fatal("rejected collate was executed")
	}
}
