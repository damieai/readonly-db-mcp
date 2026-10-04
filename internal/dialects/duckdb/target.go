package duckdb

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/duckdbprotocol"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
)

type Target struct {
	cfg                                          *config.TargetConfig
	limits                                       config.Limits
	root, file, worker, relative, bwrap, version string
	fileInfo, workerInfo                         os.FileInfo
	controller                                   *admission.Controller
	auditor                                      audit.Auditor
	recorder                                     metrics.Recorder
	closed                                       atomic.Bool
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	root, file, worker, err := cfg.DuckDBPath()
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return nil, err
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return nil, errors.New("DuckDB requires bubblewrap")
	}
	finfo, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	winfo, err := os.Stat(worker)
	if err != nil {
		return nil, err
	}
	t := &Target{cfg: cfg, limits: limits, root: root, file: file, worker: worker, relative: rel, bwrap: bwrap, fileInfo: finfo, workerInfo: winfo, controller: controller, auditor: auditor, recorder: recorder}
	probeCtx, cancel := context.WithTimeout(ctx, cfg.Connection.ConnectTimeout+30*time.Second)
	defer cancel()
	response, err := t.call(probeCtx, duckdbprotocol.Request{Operation: "probe"})
	if err != nil {
		return nil, fmt.Errorf("DuckDB target %q sandbox proof: %w", cfg.Name, err)
	}
	if response.Version != "v"+cfg.DuckDB.Version {
		return nil, errors.New("profile_mismatch: DuckDB version differs from configuration")
	}
	t.version = response.Version
	return t, nil
}

func (t *Target) profile(request duckdbprotocol.Request) duckdbprotocol.Request {
	request.Database = t.relative
	if request.MaxRows == 0 {
		request.MaxRows = t.limits.MaxRows
	}
	request.MaxResultBytes = t.limits.MaxResultBytes
	request.MaxCellBytes = t.limits.MaxCellBytes
	request.MaxSQLBytes = t.limits.MaxSQLBytes
	request.MaxParameters = t.limits.MaxParameters
	request.MaxBatchQueries = t.limits.MaxBatchQueries
	request.MaxParameterBytes = t.limits.MaxParameterBytes
	request.MemoryMB = t.cfg.DuckDB.MemoryMB
	request.Threads = t.cfg.DuckDB.Threads
	return request
}

func (t *Target) ready() error {
	if t.closed.Load() {
		return errors.New("target_closed: DuckDB target is closed")
	}
	root, file, worker, err := t.cfg.DuckDBPath()
	if err != nil || root != t.root || file != t.file || worker != t.worker {
		return errors.New("permission_denied: DuckDB configured paths changed")
	}
	f, e := os.Stat(file)
	if e != nil || !os.SameFile(f, t.fileInfo) {
		return errors.New("permission_denied: DuckDB file identity changed")
	}
	w, e := os.Stat(worker)
	if e != nil || !os.SameFile(w, t.workerInfo) {
		return errors.New("permission_denied: DuckDB worker identity changed")
	}
	return nil
}

type cappedBuffer struct {
	bytes.Buffer
	max int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.max {
		return 0, errors.New("worker output exceeds limit")
	}
	return b.Buffer.Write(p)
}

func (t *Target) call(ctx context.Context, request duckdbprotocol.Request) (duckdbprotocol.Response, error) {
	if err := t.ready(); err != nil {
		return duckdbprotocol.Response{}, err
	}
	request = t.profile(request)
	payload, err := json.Marshal(request)
	if err != nil {
		return duckdbprotocol.Response{}, err
	}
	maxRequest := t.limits.MaxParameterBytes + t.limits.MaxSQLBytes + 4096
	if request.Operation == "batch" {
		maxRequest = len(request.Queries)*(t.limits.MaxParameterBytes+t.limits.MaxSQLBytes+1024) + 4096
	}
	if maxRequest > 16<<20 {
		maxRequest = 16 << 20
	}
	if len(payload) > maxRequest {
		return duckdbprotocol.Response{}, errors.New("resource_limit: DuckDB request too large")
	}
	args := []string{"--unshare-net", "--unshare-pid", "--die-with-parent", "--new-session", "--clearenv", "--setenv", "HOME", "/tmp", "--setenv", "TMPDIR", "/tmp", "--dir", "/app", "--dir", "/data", "--dir", "/usr", "--ro-bind", "/usr/lib", "/usr/lib", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--ro-bind", t.worker, "/app/worker", "--ro-bind", t.root, "/data", "--proc", "/proc", "--tmpfs", "/tmp", "--", "/app/worker"}
	cmd := exec.CommandContext(ctx, t.bwrap, args...)
	cmd.Stdin = bytes.NewReader(append(payload, '\n'))
	output := &cappedBuffer{max: t.limits.MaxResultBytes + 4096}
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	err = cmd.Run()
	if ctx.Err() != nil {
		return duckdbprotocol.Response{}, ctx.Err()
	}
	var response duckdbprotocol.Response
	if json.Unmarshal(output.Bytes(), &response) != nil {
		return duckdbprotocol.Response{}, errors.New("transport_error: DuckDB worker did not return a valid response")
	}
	if response.Error != "" {
		return duckdbprotocol.Response{}, errors.New(response.Error)
	}
	if err != nil {
		return duckdbprotocol.Response{}, errors.New("transport_error: DuckDB worker failed")
	}
	return response, nil
}

func (t *Target) Info() core.TargetInfo {
	return core.TargetInfo{Name: t.cfg.Name, Engine: config.EngineDuckDB, Environment: t.cfg.Environment, Consistency: t.cfg.Consistency, Database: "main", Schemas: []string{"main"}, Healthy: t.ready() == nil, ReadOnlyUser: false, ServerReadOnly: true, ParameterStyle: "?", ServerVersion: t.version, Capabilities: map[string]string{"sql": "select,cte,window,parquet,json", "isolation": "bubblewrap_readonly_no_network"}}
}
func (t *Target) Close() error { t.closed.Store(true); return nil }

func (t *Target) ValidateQuery(query string) (*core.Validation, error) {
	if query == "" || len(query) > t.limits.MaxSQLBytes || !utf8.ValidString(query) || strings.IndexByte(query, 0) >= 0 {
		return nil, errors.New("invalid_request: DuckDB SQL is empty or exceeds limit")
	}
	h := sha256.Sum256([]byte(query))
	return &core.Validation{Fingerprint: hex.EncodeToString(h[:12])}, nil
}

func (t *Target) Query(ctx context.Context, request core.QueryRequest) (*core.QueryResult, error) {
	return t.query(ctx, request, false)
}
func (t *Target) Explain(ctx context.Context, request core.QueryRequest) (*core.QueryResult, error) {
	return t.query(ctx, request, true)
}

func (t *Target) query(parent context.Context, request core.QueryRequest, explain bool) (result *core.QueryResult, err error) {
	start := time.Now()
	var id [12]byte
	if _, err = rand.Read(id[:]); err != nil {
		return nil, err
	}
	queryID := hex.EncodeToString(id[:])
	fingerprint := ""
	operation := "duckdb_query"
	if explain {
		operation = "duckdb_explain"
	}
	defer func() {
		decision, reason, rows, size, truncated := "allowed", "", 0, 0, false
		if err != nil {
			decision, reason = "denied", err.Error()
		}
		if result != nil {
			rows, truncated = result.RowCount, result.Truncated
			b, _ := json.Marshal(result)
			size = len(b)
		}
		if t.auditor != nil {
			t.auditor.Record(parent, audit.Event{QueryID: queryID, Target: t.cfg.Name, Operation: operation, Fingerprint: fingerprint, Decision: decision, Reason: reason, Rows: rows, ResponseBytes: size, Truncated: truncated, Duration: time.Since(start)})
		}
		if t.recorder != nil {
			t.recorder.Add("query", 1, operation, decision)
			t.recorder.Observe("query_duration", time.Since(start), operation)
		}
	}()
	v, e := t.ValidateQuery(request.SQL)
	if e != nil {
		return nil, e
	}
	fingerprint = v.Fingerprint
	if request.PostgreSQLOptions != nil {
		return nil, errors.New("invalid_request: PostgreSQL options are unavailable for DuckDB")
	}
	if len(request.Parameters) > t.limits.MaxParameters {
		return nil, errors.New("resource_limit: too many DuckDB parameters")
	}
	b, e := json.Marshal(request.Parameters)
	if e != nil || len(b) > t.limits.MaxParameterBytes {
		return nil, errors.New("resource_limit: DuckDB parameters exceed limit")
	}
	for _, param := range request.Parameters {
		b, e := json.Marshal(param)
		if e != nil || len(b) > t.limits.MaxParameterValueBytes {
			return nil, errors.New("resource_limit: DuckDB parameter exceeds value limit")
		}
	}
	rows := request.MaxRows
	if rows == 0 {
		rows = t.limits.MaxRows
	}
	if rows < 1 || rows > t.limits.MaxRows {
		return nil, errors.New("resource_limit: DuckDB row cap exceeds maximum")
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, errors.New("resource_limit: DuckDB timeout exceeds maximum")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	permit, e := t.controller.Acquire(ctx, t.cfg.Name, admission.Interactive)
	if e != nil {
		return nil, e
	}
	defer permit.Release()
	op := "query"
	if explain {
		op = "explain"
	}
	response, e := t.call(ctx, duckdbprotocol.Request{Operation: op, SQL: request.SQL, Parameters: request.Parameters, MaxRows: rows})
	if e != nil {
		return nil, e
	}
	if response.Result == nil {
		return nil, errors.New("invalid_response: DuckDB result missing")
	}
	result = response.Result
	result.QueryID = queryID
	result.Target = t.cfg.Name
	result.Engine = config.EngineDuckDB
	result.Environment = t.cfg.Environment
	result.Consistency = t.cfg.Consistency
	result.Database = "main"
	result.DurationMS = time.Since(start).Milliseconds()
	if result.RowCount > rows {
		return nil, errors.New("invalid_response: DuckDB exceeded row cap")
	}
	encoded, e := json.Marshal(result)
	if e != nil || len(encoded) > t.limits.MaxResultBytes {
		return nil, errors.New("resource_limit: DuckDB result exceeds max_result_bytes")
	}
	return result, nil
}

func (t *Target) ListTables(parent context.Context, pattern string, _ bool) ([]core.TableSummary, error) {
	ctx, cancel := context.WithTimeout(parent, t.limits.DefaultTimeout)
	defer cancel()
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Metadata)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	response, err := t.call(ctx, duckdbprotocol.Request{Operation: "tables", Pattern: pattern})
	if err != nil {
		return nil, err
	}
	return response.Tables, nil
}
func (t *Target) DescribeTable(parent context.Context, schema, table string, _ bool) (*core.TableDescription, error) {
	if schema == "" {
		schema = "main"
	}
	ctx, cancel := context.WithTimeout(parent, t.limits.DefaultTimeout)
	defer cancel()
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Metadata)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	response, err := t.call(ctx, duckdbprotocol.Request{Operation: "describe", Schema: schema, Table: table})
	if err != nil {
		return nil, err
	}
	if response.Description == nil {
		return nil, errors.New("invalid_response: DuckDB description missing")
	}
	response.Description.Target = t.cfg.Name
	return response.Description, nil
}
