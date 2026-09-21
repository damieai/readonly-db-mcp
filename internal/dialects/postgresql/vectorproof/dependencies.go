package vectorproof

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const MaxDependencies = 8192
const MaxDependencyEdges = 32768

// Analyze checks the role's analysis surface before invoking PostgreSQL's
// analyzer, then closes the dependencies of the actual binding. The helper
// schema must be operator-owned and contain the reviewed bind(text) and
// expression(text) C functions. Their binary/deployment identity and the runtime
// role's privileges remain the caller's responsibility. This is not execution.
//
// The transaction must start with search_path=pg_catalog. On a database error
// the caller must roll it back; this function never commits or releases it.
func (s *Snapshot) Analyze(ctx context.Context, tx *sql.Tx, query, helperSchema string, allowedSchemas, deniedTables []string) (*Binding, error) {
	if len(query) > 1<<20 || strings.TrimSpace(query) == "" {
		return nil, errors.New("analysis SQL byte limit or empty statement")
	}
	if s.digest == "" || tx == nil || tx != s.tx || !schemaPattern.MatchString(helperSchema) {
		return nil, errors.New("analysis requires a verified snapshot and helper schema")
	}
	// A previous comparison cannot authorize changed extension objects, even in
	// the same transaction. Administrative concurrent DDL remains a trust boundary.
	fresh, err := Capture(ctx, tx, s.Schema)
	if err != nil {
		s.digest = ""
		return nil, err
	}
	profile, err := ReviewedProfile()
	if err != nil || fresh.Verify(profile) != nil {
		s.digest = ""
		return nil, errors.New("extension changed before analysis")
	}
	d := &dependencyCheck{ctx: ctx, tx: tx, snapshot: fresh, helper: pgx.Identifier{helperSchema}.Sanitize(), allowed: map[string]bool{}, denied: map[string]bool{}, seen: map[Reference]bool{}}
	for _, name := range allowedSchemas {
		d.allowed[name] = true
	}
	for _, name := range deniedTables {
		d.denied[name] = true
	}
	// Type callbacks do not necessarily require EXECUTE privilege. Check every
	// type visible to this role, including arrays, composites, domains and ranges,
	// before analysis can coerce a literal or invoke a subscripting/typmod handler.
	// Type USAGE/func EXECUTE revocation alone does not prevent input callbacks
	// during expression coercion. Include visible callable signatures too: an
	// argument type can live in a schema the reader cannot directly search.
	err = d.edges(`SELECT 'pg_type', t.oid::bigint FROM pg_catalog.pg_type t WHERE has_schema_privilege(current_user,t.typnamespace,'USAGE')
 UNION ALL SELECT 'pg_type',f::bigint FROM pg_catalog.pg_proc p CROSS JOIN LATERAL unnest(COALESCE(p.proallargtypes,p.proargtypes::oid[])||ARRAY[p.prorettype]) f WHERE p.oid>=16384 AND has_schema_privilege(current_user,p.pronamespace,'USAGE')
 UNION ALL SELECT 'pg_type',f::bigint FROM pg_catalog.pg_operator o CROSS JOIN LATERAL unnest(ARRAY[o.oprleft,o.oprright,o.oprresult]) f WHERE o.oid>=16384 AND has_schema_privilege(current_user,o.oprnamespace,'USAGE')`, nil)
	if err != nil {
		return nil, err
	}
	// TABLESAMPLE handlers can also run during analysis, without a normal scalar
	// call. Inventory visible handlers even when EXECUTE was revoked.
	if err := d.edges(`SELECT 'pg_proc',p.oid::bigint FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_type t ON t.oid=p.prorettype WHERE t.typname='tsm_handler' AND t.typnamespace='pg_catalog'::regnamespace AND has_schema_privilege(current_user,p.pronamespace,'USAGE')`, nil); err != nil {
		return nil, err
	}
	if err := d.close(); err != nil {
		return nil, err
	}
	// Prove readable target relations and their planner inputs before allowing
	// analysis of view/RLS expressions. No planner is used to obtain this graph.
	rows, err := tx.QueryContext(ctx, `SELECT c.oid::bigint,n.nspname,c.relname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p','v','m','f') AND n.nspname=ANY($1::text[]) AND has_table_privilege(current_user,c.oid,'SELECT') LIMIT 8193`, allowedSchemas)
	if err != nil {
		return nil, errors.New("read analysis relation inventory")
	}
	count := 0
	for rows.Next() {
		var oid uint32
		var schema, name string
		count++
		if rows.Scan(&oid, &schema, &name) != nil || count > MaxDependencies {
			rows.Close()
			return nil, errors.New("analysis relation inventory limit")
		}
		if !d.denied[name] && !d.denied[schema+"."+name] {
			if err := d.add(Reference{"pg_class", oid}); err != nil {
				rows.Close()
				return nil, err
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, errors.New("incomplete analysis relation inventory")
	}
	if err := d.close(); err != nil {
		return nil, err
	}
	path := "pg_catalog," + pgx.Identifier{s.Schema}.Sanitize()
	if _, err := tx.ExecContext(ctx, `SELECT pg_catalog.set_config('search_path',$1,true)`, path); err != nil {
		return nil, errors.New("set analysis search path")
	}
	defer func() { _, _ = tx.ExecContext(ctx, `SET LOCAL search_path=pg_catalog`) }()
	var raw string
	if err := tx.QueryRowContext(ctx, `SELECT `+d.helper+`.bind($1)`, query).Scan(&raw); err != nil {
		return nil, errors.New("native analysis failed")
	}
	b, err := DecodeBinding([]byte(raw))
	if err != nil {
		return nil, err
	}
	if err := fresh.checkBindingIdentity(ctx, tx, query, b); err != nil {
		return nil, err
	}
	for _, ref := range b.Objects {
		if err := d.add(ref); err != nil {
			return nil, err
		}
	}
	if err := d.close(); err != nil {
		return nil, err
	}
	return b, nil
}

type dependencyCheck struct {
	ctx                                     context.Context
	tx                                      *sql.Tx
	snapshot                                *Snapshot
	helper                                  string
	allowed, denied                         map[string]bool
	seen                                    map[Reference]bool
	queue                                   []Reference
	edgeCount, expressionBytes, expressions int
}

func (d *dependencyCheck) add(ref Reference) error {
	if ref.OID == 0 {
		return nil
	}
	d.edgeCount++
	if d.edgeCount > MaxDependencyEdges {
		return errors.New("dependency edge limit")
	}
	if !d.seen[ref] {
		if len(d.seen) >= MaxDependencies {
			return errors.New("dependency object limit")
		}
		d.seen[ref] = true
		d.queue = append(d.queue, ref)
	}
	return nil
}

// Catalog queries return only class-qualified OIDs. Close each result before
// following another edge: database/sql must not open a second lease for proof.
func (d *dependencyCheck) edges(query string, oid *uint32) error {
	var args []any
	if oid != nil {
		args = []any{*oid}
	}
	rows, err := d.tx.QueryContext(d.ctx, query+` LIMIT 32769`, args...)
	if err != nil {
		return errors.New("read catalog dependencies")
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		if count > MaxDependencyEdges {
			return errors.New("dependency row limit")
		}
		var ref Reference
		if rows.Scan(&ref.Class, &ref.OID) != nil {
			return errors.New("invalid catalog dependency")
		}
		if err := d.add(ref); err != nil {
			return err
		}
	}
	if rows.Err() != nil {
		return errors.New("incomplete catalog dependencies")
	}
	return nil
}

func (d *dependencyCheck) expression(query string, oid uint32) error {
	rows, err := d.tx.QueryContext(d.ctx, query+` LIMIT 1025`, oid)
	if err != nil {
		return errors.New("read catalog expressions")
	}
	var expressions []string
	count := 0
	for rows.Next() {
		count++
		if count > 1024 {
			rows.Close()
			return errors.New("catalog expression row limit")
		}
		var raw sql.NullString
		if rows.Scan(&raw) != nil {
			rows.Close()
			return errors.New("invalid catalog expression")
		}
		if !raw.Valid || raw.String == "" || raw.String == "<>" {
			continue
		}
		d.expressions++
		d.expressionBytes += len(raw.String)
		if d.expressions > 1024 || len(raw.String) > 1<<20 || d.expressionBytes > MaxCatalogBytes {
			rows.Close()
			return errors.New("catalog expression limit")
		}
		expressions = append(expressions, raw.String)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return errors.New("incomplete catalog expressions")
	}
	for _, raw := range expressions {
		var encoded string
		if d.tx.QueryRowContext(d.ctx, `SELECT `+d.helper+`.expression($1)`, raw).Scan(&encoded) != nil {
			return errors.New("inspect stored expression")
		}
		b, err := DecodeBinding([]byte(encoded))
		if err != nil {
			return err
		}
		if err = d.snapshot.checkBindingIdentity(d.ctx, d.tx, raw, b); err != nil {
			return err
		}
		for _, ref := range b.Objects {
			if err = d.add(ref); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *dependencyCheck) close() error {
	for len(d.queue) > 0 {
		if err := d.ctx.Err(); err != nil {
			return err
		}
		ref := d.queue[0]
		d.queue = d.queue[1:]
		if err := d.inspect(ref); err != nil {
			return fmt.Errorf("unproven %s dependency: %w", ref.Class, err)
		}
	}
	return nil
}

func (d *dependencyCheck) inspect(ref Reference) error {
	oid := ref.OID
	_, extension := d.snapshot.objects[ref]
	switch ref.Class {
	case "pg_proc":
		var schema, name, volatility string
		var definer bool
		if d.tx.QueryRowContext(d.ctx, `SELECT n.nspname,p.proname,p.provolatile,p.prosecdef FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE p.oid=$1`, oid).Scan(&schema, &name, &volatility, &definer) != nil {
			return errors.New("routine missing")
		}
		// PostgreSQL 16's two native sampling handlers are volatile because they
		// describe a sampling method; they do not mutate persistent state.
		sampling := (oid == 3313 && name == "bernoulli") || (oid == 3314 && name == "system")
		if !extension && (oid >= 16384 || schema != "pg_catalog" || definer || (!safeBuiltin(name, volatility) && !sampling)) {
			return errors.New("routine is not reviewed")
		}
		return d.edges(`SELECT 'pg_proc',p.prosupport::bigint FROM pg_catalog.pg_proc p WHERE p.oid=$1
 UNION ALL SELECT 'pg_proc',f::bigint FROM pg_catalog.pg_aggregate a CROSS JOIN LATERAL unnest(ARRAY[a.aggtransfn,a.aggfinalfn,a.aggcombinefn,a.aggserialfn,a.aggdeserialfn,a.aggmtransfn,a.aggminvtransfn,a.aggmfinalfn]::oid[]) f WHERE a.aggfnoid=$1`, &oid)
	case "pg_type":
		var schema string
		var defined bool
		if d.tx.QueryRowContext(d.ctx, `SELECT n.nspname,t.typisdefined FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace WHERE t.oid=$1`, oid).Scan(&schema, &defined) != nil || !defined {
			return errors.New("type missing or incomplete")
		}
		if oid < 16384 && schema == "pg_catalog" {
			return nil
		}
		if err := d.edges(typeEdges, &oid); err != nil {
			return err
		}
		return d.expression(`SELECT conbin::text FROM pg_catalog.pg_constraint WHERE contypid=$1`, oid)
	case "pg_operator":
		var schema string
		if d.tx.QueryRowContext(d.ctx, `SELECT n.nspname FROM pg_catalog.pg_operator o JOIN pg_catalog.pg_namespace n ON n.oid=o.oprnamespace WHERE o.oid=$1`, oid).Scan(&schema) != nil || (!extension && (schema != "pg_catalog" || oid >= 16384)) {
			return errors.New("operator is not reviewed")
		}
		return d.edges(`SELECT 'pg_proc',f::bigint FROM pg_catalog.pg_operator o CROSS JOIN LATERAL unnest(ARRAY[o.oprcode,o.oprrest,o.oprjoin]::oid[]) f WHERE o.oid=$1
 UNION ALL SELECT 'pg_type',t::bigint FROM pg_catalog.pg_operator o CROSS JOIN LATERAL unnest(ARRAY[o.oprleft,o.oprright,o.oprresult]) t WHERE o.oid=$1`, &oid)
	case "pg_class":
		var schema, name, kind string
		if d.tx.QueryRowContext(d.ctx, `SELECT n.nspname,c.relname,c.relkind FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.oid=$1`, oid).Scan(&schema, &name, &kind) != nil || !d.allowed[schema] || d.denied[name] || d.denied[schema+"."+name] || len(kind) != 1 || !strings.Contains("rpvm", kind) {
			return errors.New("relation outside proven scope")
		}
		if err := d.edges(relationEdges, &oid); err != nil {
			return err
		}
		return d.expression(relationExpressions, oid)
	case "pg_index":
		if err := d.edges(indexEdges, &oid); err != nil {
			return err
		}
		return d.expression(`SELECT indexprs::text FROM pg_catalog.pg_index WHERE indexrelid=$1 UNION ALL SELECT indpred::text FROM pg_catalog.pg_index WHERE indexrelid=$1`, oid)
	case "pg_am":
		var name string
		if d.tx.QueryRowContext(d.ctx, `SELECT amname FROM pg_catalog.pg_am WHERE oid=$1`, oid).Scan(&name) != nil {
			return errors.New("access method missing")
		}
		if !extension && (oid >= 16384 || !strings.Contains("|heap|btree|hash|gin|gist|spgist|brin|", "|"+name+"|")) {
			return errors.New("access method not reviewed")
		}
		// Core handler routines are trusted through the server build. Extension
		// handlers are pinned by the complete extension profile.
		return nil
	case "pg_opclass":
		if err := d.objectNamespace(ref, "opcnamespace", extension); err != nil {
			return err
		}
		return d.edges(`SELECT 'pg_opfamily',opcfamily::bigint FROM pg_catalog.pg_opclass WHERE oid=$1 UNION ALL SELECT 'pg_type',opcintype::bigint FROM pg_catalog.pg_opclass WHERE oid=$1 UNION ALL SELECT 'pg_type',opckeytype::bigint FROM pg_catalog.pg_opclass WHERE oid=$1`, &oid)
	case "pg_opfamily":
		if err := d.objectNamespace(ref, "opfnamespace", extension); err != nil {
			return err
		}
		return d.edges(`SELECT 'pg_proc',amproc::bigint FROM pg_catalog.pg_amproc WHERE amprocfamily=$1 UNION ALL SELECT 'pg_operator',amopopr::bigint FROM pg_catalog.pg_amop WHERE amopfamily=$1`, &oid)
	case "pg_collation":
		var provider string
		if d.tx.QueryRowContext(d.ctx, `SELECT collprovider FROM pg_catalog.pg_collation WHERE oid=$1`, oid).Scan(&provider) != nil || (provider != "d" && provider != "c" && provider != "i") {
			return errors.New("collation provider not reviewed")
		}
		return nil
	default:
		return errors.New("unknown dependency class")
	}
}

func (d *dependencyCheck) objectNamespace(ref Reference, column string, extension bool) error {
	// Class and column are fixed by the switch above, never supplied by a query.
	var namespace string
	if d.tx.QueryRowContext(d.ctx, `SELECT n.nspname FROM pg_catalog.`+ref.Class+` o JOIN pg_catalog.pg_namespace n ON n.oid=o.`+column+` WHERE o.oid=$1`, ref.OID).Scan(&namespace) != nil || (!extension && (namespace != "pg_catalog" || ref.OID >= 16384)) {
		return errors.New("index support object not reviewed")
	}
	return nil
}

const typeEdges = `SELECT 'pg_proc',f::bigint FROM pg_catalog.pg_type t CROSS JOIN LATERAL unnest(ARRAY[t.typinput,t.typoutput,t.typreceive,t.typsend,t.typmodin,t.typmodout,t.typanalyze,t.typsubscript]::oid[]) f WHERE t.oid=$1
 UNION ALL SELECT 'pg_type',f::bigint FROM pg_catalog.pg_type t CROSS JOIN LATERAL unnest(ARRAY[t.typelem,t.typbasetype]) f WHERE t.oid=$1
 UNION ALL SELECT 'pg_type',a.atttypid::bigint FROM pg_catalog.pg_type t JOIN pg_catalog.pg_attribute a ON a.attrelid=t.typrelid WHERE t.oid=$1 AND a.attnum>0 AND NOT a.attisdropped
 UNION ALL SELECT 'pg_collation',a.attcollation::bigint FROM pg_catalog.pg_type t JOIN pg_catalog.pg_attribute a ON a.attrelid=t.typrelid WHERE t.oid=$1 AND a.attnum>0 AND NOT a.attisdropped
 UNION ALL SELECT 'pg_type',rngsubtype::bigint FROM pg_catalog.pg_range WHERE rngtypid=$1 OR rngmultitypid=$1
 UNION ALL SELECT 'pg_proc',f::bigint FROM pg_catalog.pg_range r CROSS JOIN LATERAL unnest(ARRAY[r.rngcanonical,r.rngsubdiff]::oid[]) f WHERE r.rngtypid=$1 OR r.rngmultitypid=$1
 UNION ALL SELECT 'pg_opclass',rngsubopc::bigint FROM pg_catalog.pg_range WHERE rngtypid=$1 OR rngmultitypid=$1
 UNION ALL SELECT 'pg_collation',rngcollation::bigint FROM pg_catalog.pg_range WHERE rngtypid=$1 OR rngmultitypid=$1`

const relationEdges = `SELECT 'pg_type',atttypid::bigint FROM pg_catalog.pg_attribute WHERE attrelid=$1 AND attnum>0 AND NOT attisdropped
 UNION ALL SELECT 'pg_am',relam::bigint FROM pg_catalog.pg_class WHERE oid=$1
 UNION ALL SELECT 'pg_index',indexrelid::bigint FROM pg_catalog.pg_index WHERE indrelid=$1
 UNION ALL SELECT 'pg_class',inhrelid::bigint FROM pg_catalog.pg_inherits WHERE inhparent=$1
 UNION ALL SELECT 'pg_opclass',f::bigint FROM pg_catalog.pg_partitioned_table p CROSS JOIN LATERAL unnest(p.partclass) f WHERE p.partrelid=$1
 UNION ALL SELECT 'pg_collation',f::bigint FROM pg_catalog.pg_partitioned_table p CROSS JOIN LATERAL unnest(p.partcollation) f WHERE p.partrelid=$1`

const relationExpressions = `SELECT r.ev_action::text FROM pg_catalog.pg_rewrite r JOIN pg_catalog.pg_class c ON c.oid=r.ev_class WHERE c.oid=$1 AND c.relkind='v' AND r.ev_type='1'
 UNION ALL SELECT stxexprs::text FROM pg_catalog.pg_statistic_ext WHERE stxrelid=$1
 UNION ALL SELECT conbin::text FROM pg_catalog.pg_constraint WHERE conrelid=$1 AND contype='c'
 UNION ALL SELECT p.polqual::text FROM pg_catalog.pg_policy p JOIN pg_catalog.pg_class c ON c.oid=p.polrelid WHERE c.oid=$1 AND c.relrowsecurity AND p.polcmd IN ('r','*') AND (0=ANY(p.polroles) OR current_user::regrole::oid=ANY(p.polroles))
 UNION ALL SELECT partexprs::text FROM pg_catalog.pg_partitioned_table WHERE partrelid=$1`

const indexEdges = `SELECT 'pg_am',c.relam::bigint FROM pg_catalog.pg_class c WHERE c.oid=$1
 UNION ALL SELECT 'pg_opclass',f::bigint FROM pg_catalog.pg_index i CROSS JOIN LATERAL unnest(i.indclass) f WHERE i.indexrelid=$1
 UNION ALL SELECT 'pg_collation',f::bigint FROM pg_catalog.pg_index i CROSS JOIN LATERAL unnest(i.indcollation) f WHERE i.indexrelid=$1`
