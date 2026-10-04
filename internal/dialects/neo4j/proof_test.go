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

func TestCurrentQueryIdentityMustMatchAttestor(t *testing.T) {
	current := []map[string]any{{"user": "graph_ro", "roles": []any{"PUBLIC", "graph_reader"}, "passwordChangeRequired": false, "suspended": false}}
	roles, err := readCurrentIdentity(current, "graph_ro")
	if err != nil || !sameRoles(roles, []string{"graph_reader", "PUBLIC"}) {
		t.Fatalf("valid query identity rejected: roles=%v err=%v", roles, err)
	}
	if sameRoles(roles, []string{"PUBLIC", "admin"}) {
		t.Fatal("role drift was accepted")
	}
	for _, tc := range []struct {
		name   string
		mutate func([]map[string]any) []map[string]any
	}{
		{"missing", func(_ []map[string]any) []map[string]any { return nil }},
		{"wrong user", func(rows []map[string]any) []map[string]any { rows[0]["user"] = "admin"; return rows }},
		{"suspended", func(rows []map[string]any) []map[string]any { rows[0]["suspended"] = true; return rows }},
		{"password expired", func(rows []map[string]any) []map[string]any { rows[0]["passwordChangeRequired"] = true; return rows }},
		{"community roles", func(rows []map[string]any) []map[string]any { rows[0]["roles"] = nil; return rows }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []map[string]any{{"user": "graph_ro", "roles": []any{"PUBLIC", "graph_reader"}, "passwordChangeRequired": false, "suspended": false}}
			if _, err := readCurrentIdentity(tc.mutate(rows), "graph_ro"); err == nil {
				t.Fatal("invalid query identity accepted")
			}
		})
	}
}
