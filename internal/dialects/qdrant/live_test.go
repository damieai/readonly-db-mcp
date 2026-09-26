package qdrant

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// This optional test is the live P1 qualification gate. The configured
// collection must be disposable, so a failed denial check has no production
// effect. Delete uses an unpredictable absent point ID and must return 403.
func TestQdrantLiveReadOnlyGate(t *testing.T) {
	endpoint, collection, ca := os.Getenv("QDRANT_LIVE_URL"), os.Getenv("QDRANT_LIVE_COLLECTION"), os.Getenv("QDRANT_LIVE_CA_FILE")
	if endpoint == "" || collection == "" || os.Getenv("QDRANT_LIVE_JWT") == "" {
		t.Skip("set QDRANT_LIVE_URL, QDRANT_LIVE_COLLECTION and QDRANT_LIVE_JWT")
	}
	tlsMode := config.TLSVerifyFull
	if strings.HasPrefix(endpoint, "http://127.0.0.1:") || strings.HasPrefix(endpoint, "http://localhost:") {
		tlsMode = config.TLSDisabled
	}
	if tlsMode == config.TLSVerifyFull && ca == "" {
		t.Fatal("QDRANT_LIVE_CA_FILE is required for HTTPS")
	}
	body := os.Getenv("QDRANT_LIVE_QUERY_BODY")
	if body == "" {
		t.Fatal("QDRANT_LIVE_QUERY_BODY is required for native dense/sparse/hybrid qualification")
	}
	cfg := &config.TargetConfig{Name: "qdrant-live", Engine: config.EngineQdrant, Environment: "test", Consistency: config.ConsistencyEventual, TLS: config.TLSConfig{Mode: tlsMode, CAFile: ca}, Connection: config.ConnectionConfig{ConnectTimeout: 15 * time.Second, ReadTimeout: time.Minute, MaxOpen: 2, MaxIdle: 1, MaxIdleTime: time.Minute}, Qdrant: &config.QdrantConfig{Endpoint: endpoint, Version: config.QdrantVersion, Collections: []string{collection}, KeyEnv: "QDRANT_LIVE_JWT", PrivilegeRecheck: time.Minute, MaxRequestBytes: 1 << 20, MaxJSONDepth: 128, MaxJSONNodes: 100000}}
	limits := config.Limits{DefaultTimeout: 30 * time.Second, MaxTimeout: time.Minute, MaxRows: 5000, MaxResultBytes: 8 << 20, MaxBatchQueries: 50}
	controller := admission.New(admission.Config{Global: 4, PerTarget: 2, MaxQueued: 10, QueueTimeout: time.Second, BatchMax: 2, MaintenanceMax: 2})
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	for _, request := range []core.QdrantRequest{{Collection: collection, Operation: "metadata"}, {Collection: collection, Operation: "query", Body: []byte(body)}, {Collection: collection, Operation: "scroll", Body: []byte(`{"limit":1}`)}} {
		if _, err := target.QdrantRead(context.Background(), request); err != nil {
			t.Fatalf("live %s: %v", request.Operation, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := target.QdrantRead(ctx, core.QdrantRequest{Collection: collection, Operation: "scroll", Body: []byte(`{"limit":1}`)}); err == nil {
		t.Fatal("cancelled read succeeded")
	}
	// This ID is freshly generated and absent in the disposable fixture. A 2xx
	// response would demonstrate an unsafe write-capable credential.
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatal(err)
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	deleteBody := []byte(fmt.Sprintf(`{"points":["%08x-%04x-%04x-%04x-%012x"]}`, id[:4], id[4:6], id[6:8], id[8:10], id[10:]))
	req, err := http.NewRequest(http.MethodPost, target.origin+"/collections/"+url.PathEscape(collection)+"/points/delete?wait=true", bytes.NewReader(deleteBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("api-key", target.key)
	req.Header.Set("Content-Type", "application/json")
	response, err := target.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	if response.StatusCode != http.StatusForbidden && response.StatusCode != http.StatusUnauthorized {
		t.Fatal(errors.New("live Qdrant accepted a native mutation request with the read JWT"))
	}
}
