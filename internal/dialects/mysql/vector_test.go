package mysql

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestVectorSearchRanksCompleteReadOnlyCandidateSet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)
	target.limits.MaxParameterValueBytes = 64 << 10
	query := "SELECT id, embedding FROM items WHERE tenant = ? ORDER BY id"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, embedding FROM items WHERE tenant = \\? ORDER BY id").WithArgs("team-a").WillReturnRows(
		sqlmock.NewRows([]string{"id", "embedding"}).
			AddRow("far", `[0,1]`).AddRow("near", `[1,0]`).AddRow("other", `[0.8,0.2]`),
	)
	mock.ExpectRollback()
	result, err := target.VectorSearch(context.Background(), core.MySQLVectorRequest{
		SQL: query, Parameters: []any{"team-a"}, EmbeddingColumn: "embedding",
		Vector: []float64{1, 0}, Metric: "cosine", TopK: 2, MaxCandidates: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CandidateCount != 3 || len(result.Hits) != 2 || result.Hits[0].Row["id"] != "near" || result.Hits[1].Row["id"] != "other" {
		t.Fatalf("unexpected ranking: %+v", result)
	}
	if _, exposed := result.Hits[0].Row["embedding"]; exposed {
		t.Fatal("embedding was returned in ranked hit")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVectorSearchRejectsPartialAndMalformedCandidates(t *testing.T) {
	for _, test := range []struct {
		name string
		rows *sqlmock.Rows
		want string
	}{
		{"candidate cap", sqlmock.NewRows([]string{"id", "embedding"}).AddRow("a", `[1,0]`).AddRow("b", `[0,1]`), "exceeds"},
		{"invalid vector", sqlmock.NewRows([]string{"id", "embedding"}).AddRow("a", `[1,"bad"]`), "invalid"},
		{"wrong dimensions", sqlmock.NewRows([]string{"id", "embedding"}).AddRow("a", `[1,0,0]`), "invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			target := testTarget(db)
			target.limits.MaxParameterValueBytes = 64 << 10
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id, embedding FROM items").WillReturnRows(test.rows)
			mock.ExpectRollback()
			_, err = target.VectorSearch(context.Background(), core.MySQLVectorRequest{
				SQL: "SELECT id, embedding FROM items", EmbeddingColumn: "embedding",
				Vector: []float64{1, 0}, Metric: "l2", TopK: 1, MaxCandidates: 1,
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q rejection, got %v", test.want, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVectorSearchRejectsMutationAndInvalidQueryVectorBeforeDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)
	target.limits.MaxParameterValueBytes = 64 << 10
	for _, request := range []core.MySQLVectorRequest{
		{SQL: "DELETE FROM items", EmbeddingColumn: "embedding", Vector: []float64{1}, Metric: "l2", TopK: 1, MaxCandidates: 1},
		{SQL: "SELECT embedding FROM items", EmbeddingColumn: "embedding", Vector: []float64{0}, Metric: "cosine", TopK: 1, MaxCandidates: 1},
	} {
		if _, err := target.VectorSearch(context.Background(), request); err == nil {
			t.Fatal("expected rejection")
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVectorSearchRejectsClippedEmbedding(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := testTarget(db)
	target.limits.MaxParameterValueBytes = 64 << 10
	target.limits.MaxCellBytes = 4
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT embedding FROM items").WillReturnRows(
		sqlmock.NewRows([]string{"embedding"}).AddRow(`[1,0]`),
	)
	mock.ExpectRollback()
	_, err = target.VectorSearch(context.Background(), core.MySQLVectorRequest{
		SQL: "SELECT embedding FROM items", EmbeddingColumn: "embedding",
		Vector: []float64{1, 0}, Metric: "l2", TopK: 1, MaxCandidates: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected clipped embedding rejection, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVectorScoreDirections(t *testing.T) {
	dot, err := vectorScore([]float64{2, 0}, []float64{1, 0}, "dot")
	if err != nil || dot != 2 {
		t.Fatalf("dot=%v err=%v", dot, err)
	}
	l2, err := vectorScore([]float64{2, 0}, []float64{1, 0}, "l2")
	if err != nil || l2 != 1 {
		t.Fatalf("l2=%v err=%v", l2, err)
	}
}
