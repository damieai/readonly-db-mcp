package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/your-org/readonly-db-mcp/internal/core"
	es "github.com/your-org/readonly-db-mcp/internal/dialects/elasticsearch"
)

type ElasticsearchMetadataInput struct {
	Target    string   `json:"target" jsonschema:"Exact Elasticsearch target alias returned by list_targets"`
	Operation string   `json:"operation" jsonschema:"Metadata operation: resolve or mappings"`
	Indices   []string `json:"indices,omitempty" jsonschema:"Scoped index, alias or data-stream expressions; omitted uses configured scope"`
	TimeoutMS int      `json:"timeout_ms,omitempty" jsonschema:"Optional timeout in milliseconds, including admission and source resolution"`
}

func (v *ElasticsearchMetadataInput) UnmarshalJSON(data []byte) error {
	if len(data) > 16<<20 {
		return fmt.Errorf("Elasticsearch metadata envelope exceeds hard byte limit")
	}
	if err := es.ValidateJSON(data, 128, 100000); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return fmt.Errorf("invalid_request: metadata envelope must be an object")
	}
	for name, value := range fields {
		switch name {
		case "target", "operation", "indices", "timeout_ms":
		default:
			return fmt.Errorf("invalid_request: unknown metadata envelope field")
		}
		if bytes.Equal(value, []byte("null")) {
			return fmt.Errorf("invalid_request: metadata envelope fields cannot be null")
		}
	}
	type plain ElasticsearchMetadataInput
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode((*plain)(v)); err != nil {
		return fmt.Errorf("invalid_request: malformed metadata envelope")
	}
	if v.Target == "" || len(v.Target) > 64 || v.Operation == "" || len(v.Operation) > 64 || len(v.Indices) > 10000 {
		return fmt.Errorf("invalid_request: target and operation are required and envelope limits apply")
	}
	return nil
}

func (s *Server) registerElasticsearchTools() {
	t := tool("es_metadata", "Resolve scoped Elasticsearch indices, aliases and data streams, or read their mappings. Query capabilities are reported by inspect_target.")
	t.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"target", "operation"}, "properties": map[string]any{
		"target":     map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		"operation":  map[string]any{"type": "string", "enum": []string{"resolve", "mappings"}},
		"indices":    map[string]any{"type": "array", "maxItems": 10000, "items": map[string]any{"type": "string", "maxLength": 255}},
		"timeout_ms": map[string]any{"type": "integer", "minimum": 0, "maximum": 900000},
	}}
	t.OutputSchema = map[string]any{"type": "object", "properties": map[string]any{"data": map[string]any{"type": "object"}}}
	// The generic SDK binder normalizes through map[string]any before invoking
	// custom unmarshaling. Use raw arguments/results so duplicate keys cannot
	// disappear and large native JSON numbers cannot pass through float64.
	s.mcp.AddTool(t, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fail := func(err error) (*mcp.CallToolResult, error) {
			var r mcp.CallToolResult
			r.SetError(err)
			return &r, nil
		}
		var input ElasticsearchMetadataInput
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return fail(err)
		}
		_, output, err := s.elasticsearchMetadata(ctx, req, input)
		if err != nil {
			return fail(err)
		}
		raw, err := json.Marshal(output)
		if err != nil {
			return fail(fmt.Errorf("invalid_response: cannot encode metadata result"))
		}
		return &mcp.CallToolResult{StructuredContent: json.RawMessage(raw), Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil
	})
}

func (s *Server) elasticsearchMetadata(ctx context.Context, _ *mcp.CallToolRequest, input ElasticsearchMetadataInput) (*mcp.CallToolResult, core.ElasticsearchResult, error) {
	timeout, err := durationMilliseconds(input.TimeoutMS)
	if err != nil {
		return nil, core.ElasticsearchResult{}, err
	}
	target, err := s.registry.GetElasticsearch(input.Target)
	if err != nil {
		return nil, core.ElasticsearchResult{}, err
	}
	result, err := target.ElasticsearchMetadata(ctx, core.ElasticsearchMetadataRequest{Operation: input.Operation, Indices: input.Indices, Timeout: timeout})
	if err != nil {
		return nil, core.ElasticsearchResult{}, err
	}
	return nil, *result, nil
}
