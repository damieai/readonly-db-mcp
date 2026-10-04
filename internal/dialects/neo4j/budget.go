package neo4j

import (
	"encoding/json"
	"errors"
)

// responseBudget starts with a result whose rows field is an empty JSON array.
// Each appended row adds its encoded bytes and one separator after the first.
// Mutable summary fields are checked against the exact final encoding too.
type responseBudget struct {
	used  int
	limit int
	rows  int
}

func newResponseBudget(emptyResult any, limit int) (*responseBudget, error) {
	encoded, err := json.Marshal(emptyResult)
	if err != nil || len(encoded) > limit {
		return nil, errors.New("resource_limit: Neo4j result exceeds max_result_bytes")
	}
	return &responseBudget{used: len(encoded), limit: limit}, nil
}

func (b *responseBudget) add(row map[string]any) error {
	encoded, err := json.Marshal(row)
	if err != nil {
		return errors.New("invalid_response: Neo4j row cannot be encoded")
	}
	extra := len(encoded)
	if b.rows > 0 {
		extra++
	}
	if extra > b.limit-b.used {
		return errors.New("resource_limit: Neo4j result exceeds max_result_bytes")
	}
	b.used += extra
	b.rows++
	return nil
}
