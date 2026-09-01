package postgresql

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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
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
	policy          *Policy
	admission       *admission.Controller
	auditor         audit.Auditor
	metrics         metrics.Recorder
	info            core.TargetInfo
	allowed, denied map[string]struct{}
	cache           *metadataCache
	policyRevision  string
	healthy         atomic.Bool
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
			db.Close()
		}
	}()
	pingCtx, cancel := context.WithTimeout(ctx, cfg.Connection.ConnectTimeout)
	defer cancel()
	if db.PingContext(pingCtx) != nil {
		return nil, fmt.Errorf("target %q is unreachable", cfg.Name)
	}
	identity, err := verifyIdentityAndPrivileges(pingCtx, db, cfg)
	if err != nil {
		return nil, fmt.Errorf("target %q startup verification failed: %w", cfg.Name, err)
	}
	t := &Target{cfg: cfg, limits: limits, db: db, policy: NewPolicy(cfg.AllowedSchemas, cfg.DeniedTables, limits.MaxSQLBytes, identity.safeFunctions), admission: controller, auditor: auditor, metrics: recorder, allowed: lowerSet(cfg.AllowedSchemas), denied: lowerSet(cfg.DeniedTables), cache: newMetadataCache(cfg.MetadataCache.IsEnabled(), cfg.MetadataCache.MaxEntries, cfg.MetadataCache.MaxBytes), policyRevision: postgresPolicyRevision(cfg), info: core.TargetInfo{Name: cfg.Name, Engine: cfg.Engine, Environment: cfg.Environment, Consistency: cfg.Consistency, Database: cfg.Database, Schemas: append([]string(nil), cfg.AllowedSchemas...), Healthy: true, ReadOnlyUser: true, ServerReadOnly: identity.readOnly, ParameterStyle: "$1", ServerVersion: identity.version}}
	t.healthy.Store(true)
	t.startPrivilegeRecheck()
	cleanup = false
	return t, nil
}

func openDB(c *config.TargetConfig, password string) (*sql.DB, error) {
	pc, err := pgx.ParseConfig("postgres://placeholder@localhost/placeholder?sslmode=disable")
	if err != nil {
		return nil, errors.New("initialize PostgreSQL configuration")
	}
	pc.Host = c.Host
	pc.Port = uint16(c.Port)
	pc.Database = c.Database
	pc.User = c.Username
	pc.Password = password
	pc.ConnectTimeout = c.Connection.ConnectTimeout
	pc.RuntimeParams = map[string]string{"application_name": c.PostgreSQL.ApplicationName, "search_path": "pg_catalog", "default_transaction_read_only": "on", "row_security": "on", "client_min_messages": "warning"}
	pc.Fallbacks = nil
	tlsCfg, err := postgresTLS(c)
	if err != nil {
		return nil, err
	}
	pc.TLSConfig = tlsCfg
	db := stdlib.OpenDB(*pc)
	db.SetMaxOpenConns(c.Connection.MaxOpen)
	db.SetMaxIdleConns(c.Connection.MaxIdle)
	db.SetConnMaxLifetime(c.Connection.MaxLifetime)
	db.SetConnMaxIdleTime(c.Connection.MaxIdleTime)
	return db, nil
}
func postgresTLS(c *config.TargetConfig) (*tls.Config, error) {
	if c.TLS.Mode == config.TLSDisabled {
		return nil, nil
	}
	tc := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: c.TLS.ServerName}
	if c.TLS.Mode == config.TLSRequired {
		tc.InsecureSkipVerify = true
	} else {
		pem, err := os.ReadFile(c.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read TLS CA file: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("TLS CA file contains no certificates")
		}
		tc.RootCAs = roots
	}
	if c.TLS.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(c.TLS.CertFile, c.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS client certificate: %w", err)
		}
		tc.Certificates = []tls.Certificate{cert}
	}
	return tc, nil
}

type identity struct {
	version       string
	readOnly      bool
	safeFunctions map[string]struct{}
}

func verifyIdentityAndPrivileges(ctx context.Context, db *sql.DB, c *config.TargetConfig) (identity, error) {
	var current, session, database, version string
	var versionNum int
	var recovery, readOnly bool
	err := db.QueryRowContext(ctx, `SELECT current_user, session_user, current_database(), current_setting('server_version'), current_setting('server_version_num')::int, pg_is_in_recovery(), current_setting('transaction_read_only')::bool`).Scan(&current, &session, &database, &version, &versionNum, &recovery, &readOnly)
	if err != nil {
		return identity{}, errors.New("inspect PostgreSQL identity")
	}
	if current != c.Username || session != c.Username || database != c.Database {
		return identity{}, errors.New("connected PostgreSQL identity does not match configuration")
	}
	major := versionNum / 10000
	if major < 15 || major > 17 {
		return identity{}, fmt.Errorf("PostgreSQL major version %d is not supported", major)
	}
	if c.PostgreSQL.RequireHotStandby && !recovery {
		return identity{}, errors.New("target is not a hot standby")
	}
	var super, createdb, createrole, repl, bypass bool
	var oid int64
	if db.QueryRowContext(ctx, `SELECT oid, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls FROM pg_catalog.pg_roles WHERE rolname=current_user`).Scan(&oid, &super, &createdb, &createrole, &repl, &bypass) != nil {
		return identity{}, errors.New("inspect PostgreSQL role attributes")
	}
	if super || createdb || createrole || repl || bypass {
		return identity{}, errors.New("PostgreSQL role has forbidden attributes")
	}
	var count int
	checks := []struct{ q, msg string }{
		{`SELECT count(*) FROM pg_catalog.pg_auth_members WHERE member=$1`, `PostgreSQL role memberships are not allowed`},
		{`SELECT count(*) FROM pg_catalog.pg_database WHERE datdba=$1`, `PostgreSQL role must not own databases`},
		{`SELECT count(*) FROM pg_catalog.pg_namespace WHERE nspowner=$1`, `PostgreSQL role must not own schemas`},
		{`SELECT count(*) FROM pg_catalog.pg_class WHERE relowner=$1`, `PostgreSQL role must not own relations`},
		{`SELECT count(*) FROM pg_catalog.pg_proc WHERE proowner=$1`, `PostgreSQL role must not own functions`},
		{`SELECT count(*) FROM pg_catalog.pg_type WHERE typowner=$1`, `PostgreSQL role must not own types`},
	}
	for _, x := range checks {
		if db.QueryRowContext(ctx, x.q, oid).Scan(&count) != nil {
			return identity{}, errors.New("inspect PostgreSQL ownership")
		}
		if count > 0 {
			return identity{}, errors.New(x.msg)
		}
	}
	var connect, create, temp bool
	if db.QueryRowContext(ctx, `SELECT has_database_privilege(current_user,current_database(),'CONNECT'),has_database_privilege(current_user,current_database(),'CREATE'),has_database_privilege(current_user,current_database(),'TEMP')`).Scan(&connect, &create, &temp) != nil {
		return identity{}, errors.New("inspect PostgreSQL database privileges")
	}
	if !connect || create || temp {
		return identity{}, errors.New("PostgreSQL database privileges are not strictly read-only")
	}
	if db.QueryRowContext(ctx, `SELECT count(*) FROM pg_catalog.pg_database WHERE datallowconn AND datname<>current_database() AND has_database_privilege(current_user,oid,'CONNECT')`).Scan(&count) != nil {
		return identity{}, errors.New("inspect cross-database PostgreSQL privileges")
	}
	if count > 0 {
		return identity{}, errors.New("PostgreSQL role can connect outside the configured database")
	}
	allowed := lowerSet(c.AllowedSchemas)
	rows, err := db.QueryContext(ctx, `SELECT nspname,has_schema_privilege(current_user,oid,'USAGE'),has_schema_privilege(current_user,oid,'CREATE') FROM pg_catalog.pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname<>'information_schema'`)
	if err != nil {
		return identity{}, errors.New("inspect PostgreSQL schema privileges")
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		var usage, create bool
		if rows.Scan(&n, &usage, &create) != nil {
			return identity{}, errors.New("read PostgreSQL schema privileges")
		}
		_, ok := allowed[strings.ToLower(n)]
		if create || (!ok && usage) || (ok && !usage) {
			return identity{}, errors.New("PostgreSQL schema privileges exceed configured scope")
		}
	}
	rows.Close()
	rows, err = db.QueryContext(ctx, `
		SELECT n.nspname,
		       c.relname,
		       c.relkind,
		       has_table_privilege(current_user,c.oid,'SELECT'),
		       has_table_privilege(current_user,c.oid,'INSERT'),
		       has_table_privilege(current_user,c.oid,'UPDATE'),
		       has_table_privilege(current_user,c.oid,'DELETE'),
		       has_table_privilege(current_user,c.oid,'TRUNCATE'),
		       has_table_privilege(current_user,c.oid,'REFERENCES'),
		       has_table_privilege(current_user,c.oid,'TRIGGER'),
		       EXISTS (
		           SELECT 1 FROM pg_catalog.pg_attribute a
		           WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
		             AND has_column_privilege(current_user,c.oid,a.attnum,'SELECT')
		       ),
		       EXISTS (
		           SELECT 1 FROM pg_catalog.pg_attribute a
		           WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
		             AND (has_column_privilege(current_user,c.oid,a.attnum,'INSERT')
		                  OR has_column_privilege(current_user,c.oid,a.attnum,'UPDATE')
		                  OR has_column_privilege(current_user,c.oid,a.attnum,'REFERENCES'))
		       ),
		       CASE WHEN c.relkind='S'
		            THEN has_sequence_privilege(current_user,c.oid,'USAGE')
		                 OR has_sequence_privilege(current_user,c.oid,'UPDATE')
		            ELSE false END
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname NOT LIKE 'pg_%'
		  AND n.nspname<>'information_schema'
		  AND c.relkind IN ('r','p','v','m','f','S')`)
	if err != nil {
		return identity{}, errors.New("inspect PostgreSQL relation privileges")
	}
	defer rows.Close()
	for rows.Next() {
		var schema, name, kind string
		var sel, ins, upd, del, trunc, refs, trig, columnSelect, columnWrite, sequenceWrite bool
		if rows.Scan(&schema, &name, &kind, &sel, &ins, &upd, &del, &trunc, &refs, &trig, &columnSelect, &columnWrite, &sequenceWrite) != nil {
			return identity{}, errors.New("read PostgreSQL relation privileges")
		}
		_, ok := allowed[strings.ToLower(schema)]
		if ins || upd || del || trunc || refs || trig || columnWrite || sequenceWrite || (!ok && (sel || columnSelect)) {
			return identity{}, errors.New("PostgreSQL relation privileges are not strictly SELECT-only")
		}
	}
	rows.Close()
	if db.QueryRowContext(ctx, `SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND has_function_privilege(current_user,p.oid,'EXECUTE')`).Scan(&count) != nil {
		return identity{}, errors.New("inspect PostgreSQL function privileges")
	}
	if count > 0 {
		return identity{}, errors.New("PostgreSQL role can execute untrusted functions")
	}
	safeFunctions := map[string]struct{}{}
	rows, err = db.QueryContext(ctx, `SELECT p.proname,bool_and(NOT p.prosecdef AND (p.provolatile<>'v' OR p.proname=ANY($1::text[]))) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='pg_catalog' AND p.prokind IN ('f','a','w') GROUP BY p.proname`, []string{"random", "setseed", "clock_timestamp", "timeofday", "gen_random_uuid"})
	if err != nil {
		return identity{}, errors.New("build PostgreSQL function capability catalog")
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var safe bool
		if rows.Scan(&name, &safe) != nil {
			return identity{}, errors.New("read PostgreSQL function capability catalog")
		}
		if safe {
			if _, danger := dangerousFunctions[strings.ToLower(name)]; !danger {
				safeFunctions[strings.ToLower(name)] = struct{}{}
			}
		}
	}
	if rows.Err() != nil {
		return identity{}, errors.New("read PostgreSQL function capability catalog")
	}
	return identity{version: version, readOnly: readOnly, safeFunctions: safeFunctions}, nil
}
func (t *Target) Info() core.TargetInfo {
	x := t.info
	x.Healthy = t.healthy.Load()
	x.Schemas = append([]string(nil), x.Schemas...)
	return x
}
func (t *Target) ValidateQuery(q string) (*core.Validation, error) { return t.policy.Validate(q, -1) }
func (t *Target) Query(ctx context.Context, r core.QueryRequest) (*core.QueryResult, error) {
	v, err := t.policy.Validate(r.SQL, len(r.Parameters))
	if err != nil {
		t.audit(ctx, audit.Event{Target: t.cfg.Name, Operation: "query_select", Decision: "rejected", Reason: err.Error()})
		return nil, err
	}
	return t.execute(ctx, "query_select", r.SQL, r, v)
}
func (t *Target) Explain(ctx context.Context, r core.QueryRequest) (*core.QueryResult, error) {
	v, err := t.policy.Validate(r.SQL, len(r.Parameters))
	if err != nil {
		return nil, err
	}
	return t.execute(ctx, "query_explain", "EXPLAIN (FORMAT JSON, ANALYZE FALSE, VERBOSE FALSE, COSTS TRUE) "+r.SQL, r, v)
}
func (t *Target) execute(ctx context.Context, op, q string, r core.QueryRequest, v *core.Validation) (*core.QueryResult, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	started := time.Now()
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout > t.limits.MaxTimeout {
		return nil, fmt.Errorf("requested timeout exceeds configured maximum")
	}
	maxRows := r.MaxRows
	if maxRows <= 0 {
		maxRows = t.limits.MaxRows
	}
	if maxRows > t.limits.MaxRows {
		return nil, fmt.Errorf("requested row limit exceeds configured maximum")
	}
	if len(r.Parameters) > t.limits.MaxParameters {
		return nil, fmt.Errorf("query has too many parameters")
	}
	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	permit, err := t.admission.Acquire(qctx, t.cfg.Name, admission.Interactive)
	if err != nil {
		return nil, fmt.Errorf("query concurrency limit: %w", err)
	}
	defer permit.Release()
	tx, err := t.db.BeginTx(qctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, sanitize(err)
	}
	defer tx.Rollback()
	serverTimeout := timeout - t.cfg.PostgreSQL.StatementTimeoutMargin
	if serverTimeout <= 0 {
		serverTimeout = timeout
	}
	if _, err = tx.ExecContext(qctx, `SELECT pg_catalog.set_config('statement_timeout',$1,true)`, strconv.FormatInt(serverTimeout.Milliseconds(), 10)); err != nil {
		return nil, sanitize(err)
	}
	rows, err := tx.QueryContext(qctx, q, r.Parameters...)
	if err != nil {
		return nil, sanitize(err)
	}
	defer rows.Close()
	result, err := t.collect(rows, maxRows)
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
	if err = enforceBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(result)
	t.audit(qctx, audit.Event{QueryID: result.QueryID, Target: t.cfg.Name, Operation: op, Fingerprint: v.Fingerprint, Tables: v.Tables, Decision: "allowed", Rows: result.RowCount, Truncated: result.Truncated, Duration: time.Since(started), ResponseBytes: len(encoded)})
	return result, nil
}
func (t *Target) collect(rows *sql.Rows, maxRows int) (*core.QueryResult, error) {
	names, err := rows.Columns()
	if err != nil {
		return nil, errors.New("read result columns")
	}
	if len(names) > 1024 {
		return nil, errors.New("query result has too many columns")
	}
	types, _ := rows.ColumnTypes()
	names = uniqueNames(names)
	cols := make([]core.Column, len(names))
	for i, n := range names {
		nullable, _ := types[i].Nullable()
		cols[i] = core.Column{Name: n, DatabaseType: types[i].DatabaseTypeName(), Nullable: nullable}
	}
	out := &core.QueryResult{Columns: cols, Rows: make([]map[string]any, 0, min(maxRows, 32))}
	used := 0
	for rows.Next() {
		if len(out.Rows) >= maxRows {
			out.Truncated = true
			break
		}
		vals := make([]any, len(names))
		dest := make([]any, len(names))
		for i := range vals {
			dest[i] = &vals[i]
		}
		if rows.Scan(dest...) != nil {
			return nil, errors.New("scan query result")
		}
		row := map[string]any{}
		for i, v := range vals {
			n, tr := normalize(v, t.limits.MaxCellBytes)
			if tr {
				out.TruncatedCells++
			}
			row[names[i]] = n
		}
		b, _ := json.Marshal(row)
		if used+len(b) > t.limits.MaxResultBytes {
			out.Truncated = true
			break
		}
		used += len(b)
		out.Rows = append(out.Rows, row)
	}
	if rows.Err() != nil {
		return nil, sanitize(rows.Err())
	}
	out.RowCount = len(out.Rows)
	return out, nil
}
func normalize(v any, max int) (any, bool) {
	switch x := v.(type) {
	case nil:
		return nil, false
	case time.Time:
		return x.Format(time.RFC3339Nano), false
	case []byte:
		tr := len(x) > max
		if tr {
			x = x[:max]
		}
		if utf8.Valid(x) {
			return string(x), tr
		}
		return "base64:" + base64.StdEncoding.EncodeToString(x), tr
	case string:
		if len(x) <= max {
			return x, false
		}
		cut := max
		for cut > 0 && !utf8.RuneStart(x[cut]) {
			cut--
		}
		return x[:cut], true
	case int64:
		if x > 1<<53-1 || x < -(1<<53-1) {
			return strconv.FormatInt(x, 10), false
		}
		return x, false
	case float64:
		return x, false
	default:
		return fmt.Sprint(x), false
	}
}
func enforceBudget(r *core.QueryResult, max int) error {
	b, err := json.Marshal(r)
	if err != nil {
		return errors.New("encode query result")
	}
	if len(b) <= max {
		return nil
	}
	all := r.Rows
	lo, hi := 0, len(all)
	for lo < hi {
		m := (lo + hi + 1) / 2
		r.Rows = all[:m]
		r.RowCount = m
		b, _ = json.Marshal(r)
		if len(b) <= max {
			lo = m
		} else {
			hi = m - 1
		}
	}
	r.Rows = all[:lo]
	r.RowCount = lo
	r.Truncated = true
	b, _ = json.Marshal(r)
	if len(b) > max {
		return errors.New("query result metadata exceeds configured result-byte limit")
	}
	return nil
}
func uniqueNames(v []string) []string {
	seen := map[string]int{}
	out := make([]string, len(v))
	for i, x := range v {
		seen[x]++
		out[i] = x
		if seen[x] > 1 {
			out[i] = fmt.Sprintf("%s#%d", x, seen[x])
		}
	}
	return out
}
func sanitize(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return fmt.Errorf("database rejected query (PostgreSQL SQLSTATE %s)", pe.Code)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("query timed out")
	}
	if errors.Is(err, context.Canceled) {
		return errors.New("query was canceled")
	}
	return errors.New("database query failed")
}
func (t *Target) audit(ctx context.Context, e audit.Event) {
	if t.auditor != nil {
		t.auditor.Record(ctx, e)
	}
}
func (t *Target) requireHealthy() error {
	if !t.healthy.Load() {
		return errors.New("PostgreSQL target failed its latest privilege attestation")
	}
	return nil
}

func (t *Target) startPrivilegeRecheck() {
	ctx, cancel := context.WithCancel(context.Background())
	t.maintenanceStop = cancel
	t.maintenanceWG.Add(1)
	go func() {
		defer t.maintenanceWG.Done()
		ticker := time.NewTicker(t.cfg.PostgreSQL.PrivilegeRecheck)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkCtx, checkCancel := context.WithTimeout(ctx, t.cfg.Connection.ConnectTimeout)
				permit, err := t.admission.Acquire(checkCtx, t.cfg.Name, admission.Maintenance)
				if err == nil {
					_, err = verifyIdentityAndPrivileges(checkCtx, t.db, t.cfg)
					permit.Release()
				}
				t.healthy.Store(err == nil)
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
func postgresPolicyRevision(c *config.TargetConfig) string {
	sum := sha256.Sum256([]byte(strings.ToLower(c.Engine + "\x00" + c.Database + "\x00" + strings.Join(c.AllowedSchemas, "\x00") + "\x00" + strings.Join(c.DeniedTables, "\x00"))))
	return hex.EncodeToString(sum[:12])
}
