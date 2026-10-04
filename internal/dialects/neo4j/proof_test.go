package neo4j

import "testing"

func TestAuthRuleProof(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rows      []map[string]any
		wantError bool
	}{
		{name: "no rules"},
		{name: "disabled role", rows: []map[string]any{{"name": "old", "enabled": false, "roles": []any{"admin"}}}},
		{name: "enabled no role", rows: []map[string]any{{"name": "empty", "enabled": true, "roles": []any{}}}},
		{name: "enabled role", rows: []map[string]any{{"name": "dynamic", "enabled": true, "roles": []any{"reader"}}}, wantError: true},
		{name: "missing enablement", rows: []map[string]any{{"name": "unknown", "roles": []any{}}}, wantError: true},
		{name: "hidden roles", rows: []map[string]any{{"name": "hidden", "enabled": false, "roles": nil}}, wantError: true},
		{name: "malformed role", rows: []map[string]any{{"name": "bad", "enabled": false, "roles": []any{1}}}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAuthRules(tc.rows)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateAuthRules() error = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}
