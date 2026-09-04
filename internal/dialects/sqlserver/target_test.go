package sqlserver

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

func sqlServerTestTarget(db *sql.DB) *Target {
	limits := config.Limits{GlobalConcurrency: 2, PerTargetConcurrency: 1, DefaultTimeout: time.Second, MaxTimeout: 2 * time.Second, MaxRows: 10, MaxResultBytes: 1 << 20, MaxCellBytes: 64 << 10, MaxSQLBytes: 32 << 10, MaxParameters: 10, MaxParameterBytes: 1 << 20, MaxParameterValueBytes: 256 << 10, MaxBatchQueries: 4}
	cfg := &config.TargetConfig{Name: "sqlserver-test", Engine: config.EngineSQLServer, Environment: "test", Consistency: config.ConsistencyCurrent, Database: "analytics", AllowedSchemas: []string{"reporting"}, Connection: config.ConnectionConfig{WriteTimeout: time.Second}, SQLServer: config.SQLServerConfig{LockTimeout: time.Second}}
	target := &Target{cfg: cfg, limits: limits, db: db, admission: admission.New(admission.Config{Global: 2, PerTarget: 1, MaxQueued: 10, QueueTimeout: time.Second, BatchMax: 1, MaintenanceMax: 1}), allowed: lowerSet(cfg.AllowedSchemas), denied: map[string]struct{}{}, cache: newMetadataCache(false, 1, 1024), policyRevision: "test", defaultSchema: "reporting", info: core.TargetInfo{Name: cfg.Name, Engine: cfg.Engine}}
	target.policy.Store(NewPolicy(cfg.Database, "reporting", cfg.AllowedSchemas, nil, limits.MaxSQLBytes))
	target.healthy.Store(true)
	return target
}

func TestQueryRequiresShowPlanBeforeExecutionAndBindsParameters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := sqlServerTestTarget(db)
	query := `SELECT id,status FROM reporting.items WHERE id=@p1`
	mock.ExpectExec(regexp.QuoteMeta("SET SHOWPLAN_XML ON")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(float64(42)).WillReturnRows(sqlmock.NewRows([]string{"Microsoft SQL Server 2005 XML Showplan"}).AddRow(selectPlan("analytics", "reporting", "items")))
	mock.ExpectExec(regexp.QuoteMeta("SET SHOWPLAN_XML OFF")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(float64(42)).WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(42, "ready"))
	mock.ExpectRollback()

	result, err := target.Query(context.Background(), core.QueryRequest{SQL: query, Parameters: []any{float64(42)}})
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

func TestQueryDoesNotExecuteWhenShowPlanIsUnsafe(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := sqlServerTestTarget(db)
	query := `SELECT id FROM reporting.items`
	unsafePlan := stringsReplaceOnce(selectPlan("analytics", "reporting", "items"), `LogicalOp="Index Scan"`, `LogicalOp="Update"`)
	mock.ExpectExec(regexp.QuoteMeta("SET SHOWPLAN_XML ON")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(unsafePlan))
	mock.ExpectExec(regexp.QuoteMeta("SET SHOWPLAN_XML OFF")).WillReturnResult(sqlmock.NewResult(0, 0))
	if _, err := target.Query(context.Background(), core.QueryRequest{SQL: query}); err == nil {
		t.Fatal("expected unsafe SHOWPLAN rejection")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func stringsReplaceOnce(value, old, replacement string) string {
	return regexp.MustCompile(regexp.QuoteMeta(old)).ReplaceAllString(value, replacement)
}

func TestTargetImplementsSQLAndBatchInterfaces(t *testing.T) {
	var _ core.SQLTarget = (*Target)(nil)
	var _ core.BatchTarget = (*Target)(nil)
}
