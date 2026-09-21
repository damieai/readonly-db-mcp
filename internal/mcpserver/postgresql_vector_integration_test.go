package mcpserver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func vectorPostgresFixture(t *testing.T) (*sql.DB, *sql.DB, *config.Config) {
	t.Helper()
	return vectorPostgresTransportFixture(t, false)
}

func vectorPostgresTransportFixture(t *testing.T, useTLS bool) (*sql.DB, *sql.DB, *config.Config) {
	t.Helper()
	bin, helper := os.Getenv("READONLY_DB_MCP_PG_BIN"), os.Getenv("READONLY_DB_MCP_PGVECTOR_PROOF")
	if bin == "" || helper == "" {
		t.Skip("local PostgreSQL/pgvector helper artifacts are not configured")
	}
	if !filepath.IsAbs(bin) || !filepath.IsAbs(helper) {
		t.Fatal("absolute fixture artifact paths required")
	}
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	if out, err := exec.Command(filepath.Join(bin, "initdb"), "-D", data, "-U", "fixture_owner", "--auth=trust", "--no-sync", "--encoding=UTF8", "--locale=C").CombinedOutput(); err != nil {
		t.Fatalf("initdb: %v %s", err, out)
	}
	port, host, tlsYAML := 5432, dir, "mode: disabled\n      allow_insecure_remote: true"
	args := []string{"-D", data, "-k", dir, "-h", "", "-c", "fsync=off", "-c", "max_connections=12"}
	if useTLS {
		ca, cert, key := vectorTLSCertificates(t, dir)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		host = "127.0.0.1"
		args = append(args, "-h", host, "-p", fmt.Sprint(port), "-c", "ssl=on", "-c", "ssl_cert_file="+cert, "-c", "ssl_key_file="+key)
		// Only the reader may authenticate over loopback TCP, with TLS and
		// SCRAM. Provisioning still uses the private local owner socket.
		if err := os.WriteFile(filepath.Join(data, "pg_hba.conf"), []byte("local all all trust\nhostssl postgres vector_reader 127.0.0.1/32 scram-sha-256\nhost all all 127.0.0.1/32 reject\n"), 0600); err != nil {
			t.Fatal(err)
		}
		tlsYAML = fmt.Sprintf("mode: verify-full\n      ca_file: %q\n      server_name: vector.fixture.test", ca)
	}
	cmd := exec.Command(filepath.Join(bin, "postgres"), args...)
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
	pc, err := pgx.ParseConfig("postgres://fixture_owner@localhost/postgres?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pc.Host = dir
	pc.Port = uint16(port)
	admin := stdlib.OpenDB(*pc)
	t.Cleanup(func() { admin.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for admin.PingContext(ctx) != nil {
		if ctx.Err() != nil {
			t.Fatal("fixture startup failed")
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, q := range []string{
		`CREATE SCHEMA vectors; CREATE EXTENSION vector WITH SCHEMA vectors VERSION '0.8.2'; CREATE SCHEMA reporting; CREATE SCHEMA mcp_proof`,
		`CREATE ROLE vector_reader LOGIN PASSWORD 'private-unix-fixture'; REVOKE ALL ON DATABASE postgres FROM PUBLIC; REVOKE CONNECT ON DATABASE template1 FROM PUBLIC; REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT CONNECT ON DATABASE postgres TO vector_reader; GRANT USAGE ON SCHEMA reporting,vectors,mcp_proof TO vector_reader`,
		`CREATE TABLE reporting.items(id integer PRIMARY KEY, embedding vectors.vector(3), halfembedding vectors.halfvec(3), category text); INSERT INTO reporting.items VALUES(1,'[1,0,0]','[1,0,0]','a'),(2,'[0.8,0.6,0]','[0.8,0.6,0]','a'),(3,'[0,1,0]','[0,1,0]','b'),(4,'[-1,0,0]','[-1,0,0]','b'); GRANT SELECT ON reporting.items TO vector_reader`,
		// Force index preference only in this functional test deployment. This is
		// not a client query option or a realistic ANN cost/recall qualification.
		`ALTER ROLE vector_reader SET enable_seqscan=off; ALTER ROLE vector_reader SET jit=off`,
	} {
		if _, err := admin.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"bind", "expression"} {
		q := fmt.Sprintf(`CREATE FUNCTION mcp_proof.%s(text) RETURNS text AS '%s','mcp_vector_%s' LANGUAGE C STRICT VOLATILE PARALLEL UNSAFE; REVOKE ALL ON FUNCTION mcp_proof.%s(text) FROM PUBLIC; GRANT EXECUTE ON FUNCTION mcp_proof.%s(text) TO vector_reader`, name, strings.ReplaceAll(helper, "'", "''"), name, name, name)
		if _, err := admin.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	hash := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(b))
	}
	libraryDir, err := exec.Command(filepath.Join(bin, "pg_config"), "--pkglibdir").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCP_PG_VECTOR_PASSWORD", "private-unix-fixture")
	contents := fmt.Sprintf(`server:
  strict_startup: true
limits:
  per_target_concurrency: 1
targets:
  vectors:
    engine: postgresql
    environment: test
    database: postgres
    host: %q
    port: %d
    username: vector_reader
    password_env: MCP_PG_VECTOR_PASSWORD
    allowed_schemas: [reporting]
    tls:
      %s
    connection:
      max_open: 1
      max_idle: 1
    postgresql:
      pgvector:
        profile: pg16-vector0.8.2
        schema: vectors
        owner: fixture_owner
        helper_schema: mcp_proof
        helper_library: %q
        deployment_id: native-test
        vector_library_sha256: %s
        helper_library_sha256: %s
`, host, port, tlsYAML, helper, hash(filepath.Join(strings.TrimSpace(string(libraryDir)), "vector.so")), hash(helper))
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	pc.User = "vector_reader"
	pc.RuntimeParams = map[string]string{"search_path": "pg_catalog,vectors"}
	reader := stdlib.OpenDB(*pc)
	reader.SetMaxOpenConns(1)
	t.Cleanup(func() { reader.Close() })
	return admin, reader, cfg
}

func TestLocalPostgreSQLDenseVectorsThroughMCP(t *testing.T) {
	admin, reader, cfg := vectorPostgresFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	request := vectorMCPClient(t, ctx, cfg)
	call := func(tool string, input, out any) error {
		return request(ctx, tool, input, out)
	}
	query := func(q string, params []any, options *core.PostgreSQLQueryOptions) (core.QueryResult, error) {
		var result core.QueryResult
		err := call("query_select", QueryInput{Target: "vectors", SQL: q, Parameters: params, PostgreSQLOptions: options}, &result)
		return result, err
	}
	// Compare typed text parameters, IDs and native distances for every dense
	// dtype/metric/index combination implemented by the pinned extension.
	for _, dtype := range []string{"vector", "halfvec"} {
		for _, metric := range []struct{ name, op string }{{"l2", "<->"}, {"ip", "<#>"}, {"cosine", "<=>"}, {"l1", "<+>"}} {
			for _, algorithm := range []string{"exact", "hnsw", "ivfflat"} {
				if algorithm == "ivfflat" && metric.name == "l1" {
					continue
				}
				t.Run(dtype+"/"+metric.name+"/"+algorithm, func(t *testing.T) {
					column := "embedding"
					if dtype == "halfvec" {
						column = "halfembedding"
					}
					if _, err := admin.Exec(`DROP INDEX IF EXISTS reporting.vector_idx`); err != nil {
						t.Fatal(err)
					}
					if algorithm != "exact" {
						ddl := fmt.Sprintf(`CREATE INDEX vector_idx ON reporting.items USING %s (%s vectors.%s_%s_ops)`, algorithm, column, dtype, metric.name)
						if algorithm == "ivfflat" {
							ddl += ` WITH(lists=1)`
						}
						if _, err := admin.Exec(ddl); err != nil {
							t.Fatal(err)
						}
					}
					q := fmt.Sprintf(`SELECT id, %s OPERATOR(vectors.%s) $1::vectors.%s AS distance, %s AS embedding FROM reporting.items ORDER BY %s OPERATOR(vectors.%s) $1::vectors.%s LIMIT 3`, column, metric.op, dtype, column, column, metric.op, dtype)
					got, err := query(q, []any{"[1,0,0]"}, nil)
					if err != nil {
						t.Fatal(err)
					}
					if got.RowCount != 3 || got.Truncated {
						t.Fatalf("incomplete result: %#v", got)
					}
					rows, err := reader.QueryContext(ctx, q, "[1,0,0]")
					if err != nil {
						t.Fatal(err)
					}
					defer rows.Close()
					for i := 0; rows.Next(); i++ {
						var id int
						var distance float64
						var native any
						if err := rows.Scan(&id, &distance, &native); err != nil {
							t.Fatal(err)
						}
						if got.Rows[i]["id"] != float64(id) || math.Abs(got.Rows[i]["distance"].(float64)-distance) > 1e-6 {
							t.Fatal("native/MCP rank or distance mismatch")
						}
						point := [][3]float64{{1, 0, 0}, {float64(float32(0.8)), float64(float32(0.6)), 0}, {0, 1, 0}, {-1, 0, 0}}[id-1]
						if dtype == "halfvec" && id == 2 {
							point = [3]float64{0.7998046875, 0.60009765625, 0}
						}
						var expected float64
						switch metric.name {
						case "l2":
							expected = math.Sqrt((point[0]-1)*(point[0]-1) + point[1]*point[1])
						case "ip":
							expected = -point[0]
						case "cosine":
							expected = 1 - point[0]/math.Sqrt(point[0]*point[0]+point[1]*point[1])
						case "l1":
							expected = math.Abs(point[0]-1) + math.Abs(point[1])
						}
						if math.Abs(distance-expected) > 1e-6 {
							t.Fatal("distance differs from independent calculation", distance, expected)
						}
						if len(got.Rows[i]["embedding"].([]any)) != 3 {
							t.Fatal("vector result was not decoded")
						}
					}
					if err := rows.Err(); err != nil {
						t.Fatal(err)
					}
					if got.Rows[0]["id"] != float64(1) {
						t.Fatal("wrong nearest vector")
					}
					if algorithm != "exact" {
						var plan core.QueryResult
						if err := call("query_explain", QueryInput{Target: "vectors", SQL: q, Parameters: []any{"[1,0,0]"}}, &plan); err != nil {
							t.Fatal(err)
						}
						encoded, _ := json.Marshal(plan.Rows)
						if !strings.Contains(string(encoded), "vector_idx") {
							t.Fatalf("ANN index not used: %s", encoded)
						}
					}
				})
			}
		}
	}
	for _, q := range []string{
		`WITH ranked AS (SELECT id,embedding <-> $1::vectors.vector d,row_number() OVER(ORDER BY id) n FROM reporting.items WHERE category='a') SELECT a.id FROM ranked a JOIN ranked b USING(id) ORDER BY a.d`,
		`SELECT a.id FROM reporting.items a JOIN LATERAL (SELECT id FROM reporting.items b ORDER BY b.embedding <-> a.embedding LIMIT 1) b ON true`,
		`SELECT category,avg(embedding) FROM reporting.items GROUP BY category`,
		`SELECT vectors.l2_distance(embedding,'[1,0,0]'::vectors.vector) FROM reporting.items`,
	} {
		var params []any
		if strings.Contains(q, "$1") {
			params = []any{"[1,0,0]"}
		}
		if _, err := query(q, params, nil); err != nil {
			t.Fatal(q, err)
		}
	}
	ef := 123
	mode := "strict_order"
	probes := 2
	options := &core.PostgreSQLQueryOptions{HNSWEfSearch: &ef, HNSWIterativeScan: &mode, IVFFlatProbes: &probes}
	for _, algorithm := range []string{"hnsw", "ivfflat"} {
		if _, err := admin.Exec(`DROP INDEX reporting.vector_idx`); err != nil {
			t.Fatal(err)
		}
		ddl := fmt.Sprintf(`CREATE INDEX vector_idx ON reporting.items USING %s(embedding vectors.vector_l2_ops)`, algorithm)
		if algorithm == "ivfflat" {
			ddl += ` WITH(lists=2)`
		}
		if _, err := admin.Exec(ddl); err != nil {
			t.Fatal(err)
		}
		one, two := 1, 2
		strict, relaxed := "strict_order", "relaxed_order"
		filtered := &core.PostgreSQLQueryOptions{HNSWEfSearch: &one, HNSWIterativeScan: &strict, IVFFlatProbes: &one, IVFFlatMaxProbes: &two, IVFFlatIterativeScan: &relaxed}
		result, err := query(`SELECT id FROM reporting.items WHERE category='b' ORDER BY embedding <-> $1::vectors.vector LIMIT 2`, []any{"[1,0,0]"}, filtered)
		if err != nil || result.RowCount != 2 {
			t.Fatal("filtered iterative retrieval", algorithm, result, err)
		}
		for _, row := range result.Rows {
			if row["id"] != float64(3) && row["id"] != float64(4) {
				t.Fatal("filtered search escaped category")
			}
		}
	}
	settingSQL := `SELECT current_setting('hnsw.ef_search') AS ef,current_setting('hnsw.iterative_scan') AS mode,current_setting('ivfflat.probes') AS probes`
	result, err := query(settingSQL, nil, options)
	if err != nil || result.Rows[0]["ef"] != "123" || result.Rows[0]["mode"] != "strict_order" || result.Rows[0]["probes"] != "2" {
		t.Fatal("request tuning", result, err)
	}
	nonfinite, err := query(`SELECT '[0,0,0]'::vectors.vector <=> '[1,0,0]'::vectors.vector AS distance`, nil, nil)
	if err != nil || nonfinite.Rows[0]["distance"].(map[string]any)["value"] != "NaN" {
		t.Fatal("native NaN distance encoding", nonfinite, err)
	}
	var batch core.BatchResult
	if err := call("query_batch", BatchInput{Target: "vectors", PostgreSQLOptions: options, Queries: []BatchQueryInput{{SQL: settingSQL}, {SQL: settingSQL}}}, &batch); err != nil || batch.CompletedQueries != 2 {
		t.Fatal("batch", batch, err)
	}
	for _, r := range batch.Results {
		if r.Rows[0]["ef"] != "123" {
			t.Fatal("batch tuning scope")
		}
	}
	result, err = query(settingSQL, nil, nil)
	if err != nil || result.Rows[0]["ef"] != "40" || result.Rows[0]["mode"] != "off" || result.Rows[0]["probes"] != "1" {
		t.Fatal("pool settings leaked", result, err)
	}
	for _, q := range []string{`DELETE FROM reporting.items`, `UPDATE reporting.items SET category='x'`, `CREATE TABLE reporting.bad(id int)`, `SELECT set_config('hnsw.ef_search','999',false)`, `SELECT mcp_proof.bind('SELECT 1')`, `SELECT '[1,2]'::vectors.vector <-> '[1,2,3]'::vectors.vector`, `SELECT '[NaN,0,0]'::vectors.vector`} {
		if _, err := query(q, nil, options); err == nil {
			t.Fatal("unsafe or invalid query accepted", q)
		}
	}
	tooLarge := 1001
	if _, err := query(`SELECT 1`, nil, &core.PostgreSQLQueryOptions{HNSWEfSearch: &tooLarge}); err == nil {
		t.Fatal("native tuning bound ignored")
	}
	memory := 1000.0
	if _, err := query(`SELECT 1`, nil, &core.PostgreSQLQueryOptions{HNSWScanMemMultiplier: &memory}); err == nil {
		t.Fatal("scan memory budget ignored")
	}
	var ignored core.QueryResult
	if err := call("query_select", map[string]any{"target": "vectors", "sql": "SELECT 1", "postgresql_options": map[string]any{"statement_timeout": 0}}, &ignored); err == nil {
		t.Fatal("unrecognized tuning key silently accepted")
	}
	for _, q := range []string{`INSERT INTO reporting.items(id) VALUES(99)`, `UPDATE reporting.items SET category='x'`, `DELETE FROM reporting.items`, `DROP TABLE reporting.items`, `CREATE INDEX forbidden ON reporting.items(category)`} {
		if _, err := reader.ExecContext(ctx, q); err == nil {
			t.Fatal("native reader mutation succeeded", q)
		}
	}
	// Helper/privilege drift must be caught before invoking any helper on the
	// next request, without waiting for background attestation.
	if _, err := admin.Exec(`ALTER FUNCTION mcp_proof.bind(text) SECURITY DEFINER`); err != nil {
		t.Fatal(err)
	}
	if _, err := query(`SELECT 1`, nil, nil); err == nil {
		t.Fatal("helper drift accepted")
	}
	if _, err := admin.Exec(`ALTER FUNCTION mcp_proof.bind(text) SECURITY INVOKER; CREATE FUNCTION reporting.unreviewed() RETURNS int LANGUAGE SQL IMMUTABLE AS 'SELECT 1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := query(`SELECT 1`, nil, nil); err == nil {
		t.Fatal("new PUBLIC executable routine accepted")
	}
	if _, err := admin.Exec(`REVOKE ALL ON FUNCTION reporting.unreviewed() FROM PUBLIC`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`CREATE FUNCTION reporting.statistic_value(integer) RETURNS integer LANGUAGE SQL IMMUTABLE AS 'SELECT $1'; CREATE STATISTICS reporting.derived_stats ON (reporting.statistic_value(id)) FROM reporting.items; REVOKE ALL ON FUNCTION reporting.statistic_value(integer) FROM PUBLIC`); err != nil {
		t.Fatal(err)
	}
	if _, err := query(`SELECT 1`, nil, nil); err == nil {
		t.Fatal("unreviewed extended-statistics expression accepted")
	}
	if _, err := admin.Exec(`DROP STATISTICS reporting.derived_stats; DROP FUNCTION reporting.statistic_value(integer)`); err != nil {
		t.Fatal(err)
	}
	var timed core.QueryResult
	if err := call("query_select", QueryInput{Target: "vectors", SQL: `SELECT sum(sin(i)) FROM generate_series(1,100000000) g(i)`, TimeoutMS: 1200, PostgreSQLOptions: options}, &timed); err == nil {
		t.Fatal("expensive query was not cancelled")
	}
	result, err = query(settingSQL, nil, nil)
	if err != nil || result.Rows[0]["ef"] != "40" {
		t.Fatal("post-error/cancellation recovery or settings leak", result, err)
	}
}
