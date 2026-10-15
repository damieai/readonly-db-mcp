package neo4j

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/your-org/readonly-db-mcp/internal/dialects/neo4j/authority"
)

func (t *Target) ensureProof(ctx context.Context) error {
	t.mu.Lock()
	ready := !t.closed && time.Since(t.checked) < t.cfg.Neo4j.PrivilegeRecheck
	checked := t.checked
	closed := t.closed
	t.mu.Unlock()
	t.recordProofAge(checked)
	if closed {
		return errors.New("target_closed: Neo4j target is closed")
	}
	if ready {
		return nil
	}
	if !checked.IsZero() && t.recorder != nil {
		t.recorder.Add("neo4j_proof_expiry", 1, t.cfg.Name, "expired")
	}
	return t.prove(ctx)
}

func (t *Target) prove(ctx context.Context) (err error) {
	started := time.Now()
	stage := "server_identity"
	attempted := false
	t.mu.Lock()
	defer func() {
		checked := t.checked
		t.mu.Unlock()
		if attempted {
			t.recordProofOutcome(ctx, started, checked, stage, err)
		}
	}()
	if t.closed {
		return errors.New("target_closed: Neo4j target is closed")
	}
	if !t.checked.IsZero() && time.Since(t.checked) < t.cfg.Neo4j.PrivilegeRecheck {
		return nil
	}
	attempted = true
	t.checked = time.Time{} // A failed refresh blocks new requests.
	for _, d := range []driver.Driver{t.query, t.attestor} {
		info, err := d.GetServerInfo(ctx)
		if err != nil || info == nil {
			return errors.New("transport_error: Neo4j server identity inspection failed")
		}
		if info.Agent() != "Neo4j/"+t.cfg.Neo4j.Version {
			return errors.New("profile_mismatch: Neo4j server version or identity differs from configuration")
		}
	}
	stage = "query_identity"
	name := t.cfg.Username // The config validator restricts this to a safe identifier.
	querySession := t.query.NewSession(ctx, driver.SessionConfig{DatabaseName: "system", AccessMode: driver.AccessModeRead, FetchSize: 2})
	currentRows, currentErr := readFixed(ctx, querySession, "SHOW CURRENT USER YIELD user, roles, passwordChangeRequired, suspended RETURN user, roles, passwordChangeRequired, suspended", 2)
	closeErr := querySession.Close(ctx)
	if currentErr != nil || closeErr != nil {
		return errors.New("permission_denied: Neo4j query identity inspection failed")
	}
	currentRoles, err := readCurrentIdentity(currentRows, name)
	if err != nil {
		return err
	}
	stage = "auth_rules"
	userQuery := "SHOW USERS YIELD user, roles WHERE user = $name RETURN user, roles"
	privQuery := "SHOW USER `" + name + "` PRIVILEGES YIELD access, action, resource, graph, segment, role RETURN access, action, resource, graph, segment, role"
	session := t.attestor.NewSession(ctx, driver.SessionConfig{DatabaseName: "system", AccessMode: driver.AccessModeRead, FetchSize: 128})
	defer session.Close(ctx)
	authRules, err := readFixed(ctx, session, "SHOW AUTH RULES YIELD name, enabled, roles RETURN name, enabled, roles", 1001)
	if err != nil || len(authRules) > 1000 {
		return errors.New("permission_denied: Neo4j auth-rule inspection failed")
	}
	if err := validateAuthRules(authRules); err != nil {
		return err
	}
	stage = "user_roles"
	userRows, err := readFixedParams(ctx, session, userQuery, map[string]any{"name": name}, 2)
	if err != nil || len(userRows) != 1 {
		return errors.New("permission_denied: Neo4j user/role inspection failed")
	}
	user, ok := userRows[0]["user"].(string)
	if !ok || user != name {
		return errors.New("profile_mismatch: Neo4j attestor reported another user")
	}
	roles, err := readRoles(userRows[0]["roles"])
	if err != nil {
		return err
	}
	if !sameRoles(currentRoles, roles) {
		return errors.New("profile_mismatch: Neo4j query and attestor roles differ")
	}
	stage = "privileges"
	privRows, err := readFixed(ctx, session, privQuery, 1001)
	if err != nil || len(privRows) == 0 || len(privRows) > 1000 {
		return errors.New("permission_denied: Neo4j effective privilege inspection failed")
	}
	proof := authority.Proof{User: user, Roles: roles, Privileges: make([]authority.Privilege, 0, len(privRows))}
	for _, row := range privRows {
		get := func(key string) (string, bool) {
			value, ok := row[key].(string)
			return value, ok
		}
		access, a := get("access")
		action, b := get("action")
		resource, c := get("resource")
		graph, d := get("graph")
		segment, e := get("segment")
		role, f := get("role")
		if !a || !b || !c || !d || !e || !f {
			return errors.New("profile_mismatch: Neo4j privilege row is incomplete")
		}
		proof.Privileges = append(proof.Privileges, authority.Privilege{Access: access, Action: action, Resource: resource, Graph: graph, Segment: segment, Role: role})
	}
	stage = "authority"
	if err := authority.Validate(t.cfg.Database, name, proof); err != nil {
		return err
	}
	t.checked = time.Now()
	return nil
}

func (t *Target) recordProofAge(checked time.Time) {
	if t.recorder == nil {
		return
	}
	age := int64(-1)
	if !checked.IsZero() {
		age = max(0, time.Since(checked).Milliseconds())
	}
	t.recorder.Set("neo4j_proof_age_ms", age, t.cfg.Name)
}

func (t *Target) recordProofOutcome(ctx context.Context, started, checked time.Time, stage string, err error) {
	if t.recorder == nil {
		return
	}
	outcome, ready := "success", int64(1)
	if err != nil {
		ready = 0
		category := "transport"
		switch {
		case ctx.Err() != nil:
			category = "deadline"
		case strings.HasPrefix(err.Error(), "permission_denied:"):
			category = "permission"
		case strings.HasPrefix(err.Error(), "profile_mismatch:"):
			category = "profile"
		case strings.HasPrefix(err.Error(), "resource_limit:"):
			category = "resource"
		}
		outcome = "failure:" + stage + ":" + category
	}
	t.recorder.Add("neo4j_proof", 1, t.cfg.Name, outcome)
	t.recorder.Observe("neo4j_proof_duration", time.Since(started), t.cfg.Name)
	t.recorder.Set("neo4j_proof_ready", ready, t.cfg.Name)
	t.recordProofAge(checked)
}

func readCurrentIdentity(rows []map[string]any, expectedUser string) ([]string, error) {
	if len(rows) != 1 || rows[0]["user"] != expectedUser || rows[0]["passwordChangeRequired"] != false || rows[0]["suspended"] != false {
		return nil, errors.New("profile_mismatch: Neo4j query identity is unavailable or inactive")
	}
	return readRoles(rows[0]["roles"])
}

func sameRoles(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, role := range left {
		counts[role]++
	}
	for _, role := range right {
		counts[role]--
		if counts[role] < 0 {
			return false
		}
	}
	return true
}

// An enabled auth rule may grant roles at login or after a metadata change.
// SHOW USER PRIVILEGES alone cannot establish the query user's future grants.
func validateAuthRules(rows []map[string]any) error {
	for _, row := range rows {
		name, nameOK := row["name"].(string)
		enabled, enabledOK := row["enabled"].(bool)
		roles, rolesErr := readAuthRuleRoles(row["roles"])
		if !nameOK || name == "" || !enabledOK || rolesErr != nil {
			return errors.New("profile_mismatch: Neo4j auth-rule row is incomplete")
		}
		if enabled && len(roles) != 0 {
			return errors.New("permission_denied: Neo4j enabled auth rule may change query privileges")
		}
	}
	return nil
}

func readAuthRuleRoles(raw any) ([]string, error) {
	if raw == nil {
		return nil, errors.New("auth-rule roles are hidden")
	}
	switch list := raw.(type) {
	case []string:
		for _, role := range list {
			if role == "" {
				return nil, errors.New("malformed auth-rule role")
			}
		}
		return list, nil
	case []any:
		roles := make([]string, 0, len(list))
		for _, value := range list {
			role, ok := value.(string)
			if !ok || role == "" {
				return nil, errors.New("malformed auth-rule role")
			}
			roles = append(roles, role)
		}
		return roles, nil
	default:
		return nil, errors.New("malformed auth-rule roles")
	}
}

func readRoles(raw any) ([]string, error) {
	var roles []string
	switch list := raw.(type) {
	case []string:
		roles = append(roles, list...)
	case []any:
		for _, value := range list {
			role, ok := value.(string)
			if !ok {
				return nil, errors.New("profile_mismatch: Neo4j role list is malformed")
			}
			roles = append(roles, role)
		}
	default:
		return nil, errors.New("profile_mismatch: Neo4j roles are unavailable")
	}
	if len(roles) == 0 || len(roles) > 64 {
		return nil, errors.New("profile_mismatch: Neo4j role list is incomplete")
	}
	return roles, nil
}

func readFixed(ctx context.Context, session driver.Session, query string, maxRows int) ([]map[string]any, error) {
	return readFixedParams(ctx, session, query, nil, maxRows)
}

func readFixedParams(ctx context.Context, session driver.Session, query string, params map[string]any, maxRows int) ([]map[string]any, error) {
	result, err := session.Run(ctx, query, params, driver.WithTxTimeout(30*time.Second))
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0)
	for result.Next(ctx) {
		if len(rows) >= maxRows {
			return nil, fmt.Errorf("resource_limit: Neo4j proof exceeds %d rows", maxRows)
		}
		row := result.Record().AsMap()
		if tooLarge(row, 1<<20) {
			return nil, errors.New("resource_limit: Neo4j proof row is too large")
		}
		rows = append(rows, row)
	}
	if err := result.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}
