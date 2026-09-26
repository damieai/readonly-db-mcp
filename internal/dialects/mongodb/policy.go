package mongodb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

func strictJSON(data []byte, maxDepth, maxNodes int) error {
	if !utf8.Valid(data) {
		return errors.New("invalid_request: MongoDB JSON must be UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	nodes := 0
	var walk func(int) error
	walk = func(depth int) error {
		nodes++
		if depth > maxDepth || nodes > maxNodes {
			return errors.New("resource_limit: MongoDB JSON structure exceeds configured limit")
		}
		token, err := d.Token()
		if err != nil {
			return errors.New("invalid_request: malformed MongoDB JSON")
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					key, err := d.Token()
					if err != nil {
						return errors.New("invalid_request: malformed MongoDB JSON key")
					}
					name, ok := key.(string)
					if !ok || seen[name] {
						return errors.New("invalid_request: duplicate MongoDB JSON key")
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
				return errors.New("invalid_request: unexpected MongoDB JSON delimiter")
			}
			if _, err := d.Token(); err != nil {
				return errors.New("invalid_request: unclosed MongoDB JSON container")
			}
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("invalid_request: trailing MongoDB JSON data")
	}
	return nil
}

type parsedRequest struct {
	fields map[string]json.RawMessage
	limit  int
	skip   int
}

func validateRequest(operation string, body []byte, limits config.Limits, cfg *config.MongoDBConfig, allowed map[string]bool) (parsedRequest, error) {
	if operation == "metadata" {
		if len(body) != 0 {
			return parsedRequest{}, errors.New("invalid_request: MongoDB metadata does not accept a body")
		}
		return parsedRequest{}, nil
	}
	if len(body) == 0 || len(body) > cfg.MaxRequestBytes {
		return parsedRequest{}, errors.New("resource_limit: MongoDB body is missing or exceeds max_request_bytes")
	}
	if err := strictJSON(body, cfg.MaxJSONDepth, cfg.MaxJSONNodes); err != nil {
		return parsedRequest{}, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || fields == nil {
		return parsedRequest{}, errors.New("invalid_request: MongoDB body must be an object")
	}
	allowedFields := map[string]bool{}
	switch operation {
	case "find":
		for _, name := range []string{"filter", "projection", "sort", "hint", "let", "min", "max", "collation", "skip", "limit"} {
			allowedFields[name] = true
		}
	case "count":
		for _, name := range []string{"filter", "hint", "collation"} {
			allowedFields[name] = true
		}
	case "aggregate":
		for _, name := range []string{"pipeline", "hint", "let", "collation", "limit"} {
			allowedFields[name] = true
		}
	default:
		return parsedRequest{}, errors.New("invalid_request: unsupported MongoDB read operation")
	}
	for name, value := range fields {
		if !allowedFields[name] || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return parsedRequest{}, fmt.Errorf("invalid_request: unsupported or null MongoDB field %q", name)
		}
	}
	if err := rejectUnsupportedOptions(fields, operation); err != nil {
		return parsedRequest{}, err
	}
	if err := rejectCode(body); err != nil {
		return parsedRequest{}, err
	}
	if operation == "aggregate" {
		pipeline := fields["pipeline"]
		if len(pipeline) == 0 {
			return parsedRequest{}, errors.New("invalid_request: aggregate requires pipeline")
		}
		stages := 0
		if err := validatePipeline(pipeline, allowed, cfg.MaxPipelineStages, &stages, 0); err != nil {
			return parsedRequest{}, err
		}
	}
	maxLimit := limits.MaxRows
	limit, err := boundedInteger(fields["limit"], maxLimit, maxLimit)
	if err != nil {
		return parsedRequest{}, fmt.Errorf("resource_limit: MongoDB limit: %w", err)
	}
	if limit == 0 {
		limit = maxLimit
	}
	skip, err := boundedInteger(fields["skip"], 0, limits.MaxRows*8)
	if err != nil {
		return parsedRequest{}, fmt.Errorf("resource_limit: MongoDB skip: %w", err)
	}
	return parsedRequest{fields: fields, limit: limit, skip: skip}, nil
}

func boundedInteger(raw json.RawMessage, fallback, maximum int) (int, error) {
	if len(raw) == 0 {
		return fallback, nil
	}
	var value json.Number
	if json.Unmarshal(raw, &value) != nil {
		return 0, errors.New("must be an integer")
	}
	n, err := strconv.ParseInt(value.String(), 10, 64)
	if err != nil || n < 0 || n > int64(maximum) {
		return 0, errors.New("outside configured bound")
	}
	return int(n), nil
}

var readStages = map[string]bool{
	"$match": true, "$project": true, "$addFields": true, "$set": true, "$unset": true,
	"$sort": true, "$limit": true, "$skip": true, "$group": true, "$count": true,
	"$unwind": true, "$replaceRoot": true, "$replaceWith": true, "$bucket": true,
	"$bucketAuto": true, "$sortByCount": true, "$sample": true, "$redact": true,
	"$densify": true, "$fill": true, "$setWindowFields": true, "$facet": true,
	"$lookup": true, "$graphLookup": true, "$unionWith": true, "$documents": true,
	"$geoNear": true,
}

func validatePipeline(raw json.RawMessage, allowed map[string]bool, maxStages int, stages *int, depth int) error {
	if depth > 12 {
		return errors.New("resource_limit: MongoDB pipeline nesting exceeds 12")
	}
	var pipeline []json.RawMessage
	if json.Unmarshal(raw, &pipeline) != nil || len(pipeline) == 0 {
		return errors.New("invalid_request: pipeline must be a nonempty array")
	}
	for _, item := range pipeline {
		*stages++
		if *stages > maxStages {
			return errors.New("resource_limit: MongoDB pipeline stage count exceeded")
		}
		var stage map[string]json.RawMessage
		if json.Unmarshal(item, &stage) != nil || len(stage) != 1 {
			return errors.New("invalid_request: each MongoDB pipeline stage needs one operator")
		}
		for name, argument := range stage {
			if !readStages[name] {
				return fmt.Errorf("permission_denied: MongoDB stage %q is unavailable", name)
			}
			switch name {
			case "$lookup":
				var spec map[string]json.RawMessage
				if json.Unmarshal(argument, &spec) != nil || spec == nil {
					return errors.New("invalid_request: $lookup must be an object")
				}
				if err := foreignCollection(spec["from"], spec["pipeline"], allowed); err != nil {
					return err
				}
				if len(spec["pipeline"]) != 0 {
					if err := validatePipeline(spec["pipeline"], allowed, maxStages, stages, depth+1); err != nil {
						return err
					}
				}
			case "$graphLookup":
				var spec map[string]json.RawMessage
				if json.Unmarshal(argument, &spec) != nil || !allowedString(spec["from"], allowed) {
					return errors.New("permission_denied: $graphLookup foreign collection is outside scope")
				}
			case "$unionWith":
				var name string
				if json.Unmarshal(argument, &name) == nil {
					if !allowed[name] {
						return errors.New("permission_denied: $unionWith collection is outside scope")
					}
				} else {
					var spec map[string]json.RawMessage
					if json.Unmarshal(argument, &spec) != nil || spec == nil {
						return errors.New("invalid_request: $unionWith must be a string or object")
					}
					if err := foreignCollection(spec["coll"], spec["pipeline"], allowed); err != nil {
						return err
					}
					if len(spec["pipeline"]) != 0 {
						if err := validatePipeline(spec["pipeline"], allowed, maxStages, stages, depth+1); err != nil {
							return err
						}
					}
				}
			case "$facet":
				var branches map[string]json.RawMessage
				if json.Unmarshal(argument, &branches) != nil || len(branches) == 0 {
					return errors.New("invalid_request: $facet needs branch pipelines")
				}
				for _, branch := range branches {
					if err := validatePipeline(branch, allowed, maxStages, stages, depth+1); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func foreignCollection(raw, pipeline json.RawMessage, allowed map[string]bool) error {
	if len(raw) != 0 {
		if !allowedString(raw, allowed) {
			return errors.New("permission_denied: MongoDB foreign collection is outside scope")
		}
		return nil
	}
	var stages []map[string]json.RawMessage
	if json.Unmarshal(pipeline, &stages) != nil || len(stages) == 0 || len(stages[0]) != 1 || len(stages[0]["$documents"]) == 0 {
		return errors.New("permission_denied: MongoDB foreign collection is required unless the pipeline starts with $documents")
	}
	return nil
}

func allowedString(raw json.RawMessage, allowed map[string]bool) bool {
	var name string
	return json.Unmarshal(raw, &name) == nil && allowed[name]
}

func rejectCode(raw json.RawMessage) error {
	var walk func(json.RawMessage) error
	walk = func(value json.RawMessage) error {
		trimmed := bytes.TrimSpace(value)
		if len(trimmed) == 0 {
			return nil
		}
		switch trimmed[0] {
		case '{':
			var fields map[string]json.RawMessage
			if json.Unmarshal(trimmed, &fields) != nil {
				return errors.New("invalid_request: malformed MongoDB object")
			}
			for name, inner := range fields {
				if name == "$where" || name == "$function" || name == "$accumulator" || name == "$code" {
					return fmt.Errorf("permission_denied: MongoDB server-side code operator %q is unavailable", name)
				}
				if err := walk(inner); err != nil {
					return err
				}
			}
		case '[':
			var items []json.RawMessage
			if json.Unmarshal(trimmed, &items) != nil {
				return errors.New("invalid_request: malformed MongoDB array")
			}
			for _, inner := range items {
				if err := walk(inner); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(raw)
}

func rejectUnsupportedOptions(fields map[string]json.RawMessage, operation string) error {
	for _, name := range []string{"filter", "projection", "sort", "let", "min", "max"} {
		if raw := fields[name]; len(raw) != 0 && !strings.HasPrefix(string(bytes.TrimSpace(raw)), "{") {
			return fmt.Errorf("invalid_request: MongoDB %s must be an object", name)
		}
	}
	if operation == "aggregate" && len(fields["pipeline"]) == 0 {
		return errors.New("invalid_request: aggregate pipeline is required")
	}
	return nil
}
