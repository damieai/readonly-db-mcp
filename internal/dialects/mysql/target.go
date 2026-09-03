package mysql

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
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	driver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
)

type Target struct {
	config               *config.TargetConfig
	limits               config.Limits
	db                   *sql.DB
	policy               *Policy
	auditor              audit.Auditor
	admission            *admission.Controller
	info                 core.TargetInfo
	tlsName              string
	allowedSchemas       map[string]struct{}
	deniedTables         map[string]struct{}
	metadataPlaceholders string
	metadataCache        *metadataCache
	resultCache          *resultCache
	metrics              metrics.Recorder
	verifiedAt           atomic.Int64
	policyRevision       string
	healthy              atomic.Bool
	gate                 sync.RWMutex
	maintenanceStop      context.CancelFunc
	maintenanceWG        sync.WaitGroup
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	password, err := cfg.Password()
	if err != nil {
		return nil, fmt.Errorf("target %q credentials: %w", cfg.Name, err)
	}
	db, tlsName, err := openDB(cfg, password, limits.MaxTimeout)
	password = ""
	if err != nil {
		return nil, fmt.Errorf("target %q connection setup: %w", cfg.Name, err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = db.Close()
			if tlsName != "" {
				driver.DeregisterTLSConfig(tlsName)
			}
		}
	}()

	pingCtx, cancel := context.WithTimeout(ctx, cfg.Connection.ConnectTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("target %q is unreachable", cfg.Name)
	}

	identity, grants, err := inspectIdentity(pingCtx, db)
	if err != nil {
		return nil, fmt.Errorf("target %q startup verification failed: %w", cfg.Name, err)
	}
	if !strings.EqualFold(identity.Database, cfg.Database) {
		return nil, fmt.Errorf("target %q connected to an unexpected database", cfg.Name)
	}
	if err := ValidateGrants(grants, cfg.AllowedSchemas); err != nil {
		return nil, fmt.Errorf("target %q is not backed by a strictly read-only account: %w", cfg.Name, err)
	}

	target := &Target{
		config:    cfg,
		limits:    limits,
		db:        db,
		policy:    NewPolicy(cfg.Database, cfg.AllowedSchemas, cfg.DeniedTables, limits.MaxSQLBytes),
		auditor:   auditor,
		admission: controller,
		tlsName:   tlsName,
		info: core.TargetInfo{
			Name:           cfg.Name,
			Engine:         cfg.Engine,
			Environment:    cfg.Environment,
			Consistency:    cfg.Consistency,
			Database:       cfg.Database,
			Schemas:        append([]string(nil), cfg.AllowedSchemas...),
			Healthy:        true,
			ReadOnlyUser:   true,
			ServerReadOnly: identity.ServerReadOnly,
			ParameterStyle: "?",
			ServerVersion:  identity.Version,
		},
		allowedSchemas:       lowerSet(cfg.AllowedSchemas),
		deniedTables:         lowerSet(cfg.DeniedTables),
		metadataPlaceholders: strings.TrimRight(strings.Repeat("?,", len(cfg.AllowedSchemas)), ","),
		metadataCache:        newMetadataCache(cfg.MetadataCache.IsEnabled(), cfg.MetadataCache.MaxEntries, cfg.MetadataCache.MaxBytes),
		resultCache:          newResultCache(cfg.ResultCache.Enabled, cfg.ResultCache.TTL, cfg.ResultCache.MaxEntries, cfg.ResultCache.MaxBytes, cfg.ResultCache.MaxEntryBytes),
		metrics:              recorder,
		policyRevision:       targetPolicyRevision(cfg),
	}
	target.verifiedAt.Store(time.Now().UnixNano())
	target.healthy.Store(true)
	target.startPrivilegeRecheck()
	cleanup = false
	return target, nil
}

func openDB(cfg *config.TargetConfig, password string, maxQueryTime time.Duration) (*sql.DB, string, error) {
	mysqlCfg := driver.NewConfig()
	mysqlCfg.User = cfg.Username
	mysqlCfg.Passwd = password
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	mysqlCfg.DBName = cfg.Database
	mysqlCfg.ParseTime = true
	mysqlCfg.Loc = time.UTC
	mysqlCfg.Timeout = cfg.Connection.ConnectTimeout
	mysqlCfg.ReadTimeout = cfg.Connection.ReadTimeout
	mysqlCfg.WriteTimeout = cfg.Connection.WriteTimeout
	mysqlCfg.MultiStatements = false
	mysqlCfg.InterpolateParams = false
	mysqlCfg.Params = map[string]string{
		"max_execution_time": strconv.FormatInt(maxQueryTime.Milliseconds(), 10),
	}

	tlsName, err := configureTLS(cfg)
	if err != nil {
		return nil, "", err
	}
	if tlsName != "" {
		mysqlCfg.TLSConfig = tlsName
	}

	db, err := sql.Open("mysql", mysqlCfg.FormatDSN())
	if err != nil {
		if tlsName != "" {
			driver.DeregisterTLSConfig(tlsName)
		}
		return nil, "", errors.New("open MySQL connection")
	}
	db.SetMaxOpenConns(cfg.Connection.MaxOpen)
	db.SetMaxIdleConns(cfg.Connection.MaxIdle)
	db.SetConnMaxLifetime(cfg.Connection.MaxLifetime)
	db.SetConnMaxIdleTime(cfg.Connection.MaxIdleTime)
	return db, tlsName, nil
}

func configureTLS(cfg *config.TargetConfig) (string, error) {
	if cfg.TLS.Mode == config.TLSDisabled {
		return "", nil
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.TLS.Mode == config.TLSRequired {
		// This mode encrypts traffic without authenticating the server and is
		// intentionally refused for production by configuration validation.
		tlsCfg.InsecureSkipVerify = true //nolint:gosec
	} else {
		caPEM, err := os.ReadFile(cfg.TLS.CAFile)
		if err != nil {
			return "", fmt.Errorf("read TLS CA file: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(caPEM) {
			return "", errors.New("TLS CA file contains no certificates")
		}
		tlsCfg.RootCAs = roots
		tlsCfg.ServerName = cfg.TLS.ServerName
	}
	if cfg.TLS.CertFile != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			return "", fmt.Errorf("load TLS client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{certificate}
	}
	sum := sha256.Sum256([]byte(cfg.Name))
	name := "readonly-db-mcp-" + hex.EncodeToString(sum[:8])
	if err := driver.RegisterTLSConfig(name, tlsCfg); err != nil {
		return "", fmt.Errorf("register TLS configuration: %w", err)
	}
	return name, nil
}

type identity struct {
	CurrentUser    string
	Database       string
	Version        string
	ServerReadOnly bool
}

func inspectIdentity(ctx context.Context, db *sql.DB) (identity, []string, error) {
	var result identity
	var readOnly, superReadOnly int
	err := db.QueryRowContext(ctx,
		"SELECT CURRENT_USER(), DATABASE(), VERSION(), @@GLOBAL.read_only, @@GLOBAL.super_read_only",
	).Scan(&result.CurrentUser, &result.Database, &result.Version, &readOnly, &superReadOnly)
	if err != nil {
		return identity{}, nil, errors.New("inspect connected database identity")
	}
	result.ServerReadOnly = readOnly == 1 || superReadOnly == 1

	rows, err := db.QueryContext(ctx, "SHOW GRANTS")
	if err != nil {
		return identity{}, nil, errors.New("inspect current account grants")
	}
	defer rows.Close()
	var grants []string
	for rows.Next() {
		var grant string
		if err := rows.Scan(&grant); err != nil {
			return identity{}, nil, errors.New("read current account grants")
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return identity{}, nil, errors.New("read current account grants")
	}
	if len(grants) == 0 {
		return identity{}, nil, errors.New("current account has no verifiable grants")
	}
	return result, grants, nil
}

func (t *Target) Info() core.TargetInfo {
	info := t.info
	info.Healthy = t.requireHealthy() == nil
	info.Schemas = append([]string(nil), t.info.Schemas...)
	return info
}

func (t *Target) ValidateQuery(query string) (*core.Validation, error) {
	return t.policy.Validate(query)
}

func (t *Target) Query(ctx context.Context, request core.QueryRequest) (*core.QueryResult, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if err := t.validateRequest(request); err != nil {
		return nil, err
	}
	policyStarted := time.Now()
	validation, err := t.policy.Validate(request.SQL)
	t.observe("policy", time.Since(policyStarted), "query_select")
	if err != nil {
		t.record(ctx, audit.Event{Target: t.config.Name, Operation: "query_select", Decision: "rejected", Reason: err.Error()})
		return nil, err
	}
	cacheKey := ""
	if t.config.ResultCache.Enabled && validation.Cacheable {
		encoded, _ := json.Marshal(struct {
			Fingerprint string
			Parameters  []any
			MaxRows     int
			Revision    string
		}{validation.Fingerprint, request.Parameters, request.MaxRows, t.policyRevision})
		sum := sha256.Sum256(encoded)
		cacheKey = hex.EncodeToString(sum[:])
		if cached, age, ok := t.resultCache.get(cacheKey); ok {
			t.count("cache", 1, "query_select", "hit")
			cached.QueryID = uuid.NewString()
			cached.CacheStatus = "hit"
			cached.CacheAgeMS = age.Milliseconds()
			cached.DurationMS = 0
			if err := enforceQueryBudget(cached, t.limits.MaxResultBytes); err != nil {
				return nil, err
			}
			encodedResult, _ := json.Marshal(cached)
			t.record(ctx, audit.Event{QueryID: cached.QueryID, Target: t.config.Name, Operation: "query_select", Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "allowed", Rows: cached.RowCount, Truncated: cached.Truncated, ResponseBytes: len(encodedResult), CacheStatus: "hit"})
			return cached, nil
		}
	}
	result, err := t.execute(ctx, "query_select", request.SQL, request, validation)
	if err == nil {
		if cacheKey != "" {
			t.count("cache", 1, "query_select", "miss")
			result.CacheStatus = "miss"
		} else if t.config.ResultCache.Enabled {
			result.CacheStatus = "bypass"
		}
		if budgetErr := enforceQueryBudget(result, t.limits.MaxResultBytes); budgetErr != nil {
			return nil, budgetErr
		}
		if cacheKey != "" {
			t.resultCache.put(cacheKey, result)
		}
	}
	return result, err
}

func (t *Target) validateRequest(request core.QueryRequest) error {
	if request.Timeout < 0 || request.Timeout > t.limits.MaxTimeout {
		return fmt.Errorf("requested timeout exceeds configured maximum")
	}
	if request.MaxRows < 0 || request.MaxRows > t.limits.MaxRows {
		return fmt.Errorf("requested row limit exceeds configured maximum")
	}
	if len(request.Parameters) > t.limits.MaxParameters {
		return fmt.Errorf("query has too many parameters")
	}
	return validateParameters(request.Parameters, t.limits.MaxParameterBytes, t.limits.MaxParameterValueBytes)
}

func (t *Target) Explain(ctx context.Context, request core.QueryRequest) (*core.QueryResult, error) {
	validation, err := t.policy.Validate(request.SQL)
	if err != nil {
		t.record(ctx, audit.Event{Target: t.config.Name, Operation: "query_explain", Decision: "rejected", Reason: err.Error()})
		return nil, err
	}
	return t.execute(ctx, "query_explain", "EXPLAIN FORMAT=JSON "+request.SQL, request, validation)
}

func (t *Target) execute(ctx context.Context, operation, query string, request core.QueryRequest, validation *core.Validation) (*core.QueryResult, error) {
	queryID := uuid.NewString()
	started := time.Now()
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout > t.limits.MaxTimeout {
		return nil, fmt.Errorf("requested timeout exceeds configured maximum")
	}
	maxRows := request.MaxRows
	if maxRows <= 0 {
		maxRows = t.limits.MaxRows
	}
	if maxRows > t.limits.MaxRows {
		return nil, fmt.Errorf("requested row limit exceeds configured maximum")
	}
	if len(request.Parameters) > t.limits.MaxParameters {
		return nil, fmt.Errorf("query has too many parameters")
	}
	if err := validateParameters(request.Parameters, t.limits.MaxParameterBytes, t.limits.MaxParameterValueBytes); err != nil {
		return nil, err
	}

	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	queueStarted := time.Now()
	permit, err := t.admission.Acquire(queryCtx, t.config.Name, admission.Interactive)
	t.observe("admission_wait", time.Since(queueStarted), operation)
	if err != nil {
		t.count("admission", 1, operation, "rejected")
		t.record(queryCtx, audit.Event{QueryID: queryID, Target: t.config.Name, Operation: operation, Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "rejected", Reason: "admission"})
		return nil, fmt.Errorf("query concurrency limit: %w", err)
	}
	defer permit.Release()
	t.gate.RLock()
	defer t.gate.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	defer t.recordDBStats()

	dbStarted := time.Now()
	tx, err := t.db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.record(queryCtx, audit.Event{QueryID: queryID, Target: t.config.Name, Operation: operation, Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "failed", Reason: "transaction_begin"})
		return nil, sanitizeDBError(err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(queryCtx, query, request.Parameters...)
	t.observe("database_to_rows", time.Since(dbStarted), operation)
	if err != nil {
		t.record(queryCtx, audit.Event{QueryID: queryID, Target: t.config.Name, Operation: operation, Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "failed", Reason: errorClass(err), Duration: time.Since(started)})
		return nil, sanitizeDBError(err)
	}
	defer rows.Close()

	collectStarted := time.Now()
	result, err := t.collectRows(rows, maxRows, t.limits.MaxResultBytes)
	t.observe("collect", time.Since(collectStarted), operation)
	if err != nil {
		t.record(queryCtx, audit.Event{QueryID: queryID, Target: t.config.Name, Operation: operation, Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "failed", Reason: "result_collection"})
		return nil, err
	}
	result.QueryID = queryID
	result.Target = t.config.Name
	result.Engine = t.config.Engine
	result.Environment = t.config.Environment
	result.Consistency = t.config.Consistency
	result.Database = t.config.Database
	result.DurationMS = time.Since(started).Milliseconds()
	if err := enforceQueryBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	encodedResult, _ := json.Marshal(result)
	t.record(queryCtx, audit.Event{QueryID: queryID, Target: t.config.Name, Operation: operation, Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "allowed", Rows: result.RowCount, Truncated: result.Truncated, Duration: time.Since(started), ResponseBytes: len(encodedResult), CacheStatus: result.CacheStatus})
	t.observe("total", time.Since(started), operation)
	t.count("requests", 1, operation, "allowed")
	return result, nil
}

func (t *Target) collectRows(rows *sql.Rows, maxRows, byteBudget int) (*core.QueryResult, error) {
	columnNames, err := rows.Columns()
	if err != nil {
		return nil, errors.New("read result columns")
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, errors.New("read result column types")
	}
	columnNames = uniqueNames(columnNames)
	if len(columnNames) > 1024 {
		return nil, errors.New("query result has too many columns")
	}
	columns := make([]core.Column, len(columnNames))
	for i, name := range columnNames {
		nullable, _ := columnTypes[i].Nullable()
		columns[i] = core.Column{Name: name, DatabaseType: columnTypes[i].DatabaseTypeName(), Nullable: nullable}
	}

	result := &core.QueryResult{Columns: columns, Rows: make([]map[string]any, 0, min(maxRows, 32))}
	resultBytes := 0
	for rows.Next() {
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			break
		}
		values := make([]any, len(columnNames))
		destinations := make([]any, len(columnNames))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, errors.New("scan query result")
		}
		row := make(map[string]any, len(columnNames))
		for i, value := range values {
			normalized, truncated := normalizeValue(value, t.limits.MaxCellBytes)
			if truncated {
				result.TruncatedCells++
			}
			row[columnNames[i]] = normalized
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			return nil, errors.New("encode query result")
		}
		if resultBytes+len(encoded) > byteBudget {
			result.Truncated = true
			break
		}
		resultBytes += len(encoded)
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, sanitizeDBError(err)
	}
	result.RowCount = len(result.Rows)
	return result, nil
}

func enforceQueryBudget(result *core.QueryResult, maxBytes int) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return errors.New("encode query result")
	}
	if len(encoded) <= maxBytes {
		return nil
	}
	all := result.Rows
	low, high := 0, len(all)
	for low < high {
		mid := (low + high + 1) / 2
		result.Rows = all[:mid]
		result.RowCount = mid
		candidate, e := json.Marshal(result)
		if e != nil {
			return errors.New("encode query result")
		}
		if len(candidate) <= maxBytes {
			low = mid
		} else {
			high = mid - 1
		}
	}
	result.Rows = all[:low]
	result.RowCount = low
	result.Truncated = true
	encoded, err = json.Marshal(result)
	if err != nil {
		return errors.New("encode query result")
	}
	if len(encoded) > maxBytes {
		return errors.New("query result metadata exceeds configured result-byte limit")
	}
	return nil
}

func (t *Target) ListTables(ctx context.Context, pattern string, fresh bool) ([]core.TableSummary, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if len(pattern) > 128 {
		return nil, fmt.Errorf("table pattern is too long")
	}
	key := t.policyRevision + "\x00list\x00" + pattern
	if fresh && !t.config.MetadataCache.IsFreshAllowed() {
		return nil, fmt.Errorf("forced metadata refresh is disabled for this target")
	}
	if !fresh {
		if cached, ok := t.metadataCache.get(key); ok {
			t.count("cache", 1, "schema_list_tables", "hit")
			return cached.([]core.TableSummary), nil
		}
	}
	var leader bool
	var err error
	if fresh {
		leader, err = t.metadataCache.leadRefresh(ctx, key, t.config.MetadataCache.FreshCooldown)
		t.count("cache", 1, "schema_list_tables", "refresh")
	} else {
		leader, err = t.metadataCache.lead(ctx, key)
		t.count("cache", 1, "schema_list_tables", "miss")
	}
	if err != nil {
		if fresh {
			t.record(ctx, audit.Event{Target: t.config.Name, Operation: "schema_list_tables", Decision: "rejected", Reason: "refresh_control"})
		}
		return nil, err
	}
	if !leader {
		if cached, ok := t.metadataCache.get(key); ok {
			return cached.([]core.TableSummary), nil
		}
		if fresh {
			return nil, fmt.Errorf("forced metadata refresh completed without a cacheable result")
		}
		return t.ListTables(ctx, pattern, false)
	}
	cacheDone := false
	defer func() {
		if !cacheDone {
			t.metadataCache.finish(key, nil, 0, false)
		}
	}()
	query := "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA IN (" + t.metadataPlaceholders + ")"
	args := make([]any, 0, len(t.config.AllowedSchemas)+1)
	for _, schema := range t.config.AllowedSchemas {
		args = append(args, schema)
	}
	if pattern != "" {
		query += " AND TABLE_NAME LIKE ? ESCAPE '\\\\'"
		args = append(args, pattern)
	}
	query += " ORDER BY TABLE_SCHEMA, TABLE_NAME LIMIT 2000"
	queryCtx, cancel := context.WithTimeout(ctx, t.limits.DefaultTimeout)
	defer cancel()
	permit, err := t.admission.Acquire(queryCtx, t.config.Name, admission.Metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata concurrency limit: %w", err)
	}
	defer permit.Release()
	t.gate.RLock()
	defer t.gate.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	defer t.recordDBStats()
	rows, err := t.db.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, sanitizeDBError(err)
	}
	defer rows.Close()
	var tables []core.TableSummary
	for rows.Next() {
		var table core.TableSummary
		if err := rows.Scan(&table.Schema, &table.Name, &table.Type); err != nil {
			return nil, errors.New("scan table metadata")
		}
		qualified := strings.ToLower(table.Schema + "." + table.Name)
		if _, blocked := t.deniedTables[strings.ToLower(table.Name)]; blocked {
			continue
		}
		if _, blocked := t.deniedTables[qualified]; blocked {
			continue
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, sanitizeDBError(err)
	}
	if err := enforceMetadataBudget(tables, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	t.metadataCache.finish(key, tables, t.config.MetadataCache.TableListTTL, true)
	cacheDone = true
	if fresh {
		t.record(ctx, audit.Event{Target: t.config.Name, Operation: "schema_list_tables", Decision: "allowed", Reason: "refresh", Rows: len(tables)})
	}
	return tables, nil
}

func (t *Target) DescribeTable(ctx context.Context, schema, table string, fresh bool) (*core.TableDescription, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if schema == "" {
		schema = t.config.Database
	}
	if !safeIdentifier(schema) || !safeIdentifier(table) {
		return nil, fmt.Errorf("schema and table must be plain identifiers")
	}
	if _, ok := t.allowedSchemas[strings.ToLower(schema)]; !ok {
		return nil, fmt.Errorf("schema is outside the selected target")
	}
	if _, ok := t.deniedTables[strings.ToLower(table)]; ok {
		return nil, fmt.Errorf("table is denied by target policy")
	}
	if _, ok := t.deniedTables[strings.ToLower(schema+"."+table)]; ok {
		return nil, fmt.Errorf("table is denied by target policy")
	}

	// Preserve identifier case in the cache key. MySQL table-name case
	// sensitivity depends on server configuration and filesystem semantics.
	key := t.policyRevision + "\x00describe\x00" + schema + "\x00" + table
	if fresh && !t.config.MetadataCache.IsFreshAllowed() {
		return nil, fmt.Errorf("forced metadata refresh is disabled for this target")
	}
	if !fresh {
		if cached, ok := t.metadataCache.get(key); ok {
			t.count("cache", 1, "schema_describe_table", "hit")
			if _, missing := cached.(missingDescription); missing {
				return nil, fmt.Errorf("table is not visible to this target")
			}
			return cached.(*core.TableDescription), nil
		}
	}
	var leader bool
	var err error
	if fresh {
		leader, err = t.metadataCache.leadRefresh(ctx, key, t.config.MetadataCache.FreshCooldown)
		t.count("cache", 1, "schema_describe_table", "refresh")
	} else {
		leader, err = t.metadataCache.lead(ctx, key)
		t.count("cache", 1, "schema_describe_table", "miss")
	}
	if err != nil {
		if fresh {
			t.record(ctx, audit.Event{Target: t.config.Name, Operation: "schema_describe_table", Decision: "rejected", Reason: "refresh_control"})
		}
		return nil, err
	}
	if !leader {
		if cached, ok := t.metadataCache.get(key); ok {
			if _, missing := cached.(missingDescription); missing {
				return nil, fmt.Errorf("table is not visible to this target")
			}
			return cached.(*core.TableDescription), nil
		}
		if fresh {
			return nil, fmt.Errorf("forced metadata refresh completed without a cacheable result")
		}
		return t.DescribeTable(ctx, schema, table, false)
	}
	cacheDone := false
	defer func() {
		if !cacheDone {
			t.metadataCache.finish(key, nil, 0, false)
		}
	}()
	queryCtx, cancel := context.WithTimeout(ctx, t.limits.DefaultTimeout)
	defer cancel()
	permit, err := t.admission.Acquire(queryCtx, t.config.Name, admission.Metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata concurrency limit: %w", err)
	}
	defer permit.Release()
	t.gate.RLock()
	defer t.gate.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	defer t.recordDBStats()
	columns, err := t.describeColumns(queryCtx, schema, table)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		t.metadataCache.finish(key, missingDescription{}, t.config.MetadataCache.NegativeTTL, true)
		cacheDone = true
		return nil, fmt.Errorf("table is not visible to this target")
	}
	indexes, err := t.describeIndexes(queryCtx, schema, table)
	if err != nil {
		return nil, err
	}
	result := &core.TableDescription{Target: t.config.Name, Schema: schema, Table: table, Columns: columns, Indexes: indexes}
	if err := enforceMetadataBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	t.metadataCache.finish(key, result, t.config.MetadataCache.TableDescriptionTTL, true)
	cacheDone = true
	if fresh {
		t.record(ctx, audit.Event{Target: t.config.Name, Operation: "schema_describe_table", Decision: "allowed", Reason: "refresh"})
	}
	return result, nil
}

func enforceMetadataBudget(value any, maximum int) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return errors.New("encode metadata result")
	}
	if len(encoded) > maximum {
		return errors.New("metadata result exceeds configured result-byte limit")
	}
	return nil
}

func (t *Target) describeColumns(ctx context.Context, schema, table string) ([]core.ColumnDescription, error) {
	rows, err := t.db.QueryContext(ctx, `
SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT,
       COLUMN_KEY, EXTRA, COLUMN_COMMENT
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION`, schema, table)
	if err != nil {
		return nil, sanitizeDBError(err)
	}
	defer rows.Close()
	var columns []core.ColumnDescription
	for rows.Next() {
		var column core.ColumnDescription
		var nullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&column.Name, &column.DataType, &column.ColumnType, &nullable, &defaultValue, &column.Key, &column.Extra, &column.Comment); err != nil {
			return nil, errors.New("scan column metadata")
		}
		column.Nullable = nullable == "YES"
		if defaultValue.Valid {
			column.Default = &defaultValue.String
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, sanitizeDBError(err)
	}
	return columns, nil
}

func (t *Target) describeIndexes(ctx context.Context, schema, table string) ([]core.IndexDescription, error) {
	rows, err := t.db.QueryContext(ctx, `
SELECT INDEX_NAME, NON_UNIQUE, COLUMN_NAME
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY INDEX_NAME, SEQ_IN_INDEX`, schema, table)
	if err != nil {
		return nil, sanitizeDBError(err)
	}
	defer rows.Close()
	byName := make(map[string]*core.IndexDescription)
	var order []string
	for rows.Next() {
		var name, column string
		var nonUnique int
		if err := rows.Scan(&name, &nonUnique, &column); err != nil {
			return nil, errors.New("scan index metadata")
		}
		index, ok := byName[name]
		if !ok {
			index = &core.IndexDescription{Name: name, Unique: nonUnique == 0}
			byName[name] = index
			order = append(order, name)
		}
		index.Columns = append(index.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, sanitizeDBError(err)
	}
	sort.Strings(order)
	indexes := make([]core.IndexDescription, 0, len(order))
	for _, name := range order {
		indexes = append(indexes, *byName[name])
	}
	return indexes, nil
}

func (t *Target) record(ctx context.Context, event audit.Event) {
	if t.auditor != nil {
		t.auditor.Record(ctx, event)
	}
}

func (t *Target) observe(name string, d time.Duration, operation string) {
	if t.metrics != nil {
		t.metrics.Observe(name, d, operation)
	}
}

func (t *Target) count(name string, delta int64, operation, outcome string) {
	if t.metrics != nil {
		t.metrics.Add(name, delta, operation, outcome)
	}
}

func (t *Target) recordDBStats() {
	if t.metrics == nil {
		return
	}
	s := t.db.Stats()
	t.metrics.Set("db_pool_open", int64(s.OpenConnections), "pool")
	t.metrics.Set("db_pool_in_use", int64(s.InUse), "pool")
	t.metrics.Set("db_pool_idle", int64(s.Idle), "pool")
	t.metrics.Set("db_pool_wait_count", s.WaitCount, "pool")
	t.metrics.Set("db_pool_wait_ms", s.WaitDuration.Milliseconds(), "pool")
	t.metrics.Set("grant_verification_age_seconds", int64(time.Since(time.Unix(0, t.verifiedAt.Load())).Seconds()), "health")
}

func targetPolicyRevision(cfg *config.TargetConfig) string {
	material := strings.Join([]string{cfg.Engine, cfg.Database, strings.Join(cfg.AllowedSchemas, "\x00"), strings.Join(cfg.DeniedTables, "\x00")}, "\x01")
	sum := sha256.Sum256([]byte(strings.ToLower(material)))
	return hex.EncodeToString(sum[:12])
}

func (t *Target) Close() error {
	if t.maintenanceStop != nil {
		t.maintenanceStop()
		t.maintenanceWG.Wait()
	}
	t.metadataCache.clear()
	t.resultCache.clear()
	err := t.db.Close()
	if t.tlsName != "" {
		driver.DeregisterTLSConfig(t.tlsName)
	}
	return err
}

func (t *Target) requireHealthy() error {
	if !t.healthy.Load() {
		return errors.New("MySQL target privilege attestation is unhealthy or stale")
	}
	if interval := t.config.MySQL.PrivilegeRecheck; interval > 0 {
		verifiedAt := t.verifiedAt.Load()
		if verifiedAt == 0 || time.Since(time.Unix(0, verifiedAt)) > 2*interval {
			return errors.New("MySQL target privilege attestation is unhealthy or stale")
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
		ticker := time.NewTicker(t.config.MySQL.PrivilegeRecheck)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkCtx, stop := context.WithTimeout(ctx, t.config.Connection.ConnectTimeout)
				permit, err := t.admission.Acquire(checkCtx, t.config.Name, admission.Maintenance)
				if err == nil {
					t.gate.Lock()
					_, grants, inspectErr := inspectIdentity(checkCtx, t.db)
					err = inspectErr
					if err == nil {
						err = ValidateGrants(grants, t.config.AllowedSchemas)
					}
					if err == nil {
						t.verifiedAt.Store(time.Now().UnixNano())
						t.healthy.Store(true)
					} else {
						t.healthy.Store(false)
					}
					t.gate.Unlock()
					permit.Release()
				} else {
					t.healthy.Store(false)
				}
				stop()
			}
		}
	}()
}

func validateParameters(parameters []any, limits ...int) error {
	total, maxValue := 0, 0
	if len(limits) > 0 {
		total = limits[0]
	}
	if len(limits) > 1 {
		maxValue = limits[1]
	}
	used := 0
	for _, value := range parameters {
		size := 0
		switch value.(type) {
		case nil, bool, float64:
			size = 8
		case string:
			size = len(value.(string))
		default:
			return fmt.Errorf("parameters must be JSON scalars; encode large integers as strings")
		}
		if maxValue > 0 && size > maxValue {
			return fmt.Errorf("SQL parameter exceeds the per-value byte limit")
		}
		used += size
		if total > 0 && used > total {
			return fmt.Errorf("SQL parameters exceed the total byte limit")
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

func uniqueNames(names []string) []string {
	seen := make(map[string]int, len(names))
	result := make([]string, len(names))
	for i, name := range names {
		seen[name]++
		result[i] = name
		if seen[name] > 1 {
			result[i] = fmt.Sprintf("%s#%d", name, seen[name])
		}
	}
	return result
}

func normalizeValue(value any, maxBytes int) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano), false
	case []byte:
		truncated := len(typed) > maxBytes
		if truncated {
			typed = typed[:maxBytes]
		}
		if utf8.Valid(typed) {
			return string(typed), truncated
		}
		return "base64:" + base64.StdEncoding.EncodeToString(typed), truncated
	case string:
		if len(typed) <= maxBytes {
			return typed, false
		}
		cut := maxBytes
		for cut > 0 && !utf8.RuneStart(typed[cut]) {
			cut--
		}
		return typed[:cut], true
	case int64:
		if typed > 1<<53-1 || typed < -(1<<53-1) {
			return strconv.FormatInt(typed, 10), false
		}
		return typed, false
	case uint64:
		if typed > 1<<53-1 {
			return strconv.FormatUint(typed, 10), false
		}
		return typed, false
	default:
		return typed, false
	}
}

func safeIdentifier(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '$' {
			continue
		}
		return false
	}
	return true
}

func sanitizeDBError(err error) error {
	var mysqlErr *driver.MySQLError
	if errors.As(err, &mysqlErr) {
		return fmt.Errorf("database rejected query (MySQL error %d)", mysqlErr.Number)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("query timed out")
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("query was canceled")
	}
	return errors.New("database query failed")
}

func errorClass(err error) string {
	var mysqlErr *driver.MySQLError
	if errors.As(err, &mysqlErr) {
		return "mysql_" + strconv.FormatUint(uint64(mysqlErr.Number), 10)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "database_error"
}
