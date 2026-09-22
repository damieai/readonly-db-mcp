package eqlparser

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

var testLimits = Limits{Bytes: 256 << 10, Tokens: 10000, Depth: 128}

func TestUpstreamEQLGrammarAndSourceContract(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		t.Run(version, func(t *testing.T) {
			for _, test := range []struct {
				query, kind      string
				filters, missing int
			}{
				{`any where true`, "event", 1, 0},
				{`process where process.name : ("cmd*", "powershell*") and ?user.id != null | head 25`, "event", 1, 0},
				{`sequence by user.id with maxspan=5m [process where true] [network where destination.port in (80,443)] until [process where event.type == "termination"]`, "sequence", 3, 0},
				{`sequence with maxspan=1h [any where true] ![file where file.name like~ "*.tmp"] [network where true]`, "sequence", 3, 1},
				{`sample by host.id [process where true] [network where true]`, "sample", 2, 0},
				{`join by user.id [process where true] [file where true] until [any where false]`, "join", 3, 0},
				{`process where descendant of [process where process.name == "parent"]`, "event", 2, 0},
				{`any where stringContains~(name, "delete FROM private; /* update */") and length(name) > 3`, "event", 1, 0},
				{"any where `FROM private-*` == \"delete\" // ; DROP TABLE x\n | tail 5", "event", 1, 0},
				{`"private-index" where name regex~ ("a.*", "b.*")`, "event", 1, 0},
				{`any where value == 9007199254740993 and ratio >= -1.25e+2`, "event", 1, 0},
				{`any where text == """SELECT * FROM private-*; \""" /* nested /* comment */ fine */`, "event", 1, 0},
				{`sequence [any where a not in~ ("a","b")] with runs=3 [any where endsWith(name,".exe")]`, "sequence", 2, 0},
			} {
				t.Run(test.query, func(t *testing.T) {
					analysis, err := Analyze(context.Background(), version, test.query, testLimits)
					if err != nil {
						t.Fatal(err)
					}
					if analysis.Kind != test.kind || analysis.SourceMode != "request_indices" || analysis.EventFilters != test.filters || analysis.MissingTerms != test.missing {
						t.Fatalf("unexpected AST summary: %+v", analysis)
					}
				})
			}
		})
	}
}

func TestEQLRejectsMalformedMultipleStatementsAndRedactsErrors(t *testing.T) {
	for _, query := range []string{"", `any where`, `any where true; any where false`, `any where true | head 1; DELETE private`, `SELECT * FROM private`, `FROM private | STATS count(*)`, `any where true ☃`, `sequence [any where true`, `any where name == "secret-value`, `any where true /* unfinished`, "any where \xff"} {
		_, err := Analyze(context.Background(), "8.19.21", query, testLimits)
		if err == nil || strings.Contains(err.Error(), "secret-value") || strings.Contains(err.Error(), "private") {
			t.Fatalf("invalid/unredacted failure: %q %v", query, err)
		}
	}
}

func TestEQLParserBudgetsCancellationAndIsolation(t *testing.T) {
	for _, query := range []string{
		"any where " + strings.Repeat("not ", 300) + "true",
		"any where " + strings.Repeat("(", 300) + "true" + strings.Repeat(")", 300),
		"any where " + strings.Repeat("true and ", 6000) + "true",
		"any where x == \"" + strings.Repeat("x", 256<<10) + "\"",
	} {
		if _, err := Analyze(context.Background(), "8.19.21", query, testLimits); err == nil || !strings.HasPrefix(err.Error(), "resource_limit:") {
			t.Fatal("unbounded query admitted", err)
		}
	}
	// Operator-looking text in a literal/comment does not count as recursion.
	query := "any where name == \"" + strings.Repeat("not ((( ", 300) + "\" /* " + strings.Repeat("( not ", 300) + "*/"
	if _, err := Analyze(context.Background(), "8.19.21", query, testLimits); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Analyze(ctx, "8.19.21", "any where true", testLimits); err == nil || !strings.HasPrefix(err.Error(), "deadline_exceeded:") {
		t.Fatal(err)
	}
	if _, err := Analyze(context.Background(), "9.99", "any where true", testLimits); err == nil {
		t.Fatal("unpinned grammar admitted")
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if _, err := Analyze(context.Background(), "9.1.10", `sequence [any where x == "α"] [any where true]`, testLimits); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if !memory.TryAcquire(MemoryLimit) {
		t.Fatal("parser reservations leaked")
	}
	memory.Release(MemoryLimit)
}

func FuzzEQLParser(f *testing.F) {
	for _, query := range []string{`any where true`, `sequence [process where true] ![network where false]`, `/* nested /* x */ */ any where true`, `any where x == "private-index"`, `any where (((true)))`} {
		f.Add(query)
	}
	f.Fuzz(func(t *testing.T, query string) {
		if len(query) > 4096 {
			t.Skip()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		analysis, err := Analyze(ctx, "8.19.21", query, Limits{Bytes: 4096, Tokens: 2000, Depth: 32})
		if err == nil && (analysis == nil || analysis.SourceMode != "request_indices") {
			t.Fatal("missing source contract")
		}
	})
}

func BenchmarkEQLParser(b *testing.B) {
	for _, test := range []struct{ name, query string }{
		{"event", `any where true`},
		{"sequence", `sequence by user.id with maxspan=5m [process where process.name : "cmd*"] [network where destination.port in (80,443)]`},
		{"wide_in", "any where name in (" + strings.Repeat("\"literal\",", 999) + "\"last\")"},
	} {
		b.Run(test.name, func(b *testing.B) {
			_, _ = Analyze(context.Background(), "8.19.21", test.query, testLimits)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Analyze(context.Background(), "8.19.21", test.query, testLimits); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(len(test.query)), "query_bytes")
		})
	}
}
