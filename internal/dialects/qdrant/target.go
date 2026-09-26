package qdrant

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
)

type Target struct {
	cfg        *config.TargetConfig
	limits     config.Limits
	client     *http.Client
	pool       *http.Transport
	origin     string
	key        string
	expires    time.Time
	allowed    map[string]bool
	controller *admission.Controller
	auditor    audit.Auditor
	recorder   metrics.Recorder
	mu         sync.RWMutex
	checked    time.Time
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	if cfg == nil || cfg.Qdrant == nil {
		return nil, errors.New("configuration_error: Qdrant configuration is required")
	}
	secret, err := cfg.Qdrant.Secret()
	if err != nil {
		return nil, errors.New("configuration_error: cannot read Qdrant scoped JWT")
	}
	secret = strings.TrimSpace(secret)
	expires, err := attestJWT(secret, cfg.Qdrant.Collections, time.Now())
	if err != nil {
		return nil, err
	}
	var roots *x509.CertPool
	if cfg.TLS.Mode == config.TLSVerifyFull {
		pem, err := os.ReadFile(cfg.TLS.CAFile)
		if err != nil {
			return nil, errors.New("configuration_error: cannot read Qdrant CA")
		}
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("configuration_error: invalid Qdrant CA")
		}
	}
	pool := &http.Transport{
		Proxy: nil, DisableCompression: true, ForceAttemptHTTP2: false,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots},
		DialContext:     (&net.Dialer{Timeout: cfg.Connection.ConnectTimeout, KeepAlive: 30 * time.Second}).DialContext,
		MaxConnsPerHost: cfg.Connection.MaxOpen, MaxIdleConns: cfg.Connection.MaxIdle, MaxIdleConnsPerHost: cfg.Connection.MaxIdle,
		IdleConnTimeout: cfg.Connection.MaxIdleTime, ResponseHeaderTimeout: cfg.Connection.ReadTimeout,
		MaxResponseHeaderBytes: 64 << 10, DisableKeepAlives: true,
	}
	client := &http.Client{Transport: pool, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	owned, cancel := context.WithCancel(context.Background())
	t := &Target{cfg: cfg, limits: limits, client: client, pool: pool, origin: strings.TrimSuffix(cfg.Qdrant.Endpoint, "/"), key: secret, expires: expires, controller: controller, auditor: auditor, recorder: recorder, allowed: map[string]bool{}, ctx: owned, cancel: cancel}
	for _, name := range cfg.Qdrant.Collections {
		t.allowed[name] = true
	}
	if err := t.prove(ctx); err != nil {
		cancel()
		pool.CloseIdleConnections()
		return nil, fmt.Errorf("Qdrant target %q startup proof: %w", cfg.Name, err)
	}
	t.wg.Add(1)
	go t.maintenance()
	return t, nil
}

func (t *Target) maintenance() {
	defer t.wg.Done()
	ticker := time.NewTicker(t.cfg.Qdrant.PrivilegeRecheck / 2)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			if err := t.prove(t.ctx); err != nil {
				t.mu.Lock()
				t.checked = time.Time{}
				t.mu.Unlock()
				if t.recorder != nil {
					t.recorder.Add("qdrant_attestation", 1, "maintenance", "failed")
				}
			} else if t.recorder != nil {
				t.recorder.Add("qdrant_attestation", 1, "maintenance", "allowed")
			}
		}
	}
}

func (t *Target) prove(ctx context.Context) error {
	if _, err := attestJWT(t.key, t.cfg.Qdrant.Collections, time.Now()); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, t.cfg.Connection.ConnectTimeout+30*time.Second)
	defer cancel()
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Maintenance)
	if err != nil {
		return err
	}
	defer permit.Release()
	root, err := t.request(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return err
	}
	var version struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(root, &version) != nil || version.Version != t.cfg.Qdrant.Version {
		return errors.New("profile_mismatch: Qdrant server version differs from pinned profile")
	}
	listing, err := t.request(ctx, http.MethodGet, "/collections", nil)
	if err != nil {
		return err
	}
	var catalog struct {
		Result struct {
			Collections []struct {
				Name string `json:"name"`
			} `json:"collections"`
		} `json:"result"`
	}
	if json.Unmarshal(listing, &catalog) != nil {
		return errors.New("invalid_response: Qdrant collection catalog is invalid")
	}
	found := map[string]bool{}
	for _, item := range catalog.Result.Collections {
		if !t.allowed[item.Name] || found[item.Name] {
			return errors.New("permission_denied: Qdrant scoped catalog exposes another or duplicate collection")
		}
		found[item.Name] = true
	}
	for _, name := range t.cfg.Qdrant.Collections {
		if !found[name] {
			return fmt.Errorf("permission_denied: configured Qdrant collection %q is not an exact collection in the scoped catalog", name)
		}
		if _, err := t.request(ctx, http.MethodGet, "/collections/"+url.PathEscape(name), nil); err != nil {
			return fmt.Errorf("collection %q: %w", name, err)
		}
	}
	// An otherwise valid JWT is no authority proof when the server ignores
	// authentication. Require an unauthenticated read to be denied.
	probe, err := http.NewRequestWithContext(ctx, http.MethodGet, t.origin+"/collections/"+url.PathEscape(t.cfg.Qdrant.Collections[0]), nil)
	if err != nil {
		return errors.New("configuration_error: cannot construct Qdrant auth probe")
	}
	response, err := t.client.Do(probe)
	if err != nil {
		return errors.New("transport_error: cannot prove Qdrant authentication is enabled")
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized && response.StatusCode != http.StatusForbidden {
		return errors.New("permission_denied: Qdrant did not deny an unauthenticated collection read")
	}
	t.mu.Lock()
	t.checked = time.Now()
	t.mu.Unlock()
	return nil
}

func (t *Target) ready(ctx context.Context) error {
	select {
	case <-t.ctx.Done():
		return errors.New("target_closed: Qdrant target is closed")
	default:
	}
	if time.Until(t.expires) <= time.Minute {
		return errors.New("permission_denied: Qdrant scoped JWT is expiring")
	}
	t.mu.RLock()
	checked := t.checked
	t.mu.RUnlock()
	if time.Since(checked) > t.cfg.Qdrant.PrivilegeRecheck {
		return t.prove(ctx)
	}
	return nil
}

func (t *Target) request(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if method != http.MethodGet && method != http.MethodPost {
		return nil, errors.New("invalid_request: Qdrant transport only supports fixed reads")
	}
	req, err := http.NewRequestWithContext(ctx, method, t.origin+path, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("invalid_request: cannot construct Qdrant request")
	}
	req.Header.Set("api-key", t.key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "identity")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	var connection net.Conn
	var traceMu sync.Mutex
	req = req.WithContext(httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			traceMu.Lock()
			defer traceMu.Unlock()
			connection = info.Conn
			_ = connection.SetWriteDeadline(time.Now().Add(t.cfg.Connection.WriteTimeout))
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			traceMu.Lock()
			defer traceMu.Unlock()
			if connection != nil {
				_ = connection.SetWriteDeadline(time.Time{})
			}
		},
	}))
	response, err := t.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("deadline_exceeded: Qdrant request cancelled or timed out")
		}
		return nil, errors.New("transport_error: Qdrant request failed")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, errors.New("permission_denied: Qdrant rejected configured scoped JWT")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("upstream_error: Qdrant read failed")
	}
	if response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
		return nil, errors.New("invalid_response: Qdrant response is compressed")
	}
	maxBytes := t.limits.MaxResultBytes
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(maxBytes)+1))
	if err != nil {
		return nil, errors.New("transport_error: cannot read Qdrant response")
	}
	if len(data) > maxBytes {
		return nil, errors.New("resource_limit: Qdrant response exceeds max_result_bytes")
	}
	if err := strictJSON(data, t.cfg.Qdrant.MaxJSONDepth, t.cfg.Qdrant.MaxJSONNodes); err != nil {
		return nil, errors.New("invalid_response: Qdrant response JSON is invalid or exceeds structural limits")
	}
	var envelope struct {
		Status json.RawMessage `json:"status"`
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return nil, errors.New("invalid_response: malformed Qdrant response")
	}
	if path != "/" && (string(envelope.Status) != `"ok"` || len(envelope.Result) == 0) {
		return nil, errors.New("invalid_response: Qdrant response lacks an OK result")
	}
	return data, nil
}

func (t *Target) QdrantRead(ctx context.Context, request core.QdrantRequest) (result *core.QdrantResult, err error) {
	started := time.Now()
	queryID := make([]byte, 12)
	if _, e := rand.Read(queryID); e != nil {
		return nil, errors.New("internal_error: cannot create query ID")
	}
	fingerprint := sha256.Sum256(append([]byte(request.Collection+"|"+request.Operation+"|"), request.Body...))
	defer func() {
		outcome := "allowed"
		reason := ""
		if err != nil {
			outcome = "denied"
			reason = err.Error()
		}
		if t.auditor != nil {
			t.auditor.Record(ctx, audit.Event{QueryID: hex.EncodeToString(queryID), Target: t.cfg.Name, Operation: "qdrant_" + request.Operation, Fingerprint: hex.EncodeToString(fingerprint[:12]), Tables: []string{request.Collection}, Decision: outcome, Reason: reason, Duration: time.Since(started)})
		}
		if t.recorder != nil {
			t.recorder.Add("query", 1, "qdrant_"+request.Operation, outcome)
			t.recorder.Observe("query_duration", time.Since(started), "qdrant_"+request.Operation)
		}
	}()
	if !t.allowed[request.Collection] {
		return nil, errors.New("permission_denied: Qdrant collection is outside target scope")
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, errors.New("resource_limit: Qdrant timeout exceeds configured maximum")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	unlink := context.AfterFunc(t.ctx, cancel)
	defer unlink()
	if request.Operation != "metadata" {
		if err := validateBody(request.Operation, request.Body, t.limits, t.cfg.Qdrant, t.allowed); err != nil {
			return nil, err
		}
	} else if len(request.Body) != 0 {
		return nil, errors.New("invalid_request: Qdrant metadata has no body")
	}
	if err := t.ready(ctx); err != nil {
		return nil, err
	}
	class := admission.Interactive
	if request.Operation == "metadata" {
		class = admission.Metadata
	}
	if request.Operation == "query_batch" {
		class = admission.Batch
	}
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, class)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	path := "/collections/" + url.PathEscape(request.Collection)
	method := http.MethodPost
	switch request.Operation {
	case "metadata":
		method = http.MethodGet
	case "query":
		path += "/points/query"
	case "query_batch":
		path += "/points/query/batch"
	case "query_groups":
		path += "/points/query/groups"
	case "scroll":
		path += "/points/scroll"
	case "retrieve":
		path += "/points"
	case "count":
		path += "/points/count"
	case "facet":
		path += "/facet"
	default:
		return nil, errors.New("invalid_request: unsupported Qdrant read operation")
	}
	data, err := t.request(ctx, method, path, request.Body)
	if err != nil {
		return nil, err
	}
	return &core.QdrantResult{Target: t.cfg.Name, Engine: config.EngineQdrant, Collection: request.Collection, Operation: request.Operation, Data: data, DurationMS: time.Since(started).Milliseconds()}, nil
}

func (t *Target) Info() core.TargetInfo {
	t.mu.RLock()
	checked := t.checked
	t.mu.RUnlock()
	proofAt := ""
	if !checked.IsZero() {
		proofAt = checked.UTC().Format(time.RFC3339)
	}
	healthy := !checked.IsZero() && time.Since(checked) <= t.cfg.Qdrant.PrivilegeRecheck && time.Until(t.expires) > time.Minute
	return core.TargetInfo{Name: t.cfg.Name, Engine: config.EngineQdrant, Environment: t.cfg.Environment, Consistency: t.cfg.Consistency, Healthy: healthy, ReadOnlyUser: healthy, ServerReadOnly: false, ServerVersion: t.cfg.Qdrant.Version, AllowedCollections: append([]string(nil), t.cfg.Qdrant.Collections...), Capabilities: map[string]string{"native_query": "dense,sparse,hybrid,prefetch,fusion,filters,groups", "native_read": "scroll,retrieve,count,facet", "credential_scope": "collection_read_jwt"}, ProofCheckedAt: proofAt}
}

func (t *Target) Close() error { t.cancel(); t.wg.Wait(); t.pool.CloseIdleConnections(); return nil }
