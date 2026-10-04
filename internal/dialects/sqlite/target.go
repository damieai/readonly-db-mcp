package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mattn/go-sqlite3"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
)

type Target struct {
	cfg        *config.TargetConfig
	limits     config.Limits
	db         *sql.DB
	path       string
	fileInfo   os.FileInfo
	version    string
	controller *admission.Controller
	auditor    audit.Auditor
	recorder   metrics.Recorder
	closed     atomic.Bool
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	path, err := cfg.SQLitePath()
	if err != nil {
		return nil, err
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	var nameBytes [12]byte
	if _, err := rand.Read(nameBytes[:]); err != nil {
		return nil, err
	}
	driverName := "readonly-sqlite-" + hex.EncodeToString(nameBytes[:])
	driver := &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
		current, err := os.Stat(path)
		if err != nil || !os.SameFile(fileInfo, current) {
			return errors.New("SQLite file identity changed during open")
		}
		if _, err := conn.Exec("PRAGMA trusted_schema=OFF", nil); err != nil {
			return err
		}
		rows, err := conn.Query("PRAGMA query_only", nil)
		if err != nil {
			return err
		}
		values := make([]driver.Value, 1)
		if err := rows.Next(values); err != nil || values[0] != int64(1) {
			rows.Close()
			return errors.New("SQLite query_only proof failed")
		}
		rows.Close()
		conn.SetLimit(sqlite3.SQLITE_LIMIT_SQL_LENGTH, limits.MaxSQLBytes+64)
		conn.SetLimit(sqlite3.SQLITE_LIMIT_LENGTH, limits.MaxParameterValueBytes)
		conn.SetLimit(sqlite3.SQLITE_LIMIT_VARIABLE_NUMBER, limits.MaxParameters)
		conn.SetLimit(sqlite3.SQLITE_LIMIT_ATTACHED, 0)
		conn.RegisterAuthorizer(authorize)
		return nil
	}}
	sql.Register(driverName, driver)
	dsn := (&url.URL{Scheme: "file", Path: path}).String() + "?mode=ro&cache=private&_query_only=1"
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.Connection.MaxOpen)
	db.SetMaxIdleConns(cfg.Connection.MaxIdle)
	db.SetConnMaxLifetime(cfg.Connection.MaxLifetime)
	db.SetConnMaxIdleTime(cfg.Connection.MaxIdleTime)
	startCtx, cancel := context.WithTimeout(ctx, cfg.Connection.ConnectTimeout)
	defer cancel()
	if err := db.PingContext(startCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("SQLite target %q cannot open read-only file: %w", cfg.Name, err)
	}
	var version string
	if err := db.QueryRowContext(startCtx, "SELECT sqlite_version()").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	return &Target{cfg: cfg, limits: limits, db: db, path: path, fileInfo: fileInfo, version: version, controller: controller, auditor: auditor, recorder: recorder}, nil
}

func (t *Target) ready() error {
	if t.closed.Load() {
		return errors.New("target_closed: SQLite target is closed")
	}
	path, err := t.cfg.SQLitePath()
	if err != nil || path != t.path {
		return errors.New("permission_denied: SQLite file moved outside configured scope")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(t.fileInfo, info) {
		return errors.New("permission_denied: SQLite file is no longer regular")
	}
	return nil
}

func (t *Target) Info() core.TargetInfo {
	return core.TargetInfo{Name: t.cfg.Name, Engine: config.EngineSQLite, Environment: t.cfg.Environment, Consistency: t.cfg.Consistency, Database: "main", Schemas: []string{"main"}, Healthy: t.ready() == nil, ReadOnlyUser: false, ServerReadOnly: true, ParameterStyle: "?", ServerVersion: t.version, Capabilities: map[string]string{"sql": "select,cte,window,json,fts", "metadata": "tables,columns,indexes", "file_scope": "operator_configured_existing_file"}}
}

func (t *Target) Close() error { t.closed.Store(true); return t.db.Close() }

func (t *Target) ValidateQuery(query string) (*core.Validation, error) {
	return validateSQL(query, t.limits.MaxSQLBytes)
}

func (t *Target) Query(ctx context.Context, request core.QueryRequest) (result *core.QueryResult, err error) {
	return t.run(ctx, request, false)
}

func (t *Target) Explain(ctx context.Context, request core.QueryRequest) (*core.QueryResult, error) {
	return t.run(ctx, request, true)
}

func (t *Target) run(ctx context.Context, request core.QueryRequest, explain bool) (result *core.QueryResult, err error) {
	started := time.Now()
	var id [12]byte
	if _, err = rand.Read(id[:]); err != nil {
		return nil, err
	}
	queryID := hex.EncodeToString(id[:])
	fingerprint := ""
	operation := "sqlite_query"
	if explain {
		operation = "sqlite_explain"
	}
	defer func() {
		decision, reason, rows, size, truncated := "allowed", "", 0, 0, false
		if err != nil {
			decision, reason = "denied", err.Error()
		}
		if result != nil {
			rows, truncated = result.RowCount, result.Truncated
			encoded, _ := json.Marshal(result)
			size = len(encoded)
		}
		if t.auditor != nil {
			t.auditor.Record(ctx, audit.Event{QueryID: queryID, Target: t.cfg.Name, Operation: operation, Fingerprint: fingerprint, Decision: decision, Reason: reason, Rows: rows, ResponseBytes: size, Truncated: truncated, Duration: time.Since(started)})
		}
		if t.recorder != nil {
			t.recorder.Add("query", 1, operation, decision)
			t.recorder.Observe("query_duration", time.Since(started), operation)
		}
	}()
	validation, err := t.ValidateQuery(request.SQL)
	if err != nil {
		return nil, err
	}
	fingerprint = validation.Fingerprint
	if request.PostgreSQLOptions != nil {
		return nil, errors.New("invalid_request: PostgreSQL options are unavailable for SQLite")
	}
	if len(request.Parameters) > t.limits.MaxParameters {
		return nil, errors.New("resource_limit: too many SQLite parameters")
	}
	parameterBytes, e := json.Marshal(request.Parameters)
	if e != nil || len(parameterBytes) > t.limits.MaxParameterBytes {
		return nil, errors.New("resource_limit: SQLite parameters exceed configured size")
	}
	for _, param := range request.Parameters {
		b, e := json.Marshal(param)
		if e != nil || len(b) > t.limits.MaxParameterValueBytes {
			return nil, errors.New("resource_limit: SQLite parameter exceeds cell limit")
		}
	}
	maxRows := request.MaxRows
	if maxRows == 0 {
		maxRows = t.limits.MaxRows
	}
	if maxRows < 1 || maxRows > t.limits.MaxRows {
		return nil, errors.New("resource_limit: SQLite row cap exceeds configured maximum")
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, errors.New("resource_limit: SQLite timeout exceeds configured maximum")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := t.ready(); err != nil {
		return nil, err
	}
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Interactive)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	query := request.SQL
	if explain {
		query = "EXPLAIN QUERY PLAN " + query
	}
	rows, err := t.db.QueryContext(ctx, query, request.Parameters...)
	if err != nil {
		return nil, fmt.Errorf("SQLite query failed: %w", err)
	}
	defer rows.Close()
	result, err = t.collect(ctx, rows, maxRows, queryID, started)
	return result, err
}

func (t *Target) collect(ctx context.Context, rows *sql.Rows, maxRows int, queryID string, started time.Time) (*core.QueryResult, error) {
	names, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	columns := make([]core.Column, len(names))
	for i, name := range names {
		original := name
		for suffix := 2; used[name]; suffix++ {
			name = original + "_" + strconv.Itoa(suffix)
		}
		used[name] = true
		columns[i] = core.Column{Name: name, DatabaseType: types[i].DatabaseTypeName()}
	}
	result := &core.QueryResult{QueryID: queryID, Target: t.cfg.Name, Engine: config.EngineSQLite, Environment: t.cfg.Environment, Consistency: t.cfg.Consistency, Database: "main", Columns: columns, Rows: []map[string]any{}}
	usedBytes := 256 + len(columns)*64
	for rows.Next() {
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			break
		}
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(columns))
		for i, v := range values {
			var value any = v
			switch x := v.(type) {
			case []byte:
				if len(x) > t.limits.MaxCellBytes {
					return nil, errors.New("resource_limit: SQLite cell exceeds max_cell_bytes")
				}
				value = base64.StdEncoding.EncodeToString(x)
			case string:
				if len(x) > t.limits.MaxCellBytes {
					return nil, errors.New("resource_limit: SQLite cell exceeds max_cell_bytes")
				}
			case int64:
				if x > 9007199254740991 || x < -9007199254740991 {
					value = strconv.FormatInt(x, 10)
				}
			}
			row[columns[i].Name] = value
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		usedBytes += len(encoded)
		if usedBytes > t.limits.MaxResultBytes {
			return nil, errors.New("resource_limit: SQLite result exceeds max_result_bytes")
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result.RowCount = len(result.Rows)
	result.DurationMS = time.Since(started).Milliseconds()
	final, err := json.Marshal(result)
	if err != nil || len(final) > t.limits.MaxResultBytes {
		return nil, errors.New("resource_limit: SQLite result exceeds max_result_bytes")
	}
	return result, nil
}

func quote(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func (t *Target) ListTables(ctx context.Context, pattern string, _ bool) ([]core.TableSummary, error) {
	if err := t.ready(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, t.limits.DefaultTimeout)
	defer cancel()
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Metadata)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	if len(pattern) > 256 {
		return nil, errors.New("resource_limit: SQLite pattern too long")
	}
	rows, err := t.db.QueryContext(ctx, "SELECT name,type FROM main.sqlite_schema WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' AND name LIKE ? ORDER BY name LIMIT ?", defaultPattern(pattern), t.limits.MaxRows+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []core.TableSummary{}
	for rows.Next() {
		if len(result) >= t.limits.MaxRows {
			return nil, errors.New("resource_limit: SQLite metadata exceeds row cap")
		}
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			return nil, err
		}
		result = append(result, core.TableSummary{Schema: "main", Name: name, Type: kind})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > t.limits.MaxResultBytes {
		return nil, errors.New("resource_limit: SQLite metadata exceeds max_result_bytes")
	}
	return result, nil
}

func defaultPattern(pattern string) string {
	if pattern == "" {
		return "%"
	}
	return pattern
}

func (t *Target) DescribeTable(ctx context.Context, schema, table string, _ bool) (*core.TableDescription, error) {
	if schema != "" && schema != "main" {
		return nil, errors.New("permission_denied: only main schema is available")
	}
	if table == "" || len(table) > 256 || strings.IndexByte(table, 0) >= 0 {
		return nil, errors.New("invalid_request: SQLite table name is invalid")
	}
	if err := t.ready(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, t.limits.DefaultTimeout)
	defer cancel()
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Metadata)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	var kind string
	if err := t.db.QueryRowContext(ctx, "SELECT type FROM main.sqlite_schema WHERE name=? AND type IN ('table','view') AND name NOT LIKE 'sqlite_%'", table).Scan(&kind); err != nil {
		return nil, errors.New("not_found: SQLite table or view is unavailable")
	}
	desc := &core.TableDescription{Target: t.cfg.Name, Schema: "main", Table: table}
	rows, err := t.db.QueryContext(ctx, "PRAGMA main.table_xinfo("+quote(table)+")")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var cid, notNull, pk, hidden int
		var name, dataType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk, &hidden); err != nil {
			rows.Close()
			return nil, err
		}
		if hidden == 1 {
			continue
		}
		var def *string
		if defaultValue.Valid {
			def = &defaultValue.String
		}
		key := ""
		if pk > 0 {
			key = "PRI"
		}
		desc.Columns = append(desc.Columns, core.ColumnDescription{Name: name, DataType: dataType, ColumnType: dataType, Nullable: notNull == 0, Default: def, Key: key})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if kind == "view" {
		return t.boundedDescription(desc)
	}
	indexRows, err := t.db.QueryContext(ctx, "PRAGMA main.index_list("+quote(table)+")")
	if err != nil {
		return nil, err
	}
	for indexRows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := indexRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			indexRows.Close()
			return nil, err
		}
		desc.Indexes = append(desc.Indexes, core.IndexDescription{Name: name, Unique: unique != 0, Primary: origin == "pk"})
	}
	err = indexRows.Err()
	indexRows.Close()
	if err != nil {
		return nil, err
	}
	for i := range desc.Indexes {
		idxRows, err := t.db.QueryContext(ctx, "PRAGMA main.index_xinfo("+quote(desc.Indexes[i].Name)+")")
		if err != nil {
			return nil, err
		}
		for idxRows.Next() {
			var seqno, cid, descOrder, key int
			var name, collation sql.NullString
			if err := idxRows.Scan(&seqno, &cid, &name, &descOrder, &collation, &key); err != nil {
				idxRows.Close()
				return nil, err
			}
			if key == 1 && name.Valid {
				desc.Indexes[i].Columns = append(desc.Indexes[i].Columns, name.String)
			}
		}
		err = idxRows.Err()
		idxRows.Close()
		if err != nil {
			return nil, err
		}
	}
	return t.boundedDescription(desc)
}

func (t *Target) boundedDescription(desc *core.TableDescription) (*core.TableDescription, error) {
	encoded, err := json.Marshal(desc)
	if err != nil || len(encoded) > t.limits.MaxResultBytes {
		return nil, errors.New("resource_limit: SQLite metadata exceeds max_result_bytes")
	}
	return desc, nil
}
