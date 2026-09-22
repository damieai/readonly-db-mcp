package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type elasticsearchSession struct {
	owner   string
	ctx     context.Context
	cancel  context.CancelFunc
	targets map[string]core.ElasticsearchTarget
}

// ServerSession.ID is empty on stdio. Bind ownership to the actual transport
// session, assigning an internal nonce that cannot be supplied in tool arguments.
func (s *Server) elasticsearchOwner(session *mcp.ServerSession, name string, target core.ElasticsearchTarget) (*elasticsearchSession, error) {
	if session == nil {
		return nil, fmt.Errorf("invalid_request: transport session is required")
	}
	s.esSessionsMu.Lock()
	defer s.esSessionsMu.Unlock()
	if s.esSessions == nil {
		s.esSessions = make(map[*mcp.ServerSession]*elasticsearchSession)
	}
	if owner := s.esSessions[session]; owner != nil {
		owner.targets[name] = target
		return owner, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	owner := &elasticsearchSession{owner: uuid.NewString(), ctx: ctx, cancel: cancel, targets: map[string]core.ElasticsearchTarget{name: target}}
	s.esSessions[session] = owner
	go func() {
		_ = session.Wait()
		cancel()
		s.esSessionsMu.Lock()
		delete(s.esSessions, session)
		targets := make([]core.ElasticsearchTarget, 0, len(owner.targets))
		for _, t := range owner.targets {
			targets = append(targets, t)
		}
		s.esSessionsMu.Unlock()
		for _, target := range targets {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = target.CloseElasticsearchSession(ctx, owner.owner)
			cancel()
		}
	}()
	return owner, nil
}

type ElasticsearchCursorInput struct {
	Target string `json:"target"`
	core.ElasticsearchCursorRequest
	KeepAliveMS int `json:"keep_alive_ms,omitempty"`
	TimeoutMS   int `json:"timeout_ms,omitempty"`
}

func (v *ElasticsearchCursorInput) UnmarshalJSON(data []byte) error {
	type plain ElasticsearchCursorInput
	if err := esEnvelope(data, `["target","action","kind","handle","indices","body","options","max_rows","keep_alive_ms","timeout_ms"]`, (*plain)(v)); err != nil {
		return err
	}
	if v.Target == "" || len(v.Target) > 64 || len(v.Action) > 16 || len(v.Kind) > 16 || len(v.Handle) > 43 || len(v.Indices) > 10000 {
		return fmt.Errorf("invalid_request: invalid cursor envelope")
	}
	return nil
}

func (s *Server) registerElasticsearchCursorTool() {
	t := tool("es_cursor", "Open, advance or close a session-owned PIT, scroll or native SQL cursor. PIT next without a body uses the last hit's search_after; an explicit native body supports other continuation strategies. Handles expire; native server IDs are never accepted.")
	t.Annotations.IdempotentHint = false
	t.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"target", "action"}, "properties": map[string]any{
		"target":        map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		"action":        map[string]any{"type": "string", "enum": []string{"open", "next", "close"}},
		"kind":          map[string]any{"type": "string", "enum": []string{"pit", "scroll", "sql"}},
		"handle":        map[string]any{"type": "string", "minLength": 43, "maxLength": 43},
		"indices":       map[string]any{"type": "array", "maxItems": 10000, "items": map[string]any{"type": "string", "maxLength": 255}},
		"body":          map[string]any{"type": "object", "additionalProperties": true},
		"options":       map[string]any{"type": "object", "additionalProperties": true},
		"max_rows":      map[string]any{"type": "integer", "minimum": 0, "maximum": 10000},
		"keep_alive_ms": map[string]any{"type": "integer", "minimum": 0, "maximum": 900000},
		"timeout_ms":    map[string]any{"type": "integer", "minimum": 0, "maximum": 900000},
	}}
	t.OutputSchema = map[string]any{"type": "object"}
	s.mcp.AddTool(t, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fail := func(err error) (*mcp.CallToolResult, error) {
			var result mcp.CallToolResult
			result.SetError(err)
			return &result, nil
		}
		var input ElasticsearchCursorInput
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return fail(err)
		}
		timeout, err := durationMilliseconds(input.TimeoutMS)
		if err != nil {
			return fail(err)
		}
		keep, err := durationMilliseconds(input.KeepAliveMS)
		if err != nil {
			return fail(err)
		}
		target, err := s.registry.GetElasticsearch(input.Target)
		if err != nil {
			return fail(err)
		}
		owner, err := s.elasticsearchOwner(req.Session, input.Target, target)
		if err != nil {
			return fail(err)
		}
		input.Timeout = timeout
		input.KeepAlive = keep
		input.Owner = owner.owner
		input.OwnerContext = owner.ctx
		output, err := target.ElasticsearchCursor(ctx, input.ElasticsearchCursorRequest)
		if err != nil {
			return fail(err)
		}
		raw, err := json.Marshal(output)
		if err != nil {
			return fail(fmt.Errorf("invalid_response: cannot encode cursor result"))
		}
		return &mcp.CallToolResult{StructuredContent: json.RawMessage(raw), Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil
	})
}
