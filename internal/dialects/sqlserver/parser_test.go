package sqlserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestScriptDomRealParserCorpus(t *testing.T) {
	path := os.Getenv("READONLY_DB_MCP_SQLSERVER_PARSER")
	if path == "" {
		t.Skip("build the helper and set READONLY_DB_MCP_SQLSERVER_PARSER")
	}
	parser := scriptDOM{path: path}
	for _, compatibility := range []int{150, 160, 170} {
		for _, test := range []struct {
			query           string
			module, allowed bool
		}{
			{`SELECT 1`, false, true},
			{`WITH c AS (SELECT id FROM reporting.items) SELECT id,ROW_NUMBER() OVER(ORDER BY id) FROM c OPTION(MAXRECURSION 1000)`, false, true},
			{`SELECT TOP (@p1) a.id,j.value FROM reporting.items a CROSS APPLY OPENJSON(a.data) j WHERE a.id=@p2 OPTION(RECOMPILE)`, false, true},
			{`SELECT JSON_MODIFY(N'{"x":1}','$.x',2),NEWID(),GETDATE()`, false, true},
			{`SELECT reporting.f(@p1) FROM reporting.items WITH(UPDLOCK)`, false, true},
			{`SELECT * FROM reporting.items FOR SYSTEM_TIME ALL`, false, true},
			{`SELECT 1; SELECT 2`, false, false},
			{`SELECT * INTO reporting.copy FROM reporting.items`, false, false},
			{`SELECT NEXT VALUE FOR reporting.sequence`, false, false},
			{`SELECT @p1=2`, false, false},
			{`EXEC reporting.proc`, false, false},
			{`SELECT * FROM OPENROWSET('SQLNCLI','server=x','SELECT 1')`, false, false},
			{`WITH c AS(SELECT 1 AS id) DELETE reporting.items`, false, false},
			{`CREATE FUNCTION reporting.f(@x int) RETURNS int AS BEGIN DECLARE @y int; SET @y=@x+1; RETURN @y END`, true, true},
			{`CREATE FUNCTION reporting.f() RETURNS TABLE AS RETURN (SELECT id FROM reporting.items)`, true, true},
			{`CREATE FUNCTION reporting.f() RETURNS @r TABLE(id int) AS BEGIN INSERT @r SELECT id FROM reporting.items; RETURN END`, true, true},
			{`CREATE FUNCTION reporting.f() RETURNS int AS BEGIN DELETE reporting.items; RETURN 1 END`, true, false},
			{`CREATE VIEW reporting.v AS SELECT * FROM reporting.items`, true, true},
			{`CREATE FUNCTION reporting.f() RETURNS int WITH EXECUTE AS OWNER AS BEGIN RETURN 1 END`, true, false},
		} {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := parser.Parse(ctx, test.query, compatibility, test.module)
			cancel()
			if (err == nil) != test.allowed {
				t.Errorf("compatibility=%d query=%s allowed=%v error=%v", compatibility, test.query, test.allowed, err)
			}
		}
	}
	proof, err := parser.Parse(context.Background(), `SELECT reporting.scalar_fn(@p1) FROM reporting.items a, private.secrets b CROSS APPLY reporting.rows_fn(a.id) f`, 160, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof.References) != 4 || len(proof.Parameters) != 1 {
		t.Fatalf("source closure: %#v", proof)
	}
}

func TestUnqualifiedEntryUsesNativeResolutionBeforeScopeCheck(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		target := sqlServerTestTarget(db)
		if allowed {
			target.policy.Store(NewPolicy("analytics", "reporting", []string{"reporting", "dbo"}, nil, target.limits.MaxSQLBytes))
		}
		target.parser = parserFunc(func(context.Context, string, int, bool) (*parserResult, error) {
			return &parserResult{References: []parserReference{{Parts: []string{"items"}, Kind: "relation"}}}, nil
		})
		verified := false
		target.verifyReferences = func(_ context.Context, refs []parserReference) error {
			verified = true
			if len(refs) != 1 || len(refs[0].Parts) != 2 || refs[0].Parts[0] != "dbo" {
				t.Fatalf("catalog proof received unresolved entry: %#v", refs)
			}
			return nil
		}
		mock.ExpectQuery("SELECT s.name,o.name FROM sys.objects").WithArgs("[items]").WillReturnRows(sqlmock.NewRows([]string{"schema", "name"}).AddRow("dbo", "items"))
		_, err = target.parseQuery(context.Background(), "SELECT * FROM items", 0)
		if (err == nil) != allowed || verified != allowed {
			t.Errorf("allow dbo=%v verified=%v error=%v", allowed, verified, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestScriptDomRejectsMalformedHelperAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "helper")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'not-a-frame'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := (scriptDOM{path: path}).Parse(context.Background(), "SELECT 1", 160, false); err == nil {
		t.Fatal("malformed frame accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (scriptDOM{path: path}).Parse(ctx, "SELECT 1", 160, false); err == nil {
		t.Fatal("cancelled helper accepted")
	}
	if _, err := parserPath(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing helper accepted")
	}
}
