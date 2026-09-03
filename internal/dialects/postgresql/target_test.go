package postgresql

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func postgresTestTarget(db *sql.DB) *Target {
	limits := config.Limits{GlobalConcurrency: 2, PerTargetConcurrency: 1, DefaultTimeout: time.Second, MaxTimeout: 2 * time.Second, MaxRows: 10, MaxResultBytes: 1 << 20, MaxCellBytes: 64 << 10, MaxSQLBytes: 32 << 10, MaxParameters: 10, MaxBatchQueries: 4}
	cfg := &config.TargetConfig{Name: "pg-test", Engine: config.EnginePostgreSQL, Environment: "test", Consistency: config.ConsistencyCurrent, Database: "analytics", AllowedSchemas: []string{"reporting"}, PostgreSQL: config.PostgreSQLConfig{StatementTimeoutMargin: 100 * time.Millisecond}}
	target := &Target{cfg: cfg, limits: limits, db: db, admission: admission.New(admission.Config{Global: 2, PerTarget: 1, MaxQueued: 10, QueueTimeout: time.Second, BatchMax: 1, MaintenanceMax: 1}), allowed: lowerSet(cfg.AllowedSchemas), denied: map[string]struct{}{}, cache: newMetadataCache(false, 1, 1024), policyRevision: "test", info: core.TargetInfo{Name: cfg.Name, Engine: cfg.Engine}}
	target.policy.Store(NewPolicy(cfg.AllowedSchemas, nil, limits.MaxSQLBytes))
	target.healthy.Store(true)
	return target
}

func TestQueryUsesPostgreSQLReadOnlyTransactionAndParameters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := postgresTestTarget(db)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_catalog.set_config('statement_timeout',$1,true)`)).WithArgs("900").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id,status FROM reporting.items WHERE id=$1`)).WithArgs(float64(42)).WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(42, "ready"))
	mock.ExpectRollback()
	result, err := target.Query(context.Background(), core.QueryRequest{SQL: `SELECT id,status FROM reporting.items WHERE id=$1`, Parameters: []any{float64(42)}})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 || result.Rows[0]["status"] != "ready" {
		t.Fatalf("result=%#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataCacheClones(t *testing.T) {
	c := newMetadataCache(true, 2, 4096)
	leader, err := c.lead(context.Background(), "k", false, time.Second)
	if err != nil || !leader {
		t.Fatal("leader")
	}
	c.finish("k", []core.TableSummary{{Name: "items"}}, time.Minute, true)
	v, _ := c.get("k")
	v.([]core.TableSummary)[0].Name = "changed"
	again, _ := c.get("k")
	if again.([]core.TableSummary)[0].Name != "items" {
		t.Fatal("cache mutated")
	}
}

func TestValidateParametersEnforcesByteBudgets(t *testing.T) {
	if err := validateParameters([]any{"12345"}, 16, 4); err == nil {
		t.Fatal("expected per-value byte limit rejection")
	}
	if err := validateParameters([]any{"1234", "5678"}, 7, 4); err == nil {
		t.Fatal("expected total byte limit rejection")
	}
}

func TestMetadataRequiresHealthyTarget(t *testing.T) {
	target := postgresTestTarget(nil)
	target.healthy.Store(false)
	if _, err := target.ListTables(context.Background(), "", false); err == nil {
		t.Fatal("expected unhealthy metadata request rejection")
	}
}
