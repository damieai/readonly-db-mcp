package duckdbprotocol

import "github.com/your-org/readonly-db-mcp/internal/core"

// Messages travel over one stdin/stdout exchange with a sandboxed process.
// The parent never accepts a destination path from an MCP tool argument.
type Request struct {
	Operation         string  `json:"operation"`
	Database          string  `json:"database"`
	SQL               string  `json:"sql,omitempty"`
	Parameters        []any   `json:"parameters,omitempty"`
	Queries           []Query `json:"queries,omitempty"`
	Pattern           string  `json:"pattern,omitempty"`
	Schema            string  `json:"schema,omitempty"`
	Table             string  `json:"table,omitempty"`
	MaxRows           int     `json:"max_rows"`
	MaxResultBytes    int     `json:"max_result_bytes"`
	MaxCellBytes      int     `json:"max_cell_bytes"`
	MaxSQLBytes       int     `json:"max_sql_bytes"`
	MaxParameters     int     `json:"max_parameters"`
	MaxBatchQueries   int     `json:"max_batch_queries"`
	MaxParameterBytes int     `json:"max_parameter_bytes"`
	MemoryMB          int     `json:"memory_mb"`
	Threads           int     `json:"threads"`
}

type Query struct {
	SQL        string `json:"sql"`
	Parameters []any  `json:"parameters,omitempty"`
	MaxRows    int    `json:"max_rows,omitempty"`
}

type Response struct {
	Error       string                 `json:"error,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Result      *core.QueryResult      `json:"result,omitempty"`
	Batch       *core.BatchResult      `json:"batch,omitempty"`
	Tables      []core.TableSummary    `json:"tables,omitempty"`
	Description *core.TableDescription `json:"description,omitempty"`
}
