package vectorproof

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func localPostgres(t *testing.T) (*sql.DB, string) {
	t.Helper()
	bin := os.Getenv("READONLY_DB_MCP_PG_BIN")
	helper := os.Getenv("READONLY_DB_MCP_PGVECTOR_PROOF")
	if bin == "" || helper == "" {
		t.Skip("local PostgreSQL 16 and native proof helper are not configured")
	}
	if !filepath.IsAbs(bin) || !filepath.IsAbs(helper) {
		t.Fatal("fixture paths must be absolute")
	}
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	init := exec.Command(filepath.Join(bin, "initdb"), "-D", data, "-U", "fixture_owner", "--auth=trust", "--no-sync", "--encoding=UTF8", "--locale=C")
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("initdb: %v: %s", err, out)
	}
	cmd := exec.Command(filepath.Join(bin, "postgres"), "-D", data, "-k", dir, "-h", "", "-c", "fsync=off", "-c", "max_connections=12")
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})
	cfg, err := pgx.ParseConfig("postgres://fixture_owner@localhost/postgres?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Host = dir
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for db.PingContext(ctx) != nil {
		if ctx.Err() != nil {
			t.Fatal("local PostgreSQL did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, q := range []string{
		`CREATE SCHEMA vectors; CREATE EXTENSION vector WITH SCHEMA vectors VERSION '0.8.2'`,
		`CREATE SCHEMA reporting; CREATE SCHEMA private; CREATE SCHEMA mcp_proof`,
		`CREATE ROLE fixture_reader LOGIN; REVOKE ALL ON DATABASE postgres FROM PUBLIC; REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT CONNECT ON DATABASE postgres TO fixture_reader; GRANT USAGE ON SCHEMA reporting,vectors TO fixture_reader`,
		`CREATE TABLE reporting.items(id integer PRIMARY KEY, embedding vectors.vector(3), category text); INSERT INTO reporting.items VALUES (1,'[1,0,0]','a'),(2,'[0,1,0]','a'),(3,'[0,0,1]','b'); GRANT SELECT ON reporting.items TO fixture_reader`,
		`CREATE INDEX ON reporting.items USING hnsw (embedding vectors.vector_l2_ops)`,
		`CREATE TABLE private.hidden(id integer); CREATE VIEW reporting.curated AS SELECT * FROM private.hidden; GRANT SELECT ON reporting.curated TO fixture_reader`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	// This is an isolated analysis fixture, not a production helper grant.
	q := fmt.Sprintf(`CREATE FUNCTION mcp_proof.bind(text) RETURNS text AS '%s','mcp_vector_bind' LANGUAGE C STRICT VOLATILE PARALLEL UNSAFE; REVOKE ALL ON FUNCTION mcp_proof.bind(text) FROM PUBLIC`, strings.ReplaceAll(helper, "'", "''"))
	if _, err := db.ExecContext(ctx, q); err != nil {
		t.Fatal(err)
	}
	return db, dir
}

func catalogTx(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.Exec(`SET LOCAL search_path=pg_catalog; SET LOCAL statement_timeout='10s'`); err != nil {
		t.Fatal(err)
	}
	return tx
}

func TestLocalPGVectorCatalogAndBinding(t *testing.T) {
	db, _ := localPostgres(t)
	ctx := context.Background()
	tx := catalogTx(t, db)
	snapshot, err := Capture(ctx, tx, "vectors")
	if err != nil {
		t.Fatal(err)
	}
	if output := os.Getenv("READONLY_DB_MCP_PGVECTOR_RECORD_PROFILE"); output != "" {
		// Maintainer-only capture from this freshly initialized source-built fixture.
		data, _ := json.MarshalIndent(snapshot.Profile(), "", "  ")
		data = append(data, '\n')
		if err := os.WriteFile(output, data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("captured %d objects for manual review", len(snapshot.objects))
		return
	}
	profile, err := ReviewedProfile()
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Verify(profile); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`SET LOCAL ROLE fixture_reader`); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.CheckExecuteGrants(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`RESET ROLE; SET LOCAL search_path=pg_catalog,vectors`); err != nil {
		t.Fatal(err)
	}
	positive := []string{
		`SELECT id FROM reporting.items ORDER BY embedding <-> $1::vectors.vector LIMIT 2`,
		`SELECT id,embedding OPERATOR(vectors.<=>) $1::vectors.vector FROM reporting.items`,
		`SELECT vectors.l2_distance(embedding, $1::vectors.vector) FROM reporting.items`,
		`WITH ranked AS (SELECT id,embedding <#> $1::vectors.vector d,rank() OVER (ORDER BY id) r FROM reporting.items WHERE category=$2) SELECT a.id,b.r FROM ranked a JOIN ranked b USING(id) ORDER BY a.d`,
		`SELECT category,avg(embedding) FROM reporting.items GROUP BY category`,
		`SELECT a.id FROM reporting.items a JOIN LATERAL (SELECT id FROM reporting.items b ORDER BY b.embedding <-> a.embedding LIMIT 1) b ON true`,
		`SELECT embedding::vectors.halfvec(3) <-> $1::vectors.halfvec FROM reporting.items`,
		`SELECT ARRAY[1,2,3]::vectors.vector <-> '[1,2,3]'::vectors.vector`,
		`WITH RECURSIVE n AS (SELECT 1 i UNION ALL SELECT i+1 FROM n WHERE i<3) SELECT jsonb_build_object('id',i,'text','DROP TABLE is text') FROM n`,
		`SELECT sum(id) FILTER (WHERE category='a'), percentile_cont(0.5) WITHIN GROUP (ORDER BY id) FROM reporting.items`,
	}
	for _, q := range positive {
		var raw string
		if err := tx.QueryRowContext(ctx, `SELECT mcp_proof.bind($1)`, q).Scan(&raw); err != nil {
			t.Fatalf("bind %s: %v", q, err)
		}
		binding, err := DecodeBinding([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := snapshot.CheckBinding(ctx, tx, q, binding, []string{"reporting"}, nil); err != nil {
			t.Fatalf("proof %s: %v", q, err)
		}
		if err := snapshot.CheckBinding(ctx, tx, q+" ", binding, []string{"reporting"}, nil); err == nil {
			t.Fatal("proof was reused for different SQL bytes")
		}
		wrong := *binding
		wrong.RoleOID++
		if err := snapshot.CheckBinding(ctx, tx, q, &wrong, []string{"reporting"}, nil); err == nil {
			t.Fatal("proof was reused for another role")
		}
		wrong = *binding
		wrong.DatabaseOID++
		if err := snapshot.CheckBinding(ctx, tx, q, &wrong, []string{"reporting"}, nil); err == nil {
			t.Fatal("proof was reused for another database")
		}
		// Native binding resolves the exact procedure behind each operator.
		for _, ref := range binding.Objects {
			if ref.Class == "pg_operator" {
				var proc uint32
				if err := tx.QueryRow(`SELECT oprcode::oid FROM pg_catalog.pg_operator WHERE oid=$1`, ref.OID).Scan(&proc); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, r := range binding.Objects {
					found = found || r == (Reference{"pg_proc", proc})
				}
				if !found {
					t.Fatal("bound operator procedure missing")
				}
			}
		}
	}
	var raw string
	if err := tx.QueryRow(`SELECT mcp_proof.bind($1)`, `SELECT * FROM reporting.curated`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	b, err := DecodeBinding([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshot.CheckBinding(ctx, tx, `SELECT * FROM reporting.curated`, b, []string{"reporting"}, nil); err == nil {
		t.Fatal("view expansion escaped relation scope")
	}
	tx.Rollback()
	t.Run("reader and RLS", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		for _, q := range []string{
			`SET LOCAL search_path=pg_catalog; SET LOCAL statement_timeout='10s'`,
			`ALTER TABLE reporting.items ENABLE ROW LEVEL SECURITY; CREATE POLICY scoped ON reporting.items USING(category='a' AND vectors.l2_distance(embedding,'[1,0,0]'::vectors.vector)<10)`,
			`GRANT USAGE ON SCHEMA mcp_proof TO fixture_reader; GRANT EXECUTE ON FUNCTION mcp_proof.bind(text) TO fixture_reader`,
		} {
			if _, err := tx.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
		current, err := Capture(ctx, tx, "vectors")
		if err != nil {
			t.Fatal(err)
		}
		if err := current.Verify(profile); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`SET LOCAL ROLE fixture_reader; SET LOCAL search_path=pg_catalog,vectors`); err != nil {
			t.Fatal(err)
		}
		query := `SELECT id FROM reporting.items ORDER BY embedding <-> $1::vectors.vector LIMIT 2`
		var raw string
		if err := tx.QueryRow(`SELECT mcp_proof.bind($1)`, query).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		b, err := DecodeBinding([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := current.CheckBinding(ctx, tx, query, b, []string{"reporting"}, nil); err != nil {
			t.Fatal(err)
		}
		var rlsFunction uint32
		if err := tx.QueryRow(`SELECT 'vectors.l2_distance(vectors.vector,vectors.vector)'::regprocedure::oid`).Scan(&rlsFunction); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, ref := range b.Objects {
			found = found || ref == (Reference{"pg_proc", rlsFunction})
		}
		if !found {
			t.Fatal("RLS dependency disappeared from binding proof")
		}
		if _, err := tx.Exec(`RESET ROLE; CREATE FUNCTION private.policy(integer) RETURNS boolean LANGUAGE SQL STABLE AS 'SELECT true'; ALTER POLICY scoped ON reporting.items USING(private.policy(id)); SET LOCAL ROLE fixture_reader`); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(`SELECT mcp_proof.bind($1)`, query).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		b, err = DecodeBinding([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := current.CheckBinding(ctx, tx, query, b, []string{"reporting"}, nil); err == nil {
			t.Fatal("unreviewed RLS function was accepted")
		}
	})

	for _, mutation := range []string{
		`ALTER FUNCTION vectors.l2_distance(vectors.vector,vectors.vector) SECURITY DEFINER`,
		`ALTER FUNCTION vectors.l2_distance(vectors.vector,vectors.vector) VOLATILE`,
		`UPDATE pg_catalog.pg_proc SET prosecdef=true WHERE oid='vectors.avg(vectors.vector)'::regprocedure`,
		`ALTER EXTENSION vector DROP FUNCTION vectors.l2_distance(vectors.vector,vectors.vector)`,
		`CREATE FUNCTION vectors.unreviewed(integer) RETURNS integer LANGUAGE SQL IMMUTABLE AS 'SELECT $1'; ALTER EXTENSION vector ADD FUNCTION vectors.unreviewed(integer)`,
		`ALTER OPERATOR FAMILY vectors.vector_l2_ops USING hnsw ADD FUNCTION 2 (vectors.vector,vectors.vector) vectors.vector_norm(vectors.vector)`,
		`UPDATE pg_catalog.pg_type SET typinput='pg_catalog.textin'::regproc WHERE oid='vectors.vector'::regtype`,
		`ALTER EXTENSION vector DROP CAST (vectors.vector AS vectors.halfvec)`,
	} {
		t.Run("drift", func(t *testing.T) {
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.Exec(`SET LOCAL search_path=pg_catalog`); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			changed, err := Capture(ctx, tx, "vectors")
			if err == nil && changed.Verify(profile) == nil {
				t.Fatal("modified extension accepted")
			}
		})
	}
	for _, test := range []struct{ ddl, query string }{
		{`CREATE FUNCTION vectors.l2_distance(vectors.vector,vectors.halfvec) RETURNS float8 LANGUAGE SQL IMMUTABLE AS 'SELECT 0::float8'`, `SELECT vectors.l2_distance(embedding,$1::vectors.halfvec) FROM reporting.items`},
		{`CREATE FUNCTION vectors.shadow(vectors.vector,vectors.halfvec) RETURNS float8 LANGUAGE SQL IMMUTABLE AS 'SELECT 0::float8'; CREATE OPERATOR vectors.<-> (LEFTARG=vectors.vector,RIGHTARG=vectors.halfvec,FUNCTION=vectors.shadow)`, `SELECT embedding OPERATOR(vectors.<->) $1::vectors.halfvec FROM reporting.items`},
		{`CREATE FUNCTION reporting.vector_text(vectors.vector) RETURNS text LANGUAGE SQL IMMUTABLE AS 'SELECT ''hidden''::text'; CREATE CAST (vectors.vector AS text) WITH FUNCTION reporting.vector_text(vectors.vector)`, `SELECT embedding::text FROM reporting.items`},
	} {
		t.Run("unreviewed binding", func(t *testing.T) {
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.Exec(`SET LOCAL search_path=pg_catalog; SET LOCAL statement_timeout='10s'`); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(test.ddl); err != nil {
				t.Fatal(err)
			}
			current, err := Capture(ctx, tx, "vectors")
			if err != nil {
				t.Fatal(err)
			}
			if err := current.Verify(profile); err != nil {
				t.Fatal("non-member unexpectedly changed extension profile", err)
			}
			if _, err := tx.Exec(`SET LOCAL ROLE fixture_reader`); err != nil {
				t.Fatal(err)
			}
			if err := current.CheckExecuteGrants(ctx, tx); err == nil {
				t.Fatal("PUBLIC grant to unreviewed overload was accepted")
			}
			if _, err := tx.Exec(`RESET ROLE; SET LOCAL search_path=pg_catalog,vectors`); err != nil {
				t.Fatal(err)
			}
			var raw string
			if err := tx.QueryRow(`SELECT mcp_proof.bind($1)`, test.query).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			binding, err := DecodeBinding([]byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			if err := current.CheckBinding(ctx, tx, test.query, binding, []string{"reporting"}, nil); err == nil {
				t.Fatal("native binding to unreviewed overload/cast was accepted")
			}
		})
	}
	for _, query := range []string{`SELECT 1; SELECT 2`, `UPDATE reporting.items SET id=9 RETURNING id`, `WITH x AS (DELETE FROM reporting.items RETURNING id) SELECT * FROM x`, `SELECT * FROM reporting.items FOR UPDATE`, `SELECT id INTO reporting.copy FROM reporting.items`, `SELECT $10001::integer`} {
		t.Run("reject statement", func(t *testing.T) {
			tx := catalogTx(t, db)
			var raw string
			if err := tx.QueryRow(`SELECT mcp_proof.bind($1)`, query).Scan(&raw); err == nil {
				t.Fatal("unsafe/oversized statement was analyzed")
			}
		})
	}
	t.Run("relocated namespace", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`SET LOCAL search_path=pg_catalog; CREATE SCHEMA embeddings; ALTER EXTENSION vector SET SCHEMA embeddings`); err != nil {
			t.Fatal(err)
		}
		moved, err := Capture(ctx, tx, "embeddings")
		if err != nil {
			t.Fatal(err)
		}
		if err := moved.Verify(profile); err != nil {
			t.Fatal("profile depended on one installation schema", err)
		}
	})
}
