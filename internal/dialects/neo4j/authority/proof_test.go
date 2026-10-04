package authority

import "testing"

func readerProof() Proof {
	return Proof{User: "agent_ro", Roles: []string{"PUBLIC", "agent_graph_ro"}, Privileges: []Privilege{
		{Access: "GRANTED", Action: "access", Resource: "database", Graph: "knowledge", Segment: "database", Role: "agent_graph_ro"},
		{Access: "GRANTED", Action: "match", Resource: "all_properties", Graph: "knowledge", Segment: "NODE(*)", Role: "agent_graph_ro"},
		{Access: "GRANTED", Action: "show_index", Resource: "database", Graph: "knowledge", Segment: "database", Role: "agent_graph_ro"},
		{Access: "GRANTED", Action: "show_privilege", Resource: "dbms", Graph: "*", Segment: "dbms", Role: "agent_graph_ro"},
		{Access: "GRANTED", Action: "show_user", Resource: "dbms", Graph: "*", Segment: "dbms", Role: "agent_graph_ro"},
	}}
}

func TestReadOnlyProof(t *testing.T) {
	if err := Validate("knowledge", "agent_ro", readerProof()); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		edit func(*Proof)
	}{
		{"community roles absent", func(p *Proof) { p.Roles = nil }},
		{"wrong user", func(p *Proof) { p.User = "admin" }},
		{"write role", func(p *Proof) { p.Roles = append(p.Roles, "editor") }},
		{"global read", func(p *Proof) { p.Privileges[1].Graph = "*" }},
		{"other graph", func(p *Proof) { p.Privileges[1].Graph = "private" }},
		{"unknown read resource", func(p *Proof) { p.Privileges[1].Resource = "dbms" }},
		{"write grant", func(p *Proof) {
			p.Privileges = append(p.Privileges, Privilege{Access: "GRANTED", Action: "write", Resource: "graph", Graph: "knowledge", Role: "agent_graph_ro"})
		}},
		{"outbound load", func(p *Proof) {
			p.Privileges = append(p.Privileges, Privilege{Access: "GRANTED", Action: "load", Resource: "url", Graph: "*", Role: "PUBLIC"})
		}},
		{"boosted procedure", func(p *Proof) {
			p.Privileges = append(p.Privileges, Privilege{Access: "GRANTED", Action: "execute_boosted_procedure", Resource: "dbms", Graph: "*", Role: "PUBLIC"})
		}},
		{"unknown grant", func(p *Proof) {
			p.Privileges = append(p.Privileges, Privilege{Access: "GRANTED", Action: "future_admin_action", Resource: "dbms", Role: "PUBLIC"})
		}},
		{"unassigned role", func(p *Proof) { p.Privileges[0].Role = "other" }},
		{"no database access", func(p *Proof) { p.Privileges = p.Privileges[1:] }},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := readerProof()
			test.edit(&p)
			if err := Validate("knowledge", "agent_ro", p); err == nil {
				t.Fatal("unsafe Neo4j authority accepted")
			}
		})
	}
}
