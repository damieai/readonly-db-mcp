package sqlparser

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var limits = Limits{Bytes: 256 << 10, Tokens: 10000, Depth: 128}

func TestPinnedSQLSourceGrammar(t *testing.T) {
	tests := []struct {
		sql     string
		params  []any
		sources []Source
	}{
		{`select 9007199254740993, 'DELETE FROM private'`, nil, nil},
		{`SeLeCt CAST(price AS DOUBLE), EXTRACT(YEAR FROM ts), CASE WHEN n > ? THEN 'FROM private' ELSE 'update' END FROM "reports-*" WHERE MATCH(name, ?) GROUP BY price, ts, n HAVING COUNT(*) > 1 ORDER BY price DESC LIMIT 10`, []any{1, "x"}, []Source{{Index: "reports-*"}}},
		{`WITH "reports-cte" AS (SELECT * FROM "reports-base") SELECT * FROM "reports-cte"`, nil, []Source{{Index: "reports-base"}, {Index: "reports-cte"}}},
		{`SELECT * FROM (SELECT * FROM "reports-one") a LEFT JOIN "reports-two" b ON a.id=b.id WHERE EXISTS (SELECT 1 FROM "reports-three")`, nil, []Source{{Index: "reports-one"}, {Index: "reports-two"}, {Index: "reports-three"}}},
		{`EXPLAIN (PLAN ALL FORMAT TEXT VERIFY TRUE) SELECT * FROM "reports-*"`, nil, []Source{{Index: "reports-*"}}},
		{`SELECT * FROM "local":"reports-*"`, nil, []Source{{Catalog: "local", Index: "reports-*"}}},
		{`SELECT * FROM "reports-logs"::data`, nil, []Source{{Index: "reports-logs::data"}}},
		{`SHOW TABLES LIKE 'reports_%'`, nil, []Source{{Index: "reports**"}}},
		{`SHOW COLUMNS CATALOG 'local' FROM "reports-*"`, nil, []Source{{Index: "reports-*"}}},
		{`SYS COLUMNS TABLE LIKE ? LIKE 'column_%'`, []any{"reports-%"}, []Source{{Index: "reports-*"}}},
		{`SHOW TABLES LIKE 'reports!_%' ESCAPE '!'`, nil, []Source{{Index: "reports_*"}}},
		{`SHOW TABLES`, nil, []Source{{Index: "*"}}},
		{`SHOW FUNCTIONS LIKE 'COS%'`, nil, nil},
		{`SHOW CATALOGS`, nil, nil},
		{`SYS TABLES CATALOG LIKE '%' LIKE ''`, nil, nil},
		{`SELECT * FROM "reports-*" PIVOT (SUM(amount) FOR category IN ('a' AS a, 'b' AS b))`, nil, []Source{{Index: "reports-*"}}},
		{`SELECT {fn ABS(-2)}, {d '2026-09-22'} FROM "reports-*" /* nested /* delete */ fine */`, nil, []Source{{Index: "reports-*"}}},
	}
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, test := range tests {
			t.Run(version+test.sql, func(t *testing.T) {
				a, e := Analyze(context.Background(), version, test.sql, test.params, limits)
				if e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(a.Sources, test.sources) {
					t.Fatalf("sources=%+v want=%+v", a.Sources, test.sources)
				}
			})
		}
	}
}
func TestSQLMalformedAndBudgets(t *testing.T) {
	for _, s := range []string{`SELECT`, `SELECT 1; SELECT 2`, `SELECT * FROM 123abc`, `DELETE FROM "reports-*"`, "SELECT * FROM `reports-x`", `SELECT * FROM "reports-x`, `SELECT ?`, `SELECT 1 /* secret`, "SELECT \xff"} {
		if _, e := Analyze(context.Background(), "8.19.21", s, nil, limits); e == nil {
			t.Fatalf("admitted %s", s)
		}
	}
	for _, s := range []string{"SELECT " + strings.Repeat("(", 300) + "1" + strings.Repeat(")", 300), "SELECT " + strings.Repeat("NOT ", 300) + "TRUE"} {
		if _, e := Analyze(context.Background(), "9.1.10", s, nil, limits); e == nil || !strings.HasPrefix(e.Error(), "resource_limit:") {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Analyze(ctx, "9.1.10", "SELECT 1", nil, limits); e == nil {
		t.Fatal("ignored cancellation")
	}
	if !memory.TryAcquire(MemoryLimit) {
		t.Fatal("parser memory leaked")
	}
	memory.Release(MemoryLimit)
}
func FuzzSQLParser(f *testing.F) {
	for _, s := range []string{`SELECT * FROM "reports-*"`, `WITH x AS (SELECT 1) SELECT * FROM x`, `SHOW TABLES LIKE 'reports_%'`, `SELECT 'FROM private'`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 {
			t.Skip()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		for _, version := range []string{"8.19.21", "9.1.10"} {
			_, _ = Analyze(ctx, version, s, nil, Limits{Bytes: 4096, Tokens: 2000, Depth: 32})
		}
	})
}

func BenchmarkSQLParser(b *testing.B) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, test := range []struct{ name, query string }{
			{"literal", `SELECT 9007199254740993`},
			{"aggregation", `SELECT SUM(price), DATE_TRUNC('day', ts) FROM "reports-*" WHERE price > 1 GROUP BY DATE_TRUNC('day', ts) ORDER BY 1 DESC LIMIT 5`},
			{"wide_in", `SELECT * FROM "reports-*" WHERE name IN (` + strings.Repeat("'literal',", 999) + "'last')"},
		} {
			b.Run(version+"/"+test.name, func(b *testing.B) {
				_, _ = Analyze(context.Background(), version, test.query, nil, limits)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := Analyze(context.Background(), version, test.query, nil, limits); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(len(test.query)), "query_bytes")
			})
		}
	}
}

func TestSQLConcurrentParseAndCancellationRelease(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			version := "8.19.21"
			if i%2 != 0 {
				version = "9.1.10"
			}
			for n := 0; n < 4; n++ {
				if _, err := Analyze(context.Background(), version, `SELECT COUNT(*) FROM "reports-*" WHERE value > 1`, nil, limits); err != nil {
					t.Error(err)
				}
			}
		}(i)
	}
	wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	query := `SELECT * FROM "reports-*" WHERE name IN (` + strings.Repeat("'literal',", 9999) + "'last')"
	if _, err := Analyze(ctx, "9.1.10", query, nil, Limits{Bytes: 256 << 10, Tokens: 30000, Depth: 128}); err == nil || !strings.HasPrefix(err.Error(), "deadline_exceeded:") {
		t.Fatal("large parse ignored deadline", err)
	}
	if !memory.TryAcquire(MemoryLimit) {
		t.Fatal("SQL parser reservation leaked after cancellation/concurrency")
	}
	memory.Release(MemoryLimit)
}
