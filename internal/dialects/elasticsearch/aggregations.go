package elasticsearch

import "encoding/json"

// Aggregation names and metadata are caller data. Only aggregation types and
// executable child positions are interpreted. Pipeline scripts and mutations
// of request-local scripted_metric state are ordinary reads.
func (p *queryProof) aggregations(v any, rows, depth int) error {
	if err := p.step(depth); err != nil {
		return err
	}
	aggs, err := object(v)
	if err != nil {
		return err
	}
	for _, v := range aggs {
		agg, err := object(v)
		if err != nil {
			return err
		}
		types := 0
		for kind, value := range agg {
			switch kind {
			case "meta":
				continue
			case "aggs", "aggregations":
				if err := p.aggregations(value, rows, depth+1); err != nil {
					return err
				}
				continue
			}
			types++
			m, err := object(value)
			if err != nil {
				return err
			}
			switch kind {
			case "filter":
				if err := p.query(m, rows, depth+1); err != nil {
					return err
				}
			case "filters":
				if filters, ok := m["filters"]; ok {
					switch filters := filters.(type) {
					case []any:
						for _, q := range filters {
							if err := p.query(q, rows, depth+1); err != nil {
								return err
							}
						}
					case map[string]any:
						for _, q := range filters {
							if err := p.query(q, rows, depth+1); err != nil {
								return err
							}
						}
					default:
						return failure("invalid_request", "filters aggregation requires named or array filters")
					}
				}
			case "adjacency_matrix":
				filters, err := object(m["filters"])
				if err != nil {
					return err
				}
				for _, q := range filters {
					if err := p.query(q, rows, depth+1); err != nil {
						return err
					}
				}
			case "significant_terms", "significant_text":
				if q, ok := m["background_filter"]; ok {
					if err := p.query(q, rows, depth+1); err != nil {
						return err
					}
				}
				if v, ok := m["script_heuristic"]; ok {
					heuristic, err := object(v)
					if err != nil {
						return err
					}
					if err := p.scriptAt(heuristic, "script"); err != nil {
						return err
					}
				}
			case "top_hits":
				// Native top_hits defaults to three hits, not the search default ten.
				if err := pageSize(m, rows, 3); err != nil {
					return err
				}
				copy := map[string]any{}
				for k, v := range m {
					copy[k] = v
				}
				if _, ok := copy["size"]; !ok {
					copy["size"] = json.Number("3")
				}
				if err := p.search(copy, rows, depth+1); err != nil {
					return err
				}
				for k, v := range copy {
					m[k] = v
				}
			case "scripted_metric":
				for _, key := range []string{"init_script", "map_script", "combine_script", "reduce_script"} {
					if err := p.scriptAt(m, key); err != nil {
						return err
					}
				}
			case "composite":
				sources, ok := m["sources"].([]any)
				if !ok {
					return failure("invalid_request", "composite aggregation requires sources")
				}
				for _, v := range sources {
					source, err := object(v)
					if err != nil {
						return err
					}
					for _, v := range source {
						definition, err := object(v)
						if err != nil {
							return err
						}
						for kind, v := range definition {
							switch kind {
							case "terms", "histogram", "date_histogram", "geotile_grid":
							default:
								return failure("capability_unavailable", "composite source requires a plugin profile")
							}
							options, err := object(v)
							if err != nil {
								return err
							}
							if err := p.scriptAt(options, "script"); err != nil {
								return err
							}
						}
					}
				}
			case "avg", "weighted_avg", "sum", "min", "max", "stats", "extended_stats", "value_count", "cardinality", "percentiles", "percentile_ranks", "median_absolute_deviation", "boxplot", "string_stats", "t_test", "rate", "geo_bounds", "geo_centroid", "geo_line", "cartesian_bounds", "cartesian_centroid", "top_metrics",
				"global", "missing", "nested", "reverse_nested", "children", "parent", "terms", "rare_terms", "multi_terms", "histogram", "date_histogram", "auto_date_histogram", "variable_width_histogram", "range", "date_range", "ip_range", "ip_prefix", "geo_distance", "geohash_grid", "geotile_grid", "geohex_grid", "sampler", "diversified_sampler", "random_sampler", "categorize_text", "frequent_item_sets", "time_series",
				"avg_bucket", "sum_bucket", "min_bucket", "max_bucket", "stats_bucket", "extended_stats_bucket", "percentiles_bucket", "moving_fn", "moving_percentiles", "derivative", "cumulative_sum", "cumulative_cardinality", "serial_diff", "bucket_script", "bucket_selector", "bucket_sort", "normalize", "bucket_count_ks_test", "bucket_correlation", "inference":
				if kind == "inference" {
					return failure("capability_unavailable", "aggregation inference requires an explicit model profile")
				}
			default:
				return failure("capability_unavailable", "aggregation type requires a reviewed effect/source profile")
			}
			if err := p.scriptAt(m, "script"); err != nil {
				return err
			}
			if v, ok := m["sort"]; ok {
				if err := p.sort(v); err != nil {
					return err
				}
			}
			// Multi-value metric definitions contain scripts below value/weight,
			// terms and population keys. Traverse those exact grammar positions.
			for _, key := range []string{"value", "weight", "a", "b"} {
				if v, ok := m[key]; ok {
					if field, ok := v.(map[string]any); ok {
						if err := p.scriptAt(field, "script"); err != nil {
							return err
						}
					}
				}
			}
			if kind == "multi_terms" {
				for _, v := range list(m["terms"]) {
					term, err := object(v)
					if err != nil {
						return err
					}
					if err := p.scriptAt(term, "script"); err != nil {
						return err
					}
				}
			}
		}
		if types != 1 {
			return failure("invalid_request", "aggregation must name exactly one type")
		}
	}
	return nil
}
