// agent-p0 exercises one grounded, cross-engine read through the public MCP tools.
// Fixture provisioning and exact target requirements are in README.md.
package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type toolCaller interface {
	CallTool(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error)
}

type options struct {
	postgres, redis, elastic, index, redisIndex string
	incidentID                                  int
}

type source struct {
	Target string   `json:"target"`
	Kind   string   `json:"kind"`
	ID     string   `json:"id"`
	Score  *float64 `json:"score,omitempty"`
	Text   string   `json:"text,omitempty"`
}

type report struct {
	Question       string            `json:"question"`
	ServerVersions map[string]string `json:"server_versions"`
	BusinessFact   map[string]any    `json:"business_fact"`
	Sources        []source          `json:"sources"`
	CancelRecovery bool              `json:"cancel_recovery"`
	WriteDenial    map[string]bool   `json:"write_denial"`
	Limits         map[string]int    `json:"limits"`
}

func main() {
	var cfg, server string
	var o options
	flag.StringVar(&cfg, "config", "", "MCP server configuration with provisioned P0 fixtures")
	flag.StringVar(&server, "server", "bin/readonly-db-mcp", "MCP server binary")
	flag.StringVar(&o.postgres, "postgres", "agent-pg", "PostgreSQL target alias")
	flag.StringVar(&o.redis, "redis", "agent-redis", "Redis target alias (RESP2)")
	flag.StringVar(&o.elastic, "elasticsearch", "agent-es", "Elasticsearch target alias")
	flag.StringVar(&o.index, "es-index", "agent-incidents", "concrete Elasticsearch fixture index")
	flag.StringVar(&o.redisIndex, "redis-index", "search-agent-incidents", "Redis Search fixture index")
	flag.IntVar(&o.incidentID, "incident-id", 101, "fixture incident ID")
	flag.Parse()
	if cfg == "" {
		fmt.Fprintln(os.Stderr, "-config is required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, server, "-config", cfg)
	cmd.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "agent-p0-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		fail(err)
	}
	defer session.Close()
	result, err := run(ctx, session, o)
	if err != nil {
		fail(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "P0 acceptance failed:", err)
	os.Exit(1)
}

func call(ctx context.Context, client toolCaller, name string, args any, out any) error {
	r, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if r == nil {
		return fmt.Errorf("%s: empty MCP result", name)
	}
	if r.IsError {
		return fmt.Errorf("%s: %v", name, r.Content)
	}
	if out == nil {
		return nil
	}
	var raw []byte
	if r.StructuredContent != nil {
		raw, err = json.Marshal(r.StructuredContent)
	} else if len(r.Content) == 1 {
		content, ok := r.Content[0].(*mcp.TextContent)
		if !ok {
			return fmt.Errorf("%s: expected text content", name)
		}
		raw = []byte(content.Text)
	} else {
		return fmt.Errorf("%s: missing structured result", name)
	}
	if err != nil {
		return fmt.Errorf("%s: encode result: %w", name, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s: decode result: %w", name, err)
	}
	return nil
}

func mustDeny(ctx context.Context, client toolCaller, name string, args any) error {
	r, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return fmt.Errorf("%s write probe transport failure: %w", name, err)
	}
	if r == nil || !r.IsError {
		return fmt.Errorf("%s accepted a write attempt", name)
	}
	return nil
}

func run(ctx context.Context, client toolCaller, o options) (*report, error) {
	if o.incidentID <= 0 || o.postgres == "" || o.redis == "" || o.elastic == "" || o.index == "" || o.redisIndex == "" {
		return nil, errors.New("all target/index names and a positive incident ID are required")
	}
	const maxRows, timeoutMS = 3, 10000
	var targets struct {
		Targets []struct {
			Name          string `json:"name"`
			Engine        string `json:"engine"`
			ServerVersion string `json:"server_version"`
			Healthy       bool   `json:"healthy"`
			ReadOnlyUser  bool   `json:"read_only_user"`
		} `json:"targets"`
	}
	if err := call(ctx, client, "list_targets", map[string]any{}, &targets); err != nil {
		return nil, err
	}
	version := map[string]string{}
	for _, expected := range []struct{ alias, engine string }{{o.postgres, "postgresql"}, {o.redis, "redis"}, {o.elastic, "elasticsearch"}} {
		found := false
		for _, target := range targets.Targets {
			if target.Name == expected.alias {
				if target.Engine != expected.engine || !target.Healthy || !target.ReadOnlyUser {
					return nil, fmt.Errorf("target %s is unhealthy or lacks an attested read-only identity", expected.alias)
				}
				if target.ServerVersion == "" {
					return nil, fmt.Errorf("target %s does not report a server version", expected.alias)
				}
				version[expected.alias] = target.ServerVersion
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("target %s is not configured", expected.alias)
		}
	}
	var tables struct {
		Tables []struct{ Schema, Name string } `json:"tables"`
	}
	if err := call(ctx, client, "schema_list_tables", map[string]any{"target": o.postgres}, &tables); err != nil {
		return nil, err
	}
	visible := false
	for _, table := range tables.Tables {
		visible = visible || table.Schema == "reporting" && table.Name == "incidents"
	}
	if !visible {
		return nil, errors.New("reporting.incidents is not visible")
	}
	var description struct {
		Columns []struct{ Name string } `json:"columns"`
	}
	if err := call(ctx, client, "schema_describe_table", map[string]any{"target": o.postgres, "schema": "reporting", "table": "incidents"}, &description); err != nil {
		return nil, err
	}
	columns := map[string]bool{}
	for _, column := range description.Columns {
		columns[column.Name] = true
	}
	for _, field := range []string{"id", "service", "summary", "severity"} {
		if !columns[field] {
			return nil, fmt.Errorf("missing business column %s", field)
		}
	}
	query := func(sql string, params []any) ([]map[string]any, error) {
		var result struct {
			Rows      []map[string]any `json:"rows"`
			Truncated bool             `json:"truncated"`
		}
		err := call(ctx, client, "query_select", map[string]any{"target": o.postgres, "sql": sql, "parameters": params, "max_rows": maxRows, "timeout_ms": timeoutMS}, &result)
		if err != nil {
			return nil, err
		}
		if result.Truncated {
			return nil, errors.New("PostgreSQL result truncated")
		}
		return result.Rows, nil
	}
	rows, err := query("SELECT id,service,summary,severity FROM reporting.incidents WHERE id=$1", []any{o.incidentID})
	if err != nil {
		return nil, fmt.Errorf("business fact: %w", err)
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("business fact: expected one row, got %d", len(rows))
	}
	fact := rows[0]
	factID, ok := fact["id"].(float64)
	if !ok || int(factID) != o.incidentID {
		return nil, errors.New("business fact ID mismatch")
	}
	vectorRows, err := query("SELECT incident_id, embedding <-> $1::vectors.vector AS distance FROM reporting.incident_embeddings ORDER BY embedding <-> $1::vectors.vector LIMIT 3", []any{"[1,0,0]"})
	if err != nil {
		return nil, fmt.Errorf("pgvector retrieval: %w", err)
	}
	if len(vectorRows) == 0 {
		return nil, errors.New("pgvector retrieval returned no rows")
	}
	report := &report{Question: "Which checkout timeout incident is relevant, and what do the business record and search sources say?", ServerVersions: version, BusinessFact: fact, WriteDenial: map[string]bool{}, Limits: map[string]int{"timeout_ms": timeoutMS, "max_rows": maxRows}}
	pgMatched := false
	for _, row := range vectorRows {
		id, ok := row["incident_id"].(float64)
		if !ok {
			return nil, errors.New("pgvector result lacks incident ID")
		}
		distance, ok := row["distance"].(float64)
		if !ok || math.IsNaN(distance) || math.IsInf(distance, 0) {
			return nil, errors.New("pgvector result lacks finite distance")
		}
		report.Sources = append(report.Sources, source{Target: o.postgres, Kind: "pgvector", ID: strconv.Itoa(int(id)), Score: &distance})
		pgMatched = pgMatched || int(id) == o.incidentID
	}
	if !pgMatched {
		return nil, errors.New("pgvector retrieval did not ground the business incident")
	}
	stringArg := func(s string) map[string]string { return map[string]string{"string": s} }
	blob := make([]byte, 12)
	binary.LittleEndian.PutUint32(blob, math.Float32bits(1))
	args := []any{stringArg(o.redisIndex), stringArg("*=>[KNN 3 @embedding $vec AS distance]"), stringArg("PARAMS"), stringArg("2"), stringArg("vec"), map[string]string{"base64": base64.StdEncoding.EncodeToString(blob)}}
	for _, s := range []string{"SORTBY", "distance", "ASC", "RETURN", "1", "distance", "LIMIT", "0", "3", "DIALECT", "2"} {
		args = append(args, stringArg(s))
	}
	var redisResult struct {
		Value     []any `json:"value"`
		Truncated bool  `json:"truncated"`
	}
	if err := call(ctx, client, "redis_command", map[string]any{"target": o.redis, "command": "FT.SEARCH", "arguments": args, "timeout_ms": timeoutMS, "max_elements": 100}, &redisResult); err != nil {
		return nil, err
	}
	if redisResult.Truncated || len(redisResult.Value) < 3 || len(redisResult.Value) > 1+2*maxRows || len(redisResult.Value)%2 != 1 {
		return nil, errors.New("Redis Search returned no complete hits")
	}
	redisMatched := false
	for i := 1; i < len(redisResult.Value); i += 2 {
		id, ok := redisResult.Value[i].(string)
		if !ok || !strings.HasPrefix(id, "tenant:agent:incident:") {
			return nil, errors.New("Redis Search hit lacks scoped source ID")
		}
		fields, ok := redisResult.Value[i+1].([]any)
		if !ok || len(fields) != 2 || fields[0] != "distance" {
			return nil, errors.New("Redis Search hit lacks distance")
		}
		score, err := strconv.ParseFloat(fmt.Sprint(fields[1]), 64)
		if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, errors.New("Redis Search hit has invalid distance")
		}
		report.Sources = append(report.Sources, source{Target: o.redis, Kind: "redis-vector", ID: id, Score: &score})
		redisMatched = redisMatched || id == "tenant:agent:incident:"+strconv.Itoa(o.incidentID)
	}
	if !redisMatched {
		return nil, errors.New("Redis retrieval did not ground the business incident")
	}
	var esResult struct {
		Data      json.RawMessage `json:"data"`
		Partial   bool            `json:"partial"`
		Truncated bool            `json:"truncated"`
	}
	esBody := map[string]any{"size": 3, "query": map[string]any{"match": map[string]any{"summary": "checkout timeout"}}, "_source": []string{"incident_id", "service", "summary"}}
	if err := call(ctx, client, "es_query", map[string]any{"target": o.elastic, "operation": "search", "indices": []string{o.index}, "body": esBody, "max_rows": maxRows, "timeout_ms": timeoutMS}, &esResult); err != nil {
		return nil, err
	}
	if esResult.Partial || esResult.Truncated {
		return nil, errors.New("Elasticsearch result is incomplete")
	}
	var esData struct {
		Hits struct {
			Hits []struct {
				Index  string `json:"_index"`
				ID     string `json:"_id"`
				Source struct {
					IncidentID int    `json:"incident_id"`
					Summary    string `json:"summary"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(esResult.Data, &esData); err != nil {
		return nil, fmt.Errorf("Elasticsearch result: %w", err)
	}
	if len(esData.Hits.Hits) == 0 || len(esData.Hits.Hits) > maxRows {
		return nil, errors.New("Elasticsearch lacks bounded hits")
	}
	esMatched := false
	for _, hit := range esData.Hits.Hits {
		if hit.Index != o.index || hit.ID == "" || hit.Source.IncidentID == 0 || hit.Source.Summary == "" {
			return nil, errors.New("Elasticsearch hit lacks scoped source ID")
		}
		report.Sources = append(report.Sources, source{Target: o.elastic, Kind: "elasticsearch-text", ID: hit.Index + "/" + hit.ID, Text: hit.Source.Summary})
		esMatched = esMatched || hit.Source.IncidentID == o.incidentID
	}
	if !esMatched {
		return nil, errors.New("Elasticsearch retrieval did not ground the business incident")
	}
	// EXPLAIN proves the cancellation probe is a valid read before we run it.
	// A one-connection PostgreSQL fixture then makes the follow-up query a
	// practical check that canceled server work released its connection.
	const slowRead = "SELECT COUNT(*) FROM reporting.incidents CROSS JOIN pg_catalog.generate_series(1,1000000000) AS g"
	if err := call(ctx, client, "query_explain", map[string]any{"target": o.postgres, "sql": slowRead, "timeout_ms": timeoutMS}, nil); err != nil {
		return nil, fmt.Errorf("cancellation preflight: %w", err)
	}
	cancelCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	canceled, cancelErr := client.CallTool(cancelCtx, &mcp.CallToolParams{Name: "query_select", Arguments: map[string]any{"target": o.postgres, "sql": slowRead, "timeout_ms": timeoutMS}})
	if !errors.Is(cancelCtx.Err(), context.DeadlineExceeded) || (cancelErr == nil && (canceled == nil || !canceled.IsError)) {
		return nil, fmt.Errorf("slow read did not stop on client cancellation: %v", cancelErr)
	}
	if recovered, err := query("SELECT id FROM reporting.incidents WHERE id=$1", []any{o.incidentID}); err != nil || len(recovered) != 1 {
		return nil, fmt.Errorf("PostgreSQL did not recover after cancellation: %v", err)
	}
	report.CancelRecovery = true
	for _, probe := range []struct {
		key, tool string
		args      any
	}{
		{"postgresql", "query_select", map[string]any{"target": o.postgres, "sql": "DELETE FROM reporting.incidents WHERE id=$1", "parameters": []any{o.incidentID}}},
		{"redis", "redis_command", map[string]any{"target": o.redis, "command": "DEL", "arguments": []any{stringArg("tenant:agent:incident:" + strconv.Itoa(o.incidentID))}}},
		{"elasticsearch", "es_query", map[string]any{"target": o.elastic, "operation": "index", "indices": []string{o.index}, "id": strconv.Itoa(o.incidentID), "body": map[string]any{"unsafe": true}}},
	} {
		if err := mustDeny(ctx, client, probe.tool, probe.args); err != nil {
			return nil, err
		}
		report.WriteDenial[probe.key] = true
	}
	return report, nil
}
