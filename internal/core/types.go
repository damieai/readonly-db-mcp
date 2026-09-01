package core

import (
	"context"
	"time"
)

type TargetInfo struct {
	Name           string   `json:"name"`
	Engine         string   `json:"engine"`
	Environment    string   `json:"environment"`
	Consistency    string   `json:"consistency"`
	Database       string   `json:"database"`
	Schemas        []string `json:"allowed_schemas"`
	Healthy        bool     `json:"healthy"`
	ReadOnlyUser   bool     `json:"read_only_user"`
	ServerReadOnly bool     `json:"server_read_only"`
}

type QueryRequest struct {
	SQL        string
	Parameters []any
	Timeout    time.Duration
	MaxRows    int
	Purpose    string
}

type QueryResult struct {
	QueryID        string           `json:"query_id"`
	Target         string           `json:"target"`
	Engine         string           `json:"engine"`
	Environment    string           `json:"environment"`
	Consistency    string           `json:"consistency"`
	Database       string           `json:"database"`
	Columns        []Column         `json:"columns"`
	Rows           []map[string]any `json:"rows"`
	RowCount       int              `json:"row_count"`
	Truncated      bool             `json:"truncated"`
	TruncatedCells int              `json:"truncated_cells"`
	DurationMS     int64            `json:"duration_ms"`
	CacheStatus    string           `json:"cache_status,omitempty"`
	CacheAgeMS     int64            `json:"cache_age_ms,omitempty"`
}

type Column struct {
	Name         string `json:"name"`
	DatabaseType string `json:"database_type,omitempty"`
	Nullable     bool   `json:"nullable,omitempty"`
}

type TableSummary struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

type ColumnDescription struct {
	Name       string  `json:"name"`
	DataType   string  `json:"data_type"`
	ColumnType string  `json:"column_type"`
	Nullable   bool    `json:"nullable"`
	Default    *string `json:"default,omitempty"`
	Key        string  `json:"key,omitempty"`
	Extra      string  `json:"extra,omitempty"`
	Comment    string  `json:"comment,omitempty"`
}

type IndexDescription struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"`
}

type TableDescription struct {
	Target  string              `json:"target"`
	Schema  string              `json:"schema"`
	Table   string              `json:"table"`
	Columns []ColumnDescription `json:"columns"`
	Indexes []IndexDescription  `json:"indexes"`
}

type Validation struct {
	Fingerprint string
	Tables      []string
	Cacheable   bool
}

type Target interface {
	Info() TargetInfo
	ValidateQuery(sql string) (*Validation, error)
	Query(context.Context, QueryRequest) (*QueryResult, error)
	Explain(context.Context, QueryRequest) (*QueryResult, error)
	ListTables(context.Context, string, bool) ([]TableSummary, error)
	DescribeTable(context.Context, string, string, bool) (*TableDescription, error)
	Close() error
}
