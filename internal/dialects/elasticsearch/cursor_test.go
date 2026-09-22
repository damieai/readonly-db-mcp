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

	"github.com/your-org/readonly-db-mcp/internal/core"
)

type cursorFixture struct {
	mu                                     sync.Mutex
	opened, pages, cleared                 int
	ids                                    []string
	bodies                                 []string
	partial, empty, failClear, unknownOpen bool
	onPage                                 func()
}

func serveCursors(t *testing.T, f *esFixture) *cursorFixture {
	t.Helper()
	v := &cursorFixture{}
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet {
			return false
		}
		raw, _ := io.ReadAll(r.Body)
		v.mu.Lock()
		defer v.mu.Unlock()
		switch {
		case r.Method == http.MethodDelete && (r.URL.Path == "/_pit" || r.URL.Path == "/_search/scroll"):
			v.cleared++
			v.ids = append(v.ids, string(raw))
			if strings.Contains(string(raw), "_all") {
				t.Error("mass cleanup")
			}
			if v.failClear {
				w.WriteHeader(503)
				fmt.Fprint(w, `{"error":"unavailable"}`)
			} else {
				fmt.Fprint(w, `{"succeeded":true,"num_freed":1}`)
			}
		case strings.HasSuffix(r.URL.Path, "/_pit"):
			v.opened++
			if r.URL.Query().Get("allow_partial_search_results") != "false" {
				t.Error("partial PIT creation permitted")
			}
			if v.unknownOpen {
				fmt.Fprint(w, `{"id":`)
			} else {
				fmt.Fprint(w, `{"id":"pit-open","_shards":{"total":1,"successful":1,"failed":0}}`)
			}
		case strings.HasSuffix(r.URL.Path, "/_search") || r.URL.Path == "/_search/scroll":
			v.pages++
			v.bodies = append(v.bodies, string(raw))
			var body map[string]json.RawMessage
			_ = json.Unmarshal(raw, &body)
			key := "_scroll_id"
			if _, pit := body["pit"]; pit {
				key = "pit_id"
				if r.URL.Path != "/_search" || r.URL.Query().Has("routing") || r.URL.Query().Has("preference") {
					t.Error("PIT search repeated index or routing")
				}
			}
			response := strings.TrimSuffix(nativeSearchResult, "}") + fmt.Sprintf(`,%q:"native-%d"}`, key, v.pages)
			if v.partial {
				response = strings.Replace(response, `"timed_out":false`, `"timed_out":true`, 1)
			}
			if v.empty {
				var root map[string]json.RawMessage
				_ = json.Unmarshal([]byte(response), &root)
				root["hits"] = json.RawMessage(`{"hits":[]}`)
				response = string(mustJSON(t, root))
			}
			if v.onPage != nil {
				v.onPage()
			}
			fmt.Fprint(w, response)
		default:
			return false
		}
		return true
	})
	return v
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func cursorOpen(t *testing.T, target *Target, kind string) *core.ElasticsearchCursorResult {
	t.Helper()
	result, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "open", Kind: kind, Owner: "session", Body: json.RawMessage(`{"sort":["_shard_doc"],"size":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestOwnedCursorRotationScopeAndPrecision(t *testing.T) {
	for _, test := range []struct{ kind, version string }{{"pit", "8.19.21"}, {"scroll", "8.19.21"}, {"pit", "9.1.10"}, {"scroll", "9.1.10"}} {
		t.Run(test.version+"/"+test.kind, func(t *testing.T) {
			kind := test.kind
			f := newESFixture(t)
			f.version = test.version
			v := serveCursors(t, f)
			cfg, limits, admission := fixtureConfig(t, f)
			cfg.Elasticsearch.Version = test.version
			target, err := Open(context.Background(), cfg, limits, admission, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			result := cursorOpen(t, target, kind)
			if result.Handle == "" || result.Consistency != kind || strings.Contains(string(mustJSON(t, result)), "native-") || !strings.Contains(string(result.Data), "9007199254740993") {
				t.Fatal("invalid public cursor result")
			}
			if admission.Stats().Active != 0 {
				t.Fatal("idle cursor retained admission permit")
			}
			for _, bad := range []core.ElasticsearchCursorRequest{
				{Action: "next", Owner: "other", Handle: result.Handle},
				{Action: "next", Owner: "session", Handle: "native-1"},
				{Action: "next", Owner: "session", Handle: result.Handle, Indices: []string{"private"}},
				{Action: "open", Owner: "session", Kind: kind, Body: json.RawMessage(`{"pit":{"id":"raw"}}`)},
			} {
				if _, err := target.ElasticsearchCursor(context.Background(), bad); err == nil {
					t.Fatal("unowned/native input admitted")
				}
			}
			if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session", Handle: result.Handle}); err != nil {
				t.Fatal(err)
			}
			v.mu.Lock()
			last := v.bodies[len(v.bodies)-1]
			v.mu.Unlock()
			if !strings.Contains(last, "native-1") {
				t.Fatal("did not continue newest context", last)
			}
			if kind == "pit" && !strings.Contains(last, `"search_after":[9007199254740993]`) {
				t.Fatal("sort precision lost", last)
			}
			if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "close", Owner: "session", Handle: result.Handle}); err != nil {
				t.Fatal(err)
			}
			v.mu.Lock()
			defer v.mu.Unlock()
			if v.cleared != 1 || !strings.Contains(v.ids[0], "native-2") || len(target.cursorInventory()) != 0 {
				t.Fatal("latest context not released")
			}
		})
	}
}

func TestPITSnapshotSurvivesAliasRolloverAndRetainsAdvancedDSL(t *testing.T) {
	f := newESFixture(t)
	v := serveCursors(t, f)
	target, _ := openFixture(t, f)
	result, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "open", Kind: "pit", Owner: "session", Indices: []string{"reports-alias"}, Options: map[string]json.RawMessage{"routing": json.RawMessage(`"tenant/a"`), "size": json.RawMessage(`1`), "timeout": json.RawMessage(`"1s"`)}, Body: json.RawMessage(`{"sort":["_shard_doc"],"query":{"script_score":{"query":{"match_all":{}},"script":{"source":"cosineSimilarity(params.v, 'embedding') + 1","params":{"v":[0.1,0.2]}}}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.resolution = strings.ReplaceAll(f.resolution, "reports-2026", "reports-2027")
	f.mapping = strings.ReplaceAll(f.mapping, "reports-2026", "reports-2027")
	f.mu.Unlock()
	if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session", Handle: result.Handle}); err != nil {
		t.Fatal("snapshot incorrectly re-resolved live alias", err)
	}
	v.mu.Lock()
	if !strings.Contains(v.bodies[1], "cosineSimilarity") {
		t.Error("advanced query lost")
	}
	v.mu.Unlock()
	// Runtime lookup mappings must still be proved live on subsequent pages.
	c, _ := target.lookupCursor("session", result.Handle)
	_ = c.lock(context.Background())
	c.mappings["reports-2026"].(map[string]any)["mappings"].(map[string]any)["runtime"] = map[string]any{"foreign": map[string]any{"type": "lookup", "target_index": "private-index"}}
	c.unlock()
	if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session", Handle: result.Handle}); err == nil || !strings.HasPrefix(err.Error(), "scope_denied:") {
		t.Fatal("runtime lookup bypassed live proof", err)
	}
}

func TestCursorExhaustionAndCompositeContinuation(t *testing.T) {
	for _, kind := range []string{"pit", "scroll"} {
		t.Run(kind, func(t *testing.T) {
			f := newESFixture(t)
			v := serveCursors(t, f)
			target, _ := openFixture(t, f)
			result := cursorOpen(t, target, kind)
			v.mu.Lock()
			v.empty = true
			v.mu.Unlock()
			end, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session", Handle: result.Handle})
			if err != nil || !end.Exhausted || !end.Closed || end.Handle != "" || len(target.cursorInventory()) != 0 {
				t.Fatal("exhaustion failed", err, end)
			}
		})
	}
	f := newESFixture(t)
	v := serveCursors(t, f)
	v.empty = true
	target, _ := openFixture(t, f)
	body := json.RawMessage(`{"size":0,"aggs":{"page":{"composite":{"sources":[{"id":{"terms":{"field":"id"}}}],"after":{"id":9007199254740993}}}}}`)
	result, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "open", Kind: "pit", Owner: "session", Body: body})
	if err != nil || result.Handle == "" || result.Exhausted {
		t.Fatal("aggregation-only PIT closed", err)
	}
	if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session", Handle: result.Handle, Body: body}); err != nil {
		t.Fatal("composite continuation rejected", err)
	}
}

func TestCursorFailureCleanupAndConservativeReservations(t *testing.T) {
	for _, fault := range []string{"partial", "unknown_open", "clear_failed"} {
		t.Run(fault, func(t *testing.T) {
			f := newESFixture(t)
			v := serveCursors(t, f)
			target, _ := openFixture(t, f)
			target.cfg.Elasticsearch.MaxOpenContexts = 1
			v.mu.Lock()
			v.partial = fault == "partial"
			v.unknownOpen = fault == "unknown_open"
			v.failClear = fault == "clear_failed"
			v.mu.Unlock()
			result, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "open", Kind: "pit", Owner: "session", KeepAlive: time.Minute})
			if fault == "clear_failed" {
				if err != nil {
					t.Fatal(err)
				}
				_, err = target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "close", Owner: "session", Handle: result.Handle})
			}
			if err == nil {
				t.Fatal("failure accepted")
			}
			entries := target.cursorInventory()
			if fault == "partial" {
				if len(entries) != 0 {
					t.Fatal("known partial result leaked context")
				}
				v.mu.Lock()
				defer v.mu.Unlock()
				if !strings.Contains(v.ids[0], "native-1") {
					t.Fatal("partial rotation not cleaned")
				}
				return
			}
			if len(entries) != 1 {
				t.Fatal("uncertain native outcome released reservation")
			}
			if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "open", Kind: "pit", Owner: "session"}); err == nil || !strings.HasPrefix(err.Error(), "resource_limit:") {
				t.Fatal("orphan storm admitted", err)
			}
			c := entries[0]
			_ = c.lock(context.Background())
			c.serverUntil = time.Now().Add(-time.Second)
			c.retryAfter = time.Time{}
			c.unlock()
			target.reapCursors(context.Background())
			if len(target.cursorInventory()) != 0 {
				t.Fatal("expired native lease retained reservation")
			}
		})
	}
}

func TestCursorShortRenewalDoesNotShortenNativeCleanupHorizon(t *testing.T) {
	f := newESFixture(t)
	v := serveCursors(t, f)
	target, _ := openFixture(t, f)
	result := cursorOpen(t, target, "pit")
	_, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session", Handle: result.Handle, KeepAlive: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := target.lookupCursor("session", result.Handle)
	_ = c.lock(context.Background())
	if time.Until(c.serverUntil) < 50*time.Second || time.Until(c.expires) > 2*time.Second {
		t.Error("native and local lease horizons conflated")
	}
	c.unlock()
	v.mu.Lock()
	v.failClear = true
	v.mu.Unlock()
	_, _ = target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "close", Owner: "session", Handle: result.Handle})
	if len(target.cursorInventory()) != 1 {
		t.Fatal("failed cleanup freed reservation early")
	}
	v.mu.Lock()
	v.failClear = false
	v.mu.Unlock()
}

func TestCursorAuthoritySessionExpiryAndShutdown(t *testing.T) {
	for _, event := range []string{"authority", "dls", "fls", "session", "idle", "absolute", "shutdown"} {
		t.Run(event, func(t *testing.T) {
			f := newESFixture(t)
			v := serveCursors(t, f)
			target, _ := openFixture(t, f)
			result := cursorOpen(t, target, "scroll")
			switch event {
			case "authority", "dls", "fls":
				f.mu.Lock()
				switch event {
				case "authority":
					f.privilege = strings.Replace(f.privilege, `"read"`, `"read","view_index_metadata"`, 1)
				case "dls":
					f.privilege = strings.Replace(f.privilege, `"allow_restricted_indices":false`, `"allow_restricted_indices":false,"query":[{"term":{"tenant":"new"}}]`, 1)
				case "fls":
					f.privilege = strings.Replace(f.privilege, `"allow_restricted_indices":false`, `"allow_restricted_indices":false,"field_security":[{"grant":["title"]}]`, 1)
				}
				f.mu.Unlock()
				if err := target.refresh(context.Background()); err != nil {
					t.Fatal(err)
				}
			case "session":
				if err := target.CloseElasticsearchSession(context.Background(), "session"); err != nil {
					t.Fatal(err)
				}
			case "shutdown":
				target.Close()
			default:
				c, _ := target.lookupCursor("session", result.Handle)
				_ = c.lock(context.Background())
				if event == "idle" {
					c.expires = time.Now().Add(-time.Second)
				} else {
					c.absolute = time.Now().Add(-time.Second)
				}
				c.unlock()
			}
			if _, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session", Handle: result.Handle}); err == nil {
				t.Fatal("invalid cursor advanced")
			}
			v.mu.Lock()
			defer v.mu.Unlock()
			if v.pages != 1 || v.cleared != 1 {
				t.Fatal("invalid cursor was restarted or left open", v.pages, v.cleared)
			}
		})
	}
}

func TestPITBatchSharesRotatingSnapshotAndPreflights(t *testing.T) {
	f := newESFixture(t)
	v := serveCursors(t, f)
	target, _ := openFixture(t, f)
	member := core.ElasticsearchQueryRequest{Operation: "search", Body: json.RawMessage(`{"size":0,"aggs":{"n":{"filter":{"match_all":{}}}}}`)}
	result, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Consistency: "pit", Requests: []core.ElasticsearchQueryRequest{member, member}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Consistency != "pit" || len(result.Results) != 2 || result.Results[0].Consistency != "pit" || strings.Contains(string(mustJSON(t, result)), "native-") {
		t.Fatal("invalid PIT batch result")
	}
	v.mu.Lock()
	if v.opened != 1 || v.pages != 2 || v.cleared != 1 || !strings.Contains(v.bodies[1], "native-1") {
		t.Error("batch did not share rotating PIT")
	}
	v.mu.Unlock()
	for _, bad := range []core.ElasticsearchQueryRequest{
		{Operation: "count"},
		{Operation: "search", Indices: []string{"reports-other"}},
		{Operation: "search", Options: map[string]json.RawMessage{"routing": json.RawMessage(`"other"`)}},
		{Operation: "search", Body: json.RawMessage(`{"query":{"terms":{"tenant":{"index":"private","id":"1","path":"values"}}}}`)},
	} {
		if _, err := target.ElasticsearchBatch(context.Background(), core.ElasticsearchBatchRequest{Consistency: "pit", Requests: []core.ElasticsearchQueryRequest{member, bad}}); err == nil {
			t.Fatal("incompatible PIT batch admitted")
		}
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.opened != 1 {
		t.Fatal("batch allocated before all member preflight")
	}
}

func TestCursorConcurrentAdvanceAndCloseAreSerialized(t *testing.T) {
	f := newESFixture(t)
	v := serveCursors(t, f)
	target, _ := openFixture(t, f)
	result := cursorOpen(t, target, "scroll")
	entered, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	v.mu.Lock()
	v.onPage = func() { once.Do(func() { close(entered); <-resume }) }
	v.mu.Unlock()
	advanced := make(chan error, 1)
	go func() {
		_, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "next", Owner: "session", Handle: result.Handle})
		advanced <- err
	}()
	<-entered
	var closeDone atomic.Bool
	closed := make(chan error, 1)
	go func() {
		_, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "close", Owner: "session", Handle: result.Handle})
		closeDone.Store(true)
		closed <- err
	}()
	if closeDone.Load() {
		t.Error("close bypassed active page")
	}
	close(resume)
	if err := <-advanced; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if !strings.Contains(v.ids[0], "native-2") {
		t.Fatal("concurrent close missed rotated ID")
	}
}

func TestCursorCancellationReturnsBeforeRecoveryAndNeverReplays(t *testing.T) {
	f := newESFixture(t)
	target, admission := openFixture(t, f)
	var creates, clears atomic.Int64
	clearStarted, allowClear := make(chan struct{}), make(chan struct{})
	defer close(allowClear)
	interceptQuery(f, func(w http.ResponseWriter, r *http.Request) bool {
		_, _ = io.Copy(io.Discard, r.Body)
		switch {
		case r.Method == http.MethodDelete:
			if clears.Add(1) == 1 {
				close(clearStarted)
			}
			select {
			case <-allowClear:
			case <-r.Context().Done():
			}
			fmt.Fprint(w, `{"succeeded":true,"num_freed":1}`)
			return true
		case strings.HasSuffix(r.URL.Path, "/_pit"):
			creates.Add(1)
			fmt.Fprint(w, `{"id":"known-pit","_shards":{"total":1,"successful":1,"failed":0}}`)
			return true
		case r.URL.Path == "/_search":
			<-r.Context().Done()
			return true
		}
		return false
	})
	started := time.Now()
	_, err := target.ElasticsearchCursor(context.Background(), core.ElasticsearchCursorRequest{Action: "open", Kind: "pit", Owner: "session", Timeout: 80 * time.Millisecond})
	if err == nil || time.Since(started) > time.Second {
		t.Fatal("request waited for recovery past its deadline", err)
	}
	select {
	case <-clearStarted:
	case <-time.After(time.Second):
		t.Fatal("bounded recovery was not scheduled")
	}
	if creates.Load() != 1 || len(target.cursorInventory()) != 1 {
		t.Fatal("ambiguous request replayed or released orphan capacity")
	}
	// The only active work may now be the independently admitted maintenance.
	if admission.Stats().Active > 1 {
		t.Fatal("canceled cursor retained interactive permit")
	}
}
