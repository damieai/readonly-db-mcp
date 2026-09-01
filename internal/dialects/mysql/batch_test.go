package mysql

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestBatchQueryCapsCombinedOutput(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)
	target.limits.MaxBatchQueries = 4
	target.limits.MaxResultBytes = 600

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT value FROM first_table").WillReturnRows(
		sqlmock.NewRows([]string{"value"}).AddRow(strings.Repeat("a", 160)),
	)
	mock.ExpectRollback()

	result, err := target.BatchQuery(context.Background(), core.BatchRequest{Queries: []core.QueryRequest{
		{SQL: "SELECT value FROM first_table"},
		{SQL: "SELECT value FROM second_table"},
	}})
	if err != nil || !result.Truncated || result.TruncationReason != "result_byte_limit" {
		t.Fatalf("expected bounded truncated output, result=%#v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListTablesHidesDeniedTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)
	target.config.DeniedTables = []string{"inventory.secrets", "legacy"}
	target.deniedTables = lowerSet(target.config.DeniedTables)

	query := "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES " +
		"WHERE TABLE_SCHEMA IN (?) ORDER BY TABLE_SCHEMA, TABLE_NAME LIMIT 2000"
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("inventory").WillReturnRows(
		sqlmock.NewRows([]string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE"}).
			AddRow("inventory", "items", "BASE TABLE").
			AddRow("inventory", "secrets", "BASE TABLE").
			AddRow("inventory", "legacy", "VIEW"),
	)

	tables, err := target.ListTables(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Name != "items" {
		t.Fatalf("unexpected visible tables: %#v", tables)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListTablesUsesWarmMetadataCache(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)
	target.metadataCache = newMetadataCache(true, 10, 4096)
	target.config.MetadataCache.TableListTTL = time.Minute
	query := "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA IN (?) ORDER BY TABLE_SCHEMA, TABLE_NAME LIMIT 2000"
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("inventory").WillReturnRows(sqlmock.NewRows([]string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE"}).AddRow("inventory", "items", "BASE TABLE"))
	for i := 0; i < 2; i++ {
		tables, err := target.ListTables(context.Background(), "", false)
		if err != nil || len(tables) != 1 {
			t.Fatalf("call %d tables=%#v err=%v", i, tables, err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListTablesFreshRefreshesAndReplacesCache(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)
	target.metadataCache = newMetadataCache(true, 10, 4096)
	target.config.MetadataCache.TableListTTL = time.Minute
	target.config.MetadataCache.FreshCooldown = time.Second
	query := "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA IN (?) ORDER BY TABLE_SCHEMA, TABLE_NAME LIMIT 2000"
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("inventory").WillReturnRows(sqlmock.NewRows([]string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE"}).AddRow("inventory", "old_items", "BASE TABLE"))
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("inventory").WillReturnRows(sqlmock.NewRows([]string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE"}).AddRow("inventory", "new_items", "BASE TABLE"))
	first, err := target.ListTables(context.Background(), "", false)
	if err != nil || first[0].Name != "old_items" {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	refreshed, err := target.ListTables(context.Background(), "", true)
	if err != nil || refreshed[0].Name != "new_items" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
	warm, err := target.ListTables(context.Background(), "", false)
	if err != nil || warm[0].Name != "new_items" {
		t.Fatalf("warm=%#v err=%v", warm, err)
	}
	if _, err := target.ListTables(context.Background(), "", true); !errors.Is(err, errRefreshCooldown) {
		t.Fatalf("expected cooldown, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
