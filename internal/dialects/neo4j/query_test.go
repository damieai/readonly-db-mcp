package neo4j

import (
	"context"
	"errors"
	"testing"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

func TestNeo4jReadInterruptionClassification(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !neo4jReadInterrupted(ctx, errors.New("connection closed during cancellation")) {
		t.Fatal("client cancellation was lost behind a transport error")
	}
	for _, code := range []string{
		"Neo.ClientError.Transaction.TransactionTimedOut",
		"Neo.ClientError.Transaction.TransactionTimedOutClientConfiguration",
	} {
		if !neo4jReadInterrupted(context.Background(), &driver.Neo4jError{Code: code}) {
			t.Fatalf("server timeout %s was not classified as an interruption", code)
		}
	}
	if neo4jReadInterrupted(context.Background(), &driver.Neo4jError{Code: "Neo.ClientError.Security.Forbidden"}) {
		t.Fatal("permission denial was misclassified as a deadline")
	}
}
