package elasticsearch

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
	"golang.org/x/sync/semaphore"
)

var responseMemory = semaphore.NewWeighted(512 << 20)
var queuedMemory = semaphore.NewWeighted(64 << 20)

type Target struct {
	cfg            *config.TargetConfig
	limits         config.Limits
	wire           *wire
	admission      *admission.Controller
	auditor        audit.Auditor
	metrics        metrics.Recorder
	ctx            context.Context
	stop           context.CancelFunc
	wg             sync.WaitGroup
	closed         atomic.Bool
	healthy        atomic.Bool
	checked        atomic.Int64
	next           atomic.Uint64
	auditKey       [32]byte
	authorityMu    sync.Mutex
	authorityHash  [32]byte
	authorityEpoch atomic.Uint64
	cursorMu       sync.Mutex
	cursors        map[string]*ownedCursor
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	if cfg.Elasticsearch == nil || config.ElasticsearchBuilds[cfg.Elasticsearch.Version] == "" || catalog[cfg.Elasticsearch.Version] == nil {
		return nil, failure("configuration_error", "Elasticsearch profile is required")
	}
	w, err := newWire(cfg)
	if err != nil {
		return nil, err
	}
	w.maxCell = limits.MaxCellBytes
	t := &Target{cfg: cfg, limits: limits, wire: w, admission: controller, auditor: auditor, metrics: recorder, cursors: map[string]*ownedCursor{}}
	t.ctx, t.stop = context.WithCancel(context.Background())
	if _, err := rand.Read(t.auditKey[:]); err != nil {
		t.Close()
		return nil, err
	}
	startup, cancel := context.WithTimeout(ctx, cfg.Connection.ConnectTimeout)
	defer cancel()
	if err := t.refresh(startup); err != nil {
		t.Close()
		return nil, err
	}
	t.wg.Add(2)
	go t.maintain()
	go t.maintainCursors()
	return t, nil
}

func (t *Target) memoryCost() int64 {
	return 6*int64(t.limits.MaxResultBytes) + int64(t.cfg.Elasticsearch.MaxJSONNodes)*64 + int64(t.cfg.Elasticsearch.MaxRequestBytes) + (2 << 20)
}
func (t *Target) refresh(ctx context.Context) error {
	if checked := t.checked.Load(); checked != 0 && time.Since(time.Unix(0, checked)) >= t.cfg.Elasticsearch.PrivilegeRecheck {
		t.invalidateAuthority()
	}
	permit, err := t.admission.Acquire(ctx, t.cfg.Name, admission.Maintenance)
	if err != nil {
		t.invalidateAuthority()
		return err
	}
	defer permit.Release()
	charge := t.memoryCost()
	if !responseMemory.TryAcquire(charge) {
		t.invalidateAuthority()
		return failure("resource_limit", "Elasticsearch proof memory budget is exhausted")
	}
	defer responseMemory.Release(charge)
	if err := t.attest(ctx); err != nil {
		t.invalidateAuthority()
		return err
	}
	t.checked.Store(time.Now().UnixNano())
	t.healthy.Store(true)
	return nil
}
func (t *Target) maintain() {
	defer t.wg.Done()
	ticker := time.NewTicker(t.cfg.Elasticsearch.PrivilegeRecheck / 2)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(t.ctx, t.cfg.Connection.ConnectTimeout)
			err := t.refresh(ctx)
			cancel()
			if t.metrics != nil {
				outcome := "allowed"
				if err != nil {
					outcome = "failed"
				}
				t.metrics.Add("elasticsearch_attestation", 1, "maintenance", outcome)
			}
		}
	}
}
func (t *Target) ready() error {
	if t.closed.Load() || !t.healthy.Load() || time.Since(time.Unix(0, t.checked.Load())) >= t.cfg.Elasticsearch.PrivilegeRecheck {
		t.invalidateAuthority()
		return failure("permission_proof_stale", "Elasticsearch authority proof is unavailable or stale")
	}
	return nil
}
func (t *Target) Info() core.TargetInfo {
	features := capabilities(t.cfg.Elasticsearch.Version)
	features["esql_enrich"] = "requires_explicit_cluster_snapshot_scope"
	if t.cfg.Elasticsearch.EnrichEnabled() {
		features["esql_enrich"] = "implemented_native_cluster_snapshot_scope"
		features["enrich_data_scope"] = config.ElasticsearchEnrichScope
	}
	return core.TargetInfo{Name: t.cfg.Name, Engine: config.EngineElasticsearch, Environment: t.cfg.Environment, Consistency: config.ConsistencyEventual, Healthy: t.ready() == nil, ReadOnlyUser: t.ready() == nil, ServerReadOnly: false, ServerVersion: t.cfg.Elasticsearch.Version, DeploymentMode: "elasticsearch-phase-8-templates", AllowedIndices: append([]string(nil), t.cfg.Elasticsearch.AllowedIndices...), PolicyRevision: "es-native-templates-v8", ProofCheckedAt: time.Unix(0, t.checked.Load()).UTC().Format(time.RFC3339), Capabilities: features}
}

func (t *Target) ElasticsearchMetadata(ctx context.Context, request core.ElasticsearchMetadataRequest) (result *core.ElasticsearchResult, err error) {
	started := time.Now()
	id := uuid.NewString()
	defer func() {
		outcome := "allowed"
		code := ""
		if err != nil {
			outcome = "rejected"
			var e *Error
			if errors.As(err, &e) {
				code = e.Code
			}
		}
		if code == "permission_denied" || code == "profile_mismatch" {
			t.invalidateAuthority()
		}
		if t.metrics != nil {
			t.metrics.Observe("elasticsearch_request", time.Since(started), "metadata")
			t.metrics.Add("elasticsearch_request", 1, "metadata", outcome)
		}
		if t.auditor != nil {
			mac := hmac.New(sha256.New, t.auditKey[:])
			_, _ = mac.Write([]byte(request.Operation))
			for _, index := range request.Indices {
				_, _ = mac.Write([]byte{0})
				_, _ = mac.Write([]byte(index))
			}
			t.auditor.Record(ctx, audit.Event{QueryID: id, Target: t.cfg.Name, Operation: "es_metadata", Fingerprint: hex.EncodeToString(mac.Sum(nil)[:12]), Decision: outcome, Reason: code, Duration: time.Since(started)})
		}
	}()
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, failure("resource_limit", "request timeout exceeds configured maximum")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	unlink := context.AfterFunc(t.ctx, cancel)
	defer unlink()
	if err := t.ready(); err != nil {
		return nil, err
	}
	if err := metadataOperation(t.cfg.Elasticsearch.Version, request.Operation); err != nil {
		return nil, err
	}
	indices := request.Indices
	if len(indices) == 0 {
		indices = t.cfg.Elasticsearch.AllowedIndices
	}
	if len(indices) > t.cfg.Elasticsearch.MaxResolvedIndices {
		return nil, failure("resource_limit", "too many requested index expressions")
	}
	bytes := int64(0)
	for _, i := range indices {
		bytes += int64(len(i)) + 4
	}
	if bytes > int64(t.cfg.Elasticsearch.MaxRequestBytes) || !queuedMemory.TryAcquire(bytes) {
		return nil, failure("resource_limit", "queued Elasticsearch request budget is exhausted")
	}
	defer queuedMemory.Release(bytes)
	permit, err := t.admission.Acquire(ctx, t.cfg.Name, admission.Metadata)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	charge := t.memoryCost()
	if !responseMemory.TryAcquire(charge) {
		return nil, failure("resource_limit", "Elasticsearch response memory budget is exhausted")
	}
	defer responseMemory.Release(charge)
	if err := t.ready(); err != nil {
		return nil, err
	}
	endpoint := int(t.next.Add(1)-1) % len(t.wire.clients)
	data, physical, err := t.resolve(ctx, endpoint, indices)
	if err != nil {
		return nil, err
	}
	if request.Operation == "mappings" {
		if err := t.ready(); err != nil {
			return nil, err
		}
		// Preserve alias/data-stream expressions in the request. Validate actual
		// response names as well, so an alias race cannot disclose new mappings.
		data, err = t.wire.get(ctx, endpoint, "/"+strings.Join(indices, ",")+"/_mapping", nil, t.limits.MaxResultBytes)
		if err != nil {
			return nil, err
		}
		if err := validateMappings(data, physical); err != nil {
			return nil, err
		}
	}
	result = &core.ElasticsearchResult{QueryID: id, Target: t.cfg.Name, Engine: config.EngineElasticsearch, Consistency: config.ConsistencyEventual, Operation: request.Operation, Data: json.RawMessage(data), DurationMS: time.Since(started).Milliseconds()}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, failure("invalid_response", "cannot encode Elasticsearch result")
	}
	// MCP emits both structured content and its text representation. Bound both
	// copies and the fixed transport envelope rather than just native JSON bytes.
	textCopy, _ := json.Marshal(string(encoded))
	if len(encoded)+len(textCopy)+512 > t.limits.MaxResultBytes {
		return nil, failure("resource_limit", "encoded MCP result exceeds response byte limit")
	}
	return result, nil
}
func (t *Target) Close() error {
	if t.closed.Swap(true) {
		return nil
	}
	t.healthy.Store(false)
	if t.stop != nil {
		t.stop()
	}
	t.wg.Wait()
	if t.wire != nil {
		ctx, cancel := context.WithTimeout(context.Background(), cursorCleanupTimeout)
		if permit, err := t.admission.Acquire(ctx, t.cfg.Name, admission.Maintenance); err == nil {
			_ = t.closeOwnedCursors(ctx, "")
			permit.Release()
		}
		cancel()
	}
	if t.wire != nil {
		t.wire.close()
	}
	return nil
}
