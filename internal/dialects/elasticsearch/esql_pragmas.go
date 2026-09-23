package elasticsearch

import (
	"encoding/json"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

var pragmaDecimal = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

func validateESQLPragmas(value any, version string, profile *config.ElasticsearchESQLPragmaConfig) error {
	if profile == nil {
		return failure("capability_unavailable", "ES|QL pragmas require an operator resource profile")
	}
	pragmas, ok := value.(map[string]any)
	if !ok {
		return failure("invalid_request", "ES|QL pragma must be an object")
	}
	integers := map[string]struct{ max, min int }{
		"exchange_buffer_size":                  {profile.MaxExchangeBufferSize, 1},
		"exchange_concurrent_clients":           {profile.MaxExchangeConcurrentClients, 1},
		"enrich_max_workers":                    {profile.MaxEnrichWorkers, 1},
		"task_concurrency":                      {profile.MaxTaskConcurrency, 1},
		"page_size":                             {profile.MaxPageSize, 0},
		"max_concurrent_nodes_per_cluster":      {profile.MaxConcurrentNodesPerCluster, 0},
		"max_concurrent_shards_per_node":        {profile.MaxConcurrentShardsPerNode, 1},
		"unavailable_shard_resolution_attempts": {profile.MaxShardResolutionAttempts, 0},
	}
	for key, raw := range pragmas {
		if rule, ok := integers[key]; ok {
			n, err := pragmaInteger(raw)
			if err != nil {
				return err
			}
			if n == -1 && key == "max_concurrent_nodes_per_cluster" {
				// This is the native default, so it adds no cost over ordinary ES|QL.
				continue
			}
			if n == -1 && key == "unavailable_shard_resolution_attempts" && profile.AllowUnlimitedShardRetries {
				continue
			}
			if n < int64(rule.min) {
				return failure("invalid_request", "ES|QL pragma integer is outside its native domain")
			}
			if n > int64(rule.max) {
				return failure("resource_limit", "ES|QL pragma exceeds the operator resource ceiling")
			}
			continue
		}
		switch key {
		case "node_level_reduction":
			if _, ok := raw.(bool); !ok {
				return failure("invalid_request", "ES|QL reduction pragma must be boolean")
			}
		case "data_partitioning":
			if s, ok := raw.(string); !ok || len(s) > 64 {
				return failure("invalid_request", "ES|QL partitioning pragma must be a string")
			}
		case "field_extract_preference":
			if version != "9.1.10" {
				return failure("capability_unavailable", "field extraction preference is absent from the pinned build")
			}
			if s, ok := raw.(string); !ok || len(s) > 64 {
				return failure("invalid_request", "ES|QL field extraction preference must be a string")
			}
		case "status_interval":
			interval, err := pragmaDuration(raw)
			if err != nil {
				return err
			}
			if profile.MinStatusInterval == 0 || interval < profile.MinStatusInterval {
				return failure("resource_limit", "ES|QL status interval is below the operator minimum")
			}
		case "fold_limit":
			if err := pragmaFoldLimit(raw, profile); err != nil {
				return err
			}
		default:
			return failure("invalid_request", "unknown ES|QL pragma for the pinned build")
		}
	}
	return nil
}

func pragmaInteger(raw any) (int64, error) {
	var text string
	switch v := raw.(type) {
	case json.Number:
		text = string(v)
	case string:
		text = v
	default:
		return 0, failure("invalid_request", "ES|QL pragma requires an integer")
	}
	n, err := strconv.ParseInt(text, 10, 32)
	if err != nil {
		return 0, failure("invalid_request", "ES|QL pragma requires a native 32-bit integer")
	}
	return n, nil
}

func pragmaDuration(raw any) (time.Duration, error) {
	s, ok := raw.(string)
	if !ok || s == "" || len(s) > 64 {
		return 0, failure("invalid_request", "ES|QL status interval requires a time string")
	}
	s = strings.ToLower(strings.TrimSpace(s))
	for _, alias := range []struct{ native, goUnit string }{{"nanos", "ns"}, {"micros", "us"}} {
		if strings.HasSuffix(s, alias.native) {
			s = strings.TrimSuffix(s, alias.native) + alias.goUnit
			break
		}
	}
	for _, unit := range []struct {
		suffix string
		value  time.Duration
	}{{"w", 7 * 24 * time.Hour}, {"d", 24 * time.Hour}} {
		if strings.HasSuffix(s, unit.suffix) {
			n, err := strconv.ParseInt(strings.TrimSuffix(s, unit.suffix), 10, 64)
			if err != nil || n <= 0 || n > int64((1<<63-1)/unit.value) {
				return 0, failure("invalid_request", "invalid ES|QL status interval")
			}
			return time.Duration(n) * unit.value, nil
		}
	}
	interval, err := time.ParseDuration(s)
	if err != nil || interval <= 0 {
		return 0, failure("invalid_request", "invalid ES|QL status interval")
	}
	return interval, nil
}

func pragmaFoldLimit(raw any, profile *config.ElasticsearchESQLPragmaConfig) error {
	var s string
	switch v := raw.(type) {
	case string:
		s = strings.ToLower(strings.TrimSpace(v))
	case json.Number:
		s = string(v)
	default:
		return failure("invalid_request", "ES|QL fold limit requires a byte size or percentage")
	}
	if s == "" || len(s) > 64 {
		return failure("invalid_request", "ES|QL fold limit is empty")
	}
	if strings.HasSuffix(s, "%") {
		value := strings.TrimSuffix(s, "%")
		if !pragmaDecimal.MatchString(value) {
			return failure("invalid_request", "invalid ES|QL fold percentage")
		}
		percent, ok := new(big.Rat).SetString(value)
		if !ok || percent.Sign() < 0 {
			return failure("invalid_request", "invalid ES|QL fold percentage")
		}
		if percent.Cmp(big.NewRat(int64(profile.MaxFoldPercent), 1)) > 0 {
			return failure("resource_limit", "ES|QL fold percentage exceeds the operator ceiling")
		}
		return nil
	}
	scale := int64(1)
	for _, unit := range []struct {
		suffix string
		bytes  int64
	}{{"pb", 1 << 50}, {"tb", 1 << 40}, {"gb", 1 << 30}, {"mb", 1 << 20}, {"kb", 1 << 10}, {"b", 1}} {
		if strings.HasSuffix(s, unit.suffix) {
			s = strings.TrimSuffix(s, unit.suffix)
			scale = unit.bytes
			break
		}
	}
	if !pragmaDecimal.MatchString(s) {
		return failure("invalid_request", "invalid ES|QL fold byte size")
	}
	bytes, ok := new(big.Rat).SetString(s)
	if !ok || bytes.Sign() < 0 {
		return failure("invalid_request", "invalid ES|QL fold byte size")
	}
	bytes.Mul(bytes, big.NewRat(scale, 1))
	if !bytes.IsInt() || !bytes.Num().IsInt64() {
		return failure("invalid_request", "ES|QL fold size is not a native byte count")
	}
	if bytes.Num().Int64() > profile.MaxFoldBytes {
		return failure("resource_limit", "ES|QL fold size exceeds the operator ceiling")
	}
	return nil
}
