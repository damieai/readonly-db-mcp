package vectorproof

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalPGVectorPreAnalysisDependencies(t *testing.T) {
	db, _ := localPostgres(t)
	// Keep a single backend so the C tripwire's counter survives savepoint
	// rollback. It detects attempted execution even if no SQL write could commit.
	db.SetMaxOpenConns(1)
	fixture := filepath.Join(filepath.Dir(os.Getenv("READONLY_DB_MCP_PGVECTOR_PROOF")), "pgvector-test-fixture.so")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatal("build the separate pgvector native test fixture", err)
	}
	for _, q := range []string{
		`REVOKE SELECT ON reporting.curated FROM fixture_reader`,
		`GRANT USAGE ON SCHEMA mcp_proof TO fixture_reader; GRANT EXECUTE ON FUNCTION mcp_proof.bind(text),mcp_proof.expression(text) TO fixture_reader`,
		fmt.Sprintf(`CREATE FUNCTION mcp_proof.tripwire_count() RETURNS bigint AS '%s','mcp_test_tripwire_count' LANGUAGE C VOLATILE`, strings.ReplaceAll(fixture, "'", "''")),
		`CREATE TYPE reporting.label AS ENUM ('a','b'); CREATE DOMAIN reporting.positive AS integer CHECK (VALUE>0); CREATE TYPE reporting.pair AS (label reporting.label, value reporting.positive); CREATE TYPE reporting.span AS RANGE (subtype=integer); CREATE COLLATION reporting.sorted (provider=libc,locale='C')`,
		`CREATE INDEX items_lower ON reporting.items(lower(category) COLLATE reporting.sorted) WHERE id>0`,
		`CREATE TABLE reporting.partitioned(id integer, embedding vectors.vector(3)) PARTITION BY RANGE(id); CREATE TABLE reporting.part0 PARTITION OF reporting.partitioned FOR VALUES FROM (0) TO (100); GRANT SELECT ON reporting.partitioned, reporting.part0 TO fixture_reader`,
		`CREATE VIEW reporting.visible AS SELECT id,embedding FROM reporting.items WHERE id>0; GRANT SELECT ON reporting.visible TO fixture_reader`,
		`CREATE MATERIALIZED VIEW reporting.snapshot AS SELECT * FROM private.hidden; GRANT SELECT ON reporting.snapshot TO fixture_reader`,
		`ALTER TABLE reporting.items ENABLE ROW LEVEL SECURITY; CREATE POLICY scoped ON reporting.items USING(category='a' AND vectors.l2_distance(embedding,'[1,0,0]'::vectors.vector)<10)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	prepare := func(t *testing.T, tx *sql.Tx) *Snapshot {
		t.Helper()
		if _, err := tx.Exec(`SET LOCAL ROLE fixture_reader; SET LOCAL search_path=pg_catalog; SET LOCAL statement_timeout='15s'`); err != nil {
			t.Fatal(err)
		}
		s, err := Capture(ctx, tx, "vectors")
		if err != nil {
			t.Fatal(err)
		}
		p, err := ReviewedProfile()
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Verify(p); err != nil {
			t.Fatal(err)
		}
		return s
	}
	positive := []string{
		`SELECT id FROM reporting.items ORDER BY embedding <-> $1::vectors.vector LIMIT 2`,
		`WITH ranked AS (SELECT id, row_number() OVER (ORDER BY embedding <=> $1::vectors.vector) n FROM reporting.items) SELECT a.id FROM ranked a JOIN ranked b USING(id) WHERE b.n<3`,
		`SELECT category,avg(embedding),sum(id) FILTER (WHERE id>1) FROM reporting.items GROUP BY category`,
		`SELECT a.id FROM reporting.items a JOIN LATERAL (SELECT id FROM reporting.visible b ORDER BY b.embedding <-> a.embedding LIMIT 1) b ON true`,
		`SELECT embedding::vectors.halfvec(3) <#> $1::vectors.halfvec FROM reporting.items`,
		`SELECT ('a',1)::reporting.pair, '1'::reporting.positive, '{a,b}'::reporting.label[], '[1,3)'::reporting.span`,
		`SELECT id,category COLLATE reporting.sorted FROM reporting.items WHERE (id,category)>(0,'') ORDER BY category COLLATE reporting.sorted`,
		`SELECT * FROM reporting.partitioned WHERE id>0 ORDER BY embedding <-> $1::vectors.vector`,
		`SELECT id FROM reporting.items TABLESAMPLE SYSTEM(100)`,
		`SELECT * FROM reporting.snapshot`,
		`SELECT sum(id) OVER (ORDER BY id RANGE BETWEEN 1 PRECEDING AND 1 FOLLOWING) FROM reporting.items`,
	}
	for _, q := range positive {
		t.Run("advanced read", func(t *testing.T) {
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			s := prepare(t, tx)
			binding, err := s.Analyze(ctx, tx, q, "mcp_proof", []string{"reporting"}, nil)
			if err != nil {
				t.Fatal(q, err)
			}
			if strings.Contains(q, "RANGE BETWEEN") {
				var support uint32
				if err := tx.QueryRow(`SELECT 'pg_catalog.in_range(integer,integer,integer,boolean,boolean)'::regprocedure::oid`).Scan(&support); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, ref := range binding.Objects {
					found = found || ref == (Reference{"pg_proc", support})
				}
				if !found {
					t.Fatal("window RANGE support function missing from native binding")
				}
			}
			var path string
			if err = tx.QueryRow(`SHOW search_path`).Scan(&path); err != nil || path != "pg_catalog" {
				t.Fatal("search path was not restored", path, err)
			}
		})
	}
	tripwire := func(name, args, result string) string {
		return fmt.Sprintf(`CREATE FUNCTION reporting.%s(%s) RETURNS %s AS '%s','mcp_test_tripwire' LANGUAGE C IMMUTABLE; REVOKE ALL ON FUNCTION reporting.%s(%s) FROM PUBLIC`, name, args, result, strings.ReplaceAll(fixture, "'", "''"), name, args)
	}
	cases := []struct{ name, ddl, query, control string }{
		{"type input", tripwire("input", "cstring", "reporting.label") + `; UPDATE pg_catalog.pg_type SET typinput='reporting.input(cstring)'::regprocedure WHERE oid='reporting.label'::regtype`, `SELECT 'a'::reporting.label`, `SELECT mcp_proof.bind('SELECT ''a''::reporting.label')`},
		{"revoked type usage", tripwire("input", "cstring", "reporting.label") + `; UPDATE pg_catalog.pg_type SET typinput='reporting.input(cstring)'::regprocedure WHERE oid='reporting.label'::regtype; REVOKE USAGE ON TYPE reporting.label FROM PUBLIC`, `SELECT 'a'::reporting.label`, `SELECT mcp_proof.bind('SELECT ''a''::reporting.label')`},
		{"type modifier", tripwire("typmod", "cstring[]", "integer") + `; UPDATE pg_catalog.pg_type SET typmodin='reporting.typmod(cstring[])'::regprocedure WHERE oid='reporting.label'::regtype`, `SELECT 'a'::reporting.label(3)`, `SELECT mcp_proof.bind('SELECT ''a''::reporting.label(3)')`},
		{"domain", tripwire("domain_check", "integer", "boolean") + `; ALTER DOMAIN reporting.positive ADD CONSTRAINT hostile CHECK (reporting.domain_check(VALUE))`, `SELECT '1'::reporting.positive`, `SELECT reporting.domain_check(1)`},
		{"expression index", tripwire("index_value", "integer", "integer") + `; CREATE TABLE reporting.empty(id integer); GRANT SELECT ON reporting.empty TO fixture_reader; CREATE INDEX hostile ON reporting.empty(reporting.index_value(id))`, `SELECT * FROM reporting.empty`, `SELECT reporting.index_value(1)`},
		{"partial index", tripwire("predicate", "integer", "boolean") + `; CREATE TABLE reporting.empty(id integer); GRANT SELECT ON reporting.empty TO fixture_reader; CREATE INDEX hostile ON reporting.empty(id) WHERE reporting.predicate(id)`, `SELECT * FROM reporting.empty`, `SELECT reporting.predicate(1)`},
		{"RLS", tripwire("row_check", "integer", "boolean") + `; ALTER POLICY scoped ON reporting.items USING(reporting.row_check(id))`, `SELECT * FROM reporting.items`, `SELECT reporting.row_check(1)`},
		{"range canonical", `CREATE TYPE reporting.hostile_span; ` + tripwire("canonical", "reporting.hostile_span", "reporting.hostile_span") + `; CREATE TYPE reporting.hostile_span AS RANGE (subtype=integer,canonical=reporting.canonical)`, `SELECT '[1,3)'::reporting.hostile_span`, `SELECT mcp_proof.bind('SELECT ''[1,3)''::reporting.hostile_span')`},
		{"sampling handler", tripwire("sampler", "internal", "tsm_handler"), `SELECT * FROM reporting.items TABLESAMPLE reporting.sampler(10)`, `SELECT mcp_proof.bind('SELECT * FROM reporting.items TABLESAMPLE reporting.sampler(10)')`},
		{"subscript handler", tripwire("subscript", "internal", "internal") + `; UPDATE pg_catalog.pg_type SET typsubscript='reporting.subscript(internal)'::regprocedure WHERE oid='reporting.label'::regtype`, `SELECT ('a'::reporting.label)[1]`, `SELECT mcp_proof.bind('SELECT (''a''::reporting.label)[1]')`},
		{"hidden argument type", `CREATE TYPE private.hiddenlabel AS ENUM ('a'); ` + tripwire("input", "cstring", "private.hiddenlabel") + `; UPDATE pg_catalog.pg_type SET typinput='reporting.input(cstring)'::regprocedure WHERE oid='private.hiddenlabel'::regtype; CREATE FUNCTION reporting.hidden_argument(private.hiddenlabel) RETURNS integer LANGUAGE SQL IMMUTABLE AS 'SELECT 1'; REVOKE ALL ON FUNCTION reporting.hidden_argument(private.hiddenlabel) FROM PUBLIC`, `SELECT reporting.hidden_argument('a')`, `SELECT mcp_proof.bind('SELECT reporting.hidden_argument(''a'')')`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err = tx.Exec(tc.ddl); err != nil {
				t.Fatal(err)
			}
			s := prepare(t, tx)
			var before, after int64
			if err = tx.QueryRow(`SELECT mcp_proof.tripwire_count()`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Analyze(ctx, tx, tc.query, "mcp_proof", []string{"reporting"}, nil); err == nil {
				t.Fatal("unreviewed callback was admitted")
			}
			if err = tx.QueryRow(`SELECT mcp_proof.tripwire_count()`).Scan(&after); err != nil || after != before {
				t.Fatal("callback ran before rejection", before, after, err)
			}
			// The unsafe baseline must really reach the callback; an invalid test SQL
			// that merely errors before callback entry is not sufficient evidence.
			// Keep the reader for coercion baselines, proving that revoked type
			// USAGE / routine EXECUTE did not prevent these analysis callbacks.
			readerBaseline := tc.name == "type input" || tc.name == "revoked type usage" || tc.name == "type modifier" || tc.name == "hidden argument type"
			if !readerBaseline {
				if _, err = tx.Exec(`RESET ROLE`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = tx.Exec(`SAVEPOINT control`); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(tc.control); err == nil {
				t.Fatal("tripwire baseline unexpectedly succeeded")
			}
			if _, err = tx.Exec(`ROLLBACK TO SAVEPOINT control`); err != nil {
				t.Fatal(err)
			}
			if err = tx.QueryRow(`SELECT mcp_proof.tripwire_count()`).Scan(&after); err != nil || after != before+1 {
				t.Fatal("baseline did not execute callback", before, after, err)
			}
		})
	}
	t.Run("pinned builtin in RLS", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`ALTER POLICY scoped ON reporting.items USING(pg_catalog.set_config('proof.tripwire','yes',false)='yes')`); err != nil {
			t.Fatal(err)
		}
		s := prepare(t, tx)
		if _, err = s.Analyze(ctx, tx, `SELECT * FROM reporting.items`, "mcp_proof", []string{"reporting"}, nil); err == nil {
			t.Fatal("pg_depend omission hid a mutating builtin")
		}
	})
	t.Run("refresh rejects drift", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		s := prepare(t, tx)
		if _, err = tx.Exec(`RESET ROLE; ALTER FUNCTION vectors.l2_distance(vectors.vector,vectors.vector) SECURITY DEFINER; SET LOCAL ROLE fixture_reader`); err != nil {
			t.Fatal(err)
		}
		if _, err = s.Analyze(ctx, tx, `SELECT * FROM reporting.items`, "mcp_proof", []string{"reporting"}, nil); err == nil || s.Digest() != "" {
			t.Fatal("stale extension comparison retained authority")
		}
	})
}
