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

type qdrantInput struct {
	Target     string          `json:"target"`
	Collection string          `json:"collection"`
	Operation  string          `json:"operation"`
	Body       json.RawMessage `json:"body,omitempty"`
	TimeoutMS  int             `json:"timeout_ms,omitempty"`
}

func (s *Server) registerQdrantTools() {
	t := tool("qdrant_read", "Read scoped Qdrant collection metadata or execute a native bounded read: query, query_batch, query_groups, scroll, retrieve, count, or facet. Native filters, prefetch and fusion remain available in body.")
	t.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"target", "collection", "operation"}, "properties": map[string]any{
		"target":     map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		"collection": map[string]any{"type": "string", "minLength": 1, "maxLength": 255},
		"operation":  map[string]any{"type": "string", "enum": []string{"metadata", "query", "query_batch", "query_groups", "scroll", "retrieve", "count", "facet"}},
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
		if len(arguments) > 8<<20 {
			return fail(errors.New("resource_limit: Qdrant tool envelope exceeds 8 MiB"))
		}
		if err := strictQdrantEnvelope(arguments); err != nil {
			return fail(err)
		}
		var input qdrantInput
		if err := json.Unmarshal(arguments, &input); err != nil {
			return fail(errors.New("invalid_request: malformed Qdrant tool arguments"))
		}
		if input.Target == "" || input.Collection == "" || input.Operation == "" {
			return fail(errors.New("invalid_request: target, collection and operation are required"))
		}
		timeout, err := durationMilliseconds(input.TimeoutMS)
		if err != nil {
			return fail(err)
		}
		target, err := s.registry.GetQdrant(input.Target)
		if err != nil {
			return fail(err)
		}
		result, err := target.QdrantRead(ctx, core.QdrantRequest{Collection: input.Collection, Operation: input.Operation, Body: input.Body, Timeout: timeout})
		if err != nil {
			return fail(err)
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return fail(errors.New("invalid_response: cannot encode Qdrant result"))
		}
		return &mcp.CallToolResult{StructuredContent: json.RawMessage(raw), Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil
	})
}

func strictQdrantEnvelope(data []byte) error {
	// Verify duplicates before the Go JSON decoder can normalize them away.
	if !utf8.Valid(data) {
		return errors.New("invalid_request: Qdrant tool arguments must be UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return errors.New("invalid_request: Qdrant tool arguments must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return errors.New("invalid_request: malformed Qdrant tool arguments")
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return errors.New("invalid_request: duplicate Qdrant tool field")
		}
		seen[name] = true
		switch name {
		case "target", "collection", "operation", "body", "timeout_ms":
		default:
			return fmt.Errorf("invalid_request: unknown Qdrant tool field %q", name)
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil || bytes.Equal(value, []byte("null")) {
			return errors.New("invalid_request: malformed or null Qdrant tool field")
		}
	}
	if end, err := d.Token(); err != nil || end != json.Delim('}') {
		return errors.New("invalid_request: malformed Qdrant tool arguments")
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("invalid_request: trailing Qdrant tool data")
	}
	return nil
}
