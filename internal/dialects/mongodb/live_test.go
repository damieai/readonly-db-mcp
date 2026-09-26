package mongodb

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Opt-in P2 qualification against a disposable authenticated MongoDB 8.0
// fixture. The attempted update uses a new, absent ObjectID and is safe even
// when a misconfigured credential unexpectedly has write authority.
func TestMongoDBLiveReadOnlyGate(t *testing.T) {
	host, user, password, version := os.Getenv("MONGODB_LIVE_HOST"), os.Getenv("MONGODB_LIVE_USER"), os.Getenv("MONGODB_LIVE_PASSWORD"), os.Getenv("MONGODB_LIVE_VERSION")
	if host == "" || user == "" || password == "" || version == "" {
		t.Skip("set MONGODB_LIVE_HOST, MONGODB_LIVE_USER, MONGODB_LIVE_PASSWORD and MONGODB_LIVE_VERSION")
	}
	port := 27017
	if text := os.Getenv("MONGODB_LIVE_PORT"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil {
			t.Fatal(err)
		}
		port = n
	}
	database := os.Getenv("MONGODB_LIVE_DB")
	if database == "" {
		database = "agent_data"
	}
	collection := os.Getenv("MONGODB_LIVE_COLLECTION")
	if collection == "" {
		collection = "docs"
	}
	authSource := os.Getenv("MONGODB_LIVE_AUTH_SOURCE")
	if authSource == "" {
		authSource = "admin"
	}
	mode := config.TLSVerifyFull
	if host == "127.0.0.1" || host == "localhost" {
		mode = config.TLSDisabled
	}
	ca := os.Getenv("MONGODB_LIVE_CA_FILE")
	if mode == config.TLSVerifyFull && ca == "" {
		t.Fatal("MONGODB_LIVE_CA_FILE is required for remote TLS")
	}
	cfg := &config.TargetConfig{Name: "mongo-live", Engine: config.EngineMongoDB, Environment: "test", Consistency: config.ConsistencyEventual, Host: host, Port: port, Database: database, Username: user, PasswordEnv: "MONGODB_LIVE_PASSWORD", TLS: config.TLSConfig{Mode: mode, CAFile: ca}, Connection: config.ConnectionConfig{ConnectTimeout: 5 * time.Second, ReadTimeout: time.Minute, WriteTimeout: 5 * time.Second, MaxOpen: 2, MaxIdle: 1, MaxLifetime: time.Minute, MaxIdleTime: time.Minute}, MongoDB: &config.MongoDBConfig{Version: version, AuthSource: authSource, Collections: []string{collection}, PrivilegeRecheck: time.Minute, MaxRequestBytes: 1 << 20, MaxJSONDepth: 128, MaxJSONNodes: 100000, MaxPipelineStages: 100}}
	limits := config.Limits{DefaultTimeout: 30 * time.Second, MaxTimeout: time.Minute, MaxRows: 100, MaxResultBytes: 1 << 20}
	controller := admission.New(admission.Config{Global: 4, PerTarget: 2, MaxQueued: 8, QueueTimeout: time.Second, BatchMax: 2, MaintenanceMax: 2})
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	for _, request := range []core.MongoDBRequest{
		{Collection: collection, Operation: "metadata"},
		{Collection: collection, Operation: "find", Body: []byte(`{"filter":{"tenant":"team-a"},"limit":5}`)},
		{Collection: collection, Operation: "count", Body: []byte(`{"filter":{}}`)},
		{Collection: collection, Operation: "aggregate", Body: []byte(`{"pipeline":[{"$match":{"tenant":"team-a"}},{"$lookup":{"from":"` + collection + `","localField":"_id","foreignField":"_id","as":"self"}}],"limit":5}`)},
	} {
		if _, err := target.MongoDBRead(context.Background(), request); err != nil {
			t.Fatalf("live %s: %v", request.Operation, err)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := target.MongoDBRead(canceled, core.MongoDBRequest{Collection: collection, Operation: "find", Body: []byte(`{"filter":{}}`)}); err == nil {
		t.Fatal("cancelled MongoDB read succeeded")
	}
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_, err = target.database.Collection(collection).UpdateOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}}, bson.D{{Key: "$set", Value: bson.D{{Key: "p2_probe", Value: true}}}})
	var denied mongo.CommandError
	if err == nil || !errors.As(err, &denied) || denied.Code != 13 {
		t.Fatalf("native update did not fail with Unauthorized (13): %v", err)
	}
	err = target.database.Collection("private").FindOne(ctx, bson.D{}).Err()
	if err == nil || !errors.As(err, &denied) || denied.Code != 13 {
		t.Fatalf("out-of-scope read did not fail with Unauthorized (13): %v", err)
	}
}
