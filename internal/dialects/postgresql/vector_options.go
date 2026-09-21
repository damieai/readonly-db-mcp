package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strconv"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

func validateVectorOptions(o *core.PostgreSQLQueryOptions, enabled bool) error {
	if o == nil {
		return nil
	}
	if !enabled {
		return errors.New("postgresql_options require an enabled pgvector profile")
	}
	for _, bound := range []struct {
		v   *int
		max int
	}{{o.HNSWEfSearch, 1000}, {o.HNSWMaxScanTuples, 2147483647}, {o.IVFFlatProbes, 32768}, {o.IVFFlatMaxProbes, 32768}} {
		if bound.v != nil && (*bound.v < 1 || *bound.v > bound.max) {
			return errors.New("vector option exceeds its native version range")
		}
	}
	if p := o.HNSWScanMemMultiplier; p != nil && (math.IsNaN(*p) || math.IsInf(*p, 0) || *p < 1 || *p > 1000) {
		return errors.New("hnsw_scan_mem_multiplier must be between 1 and 1000")
	}
	if p := o.HNSWIterativeScan; p != nil && *p != "off" && *p != "strict_order" && *p != "relaxed_order" {
		return errors.New("invalid HNSW iterative scan mode")
	}
	if p := o.IVFFlatIterativeScan; p != nil && *p != "off" && *p != "relaxed_order" {
		return errors.New("invalid IVFFlat iterative scan mode")
	}
	return nil
}

func (t *Target) applyVectorOptions(ctx context.Context, tx *sql.Tx, o *core.PostgreSQLQueryOptions) error {
	settings := map[string]string{}
	if o != nil {
		for name, p := range map[string]*int{"hnsw.ef_search": o.HNSWEfSearch, "hnsw.max_scan_tuples": o.HNSWMaxScanTuples, "ivfflat.probes": o.IVFFlatProbes, "ivfflat.max_probes": o.IVFFlatMaxProbes} {
			if p != nil {
				settings[name] = strconv.Itoa(*p)
			}
		}
		for name, p := range map[string]*string{"hnsw.iterative_scan": o.HNSWIterativeScan, "ivfflat.iterative_scan": o.IVFFlatIterativeScan} {
			if p != nil {
				settings[name] = *p
			}
		}
		if p := o.HNSWScanMemMultiplier; p != nil {
			settings["hnsw.scan_mem_multiplier"] = strconv.FormatFloat(*p, 'g', -1, 64)
		}
	}
	// Compare the effective backend work_mem, including an operator-set default.
	// This bounds the HNSW iterative-scan allowance, not total backend memory.
	var workMem int64
	var multiplier float64
	if tx.QueryRowContext(ctx, `SELECT pg_catalog.pg_size_bytes(current_setting('work_mem')), COALESCE(NULLIF(current_setting('hnsw.scan_mem_multiplier',true),''),'1')::float8`).Scan(&workMem, &multiplier) != nil {
		return errors.New("inspect vector scan memory allowance")
	}
	if o != nil && o.HNSWScanMemMultiplier != nil {
		multiplier = *o.HNSWScanMemMultiplier
	}
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier < 1 || float64(workMem)*multiplier > float64(t.cfg.PostgreSQL.PGVector.MaxScanMemoryBytes) {
		return errors.New("vector scan memory allowance exceeds configured maximum")
	}
	for name, value := range settings {
		if _, err := tx.ExecContext(ctx, `SELECT pg_catalog.set_config($1,$2,true)`, name, value); err != nil {
			return sanitize(err)
		}
	}
	return nil
}
