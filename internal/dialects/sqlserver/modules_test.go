package sqlserver

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

type moduleParserStub struct{ reject bool }

type parserFunc func(context.Context, string, int, bool) (*parserResult, error)

func (f parserFunc) Parse(ctx context.Context, query string, compatibility int, module bool) (*parserResult, error) {
	return f(ctx, query, compatibility, module)
}

func (p moduleParserStub) Parse(context.Context, string, int, bool) (*parserResult, error) {
	if p.reject {
		return nil, errors.New("unproven module")
	}
	return &parserResult{Accepted: true, Protocol: 1, Nodes: 1}, nil
}

func TestModuleClosureProtectsHiddenAndTransitiveCapabilities(t *testing.T) {
	for _, test := range []struct {
		name, kind, definition            string
		unsafe, unresolved, reject, allow bool
	}{
		{"ordinary curated view", "V", "CREATE VIEW", false, false, false, true},
		{"transitive CLR", "FS", "", false, false, false, false},
		{"encrypted module", "FN", "", false, false, false, false},
		{"signed module", "FN", "CREATE FUNCTION", true, false, false, false},
		{"external dependency", "FN", "CREATE FUNCTION", false, true, false, false},
		{"unsafe definition", "FN", "CREATE FUNCTION", false, false, true, false},
		{"safe recursive closure", "FN", "CREATE FUNCTION", false, false, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			target := sqlServerTestTarget(db)
			target.catalogDB = db
			target.parser = moduleParserStub{reject: test.reject}
			mock.ExpectQuery("SELECT HAS_PERMS_BY_NAME").WillReturnRows(sqlmock.NewRows([]string{"visible"}).AddRow(1))
			mock.ExpectQuery("SELECT TOP \\(10001\\) o.object_id").WillReturnRows(sqlmock.NewRows([]string{"id", "schema", "name", "type", "definition", "synonym", "unsafe"}).
				AddRow(1, "reporting", "entry", "V", "CREATE VIEW", "", false).
				AddRow(2, "private", "dependency", test.kind, test.definition, "", test.unsafe))
			mock.ExpectQuery("SELECT TOP \\(100001\\) referencing_id").WillReturnRows(sqlmock.NewRows([]string{"from", "to", "unsafe", "class"}).AddRow(1, 2, false, 1).AddRow(2, 1, test.unresolved, 1))
			err = target.verifyModuleReferences(context.Background(), []parserReference{{Parts: []string{"reporting", "entry"}, Kind: "relation"}})
			if (err == nil) != test.allow {
				t.Fatalf("allowed=%v error=%v", test.allow, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestModuleClosureRequiresCurrentCatalogVisibility(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT HAS_PERMS_BY_NAME").WillReturnRows(sqlmock.NewRows([]string{"visible"}).AddRow(0))
	if _, _, err := loadModuleGraph(context.Background(), db); err == nil {
		t.Fatal("invisible catalog accepted")
	}
}

func TestModuleUnqualifiedReferenceUsesNativeBinding(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := sqlServerTestTarget(db)
	target.catalogDB = db
	target.parser = parserFunc(func(context.Context, string, int, bool) (*parserResult, error) {
		return &parserResult{References: []parserReference{{Parts: []string{"items"}, Kind: "relation"}}}, nil
	})
	mock.ExpectQuery("SELECT HAS_PERMS_BY_NAME").WillReturnRows(sqlmock.NewRows([]string{"visible"}).AddRow(1))
	mock.ExpectQuery("SELECT TOP \\(10001\\) o.object_id").WillReturnRows(sqlmock.NewRows([]string{"id", "schema", "name", "type", "definition", "synonym", "unsafe"}).
		AddRow(1, "reporting", "entry", "V", "CREATE VIEW reporting.entry AS SELECT * FROM items", "", false).
		AddRow(2, "dbo", "items", "U", "", "", false).
		AddRow(3, "reporting", "items", "U", "", "", true))
	mock.ExpectQuery("SELECT TOP \\(100001\\) referencing_id").WillReturnRows(sqlmock.NewRows([]string{"from", "to", "unsafe", "class"}).AddRow(1, 2, false, 1))
	if err := target.verifyModuleReferences(context.Background(), []parserReference{{Parts: []string{"reporting", "entry"}, Kind: "relation"}}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDependencyIdentifiersAreSeparatedByCatalogClass(t *testing.T) {
	for _, test := range []struct {
		name          string
		class         int
		unsafe, allow bool
	}{
		{"object reaches CLR function", 1, false, false},
		{"alias type is not an object", 6, false, true},
		{"CLR type capability", 6, true, false},
		{"index metadata", 7, false, true},
		{"XML schema metadata", 10, false, true},
		{"partition metadata", 21, false, true},
		{"unknown dependency class", 99, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			target := sqlServerTestTarget(db)
			target.catalogDB = db
			target.parser = moduleParserStub{}
			mock.ExpectQuery("SELECT HAS_PERMS_BY_NAME").WillReturnRows(sqlmock.NewRows([]string{"visible"}).AddRow(1))
			mock.ExpectQuery("SELECT TOP \\(10001\\) o.object_id").WillReturnRows(sqlmock.NewRows([]string{"id", "schema", "name", "type", "definition", "synonym", "unsafe"}).
				AddRow(1, "reporting", "entry", "V", "CREATE VIEW", "", false).
				AddRow(2, "private", "same_numeric_id", "FS", "", "", false))
			mock.ExpectQuery("SELECT TOP \\(100001\\) referencing_id").WillReturnRows(sqlmock.NewRows([]string{"from", "to", "unsafe", "class"}).AddRow(1, 2, test.unsafe, test.class))
			err = target.verifyModuleReferences(context.Background(), []parserReference{{Parts: []string{"reporting", "entry"}, Kind: "relation"}})
			if (err == nil) != test.allow {
				t.Fatalf("allow=%v error=%v", test.allow, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
