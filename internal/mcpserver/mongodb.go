package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type mongoInput struct {
	Target     string          `json:"target"`
	Collection string          `json:"collection"`
	Operation  string          `json:"operation"`
	Body       json.RawMessage `json:"body,omitempty"`
	TimeoutMS  int             `json:"timeout_ms,omitempty"`
}

func (s *Server) registerMongoDBTools() {
	t := tool("mongodb_read", "Read a scoped MongoDB collection: metadata, find, count or a reviewed native aggregation pipeline. Extended JSON preserves BSON values.")
	t.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"target", "collection", "operation"}, "properties": map[string]any{
		"target":     map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		"collection": map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
		"operation":  map[string]any{"type": "string", "enum": []string{"metadata", "find", "count", "aggregate"}},
		"body":       map[string]any{"type": "object", "additionalProperties": true},
		"timeout_ms": map[string]any{"type": "integer", "minimum": 0, "maximum": 900000},
	}}
	t.OutputSchema = map[string]any{"type": "object", "properties": map[string]any{"data": map[string]any{"type": "object"}}}
	s.mcp.AddTool(t, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fail := func(err error) (*mcp.CallToolResult, error) {
			var out mcp.CallToolResult
			out.SetError(err)
			return &out, nil
		}
		arguments := req.Params.Arguments
		if len(arguments) > 9<<20 {
			return fail(errors.New("resource_limit: MongoDB tool envelope exceeds 9 MiB"))
		}
		if err := strictMongoEnvelope(arguments); err != nil {
			return fail(err)
		}
		var input mongoInput
		if json.Unmarshal(arguments, &input) != nil || input.Target == "" || input.Collection == "" || input.Operation == "" {
			return fail(errors.New("invalid_request: MongoDB target, collection and operation are required"))
		}
		timeout, err := durationMilliseconds(input.TimeoutMS)
		if err != nil {
			return fail(err)
		}
		target, err := s.registry.GetMongoDB(input.Target)
		if err != nil {
			return fail(err)
		}
		result, err := target.MongoDBRead(ctx, core.MongoDBRequest{Collection: input.Collection, Operation: input.Operation, Body: input.Body, Timeout: timeout})
		if err != nil {
			return fail(err)
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return fail(errors.New("invalid_response: cannot encode MongoDB result"))
		}
		return &mcp.CallToolResult{StructuredContent: json.RawMessage(raw), Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil
	})
}

func strictMongoEnvelope(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("invalid_request: MongoDB tool envelope must be UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return errors.New("invalid_request: MongoDB tool envelope must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return errors.New("invalid_request: malformed MongoDB tool envelope")
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return errors.New("invalid_request: duplicate MongoDB tool field")
		}
		seen[name] = true
		switch name {
		case "target", "collection", "operation", "body", "timeout_ms":
		default:
			return fmt.Errorf("invalid_request: unknown MongoDB tool field %q", name)
		}
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errors.New("invalid_request: malformed or null MongoDB tool field")
		}
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return errors.New("invalid_request: malformed MongoDB tool envelope")
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("invalid_request: trailing MongoDB tool data")
	}
	return nil
}
