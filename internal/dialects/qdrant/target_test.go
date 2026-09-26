package qdrant

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func testJWT(t *testing.T, collection string) string {
	t.Helper()
	encode := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	message := encode(map[string]any{"alg": "HS256", "typ": "JWT"}) + "." + encode(map[string]any{"exp": time.Now().Add(time.Hour).Unix(), "access": []any{map[string]any{"collection": collection, "access": "r"}}})
	mac := hmac.New(sha256.New, []byte("test-secret"))
	_, _ = mac.Write([]byte(message))
	return message + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestQdrantScopedNativeReads(t *testing.T) {
	key := testJWT(t, "docs")
	var reads atomic.Int32
	scrollStarted := make(chan struct{})
	scrollRelease := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.Header.Get("api-key") != key {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `{"title":"qdrant - vector search engine","version":"1.19.1"}`)
		case "/collections":
			fmt.Fprint(w, `{"status":"ok","result":{"collections":[{"name":"docs"}]}}`)
		case "/collections/docs":
			fmt.Fprint(w, `{"status":"ok","result":{"config":{}}}`)
		case "/collections/docs/points/query":
			reads.Add(1)
			if r.Method != http.MethodPost {
				t.Errorf("query used %s", r.Method)
			}
			fmt.Fprint(w, `{"status":"ok","result":{"points":[{"id":1,"score":0.9}]}}`)
		case "/collections/docs/points/count":
			fmt.Fprint(w, `{"status":"ok","result":{"padding":"`+strings.Repeat("x", 1<<20)+`"}}`)
		case "/collections/docs/points/scroll":
			close(scrollStarted)
			select {
			case <-r.Context().Done():
			case <-scrollRelease:
			}
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	defer close(scrollRelease)
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QDRANT_TEST_JWT", key)
	cfg := &config.TargetConfig{Name: "vector", Engine: config.EngineQdrant, Environment: "test", Consistency: config.ConsistencyEventual, TLS: config.TLSConfig{Mode: config.TLSVerifyFull, CAFile: ca}, Connection: config.ConnectionConfig{ConnectTimeout: time.Second, ReadTimeout: 2 * time.Second, MaxOpen: 2, MaxIdle: 1, MaxIdleTime: time.Minute}, Qdrant: &config.QdrantConfig{Endpoint: server.URL, Version: config.QdrantVersion, Collections: []string{"docs"}, KeyEnv: "QDRANT_TEST_JWT", PrivilegeRecheck: time.Minute, MaxRequestBytes: 1 << 20, MaxJSONDepth: 128, MaxJSONNodes: 100000}}
	limits := config.Limits{DefaultTimeout: time.Second, MaxTimeout: 2 * time.Second, MaxRows: 100, MaxResultBytes: 1 << 20, MaxBatchQueries: 10}
	controller := admission.New(admission.Config{Global: 4, PerTarget: 2, MaxQueued: 10, QueueTimeout: time.Second, BatchMax: 2, MaintenanceMax: 2})
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if !target.Info().ReadOnlyUser || target.Info().ServerReadOnly {
		t.Fatal("unexpected authority report")
	}
	body := []byte(`{"prefetch":[{"query":[0.1,0.2],"using":"dense","limit":20},{"query":{"indices":[1,5],"values":[0.2,0.3]},"using":"sparse","limit":20}],"query":{"fusion":"rrf"},"filter":{"must":[{"key":"tenant","match":{"value":"a"}}]},"limit":10}`)
	result, err := target.QdrantRead(context.Background(), core.QdrantRequest{Collection: "docs", Operation: "query", Body: body})
	if err != nil || !strings.Contains(string(result.Data), `"score":0.9`) {
		t.Fatalf("native hybrid query: %v %+v", err, result)
	}
	for _, body := range []string{
		`{"query":[1],"prefetch":{"query":[1],"lookup_from":{"collection":"other"}},"limit":10}`,
		`{"query":[1],"query":[2]}`,
		`{"query":{"text":"embed this","model":"x"}}`,
		`{"query":[1],"limit":101}`,
	} {
		if _, err := target.QdrantRead(context.Background(), core.QdrantRequest{Collection: "docs", Operation: "query", Body: []byte(body)}); err == nil {
			t.Errorf("accepted hostile query %s", body)
		}
	}
	if reads.Load() != 1 {
		t.Fatalf("hostile queries reached server: %d", reads.Load())
	}
	if _, err := target.QdrantRead(context.Background(), core.QdrantRequest{Collection: "docs", Operation: "count", Body: []byte(`{}`)}); err == nil || !strings.Contains(err.Error(), "max_result_bytes") {
		t.Fatalf("oversized response was accepted: %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := target.QdrantRead(canceled, core.QdrantRequest{Collection: "docs", Operation: "query", Body: body}); err == nil {
		t.Fatal("canceled query succeeded")
	}
	midflight, stop := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		_, err := target.QdrantRead(midflight, core.QdrantRequest{Collection: "docs", Operation: "scroll", Body: []byte(`{"limit":1}`)})
		finished <- err
	}()
	select {
	case <-scrollStarted:
	case <-time.After(time.Second):
		t.Fatal("scroll did not reach server")
	}
	stop()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("midflight cancellation succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled scroll did not stop")
	}
	if _, err := target.QdrantRead(context.Background(), core.QdrantRequest{Collection: "other", Operation: "metadata"}); err == nil {
		t.Fatal("out-of-scope collection accepted")
	}
}

func TestJWTExactReadScope(t *testing.T) {
	key := testJWT(t, "docs")
	if _, err := attestJWT(key, []string{"docs"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := attestJWT(key, []string{"other"}, time.Now()); err == nil {
		t.Fatal("accepted another collection")
	}
	if _, err := attestJWT("admin-key", []string{"docs"}, time.Now()); err == nil {
		t.Fatal("accepted opaque admin key")
	}
}

func TestQdrantRejectsUnauthenticatedServer(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprint(w, `{"version":"1.19.1"}`)
		} else if r.URL.Path == "/collections" {
			fmt.Fprint(w, `{"status":"ok","result":{"collections":[{"name":"docs"}]}}`)
		} else {
			fmt.Fprint(w, `{"status":"ok","result":{}}`)
		}
	}))
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QDRANT_TEST_JWT", testJWT(t, "docs"))
	cfg := &config.TargetConfig{Name: "vector", Engine: config.EngineQdrant, Environment: "test", Consistency: config.ConsistencyEventual, TLS: config.TLSConfig{Mode: config.TLSVerifyFull, CAFile: ca}, Connection: config.ConnectionConfig{ConnectTimeout: time.Second, ReadTimeout: 2 * time.Second, MaxOpen: 1, MaxIdle: 1, MaxIdleTime: time.Minute}, Qdrant: &config.QdrantConfig{Endpoint: server.URL, Version: config.QdrantVersion, Collections: []string{"docs"}, KeyEnv: "QDRANT_TEST_JWT", PrivilegeRecheck: time.Minute, MaxRequestBytes: 1 << 20, MaxJSONDepth: 128, MaxJSONNodes: 100000}}
	controller := admission.New(admission.Config{Global: 2, PerTarget: 1, MaxQueued: 4, QueueTimeout: time.Second, BatchMax: 1, MaintenanceMax: 1})
	_, err := Open(context.Background(), cfg, config.Limits{DefaultTimeout: time.Second, MaxTimeout: 2 * time.Second, MaxRows: 100, MaxResultBytes: 1 << 20, MaxBatchQueries: 10}, controller, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unauthenticated") {
		t.Fatalf("unauthenticated server accepted: %v", err)
	}
}
