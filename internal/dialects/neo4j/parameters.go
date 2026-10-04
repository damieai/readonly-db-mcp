package neo4j

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

func validateParameters(input map[string]any, limits config.Limits) (map[string]any, error) {
	if len(input) > limits.MaxParameters {
		return nil, errors.New("resource_limit: too many Neo4j parameters")
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > limits.MaxParameterBytes {
		return nil, errors.New("resource_limit: Neo4j parameters exceed byte limit")
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		if key == "" || len(key) > 256 || !utf8.ValidString(key) {
			return nil, errors.New("invalid_request: Neo4j parameter name is invalid")
		}
		encoded, err := json.Marshal(value)
		if err != nil || len(encoded) > limits.MaxParameterValueBytes {
			return nil, errors.New("resource_limit: Neo4j parameter value exceeds byte limit")
		}
		converted, err := convertParameter(value, 0)
		if err != nil {
			return nil, err
		}
		out[key] = converted
	}
	return out, nil
}

func convertParameter(value any, depth int) (any, error) {
	if depth > 32 {
		return nil, errors.New("resource_limit: Neo4j parameter nesting exceeds limit")
	}
	switch v := value.(type) {
	case nil, bool, string, int, int8, int16, int32, int64, float32:
		return v, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("invalid_request: Neo4j float is not finite")
		}
		if math.Trunc(v) == v && math.Abs(v) > maxSafeInteger {
			return nil, errors.New("invalid_request: Neo4j integer-valued JSON number exceeds exact float range; pass a decimal string and use toInteger")
		}
		return v, nil
	case json.Number:
		if i, err := strconv.ParseInt(string(v), 10, 64); err == nil {
			return i, nil
		}
		if !strings.ContainsAny(string(v), ".eE") {
			return nil, errors.New("invalid_request: Neo4j integer exceeds int64 range")
		}
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, errors.New("invalid_request: Neo4j number is invalid")
		}
		return f, nil
	case []any:
		if len(v) > 10000 {
			return nil, errors.New("resource_limit: Neo4j parameter list is too long")
		}
		out := make([]any, len(v))
		for i, item := range v {
			converted, err := convertParameter(item, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = converted
		}
		return out, nil
	case map[string]any:
		if len(v) > 10000 {
			return nil, errors.New("resource_limit: Neo4j parameter map is too large")
		}
		out := make(map[string]any, len(v))
		for key, item := range v {
			if len(key) > 256 || !utf8.ValidString(key) {
				return nil, errors.New("invalid_request: Neo4j parameter key is invalid")
			}
			converted, err := convertParameter(item, depth+1)
			if err != nil {
				return nil, err
			}
			out[key] = converted
		}
		return out, nil
	default:
		return nil, errors.New("invalid_request: unsupported Neo4j parameter value")
	}
}
