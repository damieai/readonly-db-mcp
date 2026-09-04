package sqlserver

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
)

type Target struct {
	cfg             *config.TargetConfig
	limits          config.Limits
	db              *sql.DB
	policy          atomic.Pointer[Policy]
	admission       *admission.Controller
	auditor         audit.Auditor
	metrics         metrics.Recorder
	info            core.TargetInfo
	allowed         map[string]struct{}
	denied          map[string]struct{}
	cache           *metadataCache
	policyRevision  string
	defaultSchema   string
	healthy         atomic.Bool
	lastAttested    atomic.Int64
	gate            sync.RWMutex
	maintenanceStop context.CancelFunc
	maintenanceWG   sync.WaitGroup
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	password, err := cfg.Password()
	if err != nil {
		return nil, fmt.Errorf("target %q credentials: %w", cfg.Name, err)
	}
	db, err := openDB(cfg, password)
	password = ""
	if err != nil {
		return nil, fmt.Errorf("target %q connection setup: %w", cfg.Name, err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = db.Close()
		}
	}()

	pingCtx, pingCancel := context.WithTimeout(ctx, cfg.Connection.ConnectTimeout)
	if err := db.PingContext(pingCtx); err != nil {
		pingCancel()
		return nil, fmt.Errorf("target %q is unreachable", cfg.Name)
	}
	pingCancel()
	attestationCtx, attestationCancel := context.WithTimeout(ctx, cfg.Connection.ReadTimeout)
	defer attestationCancel()
	identity, err := verifyIdentityAndPrivileges(attestationCtx, db, cfg)
	if err != nil {
		return nil, fmt.Errorf("target %q startup verification failed: %w", cfg.Name, err)
	}

	t := &Target{
		cfg:            cfg,
		limits:         limits,
		db:             db,
		admission:      controller,
		auditor:        auditor,
		metrics:        recorder,
		allowed:        lowerSet(cfg.AllowedSchemas),
		denied:         lowerSet(cfg.DeniedTables),
		cache:          newMetadataCache(cfg.MetadataCache.IsEnabled(), cfg.MetadataCache.MaxEntries, cfg.MetadataCache.MaxBytes),
		policyRevision: sqlServerPolicyRevision(cfg),
		defaultSchema:  identity.defaultSchema,
		info: core.TargetInfo{
			Name:           cfg.Name,
			Engine:         cfg.Engine,
			Environment:    cfg.Environment,
			Consistency:    cfg.Consistency,
			Database:       cfg.Database,
			Schemas:        append([]string(nil), cfg.AllowedSchemas...),
			Healthy:        true,
			ReadOnlyUser:   true,
			ServerReadOnly: identity.readOnly,
			ParameterStyle: "@p1",
			ServerVersion:  identity.version,
			DeploymentMode: identity.deploymentMode,
		},
	}
	t.policy.Store(NewPolicy(cfg.Database, identity.defaultSchema, cfg.AllowedSchemas, cfg.DeniedTables, limits.MaxSQLBytes))
	t.healthy.Store(true)
	t.lastAttested.Store(time.Now().UnixNano())
	t.startPrivilegeRecheck()
	cleanup = false
	return t, nil
}

func openDB(c *config.TargetConfig, password string) (*sql.DB, error) {
	tlsConfig, encryption, err := sqlServerTLS(c)
	if err != nil {
		return nil, err
	}
	driverConfig := msdsn.Config{
		Host:                      c.Host,
		Port:                      uint64(c.Port),
		Database:                  c.Database,
		User:                      c.Username,
		Password:                  password,
		Encryption:                encryption,
		TLSConfig:                 tlsConfig,
		HostInCertificateProvided: c.TLS.Mode == config.TLSVerifyFull,
		ReadOnlyIntent:            c.SQLServer.ApplicationIntent == "read-only",
		AppName:                   c.SQLServer.ApplicationName,
		DialTimeout:               c.Connection.ConnectTimeout,
		ConnTimeout:               c.Connection.ConnectTimeout,
		Protocols:                 []string{"tcp"},
		DisableRetry:              true,
	}
	connector := mssql.NewConnectorConfig(driverConfig)
	connector.SessionInitSQL = fmt.Sprintf(
		"SET NOCOUNT ON; SET XACT_ABORT ON; SET LOCK_TIMEOUT %d; SET DEADLOCK_PRIORITY LOW;",
		c.SQLServer.LockTimeout.Milliseconds(),
	)
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(c.Connection.MaxOpen)
	db.SetMaxIdleConns(c.Connection.MaxIdle)
	db.SetConnMaxLifetime(c.Connection.MaxLifetime)
	db.SetConnMaxIdleTime(c.Connection.MaxIdleTime)
	return db, nil
}

func sqlServerTLS(c *config.TargetConfig) (*tls.Config, msdsn.Encryption, error) {
	if c.TLS.Mode == config.TLSDisabled {
		return nil, msdsn.EncryptionDisabled, nil
	}
	tlsConfig := &tls.Config{
		MinVersion:                  tls.VersionTLS12,
		ServerName:                  c.TLS.ServerName,
		DynamicRecordSizingDisabled: true,
	}
	if c.TLS.Mode == config.TLSRequired {
		tlsConfig.InsecureSkipVerify = true //nolint:gosec -- explicitly non-production mode
	} else {
		caPEM, err := os.ReadFile(c.TLS.CAFile)
		if err != nil {
			return nil, 0, fmt.Errorf("read TLS CA file: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(caPEM) {
			return nil, 0, errors.New("TLS CA file contains no certificates")
		}
		tlsConfig.RootCAs = roots
	}
	if c.TLS.CertFile != "" {
		certificate, err := tls.LoadX509KeyPair(c.TLS.CertFile, c.TLS.KeyFile)
		if err != nil {
			return nil, 0, fmt.Errorf("load TLS client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, msdsn.EncryptionRequired, nil
}

func (t *Target) Info() core.TargetInfo {
	info := t.info
	info.Healthy = t.requireHealthy() == nil
	info.Schemas = append([]string(nil), info.Schemas...)
	return info
}

func (t *Target) ValidateQuery(query string) (*core.Validation, error) {
	return t.policy.Load().Validate(query, -1)
}

func (t *Target) Query(ctx context.Context, request core.QueryRequest) (*core.QueryResult, error) {
	validation, err := t.policy.Load().Validate(request.SQL, len(request.Parameters))
	if err != nil {
		t.audit(ctx, audit.Event{Target: t.cfg.Name, Operation: "query_select", Decision: "rejected", Reason: err.Error()})
		return nil, err
	}
	return t.execute(ctx, request, validation)
}

func (t *Target) Explain(ctx context.Context, request core.QueryRequest) (*core.QueryResult, error) {
	validation, err := t.policy.Load().Validate(request.SQL, len(request.Parameters))
	if err != nil {
		return nil, err
	}
	timeout, _, err := t.requestLimits(request)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	permit, err := t.admission.Acquire(qctx, t.cfg.Name, admission.Interactive)
	if err != nil {
		return nil, fmt.Errorf("query concurrency limit: %w", err)
	}
	defer permit.Release()
	t.gate.RLock()
	defer t.gate.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	plan, err := t.showPlan(qctx, request.SQL, namedParameters(request.Parameters))
	if err != nil {
		return nil, err
	}
	normalizedPlan, planTruncated := normalize(plan, t.limits.MaxCellBytes)
	truncatedCells := 0
	if planTruncated {
		truncatedCells = 1
	}
	result := &core.QueryResult{
		QueryID:        uuid.NewString(),
		Target:         t.cfg.Name,
		Engine:         t.cfg.Engine,
		Environment:    t.cfg.Environment,
		Consistency:    t.cfg.Consistency,
		Database:       t.cfg.Database,
		Columns:        []core.Column{{Name: "ShowPlanXML", DatabaseType: "xml"}},
		Rows:           []map[string]any{{"ShowPlanXML": normalizedPlan}},
		RowCount:       1,
		TruncatedCells: truncatedCells,
		DurationMS:     time.Since(started).Milliseconds(),
	}
	if err := enforceBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	t.auditResult(qctx, "query_explain", validation, result, started)
	return result, nil
}

func (t *Target) execute(ctx context.Context, request core.QueryRequest, validation *core.Validation) (*core.QueryResult, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	timeout, maxRows, err := t.requestLimits(request)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	permit, err := t.admission.Acquire(qctx, t.cfg.Name, admission.Interactive)
	if err != nil {
		return nil, fmt.Errorf("query concurrency limit: %w", err)
	}
	defer permit.Release()
	t.gate.RLock()
	defer t.gate.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	args := namedParameters(request.Parameters)
	if _, err := t.showPlan(qctx, request.SQL, args); err != nil {
		t.audit(qctx, audit.Event{Target: t.cfg.Name, Operation: "query_select", Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "rejected", Reason: err.Error()})
		return nil, err
	}
	tx, err := t.db.BeginTx(qctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, sanitize(err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(qctx, request.SQL, args...)
	if err != nil {
		return nil, sanitize(err)
	}
	result, err := t.collect(rows, maxRows, t.limits.MaxResultBytes)
	rows.Close()
	if err != nil {
		return nil, err
	}
	result.QueryID = uuid.NewString()
	result.Target = t.cfg.Name
	result.Engine = t.cfg.Engine
	result.Environment = t.cfg.Environment
	result.Consistency = t.cfg.Consistency
	result.Database = t.cfg.Database
	result.DurationMS = time.Since(started).Milliseconds()
	if err := enforceBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	t.auditResult(qctx, "query_select", validation, result, started)
	return result, nil
}

func (t *Target) requestLimits(request core.QueryRequest) (time.Duration, int, error) {
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout > t.limits.MaxTimeout {
		return 0, 0, errors.New("requested timeout exceeds configured maximum")
	}
	maxRows := request.MaxRows
	if maxRows <= 0 {
		maxRows = t.limits.MaxRows
	}
	if maxRows > t.limits.MaxRows {
		return 0, 0, errors.New("requested row limit exceeds configured maximum")
	}
	if len(request.Parameters) > t.limits.MaxParameters {
		return 0, 0, errors.New("query has too many parameters")
	}
	if err := validateParameters(request.Parameters, t.limits.MaxParameterBytes, t.limits.MaxParameterValueBytes); err != nil {
		return 0, 0, err
	}
	return timeout, maxRows, nil
}

func namedParameters(parameters []any) []any {
	result := make([]any, len(parameters))
	for i, value := range parameters {
		result[i] = sql.Named("p"+strconv.Itoa(i+1), value)
	}
	return result
}

func (t *Target) collect(rows *sql.Rows, maxRows, byteBudget int) (*core.QueryResult, error) {
	names, err := rows.Columns()
	if err != nil {
		return nil, errors.New("read result columns")
	}
	if len(names) > 1024 {
		return nil, errors.New("query result has too many columns")
	}
	types, _ := rows.ColumnTypes()
	names = uniqueNames(names)
	columns := make([]core.Column, len(names))
	for i, name := range names {
		nullable, _ := types[i].Nullable()
		columns[i] = core.Column{Name: name, DatabaseType: types[i].DatabaseTypeName(), Nullable: nullable}
	}
	result := &core.QueryResult{Columns: columns, Rows: make([]map[string]any, 0, min(maxRows, 32))}
	used := 0
	for rows.Next() {
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			break
		}
		values := make([]any, len(names))
		destinations := make([]any, len(names))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, errors.New("scan query result")
		}
		row := make(map[string]any, len(names))
		for i, value := range values {
			normalized, truncated := normalize(value, t.limits.MaxCellBytes)
			if truncated {
				result.TruncatedCells++
			}
			row[names[i]] = normalized
		}
		encoded, _ := json.Marshal(row)
		if used+len(encoded) > byteBudget {
			result.Truncated = true
			break
		}
		used += len(encoded)
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, sanitize(err)
	}
	result.RowCount = len(result.Rows)
	return result, nil
}

func normalize(value any, maximum int) (any, bool) {
	switch x := value.(type) {
	case nil:
		return nil, false
	case time.Time:
		return x.Format(time.RFC3339Nano), false
	case []byte:
		truncated := len(x) > maximum
		if truncated {
			x = x[:maximum]
		}
		if utf8.Valid(x) {
			return string(x), truncated
		}
		return "base64:" + base64.StdEncoding.EncodeToString(x), truncated
	case string:
		if len(x) <= maximum {
			return x, false
		}
		cut := maximum
		for cut > 0 && !utf8.RuneStart(x[cut]) {
			cut--
		}
		return x[:cut], true
	case int64:
		if x > 1<<53-1 || x < -(1<<53-1) {
			return strconv.FormatInt(x, 10), false
		}
		return x, false
	case float64, bool:
		return x, false
	default:
		return fmt.Sprint(x), false
	}
}

func uniqueNames(values []string) []string {
	seen := make(map[string]int, len(values))
	result := make([]string, len(values))
	for i, value := range values {
		seen[value]++
		result[i] = value
		if seen[value] > 1 {
			result[i] = fmt.Sprintf("%s#%d", value, seen[value])
		}
	}
	return result
}

func enforceBudget(result *core.QueryResult, maximum int) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return errors.New("encode query result")
	}
	if len(encoded) <= maximum {
		return nil
	}
	all := result.Rows
	low, high := 0, len(all)
	for low < high {
		middle := (low + high + 1) / 2
		result.Rows = all[:middle]
		result.RowCount = middle
		encoded, _ = json.Marshal(result)
		if len(encoded) <= maximum {
			low = middle
		} else {
			high = middle - 1
		}
	}
	result.Rows = all[:low]
	result.RowCount = low
	result.Truncated = true
	encoded, _ = json.Marshal(result)
	if len(encoded) > maximum {
		return errors.New("query result metadata exceeds configured result-byte limit")
	}
	return nil
}

func validateParameters(parameters []any, totalLimit, valueLimit int) error {
	used := 0
	for _, value := range parameters {
		size := 0
		switch x := value.(type) {
		case nil, bool, float64:
			size = 8
		case string:
			size = len(x)
		default:
			return errors.New("parameters must be JSON scalars; encode large integers as strings")
		}
		if valueLimit > 0 && size > valueLimit {
			return errors.New("SQL parameter exceeds the per-value byte limit")
		}
		used += size
		if totalLimit > 0 && used > totalLimit {
			return errors.New("SQL parameters exceed the total byte limit")
		}
	}
	return nil
}

func parameterBytes(parameters []any) int {
	total := 0
	for _, value := range parameters {
		switch x := value.(type) {
		case string:
			total += len(x)
		case nil, bool, float64:
			total += 8
		}
	}
	return total
}

func sanitize(err error) error {
	var serverError mssql.Error
	if errors.As(err, &serverError) {
		return fmt.Errorf("database rejected query (SQL Server error %d)", serverError.Number)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("query timed out")
	}
	if errors.Is(err, context.Canceled) {
		return errors.New("query was canceled")
	}
	return errors.New("database query failed")
}

func (t *Target) audit(ctx context.Context, event audit.Event) {
	if t.auditor != nil {
		t.auditor.Record(ctx, event)
	}
}

func (t *Target) auditResult(ctx context.Context, operation string, validation *core.Validation, result *core.QueryResult, started time.Time) {
	encoded, _ := json.Marshal(result)
	t.audit(ctx, audit.Event{QueryID: result.QueryID, Target: t.cfg.Name, Operation: operation, Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "allowed", Rows: result.RowCount, Truncated: result.Truncated, Duration: time.Since(started), ResponseBytes: len(encoded)})
	if t.metrics != nil {
		t.metrics.Add("query_total", 1, operation, "allowed")
		t.metrics.Observe("query_duration", time.Since(started), operation)
	}
}

func (t *Target) requireHealthy() error {
	if !t.healthy.Load() {
		return errors.New("SQL Server target failed its latest privilege attestation")
	}
	if interval := t.cfg.SQLServer.PrivilegeRecheck; interval > 0 {
		last := t.lastAttested.Load()
		if last == 0 || time.Since(time.Unix(0, last)) > 2*interval {
			return errors.New("SQL Server target failed its latest privilege attestation")
		}
	}
	return nil
}

func (t *Target) startPrivilegeRecheck() {
	ctx, cancel := context.WithCancel(context.Background())
	t.maintenanceStop = cancel
	t.maintenanceWG.Add(1)
	go func() {
		defer t.maintenanceWG.Done()
		ticker := time.NewTicker(t.cfg.SQLServer.PrivilegeRecheck)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkCtx, checkCancel := context.WithTimeout(ctx, t.cfg.Connection.ReadTimeout)
				permit, err := t.admission.Acquire(checkCtx, t.cfg.Name, admission.Maintenance)
				if err == nil {
					t.gate.Lock()
					identity, verifyErr := verifyIdentityAndPrivileges(checkCtx, t.db, t.cfg)
					if verifyErr == nil && strings.EqualFold(identity.defaultSchema, t.defaultSchema) {
						t.lastAttested.Store(time.Now().UnixNano())
						t.healthy.Store(true)
					} else {
						t.healthy.Store(false)
					}
					t.gate.Unlock()
					permit.Release()
				} else {
					t.healthy.Store(false)
				}
				checkCancel()
			}
		}
	}()
}

func (t *Target) Close() error {
	if t.maintenanceStop != nil {
		t.maintenanceStop()
		t.maintenanceWG.Wait()
	}
	t.cache.clear()
	return t.db.Close()
}

func sqlServerPolicyRevision(c *config.TargetConfig) string {
	input := strings.ToLower(c.Engine + "\x00" + c.Database + "\x00" + strings.Join(c.AllowedSchemas, "\x00") + "\x00" + strings.Join(c.DeniedTables, "\x00"))
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:12])
}
