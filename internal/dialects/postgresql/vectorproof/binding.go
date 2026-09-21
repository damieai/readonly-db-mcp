package vectorproof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type Binding struct {
	Format          int         `json:"format"`
	PostgreSQLMajor int         `json:"postgresql_major"`
	SQLSHA256       string      `json:"sql_sha256"`
	DatabaseOID     uint32      `json:"database_oid"`
	RoleOID         uint32      `json:"role_oid"`
	Parameters      []uint32    `json:"parameters"`
	Objects         []Reference `json:"objects"`
}

func DecodeBinding(raw []byte) (*Binding, error) {
	if len(raw) > 1<<20 {
		return nil, errors.New("binding response byte limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var b Binding
	if decoder.Decode(&b) != nil || decoder.Decode(new(any)) != io.EOF || b.Format != 1 || b.PostgreSQLMajor != 16 || b.Objects == nil || len(b.Objects) > 8192 || len(b.Parameters) > 10000 {
		return nil, errors.New("invalid native binding response")
	}
	if digest, err := hex.DecodeString(b.SQLSHA256); err != nil || len(digest) != sha256.Size || b.DatabaseOID == 0 || b.RoleOID == 0 {
		return nil, errors.New("binding identity is incomplete")
	}
	seen := map[Reference]bool{}
	for _, ref := range b.Objects {
		if ref.OID == 0 || seen[ref] {
			return nil, errors.New("duplicate or invalid binding reference")
		}
		switch ref.Class {
		case "pg_proc", "pg_type", "pg_operator", "pg_class", "pg_collation", "pg_opfamily":
		default:
			return nil, errors.New("unknown binding object class")
		}
		seen[ref] = true
	}
	for _, oid := range b.Parameters {
		if oid == 0 || oid == 705 || !seen[Reference{"pg_type", oid}] {
			return nil, errors.New("unresolved binding parameter")
		}
	}
	return &b, nil
}

// CheckBinding verifies the analyzed expression objects. This is the expression
// component of the proof, not a complete admission decision: full relation,
// index and type dependencies and analysis-time callbacks need closure first.
func (s *Snapshot) CheckBinding(ctx context.Context, tx *sql.Tx, query string, b *Binding, allowedSchemas, deniedTables []string) error {
	if err := s.checkBindingIdentity(ctx, tx, query, b); err != nil {
		return err
	}
	d := &dependencyCheck{ctx: ctx, tx: tx, snapshot: s, seen: map[Reference]bool{}, allowed: map[string]bool{}, denied: map[string]bool{}}
	for _, schema := range allowedSchemas {
		d.allowed[schema] = true
	}
	for _, name := range deniedTables {
		d.denied[name] = true
	}
	for _, ref := range b.Objects {
		// This legacy expression-only API has no stored-tree helper. Analyze
		// is the entry point for preflight and recursive dependency closure.
		if ref.Class == "pg_collation" || ref.Class == "pg_opfamily" {
			if err := d.inspect(ref); err != nil {
				return err
			}
		}
	}
	return s.checkExpressionObjects(ctx, tx, b, allowedSchemas, deniedTables)
}

func (s *Snapshot) checkBindingIdentity(ctx context.Context, tx *sql.Tx, query string, b *Binding) error {
	if s.digest == "" || tx == nil || s.tx != tx || b == nil || b.Format != 1 || b.PostgreSQLMajor != 16 {
		return errors.New("binding requires a verified extension snapshot")
	}
	sum := sha256.Sum256([]byte(query))
	var roleOID uint32
	if b.DatabaseOID != s.databaseOID || b.SQLSHA256 != hex.EncodeToString(sum[:]) || tx.QueryRowContext(ctx, `SELECT oid FROM pg_catalog.pg_roles WHERE rolname=current_user`).Scan(&roleOID) != nil || roleOID != b.RoleOID {
		return errors.New("binding does not match the query and execution identity")
	}
	return nil
}

func (s *Snapshot) checkExpressionObjects(ctx context.Context, tx *sql.Tx, b *Binding, allowedSchemas, deniedTables []string) error {
	allowed, denied := map[string]bool{}, map[string]bool{}
	for _, schema := range allowedSchemas {
		allowed[schema] = true
	}
	for _, name := range deniedTables {
		denied[name] = true
	}
	for _, ref := range b.Objects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, ok := s.objects[ref]; ok {
			continue
		}
		switch ref.Class {
		case "pg_collation", "pg_opfamily":
			// Checked by the dependency inspector above.
		case "pg_class":
			var schema, name, kind string
			if tx.QueryRowContext(ctx, `SELECT n.nspname,c.relname,c.relkind FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.oid=$1`, ref.OID).Scan(&schema, &name, &kind) != nil || !allowed[schema] || denied[name] || denied[schema+"."+name] || len(kind) != 1 || !strings.Contains("rpvm", kind) {
				return errors.New("bound relation is outside the proven target")
			}
		case "pg_proc":
			var schema, name, volatility string
			var definer bool
			if tx.QueryRowContext(ctx, `SELECT n.nspname,p.proname,p.provolatile,p.prosecdef FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE p.oid=$1`, ref.OID).Scan(&schema, &name, &volatility, &definer) != nil || schema != "pg_catalog" || definer || !safeBuiltin(name, volatility) {
				return errors.New("bound routine is not reviewed")
			}
		case "pg_operator":
			var namespace string
			if tx.QueryRowContext(ctx, `SELECT n.nspname FROM pg_catalog.pg_operator o JOIN pg_catalog.pg_namespace n ON n.oid=o.oprnamespace WHERE o.oid=$1`, ref.OID).Scan(&namespace) != nil || namespace != "pg_catalog" {
				return errors.New("bound operator is not reviewed")
			}
		case "pg_type":
			var namespace string
			if tx.QueryRowContext(ctx, `SELECT n.nspname FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace WHERE t.oid=$1`, ref.OID).Scan(&namespace) != nil || namespace != "pg_catalog" {
				return errors.New("bound type is not reviewed")
			}
		default:
			return errors.New("unreviewed binding reference class")
		}
	}
	return nil
}

func safeBuiltin(name, volatility string) bool {
	for _, prefix := range []string{"lo_", "pg_advisory_", "pg_try_advisory_", "pg_write_", "pg_logical_emit_", "pg_replication_origin_", "dblink_"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	switch name {
	case "nextval", "setval", "set_config", "pg_cancel_backend", "pg_terminate_backend", "pg_reload_conf", "pg_rotate_logfile", "pg_create_restore_point", "pg_switch_wal", "pg_log_standby_snapshot", "pg_export_snapshot":
		return false
	case "random", "setseed", "clock_timestamp", "timeofday", "gen_random_uuid":
		return true
	}
	return volatility == "i" || volatility == "s"
}
