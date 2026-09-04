package sqlserver

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
		return nil, errors.New("table pattern is too long")
	}
	key := t.policyRevision + "\x00list\x00" + pattern
	if fresh && !t.cfg.MetadataCache.IsFreshAllowed() {
		return nil, errors.New("forced metadata refresh is disabled for this target")
	}
	if !fresh {
		if value, ok := t.cache.get(key); ok {
			return value.([]core.TableSummary), nil
		}
	}
	leader, err := t.cache.lead(ctx, key, fresh, t.cfg.MetadataCache.FreshCooldown)
	if err != nil {
		return nil, err
	}
	if !leader {
		if value, ok := t.cache.get(key); ok {
			return value.([]core.TableSummary), nil
		}
		return nil, errors.New("metadata refresh completed without a cacheable result")
	}
	completed := false
	defer func() {
		if !completed {
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
	arguments := make([]any, 0, len(t.cfg.AllowedSchemas)+1)
	holders := make([]string, len(t.cfg.AllowedSchemas))
	for i, schema := range t.cfg.AllowedSchemas {
		holders[i] = "@p" + fmt.Sprint(i+1)
		arguments = append(arguments, sql.Named("p"+fmt.Sprint(i+1), schema))
	}
	query := `
SELECT schema_value.name,
       object_value.name,
       CASE object_value.type WHEN N'U' THEN N'BASE TABLE' WHEN N'V' THEN N'VIEW' END
FROM sys.objects AS object_value
JOIN sys.schemas AS schema_value ON schema_value.schema_id=object_value.schema_id
WHERE object_value.type IN (N'U',N'V')
  AND object_value.is_ms_shipped=0
  AND schema_value.name IN (` + strings.Join(holders, ",") + `)`
	if pattern != "" {
		name := "p" + fmt.Sprint(len(arguments)+1)
		query += " AND object_value.name LIKE @" + name + " ESCAPE N'\\'"
		arguments = append(arguments, sql.Named(name, pattern))
	}
	query += " ORDER BY schema_value.name,object_value.name OFFSET 0 ROWS FETCH NEXT 2000 ROWS ONLY"
	rows, err := t.db.QueryContext(qctx, query, arguments...)
	if err != nil {
		return nil, sanitize(err)
	}
	defer rows.Close()
	result := make([]core.TableSummary, 0)
	for rows.Next() {
		var table core.TableSummary
		if err := rows.Scan(&table.Schema, &table.Name, &table.Type); err != nil {
			return nil, errors.New("scan table metadata")
		}
		qualified := strings.ToLower(table.Schema + "." + table.Name)
		if _, denied := t.denied[strings.ToLower(table.Name)]; denied {
			continue
		}
		if _, denied := t.denied[qualified]; denied {
			continue
		}
		result = append(result, table)
	}
	if err := rows.Err(); err != nil {
		return nil, sanitize(err)
	}
	if err := enforceMetadataBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	t.cache.finish(key, result, t.cfg.MetadataCache.TableListTTL, true)
	completed = true
	return result, nil
}

func (t *Target) DescribeTable(ctx context.Context, schema, table string, fresh bool) (*core.TableDescription, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	if schema == "" || table == "" {
		return nil, errors.New("schema and table are required for SQL Server")
	}
	if _, ok := t.allowed[strings.ToLower(schema)]; !ok {
		return nil, errors.New("schema is outside the selected target")
	}
	if _, denied := t.denied[strings.ToLower(table)]; denied {
		return nil, errors.New("table is denied by target policy")
	}
	if _, denied := t.denied[strings.ToLower(schema+"."+table)]; denied {
		return nil, errors.New("table is denied by target policy")
	}
	key := t.policyRevision + "\x00describe\x00" + strings.ToLower(schema) + "\x00" + strings.ToLower(table)
	if fresh && !t.cfg.MetadataCache.IsFreshAllowed() {
		return nil, errors.New("forced metadata refresh is disabled for this target")
	}
	if !fresh {
		if value, ok := t.cache.get(key); ok {
			return value.(*core.TableDescription), nil
		}
	}
	leader, err := t.cache.lead(ctx, key, fresh, t.cfg.MetadataCache.FreshCooldown)
	if err != nil {
		return nil, err
	}
	if !leader {
		if value, ok := t.cache.get(key); ok {
			return value.(*core.TableDescription), nil
		}
		return nil, errors.New("metadata refresh completed without a cacheable result")
	}
	completed := false
	defer func() {
		if !completed {
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

	rows, err := t.db.QueryContext(qctx, `
SELECT column_value.name,
       type_value.name,
       CASE WHEN computed.definition IS NOT NULL THEN computed.definition
            ELSE QUOTENAME(TYPE_SCHEMA_NAME(type_value.schema_id))+N'.'+QUOTENAME(type_value.name) END,
       column_value.is_nullable,
       default_value.definition,
       CONCAT_WS(N',',
           CASE WHEN column_value.is_identity=1 THEN N'identity' END,
           CASE WHEN computed.definition IS NOT NULL THEN N'computed' END,
           CASE WHEN column_value.is_sparse=1 THEN N'sparse' END,
           CASE WHEN column_value.is_rowguidcol=1 THEN N'rowguidcol' END),
       CONVERT(nvarchar(4000),property_value.value)
FROM sys.columns AS column_value
JOIN sys.objects AS object_value ON object_value.object_id=column_value.object_id
JOIN sys.schemas AS schema_value ON schema_value.schema_id=object_value.schema_id
JOIN sys.types AS type_value ON type_value.user_type_id=column_value.user_type_id
LEFT JOIN sys.default_constraints AS default_value ON default_value.object_id=column_value.default_object_id
LEFT JOIN sys.computed_columns AS computed ON computed.object_id=column_value.object_id AND computed.column_id=column_value.column_id
LEFT JOIN sys.extended_properties AS property_value ON property_value.class=1 AND property_value.major_id=column_value.object_id AND property_value.minor_id=column_value.column_id AND property_value.name=N'MS_Description'
WHERE schema_value.name=@p1 AND object_value.name=@p2 AND object_value.type IN (N'U',N'V')
ORDER BY column_value.column_id`, sql.Named("p1", schema), sql.Named("p2", table))
	if err != nil {
		return nil, sanitize(err)
	}
	columns := make([]core.ColumnDescription, 0)
	for rows.Next() {
		var column core.ColumnDescription
		var defaultValue, comment sql.NullString
		if err := rows.Scan(&column.Name, &column.DataType, &column.ColumnType, &column.Nullable, &defaultValue, &column.Extra, &comment); err != nil {
			rows.Close()
			return nil, errors.New("scan column metadata")
		}
		if defaultValue.Valid {
			column.Default = &defaultValue.String
		}
		if comment.Valid {
			column.Comment = comment.String
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, sanitize(err)
	}
	rows.Close()
	if len(columns) == 0 {
		return nil, errors.New("table is not visible to this target")
	}

	rows, err = t.db.QueryContext(qctx, `
SELECT index_value.name,
       index_value.is_unique,
       index_value.is_primary_key,
       index_value.type_desc,
       index_column.key_ordinal,
       index_column.is_included_column,
       column_value.name,
       index_value.filter_definition
FROM sys.indexes AS index_value
JOIN sys.objects AS object_value ON object_value.object_id=index_value.object_id
JOIN sys.schemas AS schema_value ON schema_value.schema_id=object_value.schema_id
JOIN sys.index_columns AS index_column ON index_column.object_id=index_value.object_id AND index_column.index_id=index_value.index_id
JOIN sys.columns AS column_value ON column_value.object_id=index_column.object_id AND column_value.column_id=index_column.column_id
WHERE schema_value.name=@p1 AND object_value.name=@p2 AND index_value.name IS NOT NULL
ORDER BY index_value.index_id,index_column.is_included_column,index_column.key_ordinal,index_column.index_column_id`, sql.Named("p1", schema), sql.Named("p2", table))
	if err != nil {
		return nil, sanitize(err)
	}
	indexes := make([]core.IndexDescription, 0)
	positions := make(map[string]int)
	primaryColumns := make(map[string]struct{})
	for rows.Next() {
		var name, method, columnName string
		var unique, primary, included bool
		var ordinal int
		var predicate sql.NullString
		if err := rows.Scan(&name, &unique, &primary, &method, &ordinal, &included, &columnName, &predicate); err != nil {
			rows.Close()
			return nil, errors.New("scan index metadata")
		}
		position, ok := positions[name]
		if !ok {
			position = len(indexes)
			positions[name] = position
			index := core.IndexDescription{Name: name, Unique: unique, Primary: primary, Method: method}
			if predicate.Valid {
				index.Predicate = predicate.String
			}
			indexes = append(indexes, index)
		}
		if included {
			indexes[position].Includes = append(indexes[position].Includes, columnName)
		} else if ordinal > 0 {
			indexes[position].Columns = append(indexes[position].Columns, columnName)
			if primary {
				primaryColumns[strings.ToLower(columnName)] = struct{}{}
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, sanitize(err)
	}
	rows.Close()
	for i := range columns {
		if _, primary := primaryColumns[strings.ToLower(columns[i].Name)]; primary {
			columns[i].Key = "PRI"
		}
	}
	result := &core.TableDescription{Target: t.cfg.Name, Schema: schema, Table: table, Columns: columns, Indexes: indexes}
	if err := enforceMetadataBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	t.cache.finish(key, result, t.cfg.MetadataCache.TableDescriptionTTL, true)
	completed = true
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
