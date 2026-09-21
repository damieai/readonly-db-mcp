package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestPrivilegeRecheckFailsClosedWhileReadIsRunning(t *testing.T) {
	started := make(chan struct{})
	var once sync.Once
	const query = `SELECT id FROM reporting.items`
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		err := sqlmock.QueryMatcherRegexp.Match(expected, actual)
		if err == nil && actual == query {
			once.Do(func() { close(started) })
		}
		return err
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)
	target := postgresTestTarget(db)
	target.admission = admission.New(admission.Config{Global: 2, PerTarget: 2, MaxQueued: 10, QueueTimeout: time.Second, BatchMax: 1, MaintenanceMax: 1})
	target.cfg.Connection.ConnectTimeout = 500 * time.Millisecond
	target.cfg.PostgreSQL.PrivilegeRecheck = 50 * time.Millisecond
	target.lastAttested.Store(time.Now().UnixNano())
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_catalog.set_config('statement_timeout',$1,true)`)).WithArgs("900").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillDelayFor(750 * time.Millisecond).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectRollback()
	mock.ExpectQuery(`SELECT current_user, session_user`).WillReturnError(errors.New("identity check failed"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := target.Query(ctx, core.QueryRequest{SQL: query}); done <- err }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("read did not start")
	}
	target.startPrivilegeRecheck()
	defer func() { target.maintenanceStop(); target.maintenanceWG.Wait() }()
	deadline := time.After(500 * time.Millisecond)
	for target.healthy.Load() {
		select {
		case <-deadline:
			t.Fatal("recheck waited for running read")
		case <-time.After(time.Millisecond):
		}
	}
	target.maintenanceStop()
	target.maintenanceWG.Wait()
	select {
	case <-done:
		t.Fatal("read completed before failed recheck was published")
	default:
	}
	if _, err := target.Query(ctx, core.QueryRequest{SQL: query}); err == nil {
		t.Fatal("new query admitted after failed recheck")
	}
	if err := <-done; err != nil {
		t.Fatal("existing read interrupted by recheck", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

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
