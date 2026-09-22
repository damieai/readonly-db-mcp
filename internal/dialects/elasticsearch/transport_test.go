package elasticsearch

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type esFixture struct {
	mu                                               sync.Mutex
	version, cluster, privilege, mapping, resolution string
	requests                                         []string
	intercept                                        func(http.ResponseWriter, *http.Request) bool
	server                                           *httptest.Server
}

func newESFixture(t *testing.T) *esFixture {
	t.Helper()
	f := &esFixture{version: "8.19.21", cluster: "abcdefghijklmnopqrstuv", privilege: string(readonlyProof()), mapping: `{"reports-2026":{"mappings":{"_meta":{"precise":9007199254740993},"properties":{"title":{"type":"text"}}}}}`, resolution: `{"indices":[{"name":"reports-2026","attributes":["open"],"aliases":[]}],"aliases":[],"data_streams":[]}`}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "fixture_reader" || password != "fixture-only-password" {
			w.WriteHeader(401)
			return
		}
		f.mu.Lock()
		f.requests = append(f.requests, r.URL.Path)
		version, cluster, privilege, mapping, resolution, intercept := f.version, f.cluster, f.privilege, f.mapping, f.resolution, f.intercept
		f.mu.Unlock()
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		if intercept != nil && intercept(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected non-GET fixture request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(405)
			return
		}
		switch {
		case r.URL.Path == "/":
			fmt.Fprintf(w, `{"cluster_uuid":%q,"version":{"number":%q,"build_hash":%q,"build_flavor":"default","build_snapshot":false}}`, cluster, version, config.ElasticsearchBuilds[version])
		case r.URL.Path == "/_security/_authenticate":
			fmt.Fprint(w, `{"username":"fixture_reader","enabled":true,"authentication_type":"realm"}`)
		case r.URL.Path == "/_nodes/plugins":
			fmt.Fprintf(w, `{"_nodes":{"total":1,"successful":1,"failed":0},"nodes":{"node":{"version":%q,"build_hash":%q,"plugins":[]}}}`, version, config.ElasticsearchBuilds[version])
		case r.URL.Path == "/_security/user/_privileges":
			fmt.Fprint(w, privilege)
		case strings.HasPrefix(r.URL.Path, "/_resolve/index/"):
			fmt.Fprint(w, resolution)
		case strings.HasSuffix(r.URL.Path, "/_mapping"):
			fmt.Fprint(w, mapping)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func fixtureConfig(t *testing.T, fixtures ...*esFixture) (*config.TargetConfig, config.Limits, *admission.Controller) {
	t.Helper()
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	var ca []byte
	var endpoints []string
	for _, f := range fixtures {
		ca = append(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.server.Certificate().Raw})...)
		endpoints = append(endpoints, f.server.URL)
	}
	if err := os.WriteFile(caPath, ca, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ES_FIXTURE_PASSWORD", "fixture-only-password")
	cfg := &config.TargetConfig{Name: "es-fixture", Engine: config.EngineElasticsearch, Environment: "test", Consistency: config.ConsistencyEventual, Username: "fixture_reader", PasswordEnv: "ES_FIXTURE_PASSWORD", TLS: config.TLSConfig{Mode: config.TLSVerifyFull, CAFile: caPath}, Connection: config.ConnectionConfig{MaxOpen: 2, MaxIdle: 1, ConnectTimeout: time.Second, WriteTimeout: time.Second, ReadTimeout: 3 * time.Second, MaxLifetime: time.Minute, MaxIdleTime: time.Second}, Elasticsearch: &config.ElasticsearchConfig{Version: "8.19.21", ClusterUUID: "abcdefghijklmnopqrstuv", Endpoints: endpoints, AllowedIndices: []string{"reports-*"}, PrivilegeRecheck: time.Minute, MaxRequestBytes: 4096, MaxJSONDepth: 32, MaxJSONNodes: 1000, MaxResolvedIndices: 64, MaxAggregationBuckets: 10000, MaxOpenContexts: 32, ContextKeepAlive: time.Minute, MaxContextLifetime: time.Hour, MaxContextIDBytes: 64 << 10, MaxLanguageBytes: 256 << 10, MaxLanguageTokens: 10000, MaxLanguageDepth: 128, MaxEQLFetchSize: 10000}}
	limits := config.Limits{DefaultTimeout: time.Second, MaxTimeout: 2 * time.Second, MaxResultBytes: 32 << 10, MaxRows: 100, MaxBatchQueries: 10, MaxParameters: 100}
	controller := admission.New(admission.Config{Global: 4, PerTarget: 2, MaxQueued: 8, QueueTimeout: time.Second, MaintenanceMax: 1, MetadataReserved: 1, BatchMax: 1})
	return cfg, limits, controller
}
func openFixture(t *testing.T, f *esFixture) (*Target, *admission.Controller) {
	t.Helper()
	cfg, limits, controller := fixtureConfig(t, f)
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.Close() })
	return target, controller
}

func TestTLSMetadataKeepsPrecisionAndScopesEveryRequest(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	result, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "mappings"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Data), "9007199254740993") || result.Partial || result.Truncated || result.Consistency != "eventual" {
		t.Fatal("native metadata semantics changed")
	}
	f.mu.Lock()
	count := len(f.requests)
	f.mu.Unlock()
	for _, operation := range []string{"bulk", "index", "search", "search_template", "close_point_in_time", "https://other.invalid/_search"} {
		if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: operation}); err == nil {
			t.Fatal("unimplemented or mutating route dispatched")
		}
	}
	for _, index := range []string{"private-*", "reports-*/_search", "reports-%2f", "reports-*?x=1", "reports-*,private-*", "remote:reports-*"} {
		if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "resolve", Indices: []string{index}}); err == nil {
			t.Fatal("scope/path escape accepted")
		}
	}
	f.mu.Lock()
	if len(f.requests) != count {
		t.Error("rejected inputs reached transport")
	}
	f.mu.Unlock()
	info, _ := json.Marshal(target.Info())
	if strings.Contains(string(info), f.server.URL) || strings.Contains(string(info), "fixture_reader") || target.Info().ServerReadOnly {
		t.Fatal("target metadata leaks identity or claims server read-only state")
	}
}

func TestTLSStartupChecksEveryEndpointAndEffectivePermissions(t *testing.T) {
	for _, fault := range []string{"cluster", "version", "write", "unknown", "disabled_security"} {
		t.Run(fault, func(t *testing.T) {
			one, two := newESFixture(t), newESFixture(t)
			cfg, limits, controller := fixtureConfig(t, one, two)
			switch fault {
			case "cluster":
				two.cluster = "differentclusteridentity"
			case "version":
				two.version = "9.1.10"
			case "write":
				two.privilege = strings.Replace(two.privilege, `"read"`, `"write"`, 1)
			case "unknown":
				two.privilege = strings.Replace(two.privilege, `"global":[]`, `"global":[],"unknown_permission":true`, 1)
			case "disabled_security":
				two.intercept = func(w http.ResponseWriter, r *http.Request) bool {
					if r.URL.Path == "/_security/_authenticate" {
						fmt.Fprint(w, `{}`)
						return true
					}
					return false
				}
			}
			if target, err := Open(context.Background(), cfg, limits, controller, nil, nil); err == nil {
				target.Close()
				t.Fatal("unsafe endpoint accepted")
			}
			if controller.Stats().Active != 0 {
				t.Fatal("startup leaked admission permit")
			}
		})
	}
}

func TestPermissionDriftAndStaleProofStopDispatch(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	f.mu.Lock()
	f.privilege = strings.Replace(f.privilege, `"read"`, `"all"`, 1)
	f.mu.Unlock()
	if target.refresh(context.Background()) == nil {
		t.Fatal("write drift accepted")
	}
	if target.Info().Healthy {
		t.Fatal("failed proof remained healthy")
	}
	if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "resolve"}); err == nil {
		t.Fatal("unhealthy target dispatched")
	}
	f.mu.Lock()
	f.privilege = string(readonlyProof())
	f.mu.Unlock()
	if err := target.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	target.checked.Store(time.Now().Add(-2 * time.Minute).UnixNano())
	if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "resolve"}); err == nil {
		t.Fatal("expired proof dispatched")
	}
}

func TestTransportDoesNotRedirectRetryOrLeakNativeErrors(t *testing.T) {
	f := newESFixture(t)
	target, _ := openFixture(t, f)
	var calls atomic.Int64
	for _, status := range []int{302, 503, 403} {
		f.mu.Lock()
		f.intercept = func(w http.ResponseWriter, r *http.Request) bool {
			if strings.HasPrefix(r.URL.Path, "/_resolve/index/") {
				calls.Add(1)
				w.Header().Set("Location", f.server.URL+"/forbidden")
				w.WriteHeader(status)
				fmt.Fprint(w, "sensitive-query-fragment")
				return true
			}
			return false
		}
		f.mu.Unlock()
		before := calls.Load()
		_, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "resolve"})
		if err == nil || strings.Contains(err.Error(), "sensitive-query-fragment") {
			t.Fatal("unsafe error propagation")
		}
		if calls.Load() != before+1 {
			t.Fatal("HTTP retry or redirect multiplied requests")
		}
	}
	if target.Info().Healthy {
		t.Fatal("authorization failure did not invalidate proof")
	}
}

func TestResponseLimitsAndAliasRaceFailWithoutTruncating(t *testing.T) {
	for _, fault := range []string{"bytes", "duplicate", "encoding", "race", "envelope"} {
		t.Run(fault, func(t *testing.T) {
			f := newESFixture(t)
			target, _ := openFixture(t, f)
			f.mu.Lock()
			switch fault {
			case "bytes":
				f.mapping = `{"reports-2026":{"x":"` + strings.Repeat("x", target.limits.MaxResultBytes) + `"}}`
			case "duplicate":
				f.mapping = `{"reports-2026":{},"reports-2026":{}}`
			case "encoding":
				f.intercept = func(w http.ResponseWriter, r *http.Request) bool {
					if strings.HasSuffix(r.URL.Path, "/_mapping") {
						w.Header().Set("Content-Encoding", "gzip")
						fmt.Fprint(w, "not-decoded")
						return true
					}
					return false
				}
			case "race":
				f.mapping = `{"private-index":{"mappings":{}}}`
			case "envelope":
				f.mapping = `{"reports-2026":{"x":"` + strings.Repeat("x", target.limits.MaxResultBytes*2/3) + `"}}`
			}
			f.mu.Unlock()
			if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "mappings"}); err == nil {
				t.Fatal("unsafe response returned")
			}
		})
	}
}

func TestCancellationReleasesPermitsMemoryAndConnection(t *testing.T) {
	f := newESFixture(t)
	target, controller := openFixture(t, f)
	observed := make(chan struct{})
	f.mu.Lock()
	f.intercept = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasPrefix(r.URL.Path, "/_resolve/index/") {
			<-r.Context().Done()
			close(observed)
			return true
		}
		return false
	}
	f.mu.Unlock()
	_, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "resolve", Timeout: 50 * time.Millisecond})
	if err == nil {
		t.Fatal("deadline ignored")
	}
	select {
	case <-observed:
	case <-time.After(time.Second):
		t.Fatal("HTTP cancellation was not propagated")
	}
	if controller.Stats().Active != 0 {
		t.Fatal("permit leak")
	}
	if !responseMemory.TryAcquire(512 << 20) {
		t.Fatal("response memory leak")
	}
	responseMemory.Release(512 << 20)
	if !queuedMemory.TryAcquire(64 << 20) {
		t.Fatal("queue memory leak")
	}
	queuedMemory.Release(64 << 20)
	f.mu.Lock()
	f.intercept = nil
	f.mu.Unlock()
	if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "resolve"}); err != nil {
		t.Fatalf("pool did not recover: %v", err)
	}
}

func TestSharedSocketCeilingAcrossTLSOrigins(t *testing.T) {
	one, two := newESFixture(t), newESFixture(t)
	cfg, limits, controller := fixtureConfig(t, one, two)
	cfg.Connection.MaxOpen = 1
	cfg.Connection.MaxIdle = 1
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	for i := 0; i < 4; i++ {
		if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "resolve"}); err != nil {
			t.Fatal(err)
		}
		if len(target.wire.slots) > 1 {
			t.Fatal("per-host pools exceeded total socket budget")
		}
	}
	target.Close()
	deadline := time.Now().Add(time.Second)
	for len(target.wire.slots) > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(target.wire.slots) != 0 {
		t.Fatal("socket leases leaked at shutdown")
	}
}

func TestConcurrentWireRequestsRespectSharedSocketBudget(t *testing.T) {
	one, two := newESFixture(t), newESFixture(t)
	cfg, limits, controller := fixtureConfig(t, one, two)
	cfg.Connection.MaxIdle = 2
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	var active, peak atomic.Int64
	intercept := func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasPrefix(r.URL.Path, "/_resolve/index/") {
			return false
		}
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
		fmt.Fprint(w, one.resolution)
		return true
	}
	one.mu.Lock()
	one.intercept = intercept
	one.mu.Unlock()
	two.mu.Lock()
	two.intercept = intercept
	two.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 8)
	// Exercise the wire without the public admission limit: per-host pooling
	// alone would allow four concurrent requests to these two TLS origins.
	for i := 0; i < 8; i++ {
		go func(endpoint int) {
			<-start
			_, err := target.wire.get(ctx, endpoint, "/_resolve/index/reports-*", nil, limits.MaxResultBytes)
			results <- err
		}(i % 2)
	}
	close(start)
	for i := 0; i < 8; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if peak.Load() != 2 {
		t.Fatalf("observed %d concurrent upstream requests, want two", peak.Load())
	}
}

func TestConnectionLifetimeWaitsForActiveRequestThenRetires(t *testing.T) {
	f := newESFixture(t)
	cfg, limits, controller := fixtureConfig(t, f)
	cfg.Connection.MaxLifetime = 20 * time.Millisecond
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	f.mu.Lock()
	f.intercept = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasPrefix(r.URL.Path, "/_resolve/index/") {
			time.Sleep(50 * time.Millisecond)
			fmt.Fprint(w, f.resolution)
			return true
		}
		return false
	}
	f.mu.Unlock()
	if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "resolve"}); err != nil {
		t.Fatalf("lifetime interrupted active work: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for len(target.wire.slots) > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(target.wire.slots) != 0 {
		t.Fatal("expired idle socket retained")
	}
}

func TestScheduledProofRefreshDetectsDrift(t *testing.T) {
	f := newESFixture(t)
	cfg, limits, controller := fixtureConfig(t, f)
	cfg.Elasticsearch.PrivilegeRecheck = 100 * time.Millisecond
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	f.mu.Lock()
	f.privilege = strings.Replace(f.privilege, `"monitor"`, `"manage"`, 1)
	f.mu.Unlock()
	deadline := time.Now().Add(time.Second)
	for target.healthy.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if target.healthy.Load() {
		t.Fatal("scheduled attestation missed privilege drift")
	}
}

func TestBothPinnedBuildsWithTLSFixture(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		t.Run(version, func(t *testing.T) {
			f := newESFixture(t)
			f.mu.Lock()
			f.version = version
			f.mu.Unlock()
			cfg, limits, controller := fixtureConfig(t, f)
			cfg.Elasticsearch.Version = version
			target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			if _, err := target.ElasticsearchMetadata(context.Background(), core.ElasticsearchMetadataRequest{Operation: "mappings"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
