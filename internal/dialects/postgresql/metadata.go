package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func (t *Target) ListTables(ctx context.Context, pattern string, fresh bool) ([]core.TableSummary, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if len(pattern) > 128 {
		return nil, fmt.Errorf("table pattern is too long")
	}
	key := t.policyRevision + "\x00list\x00" + pattern
	if fresh && !t.cfg.MetadataCache.IsFreshAllowed() {
		return nil, fmt.Errorf("forced metadata refresh is disabled for this target")
	}
	if !fresh {
		if v, ok := t.cache.get(key); ok {
			return v.([]core.TableSummary), nil
		}
	}
	leader, err := t.cache.lead(ctx, key, fresh, t.cfg.MetadataCache.FreshCooldown)
	if err != nil {
		return nil, err
	}
	if !leader {
		if v, ok := t.cache.get(key); ok {
			return v.([]core.TableSummary), nil
		}
		return nil, fmt.Errorf("metadata refresh completed without a cacheable result")
	}
	done := false
	defer func() {
		if !done {
			t.cache.finish(key, nil, 0, false)
		}
	}()
	qctx, cancel := context.WithTimeout(ctx, t.limits.DefaultTimeout)
	defer cancel()
	permit, err := t.admission.Acquire(qctx, t.cfg.Name, admission.Metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata concurrency limit: %w", err)
	}
	defer permit.Release()
	t.gate.RLock()
	defer t.gate.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	args := make([]any, len(t.cfg.AllowedSchemas))
	holders := make([]string, len(args))
	for i, s := range t.cfg.AllowedSchemas {
		args[i] = s
		holders[i] = fmt.Sprintf("$%d", i+1)
	}
	q := `SELECT n.nspname,c.relname,CASE c.relkind WHEN 'r' THEN 'BASE TABLE' WHEN 'p' THEN 'PARTITIONED TABLE' WHEN 'v' THEN 'VIEW' WHEN 'm' THEN 'MATERIALIZED VIEW' END FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname IN (` + strings.Join(holders, ",") + `) AND c.relkind IN ('r','p','v','m')`
	if pattern != "" {
		args = append(args, pattern)
		q += fmt.Sprintf(" AND c.relname LIKE $%d ESCAPE '\\\\'", len(args))
	}
	q += " ORDER BY n.nspname,c.relname LIMIT 2000"
	rows, err := t.db.QueryContext(qctx, q, args...)
	if err != nil {
		return nil, sanitize(err)
	}
	defer rows.Close()
	var out []core.TableSummary
	for rows.Next() {
		var x core.TableSummary
		if rows.Scan(&x.Schema, &x.Name, &x.Type) != nil {
			return nil, errors.New("scan table metadata")
		}
		qualified := strings.ToLower(x.Schema + "." + x.Name)
		if _, bad := t.denied[strings.ToLower(x.Name)]; bad {
			continue
		}
		if _, bad := t.denied[qualified]; bad {
			continue
		}
		out = append(out, x)
	}
	if rows.Err() != nil {
		return nil, sanitize(rows.Err())
	}
	if err := enforceMetadataBudget(out, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	t.cache.finish(key, out, t.cfg.MetadataCache.TableListTTL, true)
	done = true
	return out, nil
}
func (t *Target) DescribeTable(ctx context.Context, schema, table string, fresh bool) (*core.TableDescription, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if schema == "" || table == "" {
		return nil, fmt.Errorf("schema and table are required for PostgreSQL")
	}
	if _, ok := t.allowed[strings.ToLower(schema)]; !ok {
		return nil, fmt.Errorf("schema is outside the selected target")
	}
	if _, bad := t.denied[strings.ToLower(table)]; bad {
		return nil, fmt.Errorf("table is denied by target policy")
	}
	if _, bad := t.denied[strings.ToLower(schema+"."+table)]; bad {
		return nil, fmt.Errorf("table is denied by target policy")
	}
	key := t.policyRevision + "\x00describe\x00" + schema + "\x00" + table
	if fresh && !t.cfg.MetadataCache.IsFreshAllowed() {
		return nil, fmt.Errorf("forced metadata refresh is disabled for this target")
	}
	if !fresh {
		if v, ok := t.cache.get(key); ok {
			return v.(*core.TableDescription), nil
		}
	}
	leader, err := t.cache.lead(ctx, key, fresh, t.cfg.MetadataCache.FreshCooldown)
	if err != nil {
		return nil, err
	}
	if !leader {
		if v, ok := t.cache.get(key); ok {
			return v.(*core.TableDescription), nil
		}
		return nil, fmt.Errorf("metadata refresh completed without a cacheable result")
	}
	done := false
	defer func() {
		if !done {
			t.cache.finish(key, nil, 0, false)
		}
	}()
	qctx, cancel := context.WithTimeout(ctx, t.limits.DefaultTimeout)
	defer cancel()
	permit, err := t.admission.Acquire(qctx, t.cfg.Name, admission.Metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata concurrency limit: %w", err)
	}
	defer permit.Release()
	t.gate.RLock()
	defer t.gate.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	rows, err := t.db.QueryContext(qctx, `SELECT a.attname,pg_catalog.format_type(a.atttypid,a.atttypmod),NOT a.attnotnull,pg_catalog.pg_get_expr(d.adbin,d.adrelid),CASE WHEN a.attidentity<>'' THEN 'identity' WHEN a.attgenerated<>'' THEN 'generated' ELSE '' END,pg_catalog.col_description(a.attrelid,a.attnum) FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE n.nspname=$1 AND c.relname=$2 AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`, schema, table)
	if err != nil {
		return nil, sanitize(err)
	}
	defer rows.Close()
	var cols []core.ColumnDescription
	for rows.Next() {
		var c core.ColumnDescription
		var def, comment sql.NullString
		if rows.Scan(&c.Name, &c.ColumnType, &c.Nullable, &def, &c.Extra, &comment) != nil {
			return nil, errors.New("scan column metadata")
		}
		c.DataType = c.ColumnType
		if def.Valid {
			c.Default = &def.String
		}
		if comment.Valid {
			c.Comment = comment.String
		}
		cols = append(cols, c)
	}
	rows.Close()
	if len(cols) == 0 {
		return nil, fmt.Errorf("table is not visible to this target")
	}
	rows, err = t.db.QueryContext(qctx, `SELECT i.relname,ix.indisunique,ix.indisprimary,am.amname,pg_catalog.pg_get_indexdef(ix.indexrelid),COALESCE(pg_catalog.pg_get_expr(ix.indpred,ix.indrelid),'') FROM pg_catalog.pg_index ix JOIN pg_catalog.pg_class t ON t.oid=ix.indrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace JOIN pg_catalog.pg_class i ON i.oid=ix.indexrelid JOIN pg_catalog.pg_am am ON am.oid=i.relam WHERE n.nspname=$1 AND t.relname=$2 ORDER BY i.relname`, schema, table)
	if err != nil {
		return nil, sanitize(err)
	}
	defer rows.Close()
	var indexes []core.IndexDescription
	for rows.Next() {
		var x core.IndexDescription
		var definition string
		if rows.Scan(&x.Name, &x.Unique, &x.Primary, &x.Method, &definition, &x.Predicate) != nil {
			return nil, errors.New("scan index metadata")
		}
		x.Expressions = []string{definition}
		indexes = append(indexes, x)
	}
	result := &core.TableDescription{Target: t.cfg.Name, Schema: schema, Table: table, Columns: cols, Indexes: indexes}
	if err := enforceMetadataBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	t.cache.finish(key, result, t.cfg.MetadataCache.TableDescriptionTTL, true)
	done = true
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
