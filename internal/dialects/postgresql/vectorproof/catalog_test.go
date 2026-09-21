package vectorproof

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReviewedProfileInventoryAndDrift(t *testing.T) {
	p, err := ReviewedProfile()
	if err != nil {
		t.Fatal(err)
	}
	s := &Snapshot{objects: map[Reference]Object{}}
	for i, obj := range p.Objects {
		s.objects[Reference{obj.Class, uint32(10000 + i)}] = obj
	}
	if err := s.Verify(p); err != nil {
		t.Fatal(err)
	}
	if s.digest == "" {
		t.Fatal("no proof revision")
	}
	copy := s.Profile()
	copy.Objects[0].Definition[0] = '!'
	if err := s.Verify(p); err != nil {
		t.Fatal("exported profile mutated the captured snapshot", err)
	}
	for ref, obj := range s.objects {
		obj.Member = !obj.Member
		s.objects[ref] = obj
		break
	}
	if s.Verify(p) == nil || s.digest != "" {
		t.Fatal("failed recheck retained authority")
	}
}

func TestBindingEnvelopeAndExactIdentity(t *testing.T) {
	header := `{"format":1,"postgresql_major":16,"sql_sha256":"` + strings.Repeat("a", 64) + `","database_oid":1,"role_oid":1,`
	for _, raw := range []string{
		`{}`, `{"format":1,"postgresql_major":16,"objects":null}`,
		`{"format":1,"postgresql_major":17,"objects":[]}`,
		header + `"objects":[{"class":"pg_proc","oid":0}]}`,
		header + `"objects":[{"class":"pg_proc","oid":1},{"class":"pg_proc","oid":1}]}`,
		header + `"objects":[],"parameters":[705]}`,
		header + `"objects":[],"execute":true}`,
		header + `"objects":[{"class":"pg_authid","oid":1}]}`,
	} {
		if _, err := DecodeBinding([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	b := Binding{Format: 1, PostgreSQLMajor: 16, SQLSHA256: strings.Repeat("a", 64), DatabaseOID: 1, RoleOID: 1, Parameters: []uint32{23}, Objects: []Reference{{"pg_type", 23}, {"pg_proc", 23}}}
	raw, _ := json.Marshal(b)
	if _, err := DecodeBinding(raw); err != nil {
		t.Fatal("class-qualified OIDs collided", err)
	}
}
