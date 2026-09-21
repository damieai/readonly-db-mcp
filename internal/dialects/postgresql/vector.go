package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/dialects/postgresql/vectorproof"
)

func attestVector(ctx context.Context, tx *sql.Tx, c *config.TargetConfig) (*vectorproof.Snapshot, error) {
	v := c.PostgreSQL.PGVector
	var count int
	// Only the configured deployment owner may create/replace objects in these
	// two namespaces. Superusers remain a trusted administrative boundary.
	if tx.QueryRowContext(ctx, `SELECT count(*) FROM pg_catalog.pg_namespace n JOIN pg_catalog.pg_roles r ON r.oid=n.nspowner WHERE n.nspname=ANY($1::text[]) AND r.rolname=$2 AND NOT EXISTS (SELECT 1 FROM pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) a WHERE a.privilege_type='CREATE' AND a.grantee<>n.nspowner)`, []string{v.Schema, v.HelperSchema}, v.Owner).Scan(&count) != nil || count != 2 {
		return nil, errors.New("pgvector namespaces are not controlled by the deployment owner")
	}
	s, err := vectorproof.Capture(ctx, tx, v.Schema)
	if err != nil {
		return nil, err
	}
	p, err := vectorproof.ReviewedProfile()
	if err != nil {
		return nil, err
	}
	if err = s.Verify(p); err != nil {
		return nil, err
	}
	if tx.QueryRowContext(ctx, `SELECT count(*) FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_roles r ON r.oid=e.extowner WHERE e.oid=$1 AND r.rolname=$2`, s.ExtensionOID, v.Owner).Scan(&count) != nil || count != 1 {
		return nil, errors.New("pgvector extension owner mismatch")
	}
	if tx.QueryRowContext(ctx, `SELECT count(*) FROM pg_catalog.pg_depend d JOIN pg_catalog.pg_proc p ON d.classid='pg_catalog.pg_proc'::regclass AND p.oid=d.objid JOIN pg_catalog.pg_roles r ON r.oid=p.proowner WHERE d.refclassid='pg_catalog.pg_extension'::regclass AND d.refobjid=$1 AND d.deptype='e' AND r.rolname<>$2`, s.ExtensionOID, v.Owner).Scan(&count) != nil || count != 0 {
		return nil, errors.New("pgvector routine owner mismatch")
	}
	if tx.QueryRowContext(ctx, `WITH members AS (SELECT classid,objid FROM pg_catalog.pg_depend WHERE refclassid='pg_catalog.pg_extension'::regclass AND refobjid=$1 AND deptype='e'), owners AS (
 SELECT t.typowner AS owner FROM pg_catalog.pg_type t WHERE EXISTS(SELECT 1 FROM members m WHERE m.classid='pg_catalog.pg_type'::regclass AND (m.objid=t.oid OR m.objid=t.typelem))
 UNION ALL SELECT o.oprowner FROM pg_catalog.pg_operator o JOIN members m ON m.classid='pg_catalog.pg_operator'::regclass AND m.objid=o.oid
 UNION ALL SELECT o.opcowner FROM pg_catalog.pg_opclass o JOIN members m ON m.classid='pg_catalog.pg_opclass'::regclass AND m.objid=o.oid
 UNION ALL SELECT o.opfowner FROM pg_catalog.pg_opfamily o JOIN members m ON m.classid='pg_catalog.pg_opfamily'::regclass AND m.objid=o.oid)
 SELECT count(*) FROM owners o JOIN pg_catalog.pg_roles r ON r.oid=o.owner WHERE r.rolname<>$2`, s.ExtensionOID, v.Owner).Scan(&count) != nil || count != 0 {
		return nil, errors.New("pgvector type or operator owner mismatch")
	}
	var helpers []uint32
	for _, name := range []string{"bind", "expression"} {
		var oid uint32
		err = tx.QueryRowContext(ctx, `SELECT p.oid FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace JOIN pg_catalog.pg_roles r ON r.oid=p.proowner JOIN pg_catalog.pg_language l ON l.oid=p.prolang WHERE n.nspname=$1 AND p.proname=$2 AND p.proargtypes='25'::oidvector AND p.prorettype=25 AND p.pronargs=1 AND p.pronargdefaults=0 AND p.provariadic=0 AND p.prosupport=0 AND p.proallargtypes IS NULL AND p.proargmodes IS NULL AND p.prokind='f' AND NOT p.prosecdef AND NOT p.proleakproof AND NOT p.proretset AND p.proisstrict AND p.provolatile='v' AND p.proparallel='u' AND p.proconfig IS NULL AND l.lanname='c' AND r.rolname=$3 AND p.probin=$4 AND p.prosrc=$5 AND has_function_privilege(current_user,p.oid,'EXECUTE')`, v.HelperSchema, name, v.Owner, v.HelperLibrary, "mcp_vector_"+name).Scan(&oid)
		if err != nil {
			return nil, errors.New("pgvector helper definition, owner or permission mismatch")
		}
		helpers = append(helpers, oid)
	}
	if err = s.CheckExecuteGrants(ctx, tx, helpers...); err != nil {
		return nil, err
	}
	return s, nil
}

func (t *Target) prepareVector(ctx context.Context, tx *sql.Tx, r core.QueryRequest) error {
	if t.cfg.PostgreSQL.PGVector == nil {
		if r.PostgreSQLOptions != nil {
			return errors.New("postgresql_options require an enabled pgvector profile")
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL search_path=pg_catalog`); err != nil {
		return sanitize(err)
	}
	id, err := verifyIdentityOn(ctx, tx, t.cfg)
	if err != nil {
		return err
	}
	if id.vector.DenseTypeOIDs() != t.vectorTypes {
		return errors.New("vector type identities changed; reopen the target")
	}
	b, err := id.vector.Analyze(ctx, tx, r.SQL, t.cfg.PostgreSQL.PGVector.HelperSchema, t.cfg.AllowedSchemas, t.cfg.DeniedTables)
	if err != nil {
		return err
	}
	if len(b.Parameters) != len(r.Parameters) {
		return errors.New("native vector parameter count mismatch")
	}
	path := "pg_catalog," + pgx.Identifier{t.cfg.PostgreSQL.PGVector.Schema}.Sanitize()
	if _, err = tx.ExecContext(ctx, `SELECT pg_catalog.set_config('search_path',$1,true)`, path); err != nil {
		return sanitize(err)
	}
	if err = t.applyVectorOptions(ctx, tx, r.PostgreSQLOptions); err != nil {
		return err
	}
	// Charge proof and admission against the original request deadline. This
	// never renews a batch's timeout for a later statement.
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline) - t.cfg.PostgreSQL.StatementTimeoutMargin
		if remaining <= 0 {
			return context.DeadlineExceeded
		}
		_, err = tx.ExecContext(ctx, `SELECT pg_catalog.set_config('statement_timeout',$1,true)`, strconv.FormatInt(max(1, remaining.Milliseconds()), 10))
	}
	if err != nil {
		return sanitize(err)
	}
	return nil
}

func registerVectorText(ctx context.Context, conn *pgx.Conn, schema string) error {
	rows, err := conn.Query(ctx, `SELECT t.oid,t.typname FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname=$1 AND t.typname IN ('vector','halfvec')`, schema)
	if err != nil {
		return errors.New("read vector text codec OIDs")
	}
	defer rows.Close()
	for rows.Next() {
		var oid uint32
		var name string
		if rows.Scan(&oid, &name) != nil {
			return errors.New("read vector type identity")
		}
		conn.TypeMap().RegisterType(&pgtype.Type{Name: name, OID: oid, Codec: pgtype.TextCodec{}})
	}
	return rows.Err()
}

func normalizeVector(value any, kind string, maxBytes int) (any, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	var raw []byte
	switch v := value.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return nil, false, errors.New("unexpected vector result codec")
	}
	if len(raw) > maxBytes {
		return map[string]any{"type": kind, "truncated": true}, true, nil
	}
	var values []float64
	if strings.Contains(string(raw), "null") || json.Unmarshal(raw, &values) != nil || len(values) == 0 || len(values) > 16000 {
		return nil, false, errors.New("invalid dense vector result")
	}
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, false, errors.New("non-finite dense vector result")
		}
	}
	return values, false, nil
}
