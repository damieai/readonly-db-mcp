package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

func BenchmarkPolicyValidate(b *testing.B) {
	p := NewPolicy("inventory", []string{"inventory"}, nil, 32<<10)
	q := "WITH x AS (SELECT company_id, SUM(quantity) total FROM inventory.movements GROUP BY company_id) SELECT * FROM x WHERE total > ? ORDER BY total DESC LIMIT 100"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Validate(q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMetadataCacheHit(b *testing.B) {
	c := newMetadataCache(true, 256, 8<<20)
	c.lead(context.Background(), "k")
	c.finish("k", []core.TableSummary{{Schema: "inventory", Name: "items", Type: "BASE TABLE"}}, time.Minute, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := c.get("k"); !ok {
			b.Fatal("miss")
		}
	}
}

func BenchmarkResponseBudget(b *testing.B) {
	r := &core.QueryResult{Target: "test", Columns: []core.Column{{Name: "id"}, {Name: "status"}}, Rows: make([]map[string]any, 100)}
	for i := range r.Rows {
		r.Rows[i] = map[string]any{"id": int64(i), "status": "ready"}
	}
	r.RowCount = len(r.Rows)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy := cloneQueryResult(r)
		if err := enforceQueryBudget(copy, 1<<20); err != nil {
			b.Fatal(err)
		}
	}
}
