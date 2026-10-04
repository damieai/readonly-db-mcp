package duckdbworker

import "testing"

func TestSingleStatementBoundary(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"SELECT 1", true},
		{"SELECT ';' AS value; -- trailing comment", true},
		{"SELECT $$a;b$$ AS value", true},
		{"SELECT $tag$a;b$tag$ AS value", true},
		{"SELECT /* nested /* ; */ comment */ 1", true},
		{"SELECT 1; SELECT 2", false},
		{"SELECT 1; 'second statement'", false},
		{"SELECT 1; $tag$second statement$tag$", false},
		{"SELECT 1; /* comment */ DELETE FROM items", false},
		{"SELECT 1; -- comment\n DELETE FROM items", false},
		{"SELECT 1; /* unclosed", false},
		{"SELECT $tag$unterminated", false},
	}
	for _, test := range cases {
		if got := singleStatement(test.sql); got != test.want {
			t.Errorf("singleStatement(%q) = %v, want %v", test.sql, got, test.want)
		}
	}
}
