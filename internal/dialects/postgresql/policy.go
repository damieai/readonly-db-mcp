package postgresql

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

var forbiddenNodes = map[string]struct{}{
	"InsertStmt": {}, "UpdateStmt": {}, "DeleteStmt": {}, "MergeStmt": {}, "CopyStmt": {}, "CallStmt": {}, "DoStmt": {},
	"VariableSetStmt": {}, "VariableShowStmt": {}, "TransactionStmt": {}, "CreateStmt": {}, "AlterTableStmt": {}, "DropStmt": {},
	"GrantStmt": {}, "GrantRoleStmt": {}, "TruncateStmt": {}, "VacuumStmt": {}, "ExplainStmt": {}, "ListenStmt": {}, "NotifyStmt": {},
	"UnlistenStmt": {}, "CreateFunctionStmt": {}, "CreateRoleStmt": {}, "AlterRoleStmt": {}, "SecLabelStmt": {}, "RefreshMatViewStmt": {},
}
var dangerousFunctions = map[string]struct{}{
	"nextval": {}, "setval": {}, "pg_advisory_lock": {}, "pg_advisory_lock_shared": {}, "pg_try_advisory_lock": {}, "pg_try_advisory_lock_shared": {},
	"pg_advisory_xact_lock": {}, "pg_advisory_xact_lock_shared": {}, "pg_try_advisory_xact_lock": {}, "pg_try_advisory_xact_lock_shared": {},
	"pg_cancel_backend": {}, "pg_terminate_backend": {}, "pg_reload_conf": {}, "pg_rotate_logfile": {}, "pg_create_restore_point": {},
	"pg_switch_wal": {}, "pg_log_standby_snapshot": {}, "pg_export_snapshot": {}, "lo_create": {}, "lo_import": {}, "lo_unlink": {},
	"dblink_exec": {}, "set_config": {},
}
var systemSchemas = map[string]struct{}{"pg_catalog": {}, "information_schema": {}}

type Policy struct {
	allowed, denied map[string]struct{}
	safeFunctions   map[string]struct{}
	maxSQL          int
}

func NewPolicy(allowed, denied []string, maxSQL int, safeFunctions ...map[string]struct{}) *Policy {
	p := &Policy{allowed: lowerSet(allowed), denied: lowerSet(denied), maxSQL: maxSQL}
	if len(safeFunctions) > 0 {
		p.safeFunctions = safeFunctions[0]
	}
	return p
}

func (p *Policy) Validate(query string, paramCount int) (*core.Validation, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if len(trimmed) > p.maxSQL {
		return nil, fmt.Errorf("query exceeds the configured SQL size limit")
	}
	tree, err := pgquery.ParseToJSON(trimmed)
	if err != nil {
		return nil, fmt.Errorf("query is not one supported PostgreSQL statement")
	}
	var root map[string]any
	if json.Unmarshal([]byte(tree), &root) != nil {
		return nil, fmt.Errorf("inspect PostgreSQL parse tree")
	}
	stmts, ok := root["stmts"].([]any)
	if !ok || len(stmts) != 1 {
		return nil, fmt.Errorf("exactly one SELECT statement is required")
	}
	stmt := object(object(stmts[0])["stmt"])
	if _, ok := stmt["SelectStmt"]; !ok {
		return nil, fmt.Errorf("only SELECT statements are allowed")
	}
	ctes := map[string]struct{}{}
	walk(root, func(k string, v any) error {
		if k == "CommonTableExpr" {
			if n := stringField(object(v), "ctename"); n != "" {
				ctes[strings.ToLower(n)] = struct{}{}
			}
		}
		return nil
	})
	tables := map[string]struct{}{}
	params := map[int]struct{}{}
	err = walk(root, func(k string, v any) error {
		if _, bad := forbiddenNodes[k]; bad {
			return fmt.Errorf("PostgreSQL statement node %s is not allowed", k)
		}
		o := object(v)
		switch k {
		case "SelectStmt":
			if o["intoClause"] != nil {
				return fmt.Errorf("SELECT INTO is not allowed")
			}
			if a, ok := o["lockingClause"].([]any); ok && len(a) > 0 {
				return fmt.Errorf("locking SELECT queries are not allowed")
			}
		case "RangeVar":
			rel := stringField(o, "relname")
			schema := stringField(o, "schemaname")
			if rel == "" {
				return fmt.Errorf("relation name is empty")
			}
			if schema == "" {
				if _, ok := ctes[strings.ToLower(rel)]; ok {
					return nil
				}
				return fmt.Errorf("PostgreSQL relations must be schema-qualified")
			}
			ls := strings.ToLower(schema)
			if _, bad := systemSchemas[ls]; bad || strings.HasPrefix(ls, "pg_") {
				return fmt.Errorf("system schema is not available to free-form queries")
			}
			if _, ok := p.allowed[ls]; !ok {
				return fmt.Errorf("schema is outside the selected target")
			}
			qualified := ls + "." + strings.ToLower(rel)
			if _, bad := p.denied[strings.ToLower(rel)]; bad {
				return fmt.Errorf("table is denied by target policy")
			}
			if _, bad := p.denied[qualified]; bad {
				return fmt.Errorf("table is denied by target policy")
			}
			tables[qualified] = struct{}{}
		case "FuncCall":
			names := stringList(o["funcname"])
			if len(names) == 0 {
				return fmt.Errorf("unresolved function call")
			}
			name := strings.ToLower(names[len(names)-1])
			if _, bad := dangerousFunctions[name]; bad {
				return fmt.Errorf("function is not read-only")
			}
			for _, prefix := range []string{"lo_", "pg_advisory_", "pg_try_advisory_", "pg_write_", "pg_logical_emit_", "pg_replication_origin_", "dblink_"} {
				if strings.HasPrefix(name, prefix) {
					return fmt.Errorf("function is not read-only")
				}
			}
			if p.safeFunctions != nil {
				if _, ok := p.safeFunctions[name]; !ok {
					return fmt.Errorf("function capability has not been proven read-only")
				}
			}
			if len(names) > 1 && strings.ToLower(names[len(names)-2]) != "pg_catalog" {
				return fmt.Errorf("user-defined functions require explicit attestation")
			}
		case "ParamRef":
			n := intField(o, "number")
			if n < 1 {
				return fmt.Errorf("invalid PostgreSQL parameter")
			}
			params[n] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if paramCount >= 0 {
		if len(params) != paramCount {
			return nil, fmt.Errorf("PostgreSQL parameter count does not match placeholders")
		}
		for i := 1; i <= paramCount; i++ {
			if _, ok := params[i]; !ok {
				return nil, fmt.Errorf("PostgreSQL parameter placeholders must be contiguous")
			}
		}
	}
	list := make([]string, 0, len(tables))
	for x := range tables {
		list = append(list, x)
	}
	sort.Strings(list)
	sum := sha256.Sum256([]byte(trimmed))
	return &core.Validation{Fingerprint: hex.EncodeToString(sum[:12]), Tables: list, Cacheable: false}, nil
}

func walk(v any, fn func(string, any) error) error {
	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			if err := fn(k, v); err != nil {
				return err
			}
			if err := walk(v, fn); err != nil {
				return err
			}
		}
	case []any:
		for _, v := range x {
			if err := walk(v, fn); err != nil {
				return err
			}
		}
	}
	return nil
}
func object(v any) map[string]any                   { m, _ := v.(map[string]any); return m }
func stringField(m map[string]any, k string) string { v, _ := m[k].(string); return v }
func intField(m map[string]any, k string) int       { v, _ := m[k].(float64); return int(v) }
func stringList(v any) []string {
	var out []string
	a, _ := v.([]any)
	for _, x := range a {
		m := object(x)
		s := object(m["String"])
		if v := stringField(s, "sval"); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func lowerSet(v []string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, x := range v {
		m[strings.ToLower(x)] = struct{}{}
	}
	return m
}
