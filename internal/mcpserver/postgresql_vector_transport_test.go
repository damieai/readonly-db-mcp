package mcpserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/registry"
)

// Generate disposable CA/server keys in the private test directory. No system
// trust store is changed and no private key is checked into the repository.
func vectorTLSCertificates(t *testing.T, dir string) (string, string, string) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "vector fixture CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), DNSNames: []string{"vector.fixture.test"}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name, kind string, b []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: b}), 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	return write("ca.pem", "CERTIFICATE", caDER), write("server.pem", "CERTIFICATE", der), write("server.key", "PRIVATE KEY", keyDER)
}

func vectorWait(t *testing.T, ctx context.Context, check func() bool) {
	t.Helper()
	for !check() {
		select {
		case <-ctx.Done():
			t.Fatal("native observation deadline", ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// Opaque TCP relay: TLS remains end-to-end between pgx and PostgreSQL. In
// blackhole mode both directions discard bytes (including new cancel sockets).
// Upstream sockets remain open even after client EOF until explicit restoration.
type vectorRelay struct {
	listener net.Listener
	upstream string
	blocked  atomic.Bool
	mu       sync.Mutex
	closed   bool
	pairs    map[*vectorRelayPair]bool
	wg       sync.WaitGroup
}
type vectorRelayPair struct {
	a, b net.Conn
	stop chan struct{}
	once sync.Once
}

func (p *vectorRelayPair) close() { p.once.Do(func() { close(p.stop); p.a.Close(); p.b.Close() }) }

type vectorRelayWriter struct {
	relay *vectorRelay
	dst   net.Conn
}

func (w vectorRelayWriter) Write(b []byte) (int, error) {
	if w.relay.blocked.Load() {
		return len(b), nil
	}
	return w.dst.Write(b)
}
func newVectorRelay(t *testing.T, upstream string) *vectorRelay {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &vectorRelay{listener: l, upstream: upstream, pairs: map[*vectorRelayPair]bool{}}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			a, err := l.Accept()
			if err != nil {
				return
			}
			p.wg.Add(1)
			go func() {
				defer p.wg.Done()
				b, err := net.DialTimeout("tcp", p.upstream, time.Second)
				if err != nil {
					a.Close()
					return
				}
				pair := &vectorRelayPair{a: a, b: b, stop: make(chan struct{})}
				p.mu.Lock()
				if p.closed {
					p.mu.Unlock()
					pair.close()
					return
				}
				p.pairs[pair] = true
				p.mu.Unlock()
				defer func() { pair.close(); p.mu.Lock(); delete(p.pairs, pair); p.mu.Unlock() }()
				done := make(chan struct{}, 2)
				copyOne := func(dst, src net.Conn) {
					_, _ = io.Copy(vectorRelayWriter{p, dst}, src)
					if p.blocked.Load() {
						<-pair.stop
					}
					pair.close()
					done <- struct{}{}
				}
				go copyOne(a, b)
				go copyOne(b, a)
				<-done
				<-done
			}()
		}
	}()
	t.Cleanup(func() {
		l.Close()
		p.mu.Lock()
		p.closed = true
		for pair := range p.pairs {
			pair.close()
		}
		p.mu.Unlock()
		p.wg.Wait()
	})
	return p
}
func (p *vectorRelay) cut() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for pair := range p.pairs {
		pair.close()
	}
}
func (p *vectorRelay) restore() { p.blocked.Store(false); p.cut() }

func TestLocalPostgreSQLVectorTLS(t *testing.T) {
	if os.Getenv("READONLY_DB_MCP_PG_TLS") != "1" {
		t.Skip("TLS fixture not explicitly enabled")
	}
	admin, _, cfg := vectorPostgresTransportFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	for _, test := range []string{"wrong_name", "wrong_ca", "wrong_password"} {
		t.Run(test, func(t *testing.T) {
			c := *cfg
			target := *cfg.Targets["vectors"]
			c.Targets = map[string]*config.TargetConfig{"vectors": &target}
			switch test {
			case "wrong_name":
				target.TLS.ServerName = "wrong.fixture.test"
			case "wrong_ca":
				target.TLS.CAFile, _, _ = vectorTLSCertificates(t, t.TempDir())
			case "wrong_password":
				t.Setenv("MCP_PG_WRONG_PASSWORD", "incorrect-fixture-password")
				target.PasswordEnv = "MCP_PG_WRONG_PASSWORD"
			}
			r, err := registry.Open(ctx, &c, nil, nil)
			if err == nil {
				r.Close()
				t.Fatal("invalid TLS/identity configuration accepted")
			}
		})
	}
	t.Run("plaintext_denied", func(t *testing.T) {
		pc, err := pgx.ParseConfig("postgres://vector_reader@127.0.0.1/postgres?sslmode=disable")
		if err != nil {
			t.Fatal(err)
		}
		pc.Port = uint16(cfg.Targets["vectors"].Port)
		pc.Password = "private-unix-fixture"
		c, err := pgx.ConnectConfig(ctx, pc)
		if err == nil {
			c.Close(ctx)
			t.Fatal("non-TLS reader connected")
		}
	})
	relay := newVectorRelay(t, net.JoinHostPort(cfg.Targets["vectors"].Host, fmt.Sprint(cfg.Targets["vectors"].Port)))
	cfg.Targets["vectors"].Port = relay.listener.Addr().(*net.TCPAddr).Port
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	call := vectorMCPClient(t, ctx, cfg)
	probe := func(t *testing.T) int {
		t.Helper()
		var r core.QueryResult
		if err := call(ctx, "query_select", QueryInput{Target: "vectors", SQL: `SELECT pg_backend_pid() AS pid,'[1,0,0]'::vectors.vector AS embedding,current_setting('hnsw.ef_search') AS ef`}, &r); err != nil {
			t.Fatal(err)
		}
		if r.RowCount != 1 || r.Rows[0]["ef"] != "40" || len(r.Rows[0]["embedding"].([]any)) != 3 {
			t.Fatalf("invalid TLS recovery: %+v", r)
		}
		pid := int(r.Rows[0]["pid"].(float64))
		var ssl bool
		var version string
		if err := admin.QueryRowContext(ctx, `SELECT ssl,version FROM pg_stat_ssl WHERE pid=$1`, pid).Scan(&ssl, &version); err != nil || !ssl {
			t.Fatal("reader is not using native TLS", ssl, err)
		}
		t.Logf("native TLS=%s pid=%d", version, pid)
		return pid
	}
	prior := probe(t)
	for _, mode := range []string{"disconnect", "blackhole"} {
		t.Run(mode, func(t *testing.T) {
			marker := "tls_vector_" + mode
			query := `SELECT sum((embedding <-> '[1,0,0]'::vectors.vector)+sin(g.i+h.j)) FROM reporting.items CROSS JOIN generate_series(1,10000) g(i) CROSS JOIN generate_series(1,10000) h(j) /* ` + marker + ` */`
			ef := 123
			done := make(chan error, 1)
			started := time.Now()
			go func() {
				var r core.QueryResult
				done <- call(ctx, "query_select", QueryInput{Target: "vectors", SQL: query, TimeoutMS: 5000, PostgreSQLOptions: &core.PostgreSQLQueryOptions{HNSWEfSearch: &ef}}, &r)
			}()
			observe, stop := context.WithTimeout(ctx, 4*time.Second)
			defer stop()
			vectorWait(t, observe, func() bool {
				var active bool
				if err := admin.QueryRowContext(observe, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND state='active' AND query LIKE $2 AND query_start < clock_timestamp()-interval '100 milliseconds')`, prior, "%"+marker+"%").Scan(&active); err != nil {
					t.Fatal(err)
				}
				return active
			})
			if mode == "disconnect" {
				relay.cut()
			} else {
				relay.blocked.Store(true)
				defer relay.restore()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("interrupted query succeeded")
				}
			case <-time.After(7 * time.Second):
				t.Fatal("network failure ignored request deadline")
			}
			if elapsed := time.Since(started); elapsed > 6500*time.Millisecond {
				t.Fatalf("network failure exceeded deadline: %s", elapsed)
			} else {
				t.Logf("request=%s", elapsed)
			}
			cleanup, stopCleanup := context.WithTimeout(ctx, 5*time.Second)
			defer stopCleanup()
			vectorWait(t, cleanup, func() bool {
				var active int
				if err := admin.QueryRowContext(cleanup, `SELECT count(*) FROM pg_stat_activity WHERE pid=$1 AND state='active'`, prior).Scan(&active); err != nil {
					t.Fatal(err)
				}
				return active == 0
			})
			if mode == "blackhole" {
				var state string
				if err := admin.QueryRowContext(ctx, `SELECT state FROM pg_stat_activity WHERE pid=$1`, prior).Scan(&state); err != nil || state != "idle in transaction (aborted)" {
					t.Fatal("server timeout did not stop isolated query", state, err)
				}
				relay.restore()
			}
			vectorWait(t, cleanup, func() bool {
				var n int
				if err := admin.QueryRowContext(cleanup, `SELECT count(*) FROM pg_stat_activity WHERE pid=$1`, prior).Scan(&n); err != nil {
					t.Fatal(err)
				}
				return n == 0
			})
			next := probe(t)
			if next == prior {
				t.Fatal("discarded TLS backend was reused")
			}
			prior = next
		})
	}
}

// An actual recursive read does vector arithmetic until a wall-clock bound,
// retaining one small aggregate per iteration instead of millions of vectors.
const vectorLongReadSQL = `WITH RECURSIVE work(step,distance) AS (
 SELECT 0,0::double precision
 UNION ALL
 SELECT step+1,(SELECT sum(embedding <-> format('[%s,0,0]',(g.i+work.step)%2)::vectors.vector) FROM reporting.items CROSS JOIN generate_series(1,1000) g(i))
 FROM work WHERE clock_timestamp() < $1::timestamptz
) SELECT max(step) AS iterations,max(distance) AS distance FROM work /* vector_long_read */`

func TestLocalPostgreSQLVectorLongRead(t *testing.T) {
	if os.Getenv("READONLY_DB_MCP_PG_LONG_READ") != "1" {
		t.Skip("multi-minute native qualification not explicitly enabled")
	}
	admin, _, cfg := vectorPostgresTransportFixture(t, true)
	cfg.Limits.PerTargetConcurrency = 2
	cfg.Targets["vectors"].Connection.MaxOpen = 2
	cfg.Targets["vectors"].Connection.MaxIdle = 2
	cfg.Targets["vectors"].PostgreSQL.PrivilegeRecheck = 10 * time.Second
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer func() {
		cancel()
		if t.Failed() {
			// Bound teardown even if an RPC cancellation cannot be delivered.
			// Only this disposable cluster's fixture reader is affected.
			cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
			defer stop()
			_, _ = admin.ExecContext(cleanup, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE usename='vector_reader'`)
		}
	}()
	call := vectorMCPClient(t, ctx, cfg)
	// Exercise recursive binding before starting the long acceptance window.
	var warm core.QueryResult
	if err := call(ctx, "query_select", QueryInput{Target: "vectors", SQL: vectorLongReadSQL, Parameters: []any{time.Now().Add(3 * time.Second).UTC().Format(time.RFC3339Nano)}}, &warm); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	done := make(chan error, 1)
	var result core.QueryResult
	go func() {
		done <- call(ctx, "query_select", QueryInput{Target: "vectors", SQL: vectorLongReadSQL, Parameters: []any{started.Add(125 * time.Second).UTC().Format(time.RFC3339Nano)}}, &result)
	}()
	activeCtx, activeCancel := context.WithTimeout(ctx, 5*time.Second)
	defer activeCancel()
	var pid int
	vectorWait(t, activeCtx, func() bool {
		if err := admin.QueryRowContext(activeCtx, `SELECT COALESCE(max(pid),0) FROM pg_stat_activity WHERE usename='vector_reader' AND state='active' AND query LIKE '%vector_long_read%' AND query_start < clock_timestamp()-interval '100 milliseconds'`).Scan(&pid); err != nil {
			t.Fatal(err)
		}
		return pid != 0
	})
	nativeStarted := time.Now()
	metadataCalls, interactiveCalls, peak := 0, 0, 0
	for {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			goto finished
		case <-time.After(2 * time.Second):
		}
		probeCtx, probeCancel := context.WithTimeout(ctx, 5*time.Second)
		var tables ListTablesOutput
		err := call(probeCtx, "schema_list_tables", ListTablesInput{Target: "vectors", Fresh: true}, &tables)
		probeCancel()
		if err != nil || len(tables.Tables) != 1 {
			t.Fatal("metadata stalled behind long read/recheck", err, tables)
		}
		metadataCalls++
		probeCtx, probeCancel = context.WithTimeout(ctx, 5*time.Second)
		var quick core.QueryResult
		err = call(probeCtx, "query_select", QueryInput{Target: "vectors", SQL: `SELECT '[1,0,0]'::vectors.vector <-> '[0,1,0]'::vectors.vector AS distance`, TimeoutMS: 5000}, &quick)
		probeCancel()
		if err != nil || quick.RowCount != 1 {
			t.Fatal("interactive query stalled behind long read/recheck", err)
		}
		interactiveCalls++
		var count int
		if err := admin.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE usename='vector_reader'`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		peak = max(peak, count)
		if count > 2 {
			t.Fatal("physical connection ceiling exceeded", count)
		}
	}
finished:
	if time.Since(nativeStarted) < 120*time.Second || result.RowCount != 1 || result.Truncated || result.Rows[0]["iterations"].(float64) <= 0 {
		t.Fatalf("multi-minute read incomplete: duration=%s result=%+v", time.Since(nativeStarted), result)
	}
	// Each step computes 500 copies of the distances to [0,0,0] and to
	// [1,0,0], respectively, using the fixture's float32 components.
	x, y := float64(float32(.8)), float64(float32(.6))
	expected := 500 * (3 + math.Sqrt(x*x+y*y) + math.Sqrt((x-1)*(x-1)+y*y) + math.Sqrt(2) + 2)
	if math.Abs(result.Rows[0]["distance"].(float64)-expected) > 0.01 {
		t.Fatal("long-read vector aggregate mismatch", result, expected)
	}
	if metadataCalls < 10 || interactiveCalls < 10 {
		t.Fatal("insufficient mixed workload observations", metadataCalls, interactiveCalls)
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
	defer cleanupCancel()
	// A scheduled recheck may legitimately have a transaction open at the
	// instant the read finishes. Require quiescence between rechecks instead.
	vectorWait(t, cleanupCtx, func() bool {
		var inTx int
		if err := admin.QueryRowContext(cleanupCtx, `SELECT count(*) FROM pg_stat_activity WHERE usename='vector_reader' AND xact_start IS NOT NULL`).Scan(&inTx); err != nil {
			t.Fatal(err)
		}
		return inTx == 0
	})
	t.Logf("elapsed=%s native_observed=%s iterations=%.0f metadata=%d interactive=%d peak_backends=%d", time.Since(started), time.Since(nativeStarted), result.Rows[0]["iterations"], metadataCalls, interactiveCalls, peak)
}
