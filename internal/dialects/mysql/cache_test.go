package mysql

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestMetadataCacheClonesAndExpires(t *testing.T) {
	c := newMetadataCache(true, 2, 4096)
	key := "k"
	if leader, err := c.lead(context.Background(), key); err != nil || !leader {
		t.Fatalf("leader=%v err=%v", leader, err)
	}
	c.finish(key, []core.TableSummary{{Name: "items"}}, 10*time.Millisecond, true)
	v, ok := c.get(key)
	if !ok {
		t.Fatal("miss")
	}
	v.([]core.TableSummary)[0].Name = "changed"
	v, _ = c.get(key)
	if v.([]core.TableSummary)[0].Name != "items" {
		t.Fatal("cache value mutated")
	}
	time.Sleep(15 * time.Millisecond)
	if _, ok := c.get(key); ok {
		t.Fatal("expected expiry")
	}
}

func TestResultCacheClones(t *testing.T) {
	c := newResultCache(true, time.Minute, 2, 4096, 4096)
	r := &core.QueryResult{Rows: []map[string]any{{"v": "safe"}}}
	c.put("k", r)
	got, _, ok := c.get("k")
	if !ok {
		t.Fatal("miss")
	}
	got.Rows[0]["v"] = "changed"
	again, _, _ := c.get("k")
	if again.Rows[0]["v"] != "safe" {
		t.Fatal("cache value mutated")
	}
}

func TestPolicyMarksVolatileQueriesUncacheable(t *testing.T) {
	p := NewPolicy("inventory", []string{"inventory"}, nil, 1024)
	for _, q := range []string{"SELECT RAND()", "SELECT CURRENT_TIMESTAMP"} {
		v, err := p.Validate(q)
		if err != nil {
			t.Fatal(err)
		}
		if v.Cacheable {
			t.Fatalf("expected %q uncacheable", q)
		}
	}
}

func TestExactQueryBudget(t *testing.T) {
	r := &core.QueryResult{Target: "t", Columns: []core.Column{{Name: "value"}}, Rows: []map[string]any{{"value": "aaaaaaaa"}, {"value": "bbbbbbbb"}}, RowCount: 2}
	empty := *r
	empty.Rows = nil
	empty.RowCount = 0
	base, _ := json.Marshal(&empty)
	full, _ := json.Marshal(r)
	if err := enforceQueryBudget(r, (len(base)+len(full))/2); err != nil {
		t.Fatal(err)
	}
	if !r.Truncated || r.RowCount >= 2 {
		t.Fatalf("not truncated: %#v", r)
	}
}
