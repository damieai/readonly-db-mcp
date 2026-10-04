// Package authority validates a Neo4j Enterprise query identity's reported
// privileges before any untrusted Cypher is admitted. It does not connect to
// Neo4j; the future connector must collect SHOW CURRENT USER and SHOW USER
// PRIVILEGES from the server and recheck them periodically.
package authority

import (
	"errors"
	"fmt"
	"strings"
)

type Privilege struct {
	Access   string
	Action   string
	Resource string
	Graph    string
	Segment  string
	Role     string
}

type Proof struct {
	User       string
	Roles      []string
	Privileges []Privilege
}

// Validate requires native read-only authority confined to one exact graph.
// DENIED rows may narrow authority, but they never make an unsafe GRANTED row
// acceptable: a future role change or different segment could expose it.
func Validate(database, expectedUser string, proof Proof) error {
	if database == "" || expectedUser == "" || proof.User != expectedUser || len(proof.Roles) == 0 || len(proof.Privileges) == 0 {
		return errors.New("profile_mismatch: Neo4j identity or privilege proof is incomplete")
	}
	roleSet := make(map[string]bool, len(proof.Roles))
	for _, role := range proof.Roles {
		if role == "" {
			return errors.New("profile_mismatch: Neo4j role is empty")
		}
		switch strings.ToLower(role) {
		case "admin", "architect", "publisher", "editor":
			return errors.New("permission_denied: Neo4j query identity has a built-in write role")
		}
		roleSet[role] = true
	}
	access, graphRead := false, false
	for _, p := range proof.Privileges {
		if !roleSet[p.Role] {
			return errors.New("profile_mismatch: Neo4j privilege refers to an unassigned role")
		}
		switch strings.ToUpper(p.Access) {
		case "DENIED":
			continue
		case "GRANTED":
		default:
			return errors.New("profile_mismatch: Neo4j privilege access is unknown")
		}
		action := strings.ToLower(p.Action)
		switch action {
		case "access":
			if p.Resource != "database" || p.Graph != database || p.Segment != "database" {
				return errors.New("permission_denied: Neo4j database access is out of scope")
			}
			access = true
		case "traverse", "read", "match":
			if p.Graph != database || (p.Resource != "graph" && p.Resource != "all_properties" && p.Resource != "property") {
				return errors.New("permission_denied: Neo4j graph read is out of scope")
			}
			graphRead = true
		case "show_index", "show_constraint":
			if p.Resource != "database" || p.Graph != database {
				return errors.New("permission_denied: Neo4j metadata grant is out of scope")
			}
		case "show_privilege", "show_user":
			if p.Resource != "dbms" {
				return errors.New("permission_denied: Neo4j identity-inspection grant is malformed")
			}
		default:
			return fmt.Errorf("permission_denied: Neo4j action %q is outside the reviewed read profile", p.Action)
		}
	}
	if !access || !graphRead {
		return errors.New("profile_mismatch: Neo4j database access and graph read are required")
	}
	return nil
}
