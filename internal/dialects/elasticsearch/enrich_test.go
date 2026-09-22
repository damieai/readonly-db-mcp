package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Matches IndexMetadata's API serialization, including flat settings, typed
// mappings and alias name arrays. Historical snapshots have no active alias.
type enrichFixture struct {
	mu       sync.Mutex
	state    map[string]any
	policies map[string]any
	calls    atomic.Int64
	onQuery  func(map[string]any)
}

func newEnrichFixture(t *testing.T, f *esFixture) *enrichFixture {
	t.Helper()
	f.privilege = strings.Replace(f.privilege, `"monitor"`, `"monitor","monitor_enrich"`, 1)
	e := &enrichFixture{policies: map[string]any{}}
	indices := map[string]any{}
	for i, kind := range []string{"match", "range", "geo_match"} {
		name := []string{"customers", "ranges", "geo"}[i]
		fieldType := []string{"keyword", "long_range", "geo_shape"}[i]
		indices[".enrich-"+name+"-1234"] = map[string]any{
			"state": "open", "aliases": []any{".enrich-" + name},
			"settings": map[string]any{"index.uuid": fmt.Sprintf("snapshot-uuid-%016d", i), "index.blocks.write": "true", "index.number_of_shards": "1"},
			"mappings": map[string]any{"_doc": map[string]any{"dynamic": "false", "_meta": map[string]any{"enrich_policy_name": name, "enrich_policy_type": kind, "enrich_match_field": "id"}, "properties": map[string]any{"id": map[string]any{"type": fieldType}, "label": map[string]any{"type": "keyword", "index": false}}}},
		}
		// These sources are outside ordinary allowed_indices. They describe a
		// prior snapshot build; explicit cluster snapshot scope grants its data.
		e.policies[name] = map[string]any{"policies": []any{map[string]any{"config": map[string]any{kind: map[string]any{"name": name, "indices": []any{"private-source-*"}, "match_field": "id", "enrich_fields": []any{"label"}, "query": map[string]any{"historical_build_filter": map[string]any{"opaque": true}}}}}}}
	}
	e.state = map[string]any{"state_uuid": "enrich-state-1", "version": 1, "metadata": map[string]any{"cluster_uuid": f.cluster, "indices": indices}}
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		switch {
		case r.URL.Path == enrichStatePath:
			if r.Method != http.MethodGet || r.URL.Query().Get("flat_settings") != "true" || r.URL.Query().Get("expand_wildcards") != "all" || r.URL.Query().Get("master_timeout") == "" {
				t.Error("invalid native snapshot proof request")
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			json.NewEncoder(w).Encode(e.state)
		case strings.HasPrefix(r.URL.Path, "/_enrich/policy/"):
			if r.Method != http.MethodGet {
				t.Error("policy proof must be read-only")
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			json.NewEncoder(w).Encode(e.policies[strings.TrimPrefix(r.URL.Path, "/_enrich/policy/")])
		case r.URL.Path == "/_query":
			e.calls.Add(1)
			raw, _ := io.ReadAll(r.Body)
			body, err := decodeObject(raw)
			if err != nil || r.Method != http.MethodPost {
				t.Error("invalid query request")
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.onQuery != nil {
				e.onQuery(body)
			}
			fmt.Fprint(w, esqlRows)
		default:
			return false
		}
		return true
	})
	return e
}

func (e *enrichFixture) indices() map[string]any {
	return e.state["metadata"].(map[string]any)["indices"].(map[string]any)
}
func (e *enrichFixture) snapshot() map[string]any {
	return e.indices()[".enrich-customers-1234"].(map[string]any)
}
func (e *enrichFixture) mapping() map[string]any {
	return e.snapshot()["mappings"].(map[string]any)["_doc"].(map[string]any)
}
func enrichRequest(t *testing.T, query string) core.ElasticsearchQueryRequest {
	return core.ElasticsearchQueryRequest{Operation: "esql.query", Body: mustJSON(t, map[string]any{"query": query, "params": []any{map[string]any{"n": json.Number("9007199254740993")}}})}
}

func TestEnrichNativeSyntaxAndSeparateSnapshotAuthority(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		t.Run(version, func(t *testing.T) {
			f := newESFixture(t)
			f.version = version
			e := newEnrichFixture(t, f)
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Version = version
			cfg.Elasticsearch.Enrich = &config.ElasticsearchEnrichConfig{Scope: config.ElasticsearchEnrichScope}
			target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			if target.Info().Capabilities["enrich_data_scope"] != config.ElasticsearchEnrichScope {
				t.Fatal("snapshot grant not advertised")
			}
			queries := []string{
				`FROM reports-* | ENRICH customers`,
				`FROM reports-* | ENRICH "customers" ON id WITH customer=label | STATS n=COUNT(*) BY customer`,
				`ROW id=?n | ENRICH customers ON id WITH label | ENRICH ranges ON id WITH range_label=label`,
				`FROM reports-* | ENRICH geo ON point WITH area=label`,
				`FROM reports-* | ENRICH _any:customers | LIMIT 1`,
				`FROM reports-* | ENRICH _coordinator:customers | LOOKUP JOIN reports-lookup ON id`,
				`FROM reports-* | ENRICH _remote:customers`,
				`FROM reports-* | ENRICH _REMOTE:customers`,
			}
			if version == "9.1.10" {
				queries = append(queries, `FROM reports-* | FORK (ENRICH customers ON id WITH label) (ENRICH ranges ON id WITH label)`)
			} else {
				queries = append(queries, `EXPLAIN [FROM reports-* | ENRICH customers | LIMIT 1]`)
			}
			for _, query := range queries {
				e.mu.Lock()
				e.onQuery = func(body map[string]any) {
					if body["query"] != query {
						t.Error("native ENRICH text rewritten")
					}
					raw, _ := json.Marshal(body["params"])
					if !strings.Contains(string(raw), "9007199254740993") {
						t.Error("parameter precision lost")
					}
				}
				e.mu.Unlock()
				out, err := target.ElasticsearchQuery(context.Background(), enrichRequest(t, query))
				if err != nil {
					t.Fatalf("%s: %v", query, err)
				}
				if !strings.Contains(string(out.Data), "9007199254740993") || out.Partial || out.Truncated {
					t.Fatal("native result changed")
				}
			}
			if e.calls.Load() != int64(len(queries)) || controller.Stats().Active != 0 || len(target.cursorInventory()) != 0 {
				t.Fatal("replay or resource leak")
			}
		})
	}
}

func TestEnrichRequiresExplicitScopeAndReadOnlyPrivilege(t *testing.T) {
	for _, fault := range []string{"no-scope", "monitor-only", "manage-enrich", "index-write", "missing-state"} {
		t.Run(fault, func(t *testing.T) {
			f := newESFixture(t)
			e := newEnrichFixture(t, f)
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Enrich = &config.ElasticsearchEnrichConfig{Scope: config.ElasticsearchEnrichScope}
			switch fault {
			case "no-scope":
				cfg.Elasticsearch.Enrich = nil
			case "monitor-only":
				f.privilege = string(readonlyProof())
			case "manage-enrich":
				f.privilege = strings.Replace(f.privilege, "monitor_enrich", "manage_enrich", 1)
			case "index-write":
				f.privilege = strings.Replace(f.privilege, `"read"`, `"write"`, 1)
			case "missing-state":
				e.state = map[string]any{}
			}
			if target, err := Open(context.Background(), cfg, limits, controller, nil, nil); err == nil {
				target.Close()
				t.Fatal("unproved snapshot authority accepted")
			}
			if e.calls.Load() != 0 || controller.Stats().Active != 0 {
				t.Fatal("dispatch or permit leak")
			}
		})
	}
}

func TestEnrichSnapshotContractFailures(t *testing.T) {
	for _, fault := range []string{"cluster", "state-id", "version", "writable", "closed", "alias", "multiple-active", "duplicate-uuid", "policy-name", "policy-type", "match-field", "runtime", "semantic", "source-disabled", "synthetic", "source-limit"} {
		t.Run(fault, func(t *testing.T) {
			f := newESFixture(t)
			e := newEnrichFixture(t, f)
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Enrich = &config.ElasticsearchEnrichConfig{Scope: config.ElasticsearchEnrichScope}
			switch fault {
			case "cluster":
				e.state["metadata"].(map[string]any)["cluster_uuid"] = "anothercluster"
			case "state-id":
				delete(e.state, "state_uuid")
			case "version":
				e.state["version"] = -1
			case "writable":
				e.snapshot()["settings"].(map[string]any)["index.blocks.write"] = "false"
			case "closed":
				e.snapshot()["state"] = "close"
			case "alias":
				e.snapshot()["aliases"] = []any{".enrich-private"}
			case "multiple-active":
				raw, _ := json.Marshal(e.snapshot())
				var second map[string]any
				json.Unmarshal(raw, &second)
				second["settings"].(map[string]any)["index.uuid"] = "other-snapshot-uuid-123"
				e.indices()[".enrich-customers-5678"] = second
			case "duplicate-uuid":
				e.indices()[".enrich-customers-5678"] = e.snapshot()
			case "policy-name":
				e.mapping()["_meta"].(map[string]any)["enrich_policy_name"] = "private"
			case "policy-type":
				e.mapping()["_meta"].(map[string]any)["enrich_policy_type"] = "custom"
			case "match-field":
				e.mapping()["properties"].(map[string]any)["id"] = map[string]any{"type": "text"}
			case "runtime":
				e.mapping()["runtime"] = map[string]any{"x": map[string]any{"type": "lookup"}}
			case "semantic":
				e.mapping()["properties"].(map[string]any)["label"] = map[string]any{"type": "semantic_text"}
			case "source-disabled":
				e.mapping()["_source"] = map[string]any{"enabled": false}
			case "synthetic":
				e.snapshot()["settings"].(map[string]any)["index.mapping.source.mode"] = "synthetic"
			case "source-limit":
				cfg.Elasticsearch.MaxResolvedIndices = 2
			}
			if target, err := Open(context.Background(), cfg, limits, controller, nil, nil); err == nil {
				target.Close()
				t.Fatal("invalid native inventory accepted")
			}
		})
	}
}

func TestEnrichPreflightAndWholeBatchDrift(t *testing.T) {
	for _, fault := range []string{"missing-policy", "private-from", "remote-from", "private-join", "policy-path", "policy-wildcard", "policy-mode", "policy-shape", "policy-drift", "snapshot-drift", "state-drift", "combined-limit", "combined-late-source", "proof-bytes"} {
		t.Run(fault, func(t *testing.T) {
			f := newESFixture(t)
			e := newEnrichFixture(t, f)
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Enrich = &config.ElasticsearchEnrichConfig{Scope: config.ElasticsearchEnrichScope}
			target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			query := `FROM reports-* | ENRICH customers`
			e.mu.Lock()
			switch fault {
			case "missing-policy":
				query = `FROM reports-* | ENRICH missing`
			case "private-from":
				query = `FROM private | ENRICH customers`
			case "remote-from":
				query = `FROM remote:reports-* | ENRICH _remote:customers`
			case "private-join":
				query += ` | LOOKUP JOIN private ON id`
			case "policy-path":
				query = `ROW id=1 | ENRICH "customers/../../_bulk"`
			case "policy-wildcard":
				query = `ROW id=1 | ENRICH "*"`
			case "policy-mode":
				query = `ROW id=1 | ENRICH "remote:customers"`
			case "policy-shape":
				e.policies["customers"] = map[string]any{"policies": []any{}}
			case "policy-drift":
				e.onQuery = func(map[string]any) { e.policies["customers"].(map[string]any)["changed"] = true }
			case "snapshot-drift":
				e.onQuery = func(map[string]any) {
					e.snapshot()["settings"].(map[string]any)["index.uuid"] = "changed-snapshot-uuid"
				}
			case "state-drift":
				e.onQuery = func(map[string]any) { e.state["state_uuid"] = "enrich-state-2"; e.state["version"] = 2 }
			case "combined-limit", "combined-late-source":
				cfg.Elasticsearch.MaxResolvedIndices = 3
			case "proof-bytes":
				target.limits.MaxResultBytes = 1000
			}
			e.mu.Unlock()
			requests := []core.ElasticsearchQueryRequest{enrichRequest(t, `ROW x=1`), enrichRequest(t, query)}
			if fault == "combined-late-source" {
				requests = []core.ElasticsearchQueryRequest{enrichRequest(t, `ROW id=1 | ENRICH customers`), enrichRequest(t, `FROM reports-*`)}
			}
			out, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Requests: requests})
			if err == nil {
				t.Fatal("unproved batch returned", out)
			}
			if !strings.HasSuffix(fault, "-drift") && e.calls.Load() != 0 {
				t.Fatal("batch dispatched before all members passed proof")
			}
			if controller.Stats().Active != 0 {
				t.Fatal("batch permit leaked")
			}
		})
	}
}

func TestEnrichNestedMappingBudgetsAndCancellation(t *testing.T) {
	f := newESFixture(t)
	e := newEnrichFixture(t, f)
	properties := e.mapping()["properties"].(map[string]any)
	properties["customer"] = map[string]any{"properties": map[string]any{"id": properties["id"]}}
	delete(properties, "id")
	e.mapping()["_meta"].(map[string]any)["enrich_match_field"] = "customer.id"
	raw := mustJSON(t, e.snapshot())
	if _, _, err := validateEnrichSnapshot(context.Background(), ".enrich-customers-1234", raw, 32<<10); err != nil {
		t.Fatal("native nested mapping rejected", err)
	}
	if _, _, err := validateEnrichSnapshot(context.Background(), ".enrich-customers-1234", raw, 1); err == nil {
		t.Fatal("expanded path work unbounded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := validateEnrichSnapshot(ctx, ".enrich-customers-1234", raw, 32<<10); err == nil {
		t.Fatal("cancelled mapping walk succeeded")
	}
}

func TestEnrichInternalAndMutationOperationsAreNotPublic(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	for _, operation := range []string{"cluster.state", "enrich.get_policy", "enrich.put_policy", "enrich.execute_policy", "enrich.delete_policy"} {
		if _, err := target.ElasticsearchQuery(context.Background(), core.ElasticsearchQueryRequest{Operation: operation}); err == nil {
			t.Fatal("internal/mutation operation accepted", operation)
		}
		if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: operation}); err == nil {
			t.Fatal("internal metadata operation exposed", operation)
		}
	}
	if target.Info().Capabilities["enrich_data_scope"] != "" {
		t.Fatal("default scope widened")
	}
}

func TestEnrichHistoricalInventoryAndRecheck(t *testing.T) {
	f := newESFixture(t)
	e := newEnrichFixture(t, f)
	raw, _ := json.Marshal(e.snapshot())
	var retired map[string]any
	json.Unmarshal(raw, &retired)
	retired["aliases"] = []any{}
	retired["settings"].(map[string]any)["index.uuid"] = "retired-snapshot-uuid-123"
	retiredMapping := retired["mappings"].(map[string]any)["_doc"].(map[string]any)
	retiredMapping["dynamic"] = false
	retiredMapping["_source"] = map[string]any{"enabled": true}
	e.indices()[".enrich-customers-1000"] = retired
	cfg, limits, controller := fixtureConfig(t, f)
	cfg.Elasticsearch.Enrich = &config.ElasticsearchEnrichConfig{Scope: config.ElasticsearchEnrichScope}
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if _, err := target.ElasticsearchQuery(context.Background(), enrichRequest(t, `ROW id=1 | ENRICH customers`)); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	retiredMapping["runtime"] = map[string]any{"bad": map[string]any{"type": "lookup"}}
	e.mu.Unlock()
	if err := target.refresh(context.Background()); err == nil || target.Info().Healthy {
		t.Fatal("historical snapshot drift did not invalidate target")
	}
	before := e.calls.Load()
	if _, err := target.ElasticsearchQuery(context.Background(), enrichRequest(t, `ROW id=1 | ENRICH customers`)); err == nil || e.calls.Load() != before {
		t.Fatal("invalid target dispatched")
	}
}

func TestEnrichPolicyProofCancellationReleasesAdmission(t *testing.T) {
	f := newESFixture(t)
	e := newEnrichFixture(t, f)
	cfg, limits, controller := fixtureConfig(t, f)
	cfg.Elasticsearch.Enrich = &config.ElasticsearchEnrichConfig{Scope: config.ElasticsearchEnrichScope}
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	entered, closed := make(chan struct{}), make(chan struct{})
	f.mu.Lock()
	previous := f.intercept
	f.mu.Unlock()
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasPrefix(r.URL.Path, "/_enrich/policy/") {
			close(entered)
			select {
			case <-r.Context().Done():
				close(closed)
			case <-time.After(2 * time.Second):
				t.Error("policy proof transport did not cancel")
			}
			return true
		}
		return previous(w, r)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := enrichRequest(t, `ROW id=1 | ENRICH customers`)
	done := make(chan error, 1)
	go func() { _, err := target.ElasticsearchQuery(ctx, request); done <- err }()
	select {
	case <-entered:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("policy proof not reached")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled proof succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("query did not cancel")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("HTTP channel remained open")
	}
	if e.calls.Load() != 0 || controller.Stats().Active != 0 {
		t.Fatal("cancelled proof dispatched or leaked permit")
	}
}
