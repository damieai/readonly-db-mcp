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
	"time"
	"unicode/utf8"

	driver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type Target struct {
	config    *config.TargetConfig
	limits    config.Limits
	db        *sql.DB
	policy    *Policy
	auditor   audit.Auditor
	globalSem chan struct{}
	targetSem chan struct{}
	info      core.TargetInfo
	tlsName   string
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, globalSem chan struct{}, auditor audit.Auditor) (*Target, error) {
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
		globalSem: globalSem,
		targetSem: make(chan struct{}, limits.PerTargetConcurrency),
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
		},
	}
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
	info.Schemas = append([]string(nil), t.info.Schemas...)
	return info
}

func (t *Target) ValidateQuery(query string) (*core.Validation, error) {
	return t.policy.Validate(query)
}

func (t *Target) Query(ctx context.Context, request core.QueryRequest) (*core.QueryResult, error) {
	validation, err := t.policy.Validate(request.SQL)
	if err != nil {
		t.record(ctx, audit.Event{Target: t.config.Name, Operation: "query_select", Decision: "rejected", Reason: err.Error()})
		return nil, err
	}
	return t.execute(ctx, "query_select", request.SQL, request, validation)
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
	if err := validateParameters(request.Parameters); err != nil {
		return nil, err
	}

	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := t.acquire(queryCtx); err != nil {
		return nil, fmt.Errorf("query concurrency limit: %w", err)
	}
	defer t.release()

	tx, err := t.db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, sanitizeDBError(err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(queryCtx, query, request.Parameters...)
	if err != nil {
		t.record(queryCtx, audit.Event{QueryID: queryID, Target: t.config.Name, Operation: operation, Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "failed", Reason: errorClass(err), Duration: time.Since(started)})
		return nil, sanitizeDBError(err)
	}
	defer rows.Close()

	result, err := t.collectRows(rows, maxRows)
	if err != nil {
		return nil, err
	}
	result.QueryID = queryID
	result.Target = t.config.Name
	result.Engine = t.config.Engine
	result.Environment = t.config.Environment
	result.Consistency = t.config.Consistency
	result.Database = t.config.Database
	result.DurationMS = time.Since(started).Milliseconds()
	t.record(queryCtx, audit.Event{QueryID: queryID, Target: t.config.Name, Operation: operation, Fingerprint: validation.Fingerprint, Tables: validation.Tables, Decision: "allowed", Rows: result.RowCount, Truncated: result.Truncated, Duration: time.Since(started)})
	return result, nil
}

func (t *Target) collectRows(rows *sql.Rows, maxRows int) (*core.QueryResult, error) {
	columnNames, err := rows.Columns()
	if err != nil {
		return nil, errors.New("read result columns")
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, errors.New("read result column types")
	}
	columnNames = uniqueNames(columnNames)
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
		if resultBytes+len(encoded) > t.limits.MaxResultBytes {
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

func (t *Target) ListTables(ctx context.Context, pattern string) ([]core.TableSummary, error) {
	if len(pattern) > 128 {
		return nil, fmt.Errorf("table pattern is too long")
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(t.config.AllowedSchemas)), ",")
	query := "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA IN (" + placeholders + ")"
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
	rows, err := t.db.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, sanitizeDBError(err)
	}
	defer rows.Close()
	var tables []core.TableSummary
	denied := lowerSet(t.config.DeniedTables)
	for rows.Next() {
		var table core.TableSummary
		if err := rows.Scan(&table.Schema, &table.Name, &table.Type); err != nil {
			return nil, errors.New("scan table metadata")
		}
		qualified := strings.ToLower(table.Schema + "." + table.Name)
		if _, blocked := denied[strings.ToLower(table.Name)]; blocked {
			continue
		}
		if _, blocked := denied[qualified]; blocked {
			continue
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, sanitizeDBError(err)
	}
	return tables, nil
}

func (t *Target) DescribeTable(ctx context.Context, schema, table string) (*core.TableDescription, error) {
	if schema == "" {
		schema = t.config.Database
	}
	if !safeIdentifier(schema) || !safeIdentifier(table) {
		return nil, fmt.Errorf("schema and table must be plain identifiers")
	}
	if _, ok := lowerSet(t.config.AllowedSchemas)[strings.ToLower(schema)]; !ok {
		return nil, fmt.Errorf("schema is outside the selected target")
	}
	denied := lowerSet(t.config.DeniedTables)
	if _, ok := denied[strings.ToLower(table)]; ok {
		return nil, fmt.Errorf("table is denied by target policy")
	}
	if _, ok := denied[strings.ToLower(schema+"."+table)]; ok {
		return nil, fmt.Errorf("table is denied by target policy")
	}

	queryCtx, cancel := context.WithTimeout(ctx, t.limits.DefaultTimeout)
	defer cancel()
	columns, err := t.describeColumns(queryCtx, schema, table)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table is not visible to this target")
	}
	indexes, err := t.describeIndexes(queryCtx, schema, table)
	if err != nil {
		return nil, err
	}
	return &core.TableDescription{Target: t.config.Name, Schema: schema, Table: table, Columns: columns, Indexes: indexes}, nil
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

func (t *Target) acquire(ctx context.Context) error {
	select {
	case t.globalSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case t.targetSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		<-t.globalSem
		return ctx.Err()
	}
}

func (t *Target) release() {
	<-t.targetSem
	<-t.globalSem
}

func (t *Target) record(ctx context.Context, event audit.Event) {
	if t.auditor != nil {
		t.auditor.Record(ctx, event)
	}
}

func (t *Target) Close() error {
	err := t.db.Close()
	if t.tlsName != "" {
		driver.DeregisterTLSConfig(t.tlsName)
	}
	return err
}

func validateParameters(parameters []any) error {
	for _, value := range parameters {
		switch value.(type) {
		case nil, bool, float64, string:
		default:
			return fmt.Errorf("parameters must be JSON scalars; encode large integers as strings")
		}
	}
	return nil
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
