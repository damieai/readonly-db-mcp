package mcpserver

import "testing"

func TestMongoDBToolEnvelopeRejectsAmbiguity(t *testing.T) {
	for _, body := range []string{
		`{"target":"docs","target":"other","collection":"docs","operation":"find","body":{"filter":{}}}`,
		`{"target":"docs","collection":"docs","operation":"find","body":null}`,
		`{"target":"docs","collection":"docs","operation":"find","body":{"filter":{}}} trailing`,
	} {
		if err := strictMongoEnvelope([]byte(body)); err == nil {
			t.Errorf("accepted ambiguous envelope %s", body)
		}
	}
	if err := strictMongoEnvelope([]byte(`{"target":"docs","collection":"docs","operation":"aggregate","body":{"pipeline":[{"$match":{"n":{"$numberLong":"9007199254740993"}}}]}}`)); err != nil {
		t.Fatal(err)
	}
}
