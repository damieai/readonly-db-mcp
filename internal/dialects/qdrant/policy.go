package qdrant

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

func strictJSON(data []byte, maxDepth, maxNodes int) error {
	if !utf8.Valid(data) {
		return errors.New("invalid_request: JSON must be UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	nodes := 0
	var walk func(int) error
	walk = func(depth int) error {
		nodes++
		if depth > maxDepth || nodes > maxNodes {
			return errors.New("resource_limit: JSON depth or node count exceeded")
		}
		tok, err := d.Token()
		if err != nil {
			return errors.New("invalid_request: malformed JSON")
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					key, err := d.Token()
					if err != nil {
						return errors.New("invalid_request: malformed JSON object")
					}
					name, ok := key.(string)
					if !ok || seen[name] {
						return errors.New("invalid_request: duplicate or invalid JSON key")
					}
					seen[name] = true
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
			default:
				return errors.New("invalid_request: unexpected JSON delimiter")
			}
			if _, err := d.Token(); err != nil {
				return errors.New("invalid_request: unclosed JSON container")
			}
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("invalid_request: trailing JSON data")
	}
	return nil
}

func attestJWT(token string, allowed []string, now time.Time) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(token) > 64<<10 {
		return time.Time{}, errors.New("configuration_error: Qdrant requires a compact scoped JWT")
	}
	decode := func(s string) (map[string]json.RawMessage, error) {
		data, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil || len(data) > 16<<10 || strictJSON(data, 16, 1000) != nil {
			return nil, errors.New("invalid JWT")
		}
		var v map[string]json.RawMessage
		if json.Unmarshal(data, &v) != nil || v == nil {
			return nil, errors.New("invalid JWT object")
		}
		return v, nil
	}
	header, err := decode(parts[0])
	if err != nil {
		return time.Time{}, errors.New("configuration_error: invalid Qdrant JWT header")
	}
	var alg string
	if json.Unmarshal(header["alg"], &alg) != nil || alg != "HS256" || len(header["crit"]) != 0 {
		return time.Time{}, errors.New("configuration_error: Qdrant JWT must use HS256 without critical extensions")
	}
	claims, err := decode(parts[1])
	if err != nil {
		return time.Time{}, errors.New("configuration_error: invalid Qdrant JWT claims")
	}
	var exp json.Number
	if json.Unmarshal(claims["exp"], &exp) != nil {
		return time.Time{}, errors.New("configuration_error: Qdrant JWT exp is required")
	}
	expSeconds, err := strconv.ParseInt(exp.String(), 10, 64)
	if err != nil || expSeconds <= now.Add(time.Minute).Unix() {
		return time.Time{}, errors.New("permission_denied: Qdrant scoped JWT is expired or about to expire")
	}
	var access []struct {
		Collection string `json:"collection"`
		Access     string `json:"access"`
	}
	if json.Unmarshal(claims["access"], &access) != nil || len(access) != len(allowed) {
		return time.Time{}, errors.New("permission_denied: Qdrant JWT collection scope differs from configuration")
	}
	expected := map[string]bool{}
	for _, name := range allowed {
		expected[name] = true
	}
	for _, entry := range access {
		if !expected[entry.Collection] || entry.Access != "r" {
			return time.Time{}, errors.New("permission_denied: Qdrant JWT must grant only configured collection reads")
		}
		delete(expected, entry.Collection)
	}
	if len(expected) != 0 {
		return time.Time{}, errors.New("permission_denied: Qdrant JWT collection scope is incomplete")
	}
	if len(claims["value_exists"]) != 0 {
		return time.Time{}, errors.New("permission_denied: Qdrant JWT value_exists requires a separate scope proof")
	}
	if parts[2] == "" {
		return time.Time{}, errors.New("configuration_error: Qdrant JWT signature is missing")
	}
	if _, err := base64.RawURLEncoding.DecodeString(parts[2]); err != nil {
		return time.Time{}, errors.New("configuration_error: invalid Qdrant JWT signature")
	}
	return time.Unix(expSeconds, 0), nil
}

func validateBody(operation string, data []byte, limits config.Limits, q *config.QdrantConfig, allowed map[string]bool) error {
	if len(data) == 0 || len(data) > q.MaxRequestBytes {
		return errors.New("resource_limit: Qdrant request body is missing or too large")
	}
	if err := strictJSON(data, q.MaxJSONDepth, q.MaxJSONNodes); err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil || root == nil {
		return errors.New("invalid_request: Qdrant body must be an object")
	}
	budget := 0
	var validateQuery func(map[string]json.RawMessage, int) error
	validateQuery = func(node map[string]json.RawMessage, depth int) error {
		if depth > 16 {
			return errors.New("resource_limit: prefetch nesting exceeds 16")
		}
		if raw := node["lookup_from"]; len(raw) != 0 {
			var lookup struct {
				Collection string `json:"collection"`
			}
			if json.Unmarshal(raw, &lookup) != nil || !allowed[lookup.Collection] {
				return errors.New("permission_denied: lookup_from collection is outside target scope")
			}
		}
		if raw := node["query"]; len(raw) != 0 {
			if containsInference(raw) {
				return errors.New("permission_denied: query-time inference is unavailable")
			}
		}
		limit, err := boundedInt(node, "limit", 10, limits.MaxRows)
		if err != nil {
			return err
		}
		budget += limit
		if budget > limits.MaxRows*8 {
			return errors.New("resource_limit: aggregate query/prefetch limit exceeded")
		}
		if _, err := boundedInt(node, "offset", 0, limits.MaxRows*8); err != nil {
			return err
		}
		if raw := node["prefetch"]; len(raw) != 0 {
			var list []json.RawMessage
			if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
				if json.Unmarshal(raw, &list) != nil || len(list) == 0 || len(list) > limits.MaxBatchQueries {
					return errors.New("resource_limit: prefetch branch count exceeded")
				}
			} else {
				list = []json.RawMessage{raw}
			}
			for _, item := range list {
				var child map[string]json.RawMessage
				if json.Unmarshal(item, &child) != nil || child == nil {
					return errors.New("invalid_request: prefetch must contain objects")
				}
				if err := validateQuery(child, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	switch operation {
	case "query", "query_groups":
		if err := validateQuery(root, 0); err != nil {
			return err
		}
		if operation == "query_groups" {
			groupSize, err := boundedInt(root, "group_size", 3, limits.MaxRows)
			if err != nil {
				return err
			}
			groups, err := boundedInt(root, "limit", 10, limits.MaxRows)
			if err != nil {
				return err
			}
			if float64(groupSize)*float64(groups) > float64(limits.MaxRows) {
				return errors.New("resource_limit: grouped result count exceeds max_rows")
			}
			if raw := root["with_lookup"]; len(raw) != 0 {
				var collection string
				if json.Unmarshal(raw, &collection) != nil {
					var lookup struct {
						Collection string `json:"collection"`
					}
					if json.Unmarshal(raw, &lookup) != nil {
						return errors.New("invalid_request: with_lookup must name a collection")
					}
					collection = lookup.Collection
				}
				if !allowed[collection] {
					return errors.New("permission_denied: with_lookup collection is outside target scope")
				}
			}
		}
	case "query_batch":
		var searches []map[string]json.RawMessage
		if json.Unmarshal(root["searches"], &searches) != nil || len(searches) == 0 || len(searches) > limits.MaxBatchQueries {
			return errors.New("resource_limit: query_batch searches count is invalid")
		}
		for _, search := range searches {
			if err := validateQuery(search, 0); err != nil {
				return err
			}
		}
	case "scroll", "facet":
		if _, err := boundedInt(root, "limit", 10, limits.MaxRows); err != nil {
			return err
		}
	case "retrieve":
		var ids []json.RawMessage
		if json.Unmarshal(root["ids"], &ids) != nil || len(ids) == 0 || len(ids) > limits.MaxRows {
			return errors.New("resource_limit: retrieve ids count is invalid")
		}
	case "count":
	default:
		return errors.New("invalid_request: unsupported Qdrant read operation")
	}
	return nil
}

func containsInference(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	switch trimmed[0] {
	case '{':
		var fields map[string]json.RawMessage
		if json.Unmarshal(trimmed, &fields) != nil {
			return false
		}
		if _, model := fields["model"]; model {
			for _, source := range []string{"text", "image", "object"} {
				if _, present := fields[source]; present {
					return true
				}
			}
		}
		for _, value := range fields {
			if containsInference(value) {
				return true
			}
		}
	case '[':
		var values []json.RawMessage
		if json.Unmarshal(trimmed, &values) != nil {
			return false
		}
		for _, value := range values {
			if containsInference(value) {
				return true
			}
		}
	}
	return false
}

func boundedInt(node map[string]json.RawMessage, field string, fallback, maxValue int) (int, error) {
	raw := node[field]
	if len(raw) == 0 {
		if fallback > maxValue {
			return 0, fmt.Errorf("resource_limit: explicit %s is required below native default", field)
		}
		return fallback, nil
	}
	var number json.Number
	if json.Unmarshal(raw, &number) != nil {
		return 0, fmt.Errorf("invalid_request: %s must be an integer", field)
	}
	value, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil || value < 0 || value > int64(maxValue) || value > math.MaxInt {
		return 0, fmt.Errorf("resource_limit: %s exceeds configured limit", field)
	}
	return int(value), nil
}
