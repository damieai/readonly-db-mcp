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

type ElasticsearchQueryInput struct {
	Target string `json:"target"`
	core.ElasticsearchQueryRequest
	TimeoutMS int    `json:"timeout_ms,omitempty"`
	Purpose   string `json:"purpose,omitempty"`
}

type ElasticsearchBatchInput struct {
	Target      string                           `json:"target"`
	Requests    []core.ElasticsearchQueryRequest `json:"requests"`
	Consistency string                           `json:"consistency,omitempty"`
	TimeoutMS   int                              `json:"timeout_ms,omitempty"`
}

func esEnvelope(data []byte, allowed string, dst any) error {
	if len(data) > 16<<20 {
		return fmt.Errorf("resource_limit: Elasticsearch envelope exceeds hard byte limit")
	}
	if err := es.ValidateJSON(data, 512, 1000000); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return fmt.Errorf("invalid_request: Elasticsearch envelope must be an object")
	}
	var names []string
	if json.Unmarshal([]byte(allowed), &names) != nil {
		panic("invalid internal envelope definition")
	}
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	for name, value := range fields {
		if !set[name] {
			return fmt.Errorf("invalid_request: unknown Elasticsearch envelope field")
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("invalid_request: envelope fields cannot be null")
		}
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return fmt.Errorf("invalid_request: malformed Elasticsearch envelope")
	}
	return nil
}

func (v *ElasticsearchQueryInput) UnmarshalJSON(data []byte) error {
	type plain ElasticsearchQueryInput
	if err := esEnvelope(data, `["target","operation","indices","id","body","options","max_rows","timeout_ms","purpose"]`, (*plain)(v)); err != nil {
		return err
	}
	if v.Target == "" || len(v.Target) > 64 || v.Operation == "" || len(v.Operation) > 64 || len(v.Indices) > 10000 || len(v.Purpose) > 1024 {
		return fmt.Errorf("invalid_request: query envelope exceeds limits or lacks target/operation")
	}
	return nil
}

func (v *ElasticsearchBatchInput) UnmarshalJSON(data []byte) error {
	var raw struct {
		Target      string            `json:"target"`
		Requests    []json.RawMessage `json:"requests"`
		Consistency string            `json:"consistency,omitempty"`
		TimeoutMS   int               `json:"timeout_ms,omitempty"`
	}
	if err := esEnvelope(data, `["target","requests","consistency","timeout_ms"]`, &raw); err != nil {
		return err
	}
	if raw.Target == "" || len(raw.Target) > 64 || len(raw.Requests) == 0 || len(raw.Requests) > 100 {
		return fmt.Errorf("resource_limit: invalid target or batch size")
	}
	*v = ElasticsearchBatchInput{Target: raw.Target, Consistency: raw.Consistency, TimeoutMS: raw.TimeoutMS}
	for _, data := range raw.Requests {
		var member core.ElasticsearchQueryRequest
		if err := esEnvelope(data, `["operation","indices","id","body","options","max_rows"]`, &member); err != nil {
			return err
		}
		if member.Operation == "" || len(member.Operation) > 64 || len(member.Indices) > 10000 {
			return fmt.Errorf("invalid_request: invalid batch member")
		}
		v.Requests = append(v.Requests, member)
	}
	return nil
}

func (s *Server) registerElasticsearchQueryTools() {
	properties := func() map[string]any {
		return map[string]any{
			"operation": map[string]any{"type": "string", "minLength": 1, "maxLength": 64, "description": "search, count, get, mget, termvectors, mtermvectors, explain, field_caps, search_shards, indices.validate_query, search_template, render_search_template eql.search, sql.query or sql.translate"},
			"indices":   map[string]any{"type": "array", "maxItems": 10000, "items": map[string]any{"type": "string", "maxLength": 255}},
			"id":        map[string]any{"type": "string", "maxLength": 1024},
			"body":      map[string]any{"type": "object", "additionalProperties": true, "description": "Native Elasticsearch JSON; advanced DSL, aggregations, scripts and supplied vectors and native language queries/parameters are preserved"},
			"options":   map[string]any{"type": "object", "additionalProperties": true, "description": "Native options from the pinned REST API; no raw URL, headers or paths"},
			"max_rows":  map[string]any{"type": "integer", "minimum": 0, "maximum": 10000},
		}
	}
	for _, name := range []string{"es_query", "es_batch"} {
		name := name
		p := properties()
		required := []string{"target", "operation"}
		description := "Execute a scoped native Elasticsearch read. Supports advanced DSL, scripts, aggregations, supplied-vector, local hybrid retrieval and native synchronous EQL/SQL; inspect_target reports pending capabilities."
		if name == "es_batch" {
			p = map[string]any{"requests": map[string]any{"type": "array", "minItems": 1, "maxItems": 100, "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"operation"}, "properties": properties()}}, "consistency": map[string]any{"type": "string", "enum": []string{"independent", "pit"}}}
			required = []string{"target", "requests"}
			description = "Execute Elasticsearch reads under one deadline and combined result budget. Independent reads have no shared snapshot; pit requires compatible searches and uses one service-owned PIT, closed before returning."
		} else {
			p["purpose"] = map[string]any{"type": "string", "maxLength": 1024}
		}
		p["target"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 64}
		p["timeout_ms"] = map[string]any{"type": "integer", "minimum": 0, "maximum": 900000}
		t := tool(name, description)
		t.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": p}
		t.OutputSchema = map[string]any{"type": "object"}
		s.mcp.AddTool(t, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			fail := func(err error) (*mcp.CallToolResult, error) {
				var result mcp.CallToolResult
				result.SetError(err)
				return &result, nil
			}
			var output any
			if name == "es_query" {
				var input ElasticsearchQueryInput
				if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
					return fail(err)
				}
				timeout, err := durationMilliseconds(input.TimeoutMS)
				if err != nil {
					return fail(err)
				}
				target, err := s.registry.GetElasticsearch(input.Target)
				if err != nil {
					return fail(err)
				}
				input.Timeout = timeout
				output, err = target.ElasticsearchQuery(ctx, input.ElasticsearchQueryRequest)
				if err != nil {
					return fail(err)
				}
			} else {
				var input ElasticsearchBatchInput
				if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
					return fail(err)
				}
				timeout, err := durationMilliseconds(input.TimeoutMS)
				if err != nil {
					return fail(err)
				}
				target, err := s.registry.GetElasticsearch(input.Target)
				if err != nil {
					return fail(err)
				}
				output, err = target.ElasticsearchBatch(ctx, core.ElasticsearchBatchRequest{Requests: input.Requests, Consistency: input.Consistency, Timeout: timeout})
				if err != nil {
					return fail(err)
				}
			}
			raw, err := json.Marshal(output)
			if err != nil {
				return fail(fmt.Errorf("invalid_response: cannot encode Elasticsearch result"))
			}
			return &mcp.CallToolResult{StructuredContent: json.RawMessage(raw), Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil
		})
	}
}
