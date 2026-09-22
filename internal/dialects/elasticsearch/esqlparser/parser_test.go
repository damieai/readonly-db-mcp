package esqlparser

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var limits = Limits{Bytes: 256 << 10, Tokens: 10000, Depth: 128}

func TestPinnedESQLGrammarAndSources(t *testing.T) {
	tests := []struct {
		query   string
		sources []string
	}{
		{`ROW x = 9007199254740993, text = "DELETE FROM private" | EVAL y = ABS(-2) | KEEP x, y`, nil},
		{`FROM reports-* METADATA _index | WHERE price > 1 | STATS total = SUM(price) WHERE price > 2 BY category | SORT total DESC NULLS LAST | LIMIT 10`, []string{"reports-*"}},
		{`FROM "reports-one, reports-two" | LOOKUP JOIN reports-lookup ON id | EVAL a = COALESCE(title, "FROM private") | DROP id | RENAME a AS b`, []string{"reports-one", "reports-two", "reports-lookup"}},
		{`FROM reports-*::data | WHERE title : "secret JOIN private" | KEEP title`, []string{"reports-*::data"}},
		{`FROM """reports-*,remote:private"""`, []string{"reports-*", "remote:private"}},
		{`FROM reports-* /* nested /* secret */ comment */ | MV_EXPAND tags | DISSECT title "%{a}-%{b}" | GROK a "%{WORD:word}" | SAMPLE 0.5`, []string{"reports-*"}},
		{`SHOW INFO`, nil},
		{`FROM reports-* | CHANGE_POINT price ON time AS type, pvalue`, []string{"reports-*"}},
	}
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, tc := range tests {
			t.Run(version+tc.query, func(t *testing.T) {
				a, err := Analyze(context.Background(), version, tc.query, nil, limits)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(a.Sources, tc.sources) {
					t.Fatalf("sources=%v want=%v", a.Sources, tc.sources)
				}
			})
		}
	}
	for _, tc := range []struct {
		version, query string
		sources        []string
	}{
		{"8.19.21", `EXPLAIN [FROM reports-* | LOOKUP JOIN private ON id]`, []string{"reports-*", "private"}},
		{"9.1.10", `FROM reports-* | FORK (WHERE price > 1 | LOOKUP JOIN private ON id) (STATS n = COUNT(*))`, []string{"reports-*", "private"}},
	} {
		a, err := Analyze(context.Background(), tc.version, tc.query, nil, limits)
		if err != nil || !reflect.DeepEqual(a.Sources, tc.sources) {
			t.Fatal(a, err)
		}
	}
}

func TestESQLEffectsAndParameters(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, tc := range []struct {
			query               string
			params              []any
			fullText, inference bool
			enrich              []string
		}{
			{query: `FROM reports-* | ENRICH _coordinator:policy ON id WITH name`, enrich: []string{"_coordinator:policy"}},
			{query: `ROW x = "text" | COMPLETION answer = x WITH endpoint`, inference: true},
			{query: `FROM reports-* | WHERE title : "text"`, fullText: true},
			{query: `FROM reports-* | WHERE QSTR(?)`, params: []any{"text"}, fullText: true},
			{query: `FROM reports-* | WHERE ??fn(title, ?v)`, params: []any{map[string]any{"fn": "match"}, map[string]any{"v": "text"}}, fullText: true},
			{query: `FROM reports-* | WHERE ?fn(title, ?v)`, params: []any{map[string]any{"fn": map[string]any{"identifier": "match"}}, map[string]any{"v": "text"}}, fullText: true},
			{query: `ROW x = ?1 | EVAL y = ??2(x)`, params: []any{json.Number("2"), "abs"}},
			{query: `ROW x = "FROM private | ENRICH secret | COMPLETION x WITH model"`},
		} {
			a, err := Analyze(context.Background(), version, tc.query, tc.params, limits)
			if err != nil {
				t.Fatalf("%s: %v", tc.query, err)
			}
			if a.FullText != tc.fullText || a.Inference != tc.inference || !reflect.DeepEqual(a.EnrichPolicies, tc.enrich) {
				t.Fatalf("%s: %+v", tc.query, a)
			}
		}
	}
}

func TestEnrichPolicyQuotingMatchesNativeLogicalPlanBuilder(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, tc := range []struct{ query, name string }{
			{`ROW id=1 | ENRICH customers`, `customers`},
			{`ROW id=1 | ENRICH "customers"`, `customers`},
			{`ROW id=1 | ENRICH "_REMOTE:customers"`, `_REMOTE:customers`},
			{`ROW id=1 | ENRICH "a\tb"`, `a\tb`},
			{`ROW id=1 | ENRICH """customers"""`, `""customers""`},
		} {
			a, err := Analyze(context.Background(), version, tc.query, nil, limits)
			if err != nil || !reflect.DeepEqual(a.EnrichPolicies, []string{tc.name}) {
				t.Fatalf("%s %s: %+v %v", version, tc.query, a, err)
			}
		}
	}
}

func TestESQLMalformedBudgetsAndReleasePredicates(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, query := range []string{`FROM`, `ROW x=1; FROM private`, `FROM reports-* /* secret`, `FROM "reports-*`, `DELETE FROM reports-*`, `ROW x=?`, `FROM reports-* | INLINESTATS n=COUNT(*)`, `FROM reports-* | LOOKUP_🐔 private ON id`, "ROW x=\xff"} {
			if _, err := Analyze(context.Background(), version, query, nil, limits); err == nil {
				t.Fatalf("admitted %q", query)
			}
		}
		for _, query := range []string{`ROW x = ` + strings.Repeat("(", 300) + "1" + strings.Repeat(")", 300), `ROW x = ` + strings.Repeat("NOT ", 300) + "TRUE"} {
			if _, err := Analyze(context.Background(), version, query, nil, limits); err == nil || !strings.HasPrefix(err.Error(), "resource_limit:") {
				t.Fatal(err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Analyze(ctx, "9.1.10", `ROW x=1`, nil, limits); err == nil {
		t.Fatal("ignored cancellation")
	}
	if !memory.TryAcquire(MemoryLimit) {
		t.Fatal("parser memory leaked")
	}
	memory.Release(MemoryLimit)
}

func TestESQLConcurrentCancellation(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			version := "8.19.21"
			if i%2 != 0 {
				version = "9.1.10"
			}
			if _, err := Analyze(context.Background(), version, `FROM reports-* | STATS n=COUNT(*) BY category`, nil, limits); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	query := `ROW x=[` + strings.Repeat("1,", 9999) + "2]"
	if _, err := Analyze(ctx, "9.1.10", query, nil, Limits{Bytes: 256 << 10, Tokens: 30000, Depth: 128}); err == nil || !strings.HasPrefix(err.Error(), "deadline_exceeded:") {
		t.Fatal(err)
	}
	if !memory.TryAcquire(MemoryLimit) {
		t.Fatal("parser memory leaked")
	}
	memory.Release(MemoryLimit)
}

func FuzzESQLParser(f *testing.F) {
	for _, query := range []string{`FROM reports-* | LOOKUP JOIN private ON id`, `ROW x="FROM private"`, `FROM reports-* | FORK (STATS n=COUNT(*)) (LIMIT 1)`, `SHOW INFO`, `ROW x=1 /* nested /* comment */ end */`} {
		f.Add(query)
	}
	f.Fuzz(func(t *testing.T, query string) {
		if len(query) > 4096 {
			t.Skip()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		for _, version := range []string{"8.19.21", "9.1.10"} {
			_, _ = Analyze(ctx, version, query, nil, Limits{Bytes: 4096, Tokens: 2000, Depth: 32})
		}
	})
}

func BenchmarkESQLParser(b *testing.B) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		b.Run(version, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Analyze(context.Background(), version, `FROM reports-* | LOOKUP JOIN reports-lookup ON id | STATS n=COUNT(*) BY category | SORT n DESC | LIMIT 10`, nil, limits); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestESQLLongAssignmentLineageHonorsDepth(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		query := `ROW x = ` + strings.Repeat("field+", 300) + "tail_field"
		_, err := Analyze(context.Background(), version, query, nil, Limits{Bytes: 4096, Tokens: 2000, Depth: 8})
		if err == nil || !strings.HasPrefix(err.Error(), "resource_limit:") {
			t.Fatal("lineage traversal ignored depth", err)
		}
	}
	if !memory.TryAcquire(MemoryLimit) {
		t.Fatal("lineage reservation leaked")
	}
	memory.Release(MemoryLimit)
}
