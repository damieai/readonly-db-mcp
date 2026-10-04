package duckdbworker

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/duckdbprotocol"
)

// Run handles exactly one request. The executable is launched inside a
// bubblewrap mount/network sandbox by the parent connector.
func Run() int {
	in := io.LimitReader(os.Stdin, 16<<20)
	decoder := json.NewDecoder(in)
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	var request duckdbprotocol.Request
	if err := decoder.Decode(&request); err != nil {
		respond(duckdbprotocol.Response{Error: "invalid_request: malformed worker request"})
		return 1
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		respond(duckdbprotocol.Response{Error: "invalid_request: trailing worker input"})
		return 1
	}
	response := handle(context.Background(), request)
	respond(response)
	if response.Error != "" {
		return 1
	}
	return 0
}

func respond(response duckdbprotocol.Response) { _ = json.NewEncoder(os.Stdout).Encode(response) }

func handle(ctx context.Context, req duckdbprotocol.Request) duckdbprotocol.Response {
	if req.Database == "" || filepath.IsAbs(req.Database) || req.Database == "." || strings.HasPrefix(req.Database, "..") || strings.ContainsRune(req.Database, 0) || strings.Contains(req.Database, "?") || req.MaxRows < 1 || req.MaxRows > 10000 || req.MaxResultBytes < 1024 || req.MaxResultBytes > 16<<20 || req.MaxCellBytes < 256 || req.MaxCellBytes > req.MaxResultBytes || req.MaxSQLBytes < 256 || req.MaxSQLBytes > 1<<20 || req.MemoryMB < 128 || req.MemoryMB > 2048 || req.Threads < 1 || req.Threads > 8 {
		return duckdbprotocol.Response{Error: "invalid_request: worker profile is invalid"}
	}
	full := filepath.Join("/data", req.Database)
	if !strings.HasPrefix(full, "/data/") {
		return duckdbprotocol.Response{Error: "permission_denied: invalid database path"}
	}
	connector, err := duckdb.NewConnector(full+"?access_mode=READ_ONLY&threads="+strconv.Itoa(req.Threads), func(execer driver.ExecerContext) error {
		setup := []string{
			"SET memory_limit = '" + strconv.Itoa(req.MemoryMB) + "MB'",
			"SET autoinstall_known_extensions = false",
			"SET autoload_known_extensions = false",
			"SET allow_unsigned_extensions = false",
			"SET allowed_directories = ['/data']",
			"SET enable_external_access = false",
			"SET lock_configuration = true",
		}
		for _, statement := range setup {
			if _, err := execer.ExecContext(ctx, statement, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return duckdbprotocol.Response{Error: "transport_error: cannot open DuckDB database read-only"}
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return duckdbprotocol.Response{Error: "transport_error: cannot open DuckDB database read-only"}
	}
	var version, mode, external, locked string
	if err := db.QueryRowContext(ctx, "SELECT version(), current_setting('access_mode'), current_setting('enable_external_access'), current_setting('lock_configuration')").Scan(&version, &mode, &external, &locked); err != nil || version != "v1.5.6" || !strings.EqualFold(mode, "read_only") || external != "false" || locked != "true" {
		return duckdbprotocol.Response{Error: "profile_mismatch: DuckDB read-only settings or version differ"}
	}
	if req.Operation == "probe" {
		return duckdbprotocol.Response{Version: version}
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return duckdbprotocol.Response{Error: "transport_error: DuckDB connection unavailable"}
	}
	defer conn.Close()
	switch req.Operation {
	case "query", "explain":
		if err := validateRead(ctx, conn, req.SQL, req); err != nil {
			return duckdbprotocol.Response{Error: err.Error()}
		}
		query := req.SQL
		if req.Operation == "explain" {
			query = "EXPLAIN " + query
		}
		result, err := runQuery(ctx, conn, query, req.Parameters, req.MaxRows, req.MaxCellBytes, req.MaxResultBytes)
		if err != nil {
			return duckdbprotocol.Response{Error: err.Error()}
		}
		return duckdbprotocol.Response{Result: result}
	case "batch":
		if len(req.Queries) == 0 || len(req.Queries) > req.MaxBatchQueries {
			return duckdbprotocol.Response{Error: "resource_limit: DuckDB batch count is invalid"}
		}
		for _, item := range req.Queries {
			part := req
			part.Parameters = item.Parameters
			if err := validateRead(ctx, conn, item.SQL, part); err != nil {
				return duckdbprotocol.Response{Error: err.Error()}
			}
			if item.MaxRows < 0 || item.MaxRows > req.MaxRows {
				return duckdbprotocol.Response{Error: "resource_limit: DuckDB batch row cap is invalid"}
			}
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return duckdbprotocol.Response{Error: "query_error: DuckDB snapshot could not start"}
		}
		defer tx.Rollback()
		batch := &core.BatchResult{Results: make([]*core.QueryResult, 0, len(req.Queries))}
		for _, item := range req.Queries {
			cap := item.MaxRows
			if cap == 0 {
				cap = req.MaxRows
			}
			result, e := runQuery(ctx, tx, item.SQL, item.Parameters, cap, req.MaxCellBytes, req.MaxResultBytes)
			if e != nil {
				return duckdbprotocol.Response{Error: e.Error()}
			}
			batch.Results = append(batch.Results, result)
			if result.Truncated {
				batch.Truncated = true
			}
			if tooLarge(batch, req.MaxResultBytes) {
				return duckdbprotocol.Response{Error: "resource_limit: DuckDB batch exceeds max_result_bytes"}
			}
		}
		batch.CompletedQueries = len(batch.Results)
		return duckdbprotocol.Response{Batch: batch}
	case "tables":
		if len(req.Pattern) > 256 {
			return duckdbprotocol.Response{Error: "resource_limit: metadata pattern too long"}
		}
		rows, err := conn.QueryContext(ctx, "SELECT table_schema, table_name, table_type FROM information_schema.tables WHERE table_catalog=current_database() AND table_schema NOT IN ('information_schema','pg_catalog') AND table_name LIKE ? ORDER BY table_schema,table_name LIMIT ?", defaultPattern(req.Pattern), req.MaxRows+1)
		if err != nil {
			return duckdbprotocol.Response{Error: "query_error: DuckDB metadata failed"}
		}
		defer rows.Close()
		out := []core.TableSummary{}
		for rows.Next() {
			if len(out) >= req.MaxRows {
				return duckdbprotocol.Response{Error: "resource_limit: metadata row cap exceeded"}
			}
			var schema, name, kind string
			if rows.Scan(&schema, &name, &kind) != nil {
				return duckdbprotocol.Response{Error: "query_error: DuckDB metadata failed"}
			}
			out = append(out, core.TableSummary{Schema: schema, Name: name, Type: kind})
		}
		if rows.Err() != nil {
			return duckdbprotocol.Response{Error: "query_error: DuckDB metadata failed"}
		}
		if tooLarge(out, req.MaxResultBytes) {
			return duckdbprotocol.Response{Error: "resource_limit: metadata result too large"}
		}
		return duckdbprotocol.Response{Tables: out}
	case "describe":
		if req.Schema == "" || req.Table == "" || len(req.Schema) > 256 || len(req.Table) > 256 {
			return duckdbprotocol.Response{Error: "invalid_request: schema and table are required"}
		}
		var kind string
		if err := conn.QueryRowContext(ctx, "SELECT table_type FROM information_schema.tables WHERE table_catalog=current_database() AND table_schema=? AND table_name=?", req.Schema, req.Table).Scan(&kind); err != nil {
			return duckdbprotocol.Response{Error: "not_found: table is unavailable"}
		}
		rows, err := conn.QueryContext(ctx, "SELECT column_name,data_type,is_nullable,column_default FROM information_schema.columns WHERE table_catalog=current_database() AND table_schema=? AND table_name=? ORDER BY ordinal_position", req.Schema, req.Table)
		if err != nil {
			return duckdbprotocol.Response{Error: "query_error: DuckDB metadata failed"}
		}
		defer rows.Close()
		desc := &core.TableDescription{Schema: req.Schema, Table: req.Table}
		for rows.Next() {
			var name, datatype, nullable string
			var def sql.NullString
			if rows.Scan(&name, &datatype, &nullable, &def) != nil {
				return duckdbprotocol.Response{Error: "query_error: DuckDB metadata failed"}
			}
			var d *string
			if def.Valid {
				d = &def.String
			}
			desc.Columns = append(desc.Columns, core.ColumnDescription{Name: name, DataType: datatype, ColumnType: datatype, Nullable: nullable == "YES", Default: d})
			if tooLarge(desc, req.MaxResultBytes) {
				return duckdbprotocol.Response{Error: "resource_limit: metadata result too large"}
			}
		}
		if rows.Err() != nil {
			return duckdbprotocol.Response{Error: "query_error: DuckDB metadata failed"}
		}
		indexRows, err := conn.QueryContext(ctx, "SELECT index_name,is_unique FROM duckdb_indexes() WHERE database_name=current_database() AND schema_name=? AND table_name=? ORDER BY index_name", req.Schema, req.Table)
		if err != nil {
			return duckdbprotocol.Response{Error: "query_error: DuckDB index metadata failed"}
		}
		defer indexRows.Close()
		for indexRows.Next() {
			var name string
			var unique bool
			if indexRows.Scan(&name, &unique) != nil {
				return duckdbprotocol.Response{Error: "query_error: DuckDB index metadata failed"}
			}
			desc.Indexes = append(desc.Indexes, core.IndexDescription{Name: name, Unique: unique, Columns: []string{}})
			if tooLarge(desc, req.MaxResultBytes) {
				return duckdbprotocol.Response{Error: "resource_limit: metadata result too large"}
			}
		}
		if indexRows.Err() != nil {
			return duckdbprotocol.Response{Error: "query_error: DuckDB index metadata failed"}
		}
		return duckdbprotocol.Response{Description: desc}
	default:
		return duckdbprotocol.Response{Error: "invalid_request: unsupported DuckDB operation"}
	}
}

func defaultPattern(p string) string {
	if p == "" {
		return "%"
	}
	return p
}
func tooLarge(v any, max int) bool { b, e := json.Marshal(v); return e != nil || len(b) > max }

func validateRead(ctx context.Context, conn *sql.Conn, query string, req duckdbprotocol.Request) error {
	if len(query) == 0 || len(query) > req.MaxSQLBytes || !utf8.ValidString(query) || strings.IndexByte(query, 0) >= 0 || !singleStatement(query) {
		return errors.New("invalid_request: DuckDB requires one valid SQL statement")
	}
	if len(req.Parameters) > req.MaxParameters {
		return errors.New("resource_limit: too many DuckDB parameters")
	}
	b, e := json.Marshal(req.Parameters)
	if e != nil || len(b) > req.MaxParameterBytes {
		return errors.New("resource_limit: DuckDB parameters are too large")
	}
	for i, v := range req.Parameters {
		converted, e := convertParameter(v)
		if e != nil {
			return e
		}
		req.Parameters[i] = converted
	}
	var statementType duckdb.StmtType
	err := conn.Raw(func(raw any) error {
		d, ok := raw.(*duckdb.Conn)
		if !ok {
			return errors.New("driver mismatch")
		}
		stmt, e := d.PrepareContext(ctx, query)
		if e != nil {
			return e
		}
		defer stmt.Close()
		typed, ok := stmt.(*duckdb.Stmt)
		if !ok {
			return errors.New("statement mismatch")
		}
		statementType, e = typed.StatementType()
		return e
	})
	if err != nil {
		return errors.New("invalid_request: DuckDB could not prepare query")
	}
	if statementType != duckdb.STATEMENT_TYPE_SELECT {
		return errors.New("permission_denied: only DuckDB SELECT statements are allowed")
	}
	return nil
}

func singleStatement(query string) bool {
	// Conservative lexical boundary before DuckDB's driver prepares SQL: the
	// driver executes all but the final statement during PrepareContext.
	state := byte(0)
	blockDepth := 0
	semicolon := false
	dollarTag := ""
	for i := 0; i < len(query); i++ {
		c := query[i]
		var next byte
		if i+1 < len(query) {
			next = query[i+1]
		}
		switch state {
		case 0:
			if semicolon && c != ' ' && c != '\n' && c != '\r' && c != '\t' && !(c == '-' && next == '-') && !(c == '/' && next == '*') {
				return false
			}
			if c == '-' && next == '-' {
				state = 'l'
				i++
				continue
			}
			if c == '/' && next == '*' {
				state = 'b'
				blockDepth = 1
				i++
				continue
			}
			if c == '\'' || c == '"' || c == '`' {
				state = c
				continue
			}
			if c == '$' {
				end := i + 1
				for end < len(query) && (query[end] == '_' || query[end] >= 'A' && query[end] <= 'Z' || query[end] >= 'a' && query[end] <= 'z' || query[end] >= '0' && query[end] <= '9') {
					end++
				}
				if end < len(query) && query[end] == '$' {
					dollarTag = query[i : end+1]
					state = 'd'
					i = end
					continue
				}
			}
			if c == ';' {
				if semicolon {
					return false
				}
				semicolon = true
				continue
			}
		case 'l':
			if c == '\n' {
				state = 0
			}
		case 'b':
			if c == '/' && next == '*' {
				blockDepth++
				i++
			} else if c == '*' && next == '/' {
				blockDepth--
				i++
				if blockDepth == 0 {
					state = 0
				}
			}
		case 'd':
			if strings.HasPrefix(query[i:], dollarTag) {
				i += len(dollarTag) - 1
				state = 0
			}
		default:
			if c == state {
				if next == state {
					i++
				} else {
					state = 0
				}
			}
		}
	}
	return state == 0 || state == 'l'
}

type querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func runQuery(ctx context.Context, conn querier, query string, params []any, maxRows, maxCell, maxBytes int) (*core.QueryResult, error) {
	rows, err := conn.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, errors.New("query_error: DuckDB query failed")
	}
	defer rows.Close()
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
		base := name
		for n := 2; used[name]; n++ {
			name = base + "_" + strconv.Itoa(n)
		}
		used[name] = true
		columns[i] = core.Column{Name: name, DatabaseType: types[i].DatabaseTypeName()}
	}
	result := &core.QueryResult{Columns: columns, Rows: []map[string]any{}}
	for rows.Next() {
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			break
		}
		values := make([]any, len(names))
		dest := make([]any, len(names))
		for i := range values {
			dest[i] = &values[i]
		}
		if rows.Scan(dest...) != nil {
			return nil, errors.New("query_error: DuckDB row decoding failed")
		}
		row := make(map[string]any, len(names))
		for i, v := range values {
			value, e := normal(v, maxCell, 0)
			if e != nil {
				return nil, e
			}
			row[columns[i].Name] = value
		}
		result.Rows = append(result.Rows, row)
		if tooLarge(result, maxBytes) {
			return nil, errors.New("resource_limit: DuckDB result exceeds max_result_bytes")
		}
	}
	if rows.Err() != nil || ctx.Err() != nil {
		return nil, errors.New("deadline_exceeded: DuckDB query interrupted")
	}
	result.RowCount = len(result.Rows)
	if tooLarge(result, maxBytes) {
		return nil, errors.New("resource_limit: DuckDB result exceeds max_result_bytes")
	}
	return result, nil
}

func normal(v any, maxCell, depth int) (any, error) {
	if depth > 32 {
		return nil, errors.New("resource_limit: DuckDB value nesting exceeds limit")
	}
	switch x := v.(type) {
	case nil, bool, float64, float32:
		return x, nil
	case int64:
		if x > 9007199254740991 || x < -9007199254740991 {
			return strconv.FormatInt(x, 10), nil
		}
		return x, nil
	case int:
		return x, nil
	case int32:
		return x, nil
	case uint:
		if uint64(x) > 9007199254740991 {
			return fmt.Sprint(x), nil
		}
		return x, nil
	case uint32:
		return x, nil
	case uint64:
		if x > 9007199254740991 {
			return fmt.Sprint(x), nil
		}
		return x, nil
	case string:
		if len(x) > maxCell {
			return nil, errors.New("resource_limit: DuckDB cell exceeds max_cell_bytes")
		}
		return x, nil
	case []byte:
		if len(x) > maxCell {
			return nil, errors.New("resource_limit: DuckDB cell exceeds max_cell_bytes")
		}
		return base64.StdEncoding.EncodeToString(x), nil
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano), nil
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil, nil
	}
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Len() > 10000 {
			return nil, errors.New("resource_limit: DuckDB list too long")
		}
		out := make([]any, rv.Len())
		for i := range out {
			item, e := normal(rv.Index(i).Interface(), maxCell, depth+1)
			if e != nil {
				return nil, e
			}
			out[i] = item
		}
		return out, nil
	case reflect.Map:
		if rv.Len() > 10000 {
			return nil, errors.New("resource_limit: DuckDB map too large")
		}
		out := map[string]any{}
		iter := rv.MapRange()
		for iter.Next() {
			key := fmt.Sprint(iter.Key().Interface())
			item, e := normal(iter.Value().Interface(), maxCell, depth+1)
			if e != nil {
				return nil, e
			}
			out[key] = item
		}
		return out, nil
	}
	s := fmt.Sprint(v)
	if len(s) > maxCell {
		return nil, errors.New("resource_limit: DuckDB cell exceeds max_cell_bytes")
	}
	return s, nil
}

func convertParameter(v any) (any, error) {
	switch x := v.(type) {
	case json.Number:
		if i, e := strconv.ParseInt(string(x), 10, 64); e == nil {
			return i, nil
		}
		if f, e := strconv.ParseFloat(string(x), 64); e == nil {
			return f, nil
		}
		return nil, errors.New("invalid_request: DuckDB number is unsupported")
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			converted, e := convertParameter(item)
			if e != nil {
				return nil, e
			}
			out[i] = converted
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for key, item := range x {
			converted, e := convertParameter(item)
			if e != nil {
				return nil, e
			}
			out[key] = converted
		}
		return out, nil
	default:
		return v, nil
	}
}
