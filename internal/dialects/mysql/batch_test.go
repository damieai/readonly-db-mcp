package mysql

import (
	"context"
	"regexp"
	"strings"
	"testing"

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
	mock.ExpectQuery("SELECT value FROM second_table").WillReturnRows(
		sqlmock.NewRows([]string{"value"}).AddRow(strings.Repeat("b", 160)),
	)
	mock.ExpectRollback()

	_, err = target.BatchQuery(context.Background(), core.BatchRequest{Queries: []core.QueryRequest{
		{SQL: "SELECT value FROM first_table"},
		{SQL: "SELECT value FROM second_table"},
	}})
	if err == nil || !strings.Contains(err.Error(), "batch result exceeds") {
		t.Fatalf("expected combined output limit, got %v", err)
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

	query := "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES " +
		"WHERE TABLE_SCHEMA IN (?) ORDER BY TABLE_SCHEMA, TABLE_NAME LIMIT 2000"
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("inventory").WillReturnRows(
		sqlmock.NewRows([]string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE"}).
			AddRow("inventory", "items", "BASE TABLE").
			AddRow("inventory", "secrets", "BASE TABLE").
			AddRow("inventory", "legacy", "VIEW"),
	)

	tables, err := target.ListTables(context.Background(), "")
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
