package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestQueryUsesReadOnlyTransactionAndRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	target := testTarget(db)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, status FROM items WHERE id = \\?").
		WithArgs(float64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(42, "ready"))
	mock.ExpectRollback()

	result, err := target.Query(context.Background(), core.QueryRequest{
		SQL:        "SELECT id, status FROM items WHERE id = ?",
		Parameters: []any{float64(42)},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if result.RowCount != 1 || result.Rows[0]["status"] != "ready" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueryRejectsMutationBeforeDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)

	if _, err := target.Query(context.Background(), core.QueryRequest{SQL: "DELETE FROM items"}); err == nil {
		t.Fatal("expected mutation rejection")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database should not have been called: %v", err)
	}
}

func TestQueryCapsRowsAndMarksTruncated(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM items").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2),
	)
	mock.ExpectRollback()
	result, err := target.Query(context.Background(), core.QueryRequest{SQL: "SELECT id FROM items", MaxRows: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 || !result.Truncated {
		t.Fatalf("expected one truncated row, got %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueryUsesResultCacheAfterValidatedMiss(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)
	target.config.ResultCache.Enabled = true
	target.resultCache = newResultCache(true, time.Minute, 10, 1<<20, 1<<20)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM items").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectRollback()
	first, err := target.Query(context.Background(), core.QueryRequest{SQL: "SELECT id FROM items"})
	if err != nil || first.CacheStatus != "miss" {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	second, err := target.Query(context.Background(), core.QueryRequest{SQL: "SELECT id FROM items"})
	if err != nil || second.CacheStatus != "hit" {
		t.Fatalf("second=%#v err=%v", second, err)
	}
	if first.QueryID == second.QueryID {
		t.Fatal("cache hit reused query id")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeValuePreservesLargeIntegerPrecision(t *testing.T) {
	value, truncated := normalizeValue(int64(1<<53), 1024)
	if truncated || value != "9007199254740992" {
		t.Fatalf("unexpected normalized integer: %#v, truncated=%v", value, truncated)
	}
}

func FuzzPolicyDoesNotPanic(f *testing.F) {
	policy := NewPolicy("inventory", []string{"inventory"}, nil, 32<<10)
	for _, seed := range []string{
		"SELECT 1",
		"WITH x AS (SELECT 1) SELECT * FROM x",
		"SELECT GET_LOCK('x', 1)",
		"SELECT 1; DROP TABLE x",
		"/*!50000 SELECT 1 */",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, query string) {
		_, _ = policy.Validate(query)
	})
}

func testTarget(db *sql.DB) *Target {
	limits := config.Limits{
		GlobalConcurrency:    2,
		PerTargetConcurrency: 1,
		DefaultTimeout:       time.Second,
		MaxTimeout:           2 * time.Second,
		MaxRows:              10,
		MaxResultBytes:       1 << 20,
		MaxCellBytes:         64 << 10,
		MaxSQLBytes:          32 << 10,
		MaxParameters:        10,
	}
	cfg := &config.TargetConfig{
		Name:           "test",
		Engine:         config.EngineMySQL,
		Environment:    "test",
		Consistency:    config.ConsistencyCurrent,
		Database:       "inventory",
		AllowedSchemas: []string{"inventory"},
	}
	return &Target{
		config:               cfg,
		limits:               limits,
		db:                   db,
		policy:               NewPolicy(cfg.Database, cfg.AllowedSchemas, nil, limits.MaxSQLBytes),
		admission:            admission.New(admission.Config{Global: 2, PerTarget: 1, MaxQueued: 10, QueueTimeout: time.Second, BatchMax: 1, MaintenanceMax: 1}),
		allowedSchemas:       lowerSet(cfg.AllowedSchemas),
		deniedTables:         lowerSet(cfg.DeniedTables),
		metadataPlaceholders: "?",
		metadataCache:        newMetadataCache(false, 1, 1024),
		resultCache:          newResultCache(false, 0, 0, 0, 0),
		policyRevision:       targetPolicyRevision(cfg),
		info: core.TargetInfo{
			Name: cfg.Name, Engine: cfg.Engine, Environment: cfg.Environment,
			Consistency: cfg.Consistency, Database: cfg.Database,
		},
	}
}
