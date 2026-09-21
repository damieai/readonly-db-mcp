package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"unicode/utf8"
)

// ValidateJSON is also used before binding the public MCP envelope.
func ValidateJSON(raw []byte, depthLimit, nodeLimit int) error {
	return strictJSON(raw, depthLimit, nodeLimit)
}

// Validate before typed decoding. Duplicate/unknown authority fields must never
// be silently overwritten, and numbers in public results remain original bytes.
func strictJSON(raw []byte, depthLimit, nodeLimit int) error {
	return strictJSONContext(context.Background(), raw, depthLimit, nodeLimit, 0)
}

func strictJSONContext(ctx context.Context, raw []byte, depthLimit, nodeLimit, scalarLimit int) error {
	if !utf8.Valid(raw) {
		return failure("invalid_response", "JSON is not valid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	nodes := 0
	var walk func(int) error
	walk = func(depth int) error {
		if ctx.Err() != nil {
			return failure("deadline_exceeded", "JSON validation exceeded request deadline")
		}
		nodes++
		if depth > depthLimit || nodes > nodeLimit {
			return failure("resource_limit", "JSON depth or node limit exceeded")
		}
		token, err := d.Token()
		if err != nil {
			return failure("invalid_response", "invalid JSON")
		}
		if value, ok := token.(string); ok && scalarLimit > 0 && len(value) > scalarLimit {
			return failure("resource_limit", "JSON scalar exceeds cell byte limit")
		}
		delim, container := token.(json.Delim)
		if !container {
			return nil
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return failure("invalid_response", "invalid JSON object")
				}
				s, ok := key.(string)
				if scalarLimit > 0 && len(s) > scalarLimit {
					return failure("resource_limit", "JSON field name exceeds cell byte limit")
				}
				if !ok || keys[s] {
					return failure("invalid_response", "duplicate JSON field")
				}
				keys[s] = true
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
			return failure("invalid_response", "unexpected JSON delimiter")
		}
		if _, err := d.Token(); err != nil {
			return failure("invalid_response", "invalid JSON container")
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return failure("invalid_response", "multiple JSON values")
	}
	return nil
}

func decodeProof(raw []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return failure("authority_unproven", "unrecognized or malformed authority fields")
	}
	return nil
}
