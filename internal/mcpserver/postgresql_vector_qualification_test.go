package mcpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/registry"
)

// Keep qualification on the real configuration/registry/MCP path. The admin
// returned by vectorPostgresFixture is used only for provisioning and observation.
func vectorMCPClient(t *testing.T, ctx context.Context, cfg *config.Config) func(context.Context, string, any, any) error {
	t.Helper()
	targets, err := registry.Open(ctx, cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { targets.Close() })
	a, b := mcp.NewInMemoryTransports()
	ss, err := New(targets, slog.Default(), "vector-qualification").mcp.Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "qualification", Version: "1"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return func(ctx context.Context, tool string, input, out any) error {
		r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: input})
		if err != nil {
			return err
		}
		if r.IsError {
			body, _ := json.Marshal(r.Content)
			return fmt.Errorf("MCP failure: %s", body)
		}
		if len(r.Content) != 1 {
			return fmt.Errorf("missing MCP result")
		}
		return json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), out)
	}
}

func TestLocalPostgreSQLVectorRecall(t *testing.T) {
	admin, reader, cfg := vectorPostgresFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)
	// Use the native planner's ordinary cost preference in this larger corpus.
	if _, err := admin.ExecContext(ctx, `ALTER ROLE vector_reader RESET enable_seqscan; CREATE TABLE reporting.corpus(id integer PRIMARY KEY, embedding vectors.vector(16), halfembedding vectors.halfvec(16), category integer); GRANT SELECT ON reporting.corpus TO vector_reader`); err != nil {
		t.Fatal(err)
	}
	const size, dimensions, k = 16384, 16, 10
	rng := rand.New(rand.NewSource(73421))
	point := func() []float64 {
		p := make([]float64, dimensions)
		for i := range p {
			// Exactly representable in both float32 and float16: the oracle
			// does not depend on pgvector's distance or conversion routines.
			p[i] = float64(rng.Intn(2049)-1024) / 1024
		}
		return p
	}
	encode := func(p []float64) string { b, _ := json.Marshal(p); return string(b) }
	points := make([][]float64, size)
	tx, err := admin.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := range points {
		points[i] = point()
		if _, err := tx.ExecContext(ctx, `INSERT INTO reporting.corpus VALUES($1,$2::vectors.vector,$2::vectors.halfvec,$3)`, i+1, encode(points[i]), (i+1)%8); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, `ANALYZE reporting.corpus`); err != nil {
		t.Fatal(err)
	}
	queries := [][]float64{point(), point(), point(), point()}
	call := vectorMCPClient(t, ctx, cfg)
	for _, dtype := range []string{"vector", "halfvec"} {
		column := "embedding"
		if dtype == "halfvec" {
			column = "halfembedding"
		}
		for _, algorithm := range []string{"exact", "hnsw", "ivfflat"} {
			t.Run(dtype+"/"+algorithm, func(t *testing.T) {
				if _, err := admin.ExecContext(ctx, `DROP INDEX IF EXISTS reporting.corpus_ann`); err != nil {
					t.Fatal(err)
				}
				if algorithm != "exact" {
					ddl := fmt.Sprintf(`CREATE INDEX corpus_ann ON reporting.corpus USING %s (%s vectors.%s_l2_ops)`, algorithm, column, dtype)
					if algorithm == "ivfflat" {
						ddl += ` WITH(lists=32)`
					}
					if _, err := admin.ExecContext(ctx, ddl); err != nil {
						t.Fatal(err)
					}
				}
				ef, probes, maxProbes := 100, 24, 32
				strict, relaxed := "strict_order", "relaxed_order"
				options := &core.PostgreSQLQueryOptions{HNSWEfSearch: &ef, HNSWIterativeScan: &strict, IVFFlatProbes: &probes, IVFFlatMaxProbes: &maxProbes, IVFFlatIterativeScan: &relaxed}
				for scenario := 0; scenario < 6; scenario++ {
					qv := queries[scenario%len(queries)]
					filtered, rerank := scenario == 4, scenario == 5
					type neighbor struct {
						id       int
						distance float64
					}
					var expected []neighbor
					distances := make(map[int]float64, size)
					for i, p := range points {
						if filtered && (i+1)%8 != 0 {
							continue
						}
						var sum float64
						for d, x := range p {
							sum += (x - qv[d]) * (x - qv[d])
						}
						distances[i+1] = math.Sqrt(sum)
						expected = append(expected, neighbor{i + 1, math.Sqrt(sum)})
					}
					sort.Slice(expected, func(i, j int) bool {
						if expected[i].distance == expected[j].distance {
							return expected[i].id < expected[j].id
						}
						return expected[i].distance < expected[j].distance
					})
					where := ""
					if filtered {
						where = " WHERE category = 0"
					}
					q := fmt.Sprintf(`SELECT id,%s <-> $1::vectors.%s AS distance FROM reporting.corpus%s ORDER BY %s <-> $1::vectors.%s LIMIT 10`, column, dtype, where, column, dtype)
					if rerank {
						// Retain a native materialized candidate scan and an outer
						// exact rerank, including relaxed iterative HNSW results.
						options.HNSWIterativeScan = &relaxed
						q = fmt.Sprintf(`WITH candidates AS MATERIALIZED (SELECT id,embedding FROM reporting.corpus ORDER BY %s <-> $1::vectors.%s LIMIT 100) SELECT id,embedding <-> $1::vectors.vector AS distance FROM candidates ORDER BY distance,id LIMIT 10`, column, dtype)
					}
					input := QueryInput{Target: "vectors", SQL: q, Parameters: []any{encode(qv)}, PostgreSQLOptions: options}
					if algorithm != "exact" {
						var plan core.QueryResult
						if err := call(ctx, "query_explain", input, &plan); err != nil {
							t.Fatal(err)
						}
						b, _ := json.Marshal(plan.Rows)
						if !strings.Contains(string(b), "corpus_ann") {
							t.Fatalf("ordinary planner did not select ANN index: %s", b)
						}
					}
					var result core.QueryResult
					if err := call(ctx, "query_select", input, &result); err != nil {
						t.Fatal(err)
					}
					if result.Truncated || result.RowCount != k {
						t.Fatalf("incomplete ranking: %+v", result)
					}
					hits, seen := 0, map[int]bool{}
					for i, row := range result.Rows {
						id := int(row["id"].(float64))
						distance, ok := distances[id]
						if !ok || seen[id] || math.Abs(row["distance"].(float64)-distance) > 1e-5 {
							t.Fatalf("invalid ID/filter/score: %v", row)
						}
						seen[id] = true
						if i > 0 && result.Rows[i-1]["distance"].(float64) > row["distance"].(float64)+1e-5 {
							t.Fatal("ranking is not ordered")
						}
						// Threshold membership admits equal-distance ties at K.
						if distance <= expected[k-1].distance+1e-9 {
							hits++
						}
					}
					recall := float64(hits) / k
					minimum := 0.9
					if algorithm == "exact" {
						minimum = 1
					}
					if recall < minimum {
						t.Fatalf("scenario %d recall@10 %.2f below %.2f", scenario, recall, minimum)
					}
					// Independent native exact baseline, deliberately expressing
					// the sort as distance+0 so an ANN index cannot supply it.
					native := fmt.Sprintf(`SELECT id,%s <-> $1::vectors.%s AS distance FROM reporting.corpus%s ORDER BY (%s <-> $1::vectors.%s)+0,id LIMIT 10`, column, dtype, where, column, dtype)
					rows, err := reader.QueryContext(ctx, native, encode(qv))
					if err != nil {
						t.Fatal(err)
					}
					n := 0
					for rows.Next() {
						var id int
						var d float64
						if err := rows.Scan(&id, &d); err != nil {
							rows.Close()
							t.Fatal(err)
						}
						if n >= k || id != expected[n].id || math.Abs(d-expected[n].distance) > 1e-5 {
							rows.Close()
							t.Fatal("native exact baseline differs from oracle")
						}
						n++
					}
					err = rows.Err()
					rows.Close()
					if err != nil || n != k {
						t.Fatal("incomplete native baseline", n, err)
					}
					t.Logf("scenario=%d filtered=%v rerank=%v recall@10=%.2f", scenario, filtered, rerank, recall)
				}
			})
		}
	}
}

// Observe native execution, not merely a client-side deadline. A fresh fixture
// and unopened native reader make vector_reader backend counts MCP-only.
func TestLocalPostgreSQLVectorRecovery(t *testing.T) {
	admin, _, cfg := vectorPostgresFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	call := vectorMCPClient(t, ctx, cfg)
	waitState := func(t *testing.T, check func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !check() {
			if time.Now().After(deadline) {
				t.Fatal("native state observation timed out")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	activePID := func(t *testing.T, marker string) int {
		var pid int
		err := admin.QueryRowContext(ctx, `SELECT COALESCE(max(pid),0) FROM pg_stat_activity WHERE usename='vector_reader' AND state='active' AND query_start < clock_timestamp()-interval '100 milliseconds' AND wait_event_type IS DISTINCT FROM 'Lock' AND query LIKE $1`, "%"+marker+"%").Scan(&pid)
		if err != nil {
			t.Fatal(err)
		}
		return pid
	}
	clean := func(t *testing.T) bool {
		var n int
		if err := admin.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE usename='vector_reader' AND (state<>'idle' OR xact_start IS NOT NULL)`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 0
	}
	settings := QueryInput{Target: "vectors", SQL: `SELECT '[1,0,0]'::vectors.vector <-> '[0,1,0]'::vectors.vector AS distance,current_setting('hnsw.ef_search') AS ef,current_setting('hnsw.iterative_scan') AS mode,current_setting('ivfflat.probes') AS probes`}
	recoverQuery := func() error {
		var r core.QueryResult
		if err := call(ctx, "query_select", settings, &r); err != nil {
			return err
		}
		if r.RowCount != 1 || r.Rows[0]["ef"] != "40" || r.Rows[0]["mode"] != "off" || r.Rows[0]["probes"] != "1" {
			return fmt.Errorf("settings leaked: %+v", r)
		}
		return nil
	}
	ef, probes := 123, 2
	mode := "relaxed_order"
	options := &core.PostgreSQLQueryOptions{HNSWEfSearch: &ef, HNSWIterativeScan: &mode, IVFFlatProbes: &probes}
	for _, scenario := range []string{"client_cancel", "server_deadline", "backend_loss", "batch_deadline"} {
		t.Run(scenario, func(t *testing.T) {
			marker := "vector_qualification_" + scenario
			// Two small series enter the repeated vector-distance calculation
			// promptly, rather than first materializing one enormous series.
			q := `SELECT sum((embedding <-> '[1,0,0]'::vectors.vector)+sin(g.i+h.j)) FROM reporting.items CROSS JOIN generate_series(1,10000) g(i) CROSS JOIN generate_series(1,10000) h(j) /* ` + marker + ` */`
			requestCtx, stop := context.WithCancel(ctx)
			defer stop()
			tool := "query_select"
			var input any = QueryInput{Target: "vectors", SQL: q, TimeoutMS: 5000, PostgreSQLOptions: options}
			var lock *sql.Tx
			if scenario == "batch_deadline" {
				var err error
				lock, err = admin.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer lock.Rollback()
				if _, err := lock.ExecContext(ctx, `LOCK TABLE reporting.items IN ACCESS EXCLUSIVE MODE`); err != nil {
					t.Fatal(err)
				}
				tool = "query_batch"
				input = BatchInput{Target: "vectors", TimeoutMS: 5000, PostgreSQLOptions: options, Queries: []BatchQueryInput{{SQL: `SELECT id FROM reporting.items /* vector_batch_first */`}, {SQL: q}}}
			}
			done := make(chan error, 1)
			started := time.Now()
			go func() { var out any; done <- call(requestCtx, tool, input, &out) }()
			if lock != nil {
				waitState(t, func() bool {
					var n int
					if err := admin.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE usename='vector_reader' AND wait_event_type='Lock'`).Scan(&n); err != nil {
						t.Fatal(err)
					}
					return n == 1
				})
				// Spend part of the same batch deadline in its first statement.
				time.Sleep(2 * time.Second)
				if err := lock.Rollback(); err != nil {
					t.Fatal(err)
				}
			}
			var pid int
			waitState(t, func() bool { pid = activePID(t, marker); return pid != 0 })
			// Linux RSS is observational evidence only, not a portable or total
			// memory ceiling; record it before the native backend disappears.
			if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
				for _, line := range strings.Split(string(b), "\n") {
					if strings.HasPrefix(line, "VmRSS:") {
						t.Log(line)
					}
				}
			}
			cleanupStarted := time.Now()
			switch scenario {
			case "client_cancel":
				// A queued call expires while the only permit/backend is busy.
				var out core.QueryResult
				queued := settings
				queued.TimeoutMS = 350
				if err := call(ctx, "query_select", queued, &out); err == nil {
					t.Fatal("queued request ignored its deadline")
				}
				cleanupStarted = time.Now()
				stop()
			case "backend_loss":
				var terminated bool
				if err := admin.QueryRowContext(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated); err != nil || !terminated {
					t.Fatal("fixture backend termination failed", terminated, err)
				}
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("interrupted query unexpectedly succeeded")
				}
			case <-time.After(6500 * time.Millisecond):
				t.Fatal("request did not terminate")
			}
			elapsed := time.Since(started)
			if elapsed > 6500*time.Millisecond {
				t.Fatalf("request renewed its deadline: %s", elapsed)
			}
			if scenario == "server_deadline" || scenario == "batch_deadline" {
				cleanupStarted = time.Now()
			}
			waitState(t, func() bool { return clean(t) })
			t.Logf("request=%s cleanup_observed=%s", elapsed, time.Since(cleanupStarted))
			if err := recoverQuery(); err != nil {
				t.Fatal("recovery", err)
			}
		})
	}
	// Sample physical connections throughout queued concurrent work, including
	// catalog/helper traffic; a configured one-connection pool must stay at one.
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- recoverQuery() }()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	peak := 0
	for {
		var count int
		if err := admin.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE usename='vector_reader'`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		peak = max(peak, count)
		if count > 1 {
			t.Fatalf("physical pool ceiling exceeded: %d", count)
		}
		select {
		case <-done:
			goto completed
		case <-time.After(10 * time.Millisecond):
		}
	}
completed:
	for i := 0; i < 8; i++ {
		if err := <-errs; err != nil {
			t.Fatal("concurrent recovery", err)
		}
	}
	waitState(t, func() bool { return clean(t) })
	t.Logf("concurrent_requests=8 sampled_peak_backends=%d", peak)
}
