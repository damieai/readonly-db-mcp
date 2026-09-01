package postgresql

import "testing"

func TestPolicyAllowsAdvancedReadOnlySQL(t *testing.T) {
	p := NewPolicy([]string{"reporting"}, nil, 32<<10)
	q := `WITH RECURSIVE tree AS (SELECT id,parent_id,1 depth FROM reporting.nodes WHERE id=$1 UNION ALL SELECT n.id,n.parent_id,t.depth+1 FROM reporting.nodes n JOIN tree t ON n.parent_id=t.id) SELECT DISTINCT ON (id) id,depth,rank() OVER (ORDER BY depth DESC),jsonb_build_object('id',id) FROM tree GROUP BY GROUPING SETS ((id,depth),(id)) ORDER BY id,depth DESC`
	v, err := p.Validate(q, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Tables) != 1 || v.Tables[0] != "reporting.nodes" {
		t.Fatalf("tables=%#v", v.Tables)
	}
}
func TestPolicyRejectsMutationShapes(t *testing.T) {
	p := NewPolicy([]string{"reporting"}, nil, 32<<10)
	tests := []string{`INSERT INTO reporting.items(id) VALUES (1) RETURNING id`, `WITH changed AS (UPDATE reporting.items SET value=1 RETURNING id) SELECT * FROM changed`, `SELECT * INTO new_items FROM reporting.items`, `SELECT * FROM reporting.items FOR UPDATE`, `COPY reporting.items TO '/tmp/x'`, `SELECT nextval('reporting.seq')`}
	for _, q := range tests {
		t.Run(q, func(t *testing.T) {
			if _, err := p.Validate(q, 0); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
func TestPolicyRequiresQualifiedRelationsAndContiguousParameters(t *testing.T) {
	p := NewPolicy([]string{"reporting"}, nil, 32<<10)
	if _, err := p.Validate(`SELECT * FROM items`, 0); err == nil {
		t.Fatal("expected qualification rejection")
	}
	if _, err := p.Validate(`SELECT * FROM reporting.items WHERE id=$2`, 2); err == nil {
		t.Fatal("expected parameter gap rejection")
	}
}
func TestPolicyRejectsUserDefinedFunctionQualification(t *testing.T) {
	p := NewPolicy([]string{"reporting"}, nil, 32<<10)
	if _, err := p.Validate(`SELECT reporting.do_work(id) FROM reporting.items`, 0); err == nil {
		t.Fatal("expected function rejection")
	}
}
