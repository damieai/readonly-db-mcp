package neo4j

import (
	"context"
	"errors"
	"testing"
	"time"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/your-org/readonly-db-mcp/internal/config"
)

type proofRecorder struct {
	adds      map[string]int64
	sets      map[string]int64
	durations map[string]int
}

func (r *proofRecorder) Add(name string, delta int64, operation, outcome string) {
	if r.adds == nil {
		r.adds = map[string]int64{}
	}
	r.adds[name+"|"+operation+"|"+outcome] += delta
}
func (r *proofRecorder) Observe(name string, _ time.Duration, operation string) {
	if r.durations == nil {
		r.durations = map[string]int{}
	}
	r.durations[name+"|"+operation]++
}
func (r *proofRecorder) Set(name string, value int64, operation string) {
	if r.sets == nil {
		r.sets = map[string]int64{}
	}
	r.sets[name+"|"+operation] = value
}

type failedServerInfoDriver struct{ driver.Driver }

func (failedServerInfoDriver) GetServerInfo(context.Context) (driver.ServerInfo, error) {
	return nil, errors.New("offline")
}

func TestNeo4jProofExpiryAndFailureMetrics(t *testing.T) {
	recorder := &proofRecorder{}
	target := &Target{
		cfg:   &config.TargetConfig{Name: "graph", Neo4j: &config.Neo4jConfig{Version: "2026.09.0", PrivilegeRecheck: time.Second}},
		query: failedServerInfoDriver{}, attestor: failedServerInfoDriver{},
		checked: time.Now().Add(-2 * time.Second), recorder: recorder,
	}
	if err := target.ensureProof(t.Context()); err == nil || target.Info().Healthy {
		t.Fatalf("failed proof did not close the target: %v", err)
	}
	if recorder.adds["neo4j_proof_expiry|graph|expired"] != 1 || recorder.adds["neo4j_proof|graph|failure:server_identity:transport"] != 1 || recorder.sets["neo4j_proof_ready|graph"] != 0 || recorder.sets["neo4j_proof_age_ms|graph"] != -1 || recorder.durations["neo4j_proof_duration|graph"] != 1 {
		t.Fatalf("proof failure metrics are incomplete: adds=%v sets=%v durations=%v", recorder.adds, recorder.sets, recorder.durations)
	}
	target.recordProofOutcome(t.Context(), time.Now(), time.Now(), "authority", nil)
	if recorder.adds["neo4j_proof|graph|success"] != 1 || recorder.sets["neo4j_proof_ready|graph"] != 1 {
		t.Fatalf("proof recovery metrics are incomplete: adds=%v sets=%v", recorder.adds, recorder.sets)
	}
}

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
