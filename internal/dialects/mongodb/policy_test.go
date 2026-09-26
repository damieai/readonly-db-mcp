package mongodb

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func policyLimits() (config.Limits, *config.MongoDBConfig, map[string]bool) {
	return config.Limits{MaxRows: 100}, &config.MongoDBConfig{MaxRequestBytes: 1 << 20, MaxJSONDepth: 128, MaxJSONNodes: 100000, MaxPipelineStages: 100}, map[string]bool{"docs": true, "sources": true}
}

func TestMongoDBReadPipelinePolicy(t *testing.T) {
	limits, cfg, scope := policyLimits()
	valid := []string{
		`{"pipeline":[{"$match":{"tenant":"a"}},{"$lookup":{"from":"sources","localField":"source_id","foreignField":"_id","as":"source","pipeline":[{"$project":{"title":1}}]}},{"$limit":10}],"limit":10}`,
		`{"pipeline":[{"$facet":{"popular":[{"$group":{"_id":"$tag","n":{"$sum":1}}},{"$sort":{"n":-1}}],"recent":[{"$sort":{"created_at":-1}},{"$limit":5}]}}]}`,
		`{"pipeline":[{"$unionWith":{"coll":"sources","pipeline":[{"$match":{"tenant":"a"}}]}}]}`,
		`{"pipeline":[{"$lookup":{"pipeline":[{"$documents":[{"x":1}]}],"as":"literal"}}]}`,
	}
	for _, body := range valid {
		if _, err := validateRequest("aggregate", []byte(body), limits, cfg, scope); err != nil {
			t.Errorf("rejected read pipeline %s: %v", body, err)
		}
	}
	hostile := []string{
		`{"pipeline":[{"$out":"other"}]}`,
		`{"pipeline":[{"$facet":{"branch":[{"$merge":"docs"}]}}]}`,
		`{"pipeline":[{"$lookup":{"from":"other","as":"x"}}]}`,
		`{"pipeline":[{"$lookup":{"from":"sources","pipeline":[{"$out":"docs"}],"as":"x"}}]}`,
		`{"pipeline":[{"$unionWith":{"coll":"other","pipeline":[{"$match":{}}]}}]}`,
		`{"pipeline":[{"$graphLookup":{"from":"other","startWith":"$_id","connectFromField":"a","connectToField":"b","as":"x"}}]}`,
		`{"pipeline":[{"$currentOp":{}}]}`,
		`{"pipeline":[{"$search":{"text":{"query":"x","path":"title"}}}]}`,
		`{"pipeline":[{"$vectorSearch":{"index":"v","queryVector":[1],"path":"v","numCandidates":10,"limit":1}}]}`,
		`{"pipeline":[{"$project":{"x":{"$function":{"body":"return 1","args":[],"lang":"js"}}}}]}`,
		`{"pipeline":[{"$match":{"$where":"return true"}}]}`,
		`{"pipeline":[{"$match":{},"$out":"other"}]}`,
		`{"pipeline":[{"$match":{}},{"$match":{}}],"limit":101}`,
		`{"pipeline":[{"$match":{"a":1,"a":2}}]}`,
	}
	for _, body := range hostile {
		if _, err := validateRequest("aggregate", []byte(body), limits, cfg, scope); err == nil {
			t.Errorf("accepted hostile pipeline %s", body)
		}
	}
	if _, err := validateRequest("find", []byte(`{"filter":{"_id":{"$oid":"507f1f77bcf86cd799439011"}},"sort":{"created_at":-1},"limit":5}`), limits, cfg, scope); err != nil {
		t.Fatal(err)
	}
	if _, err := validateRequest("find", []byte(`{"filter":{"$where":"return true"}}`), limits, cfg, scope); err == nil {
		t.Fatal("accepted JavaScript filter")
	}
}

func TestMongoDBPrivilegeProof(t *testing.T) {
	target := &Target{cfg: &config.TargetConfig{Database: "agent_data", Username: "agent_ro", MongoDB: &config.MongoDBConfig{AuthSource: "admin", Collections: []string{"docs"}}}, allowed: map[string]bool{"docs": true}}
	proofFrom := func(resource bson.D, actions ...string) authority {
		raw, _ := bson.Marshal(bson.D{{Key: "authInfo", Value: bson.D{{Key: "authenticatedUsers", Value: bson.A{bson.D{{Key: "user", Value: "agent_ro"}, {Key: "db", Value: "admin"}}}}, {Key: "authenticatedUserPrivileges", Value: bson.A{bson.D{{Key: "resource", Value: resource}, {Key: "actions", Value: actions}}}}}}})
		var result authority
		if err := bson.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if err := target.validateAuthority(proofFrom(bson.D{{Key: "db", Value: "agent_data"}, {Key: "collection", Value: "docs"}}, "find", "listIndexes")); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		resource bson.D
		actions  []string
	}{
		{bson.D{{Key: "db", Value: "agent_data"}, {Key: "collection", Value: ""}}, []string{"find"}},
		{bson.D{{Key: "db", Value: "other"}, {Key: "collection", Value: "docs"}}, []string{"find"}},
		{bson.D{{Key: "db", Value: "agent_data"}, {Key: "collection", Value: "docs"}}, []string{"find", "insert"}},
		{bson.D{{Key: "db", Value: "agent_data"}, {Key: "collection", Value: "docs"}}, []string{"find"}},
	} {
		if err := target.validateAuthority(proofFrom(test.resource, test.actions...)); err == nil {
			t.Errorf("accepted broad/mutating/incomplete grant %+v", test)
		}
	}
}

func TestMongoDBCanonicalResultBound(t *testing.T) {
	cursor, err := mongo.NewCursorFromDocuments([]any{bson.D{{Key: "n", Value: int64(1 << 54)}}, bson.D{{Key: "n", Value: int64(2)}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	target := &Target{limits: config.Limits{MaxResultBytes: 1024}}
	raw, count, truncated, err := target.collect(context.Background(), cursor, 1)
	if err != nil || count != 1 || !truncated || !strings.Contains(string(raw), `"$numberLong":"18014398509481984"`) {
		t.Fatalf("canonical result: %s count=%d truncated=%v err=%v", raw, count, truncated, err)
	}
	if !json.Valid(raw) {
		t.Fatal("invalid result JSON")
	}
	cursor, err = mongo.NewCursorFromDocuments([]any{bson.D{{Key: "large", Value: strings.Repeat("x", 2048)}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := target.collect(context.Background(), cursor, 1); err == nil {
		t.Fatal("oversized result accepted")
	}
}
