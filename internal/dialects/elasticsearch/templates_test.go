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
	"time"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestTemplateControlsMatchNativeExecutionAndSimulation(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, operation := range []string{"search_template", "render_search_template"} {
			for _, tc := range []struct {
				name             string
				controls         map[string]any
				sourceFlag       bool
				explain, profile bool
			}{
				{"defaults", nil, true, false, false},
				{"enabled", map[string]any{"explain": true, "profile": true}, false, true, true},
				{"disabled", map[string]any{"explain": false, "profile": false}, true, false, false},
				{"mixed", map[string]any{"explain": true}, true, true, false},
			} {
				t.Run(version+"/"+operation+"/"+tc.name, func(t *testing.T) {
					f := newESFixture(t)
					f.version = version
					cfg, limits, controller := fixtureConfig(t, f)
					cfg.Elasticsearch.Version = version
					target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
					if err != nil {
						t.Fatal(err)
					}
					defer target.Close()
					var renders, searches atomic.Int64
					rendered := map[string]any{"query": map[string]any{"match_all": map[string]any{}}, "explain": tc.sourceFlag, "profile": tc.sourceFlag}
					interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
						switch r.URL.Path {
						case "/_render/template":
							renders.Add(1)
							raw, _ := io.ReadAll(r.Body)
							body, _ := decodeObject(raw)
							for key, value := range tc.controls {
								if body[key] != value {
									t.Error("native render control changed")
								}
							}
							json.NewEncoder(w).Encode(map[string]any{"template_output": rendered})
						case "/reports-*/_search":
							searches.Add(1)
							raw, _ := io.ReadAll(r.Body)
							body, _ := decodeObject(raw)
							if body["explain"] != tc.explain || body["profile"] != tc.profile {
								t.Errorf("native envelope precedence lost: %s", raw)
							}
							fmt.Fprint(w, nativeSearchResult)
						default:
							return false
						}
						return true
					})
					body := map[string]any{"source": rendered}
					for key, value := range tc.controls {
						body[key] = value
					}
					out, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: operation, Body: mustJSON(t, body)})
					if err != nil {
						t.Fatal(err)
					}
					if operation == "render_search_template" {
						value, _ := decodeObject(out.Data)
						source, _ := object(value["template_output"])
						if source["explain"] != tc.sourceFlag || source["profile"] != tc.sourceFlag || searches.Load() != 0 {
							t.Fatal("simulation changed rendered source or executed a search")
						}
					} else if searches.Load() != 1 {
						t.Fatal("search replayed")
					}
					if renders.Load() != 1 || controller.Stats().Active != 0 {
						t.Fatal("render replay or admission leak")
					}
				})
			}
		}
	}
}

func TestTemplateControlsRejectInvalidTypesBeforeRendering(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var calls atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/_render/template" || strings.HasSuffix(r.URL.Path, "/_search") {
			calls.Add(1)
			fmt.Fprint(w, `{}`)
			return true
		}
		return false
	})
	for _, operation := range []string{"search_template", "render_search_template"} {
		for _, key := range []string{"explain", "profile"} {
			for _, value := range []any{nil, "true", 1, []any{}, map[string]any{}} {
				body := mustJSON(t, map[string]any{"source": map[string]any{"query": map[string]any{"match_all": map[string]any{}}}, key: value})
				if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: operation, Body: body}); err == nil {
					t.Fatal("nonboolean template flag accepted")
				}
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid flags reached render/execution")
	}
}

// Both templates and stored scripts can change after inspection. The execution
// body must contain only the rendered and proved data, never another template.
func TestTemplateBatchFreezesDefinitionsControlsAndLiteralMarkers(t *testing.T) {
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
			var scripts, renders, executions atomic.Int64
			const source = `{"query":{"terms":{"n":{{#toJson}}numbers{{/toJson}}}},"size":{{size}}{{#debug}},"profile":true{{/debug}}}`
			const literal = "{{secret}}\n{{#toJson}}private{{/toJson}}"
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				switch r.URL.Path {
				case "/_scripts/report":
					scripts.Add(1)
					value := source
					if scripts.Load() > 1 {
						value = `{"query":{"terms":{"x":{"index":"private","id":"x","path":"x"}}}}`
					}
					json.NewEncoder(w).Encode(map[string]any{"found": true, "script": map[string]any{"lang": "mustache", "source": value}})
				case "/_render/template":
					renders.Add(1)
					raw, _ := io.ReadAll(r.Body)
					body, _ := decodeObject(raw)
					if body["id"] != nil || body["source"] != source {
						t.Error("stored template was not frozen before render")
					}
					params, _ := object(body["params"])
					if params["size"] != json.Number("2") {
						t.Error("native parameters changed")
					}
					json.NewEncoder(w).Encode(map[string]any{"template_output": map[string]any{"size": params["size"], "query": map[string]any{"bool": map[string]any{"filter": []any{map[string]any{"terms": map[string]any{"n": params["numbers"]}}, map[string]any{"term": map[string]any{"label": literal}}}}}, "profile": true, "explain": true}})
				case "/_msearch":
					executions.Add(1)
					raw, _ := io.ReadAll(r.Body)
					if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-ndjson" || r.URL.Query().Get("max_concurrent_searches") != "1" || !strings.HasSuffix(string(raw), "\n") {
						t.Error("invalid native batch framing")
					}
					lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
					if len(lines) != 4 {
						t.Errorf("unexpected frames: %s", raw)
						return true
					}
					for i := 0; i < 2; i++ {
						header, _ := decodeObject([]byte(lines[2*i]))
						body, _ := decodeObject([]byte(lines[2*i+1]))
						indices, _ := header["index"].([]any)
						if len(indices) != 1 || indices[0] != "reports-alias" || header["routing"] != fmt.Sprint(i) {
							t.Error("template header scope/routing changed")
						}
						if body["profile"] != (i == 0) || body["explain"] != (i == 1) || body["source"] != nil || body["id"] != nil || body["timeout"] == nil {
							t.Error("unfrozen template or incorrect controls")
						}
						if !strings.Contains(lines[2*i+1], "9007199254740993") || !strings.Contains(lines[2*i+1], `{{secret}}\n{{#toJson}}private{{/toJson}}`) {
							t.Error("literal markers or parameter precision changed")
						}
					}
					fmt.Fprintf(w, `{"responses":[%s,%s]}`, nativeSearchResult, nativeSearchResult)
				case "/_msearch/template":
					t.Error("uncancellable template execution route used")
				default:
					return false
				}
				return true
			})
			requests := make([]core.ElasticsearchQueryRequest, 2)
			for i := range requests {
				requests[i] = core.ElasticsearchQueryRequest{Operation: "search_template", Indices: []string{"reports-alias"}, Body: mustJSON(t, map[string]any{"id": "report", "params": map[string]any{"numbers": []any{json.Number("9007199254740993")}, "size": 2, "debug": true}, "profile": i == 0, "explain": i == 1}), Options: map[string]json.RawMessage{"routing": mustJSON(t, fmt.Sprint(i))}}
			}
			out, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: requests})
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Results) != 2 || scripts.Load() != 1 || renders.Load() != 2 || executions.Load() != 1 || controller.Stats().Active != 0 {
				t.Fatal("incorrect frozen batch lifecycle")
			}
			if target.Info().Capabilities["msearch_template"] != "implemented_via_es_batch" {
				t.Fatal("template batch capability not advertised")
			}
			if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: "msearch_template"}); err == nil {
				t.Fatal("raw multi-template route exposed")
			}
		})
	}
}

func TestTemplateBatchRejectsUnprovedMemberAndPartialResults(t *testing.T) {
	for _, fault := range []string{"private-source", "write-field", "item-error", "partial", "count", "escaped-hit"} {
		t.Run(fault, func(t *testing.T) {
			f := newESFixture(t)
			target, controller := openFixture(t, f)
			var renders, executions atomic.Int64
			interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
				switch r.URL.Path {
				case "/_render/template":
					count := renders.Add(1)
					if count == 2 && fault == "private-source" {
						fmt.Fprint(w, `{"template_output":{"query":{"terms":{"n":{"index":"private","id":"x","path":"n"}}}}}`)
					} else if count == 2 && fault == "write-field" {
						fmt.Fprint(w, `{"template_output":{"update":{"x":1}}}`)
					} else {
						fmt.Fprint(w, `{"template_output":{"query":{"match_all":{}}}}`)
					}
				case "/_msearch":
					executions.Add(1)
					second := nativeSearchResult
					switch fault {
					case "item-error":
						second = `{"error":{"reason":"private upstream details"},"status":400}`
					case "partial":
						second = strings.Replace(second, `"timed_out":false`, `"timed_out":true`, 1)
					case "escaped-hit":
						second = strings.ReplaceAll(second, "reports-2026", "private")
					}
					if fault == "count" {
						fmt.Fprintf(w, `{"responses":[%s]}`, nativeSearchResult)
					} else {
						fmt.Fprintf(w, `{"responses":[%s,%s]}`, nativeSearchResult, second)
					}
				default:
					return false
				}
				return true
			})
			request := core.ElasticsearchQueryRequest{Operation: "search_template", Body: json.RawMessage(`{"source":"native template","profile":true,"explain":true}`)}
			out, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{request, request}})
			if err == nil || out != nil {
				t.Fatal("invalid template batch returned data")
			}
			want := int64(1)
			if fault == "private-source" || fault == "write-field" {
				want = 0
			}
			if executions.Load() != want || controller.Stats().Active != 0 {
				t.Fatal("batch preflight/replay or admission failure")
			}
		})
	}
}

func TestTemplateBatchCancellationClosesExecutionChannel(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	entered, closed := make(chan struct{}), make(chan struct{})
	var calls atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/_render/template" {
			fmt.Fprint(w, `{"template_output":{"query":{"match_all":{}}}}`)
			return true
		}
		if r.URL.Path != "/_msearch" {
			return false
		}
		io.ReadAll(r.Body)
		calls.Add(1)
		close(entered)
		select {
		case <-r.Context().Done():
			close(closed)
		case <-time.After(2 * time.Second):
			t.Error("batch channel did not close")
		}
		return true
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := target.ElasticsearchBatch(ctx, core.ElasticsearchBatchRequest{Requests: []core.ElasticsearchQueryRequest{{Operation: "search_template", Body: json.RawMessage(`{"source":"{}","profile":true}`)}}})
		done <- err
	}()
	select {
	case <-entered:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("execution not reached")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled batch succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("batch did not cancel")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("HTTP execution channel stayed open")
	}
	if calls.Load() != 1 || controller.Stats().Active != 0 {
		t.Fatal("cancelled batch replayed or leaked admission")
	}
}

func TestTemplateBatchNonHeaderOptionsKeepNativePrecedence(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var calls atomic.Int64
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/_render/template":
			fmt.Fprint(w, `{"template_output":{"query":{"match_all":{}}}}`)
		case "/reports-*/_search":
			i := calls.Add(1) - 1
			raw, _ := io.ReadAll(r.Body)
			body, _ := decodeObject(raw)
			// Native RestSearchTemplateAction gives the explain URL option
			// precedence over the envelope. Keep it on the individual search.
			if r.URL.Query().Get("explain") != fmt.Sprint(i != 0) || body["explain"] != (i == 0) || body["profile"] != (i == 0) {
				t.Error("sequential template controls/options changed")
			}
			fmt.Fprint(w, nativeSearchResult)
		case "/_msearch":
			t.Error("non-header options were silently discarded")
		default:
			return false
		}
		return true
	})
	requests := make([]core.ElasticsearchQueryRequest, 2)
	for i := range requests {
		requests[i] = core.ElasticsearchQueryRequest{Operation: "search_template", Body: mustJSON(t, map[string]any{"source": "{}", "profile": i == 0, "explain": i == 0}), Options: map[string]json.RawMessage{"explain": mustJSON(t, i != 0)}}
	}
	if _, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: requests}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("sequential batch did not preserve members")
	}
}
