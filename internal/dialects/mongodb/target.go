package mongodb

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type Target struct {
	cfg        *config.TargetConfig
	limits     config.Limits
	client     *mongo.Client
	database   *mongo.Database
	allowed    map[string]bool
	controller *admission.Controller
	auditor    audit.Auditor
	recorder   metrics.Recorder
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	checked    time.Time
	wg         sync.WaitGroup
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	if cfg == nil || cfg.MongoDB == nil {
		return nil, errors.New("configuration_error: MongoDB target configuration is required")
	}
	password, err := cfg.Password()
	if err != nil {
		return nil, errors.New("configuration_error: cannot read MongoDB password")
	}
	clientOptions := options.Client().SetHosts([]string{net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))}).SetDirect(true).
		SetAuth(options.Credential{AuthMechanism: "SCRAM-SHA-256", AuthSource: cfg.MongoDB.AuthSource, Username: cfg.Username, Password: password, PasswordSet: true}).
		SetConnectTimeout(cfg.Connection.ConnectTimeout).SetServerSelectionTimeout(cfg.Connection.ConnectTimeout).
		SetTimeout(limits.MaxTimeout).
		SetMaxPoolSize(uint64(cfg.Connection.MaxOpen)).SetMinPoolSize(0).
		SetMaxConnIdleTime(min(cfg.Connection.MaxIdleTime, cfg.Connection.MaxLifetime)).
		SetRetryReads(false).SetRetryWrites(false).SetMaxAdaptiveRetries(0).SetEnableOverloadRetargeting(false).
		SetReadPreference(readpref.Nearest()).SetAppName("readonly-db-mcp")
	if cfg.TLS.Mode == config.TLSVerifyFull {
		pem, err := os.ReadFile(cfg.TLS.CAFile)
		if err != nil {
			return nil, errors.New("configuration_error: cannot read MongoDB CA")
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("configuration_error: invalid MongoDB CA")
		}
		clientOptions.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: cfg.Host})
	}
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, errors.New("configuration_error: cannot initialize MongoDB client")
	}
	owned, cancel := context.WithCancel(context.Background())
	t := &Target{cfg: cfg, limits: limits, client: client, database: client.Database(cfg.Database), allowed: map[string]bool{}, controller: controller, auditor: auditor, recorder: recorder, ctx: owned, cancel: cancel}
	for _, name := range cfg.MongoDB.Collections {
		t.allowed[name] = true
	}
	if err := t.prove(ctx); err != nil {
		cancel()
		cleanup, c := context.WithTimeout(context.Background(), time.Second)
		_ = client.Disconnect(cleanup)
		c()
		return nil, fmt.Errorf("MongoDB target %q startup proof: %w", cfg.Name, err)
	}
	t.wg.Add(1)
	go t.maintenance()
	return t, nil
}

type authority struct {
	AuthInfo struct {
		Users []struct {
			User string `bson:"user"`
			DB   string `bson:"db"`
		} `bson:"authenticatedUsers"`
		Privileges []struct {
			Resource struct {
				DB          string `bson:"db"`
				Collection  string `bson:"collection"`
				AnyResource bool   `bson:"anyResource"`
				Cluster     bool   `bson:"cluster"`
			} `bson:"resource"`
			Actions []string `bson:"actions"`
		} `bson:"authenticatedUserPrivileges"`
	} `bson:"authInfo"`
}

func (t *Target) prove(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, t.cfg.Connection.ConnectTimeout+30*time.Second)
	defer cancel()
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, admission.Maintenance)
	if err != nil {
		return err
	}
	defer permit.Release()
	if err := t.client.Ping(ctx, nil); err != nil {
		return errors.New("transport_error: MongoDB ping or authentication failed")
	}
	var build struct {
		Version string `bson:"version"`
	}
	if err := t.database.RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&build); err != nil {
		return errors.New("permission_denied: MongoDB buildInfo failed")
	}
	if build.Version != t.cfg.MongoDB.Version {
		return errors.New("profile_mismatch: MongoDB server version differs from pinned version")
	}
	var proof authority
	if err := t.database.RunCommand(ctx, bson.D{{Key: "connectionStatus", Value: 1}, {Key: "showPrivileges", Value: true}}).Decode(&proof); err != nil {
		return errors.New("permission_denied: cannot inspect effective MongoDB privileges")
	}
	if err := t.validateAuthority(proof); err != nil {
		return err
	}
	cursor, err := t.database.ListCollections(ctx, bson.D{}, options.ListCollections().SetAuthorizedCollections(true).SetNameOnly(true).SetBatchSize(64))
	if err != nil {
		return errors.New("permission_denied: MongoDB scoped collection catalog failed")
	}
	defer cursor.Close(ctx)
	seen := map[string]bool{}
	for cursor.Next(ctx) {
		var item struct {
			Name string `bson:"name"`
			Type string `bson:"type"`
		}
		if cursor.Decode(&item) != nil || !t.allowed[item.Name] || seen[item.Name] || item.Type != "collection" {
			return errors.New("permission_denied: MongoDB catalog includes an unexpected collection or view")
		}
		seen[item.Name] = true
	}
	if cursor.Err() != nil || len(seen) != len(t.allowed) {
		return errors.New("permission_denied: MongoDB scoped collection catalog is incomplete")
	}
	t.mu.Lock()
	t.checked = time.Now()
	t.mu.Unlock()
	return nil
}

func (t *Target) validateAuthority(proof authority) error {
	if len(proof.AuthInfo.Users) != 1 || proof.AuthInfo.Users[0].User != t.cfg.Username || proof.AuthInfo.Users[0].DB != t.cfg.MongoDB.AuthSource {
		return errors.New("permission_denied: MongoDB authenticated identity differs from configuration")
	}
	grants := map[string]map[string]bool{}
	for _, privilege := range proof.AuthInfo.Privileges {
		resource := privilege.Resource
		if resource.AnyResource || resource.Cluster || resource.DB != t.cfg.Database || !t.allowed[resource.Collection] {
			return errors.New("permission_denied: MongoDB effective privilege exceeds configured collection scope")
		}
		if grants[resource.Collection] == nil {
			grants[resource.Collection] = map[string]bool{}
		}
		for _, action := range privilege.Actions {
			if action != "find" && action != "listIndexes" {
				return fmt.Errorf("permission_denied: MongoDB effective action %q is unavailable", action)
			}
			grants[resource.Collection][action] = true
		}
	}
	for _, name := range t.cfg.MongoDB.Collections {
		if !grants[name]["find"] || !grants[name]["listIndexes"] {
			return fmt.Errorf("permission_denied: MongoDB collection %q lacks find/listIndexes", name)
		}
	}
	return nil
}

func (t *Target) maintenance() {
	defer t.wg.Done()
	ticker := time.NewTicker(t.cfg.MongoDB.PrivilegeRecheck / 2)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			if err := t.prove(t.ctx); err != nil {
				t.mu.Lock()
				t.checked = time.Time{}
				t.mu.Unlock()
				if t.recorder != nil {
					t.recorder.Add("mongodb_attestation", 1, "maintenance", "failed")
				}
			} else if t.recorder != nil {
				t.recorder.Add("mongodb_attestation", 1, "maintenance", "allowed")
			}
		}
	}
}

func (t *Target) ready(ctx context.Context) error {
	select {
	case <-t.ctx.Done():
		return errors.New("target_closed: MongoDB target is closed")
	default:
	}
	t.mu.RLock()
	checked := t.checked
	t.mu.RUnlock()
	if checked.IsZero() || time.Since(checked) > t.cfg.MongoDB.PrivilegeRecheck {
		return t.prove(ctx)
	}
	return nil
}

func (t *Target) MongoDBRead(ctx context.Context, request core.MongoDBRequest) (result *core.MongoDBResult, err error) {
	started := time.Now()
	queryID := make([]byte, 12)
	if _, e := rand.Read(queryID); e != nil {
		return nil, errors.New("internal_error: cannot create MongoDB query ID")
	}
	fingerprint := sha256.Sum256(append([]byte(request.Collection+"|"+request.Operation+"|"), request.Body...))
	defer func() {
		decision, reason := "allowed", ""
		if err != nil {
			decision = "denied"
			reason = err.Error()
		}
		if t.auditor != nil {
			rows, size, truncated := 0, 0, false
			if result != nil {
				rows, size, truncated = result.Count, len(result.Data), result.Truncated
			}
			t.auditor.Record(ctx, audit.Event{QueryID: hex.EncodeToString(queryID), Target: t.cfg.Name, Operation: "mongodb_" + request.Operation, Fingerprint: hex.EncodeToString(fingerprint[:12]), Tables: []string{request.Collection}, Decision: decision, Reason: reason, Rows: rows, Truncated: truncated, ResponseBytes: size, Duration: time.Since(started)})
		}
		if t.recorder != nil {
			t.recorder.Add("query", 1, "mongodb_"+request.Operation, decision)
			t.recorder.Observe("query_duration", time.Since(started), "mongodb_"+request.Operation)
		}
	}()
	if !t.allowed[request.Collection] {
		return nil, errors.New("permission_denied: MongoDB collection is outside target scope")
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout < 0 || timeout > t.limits.MaxTimeout {
		return nil, errors.New("resource_limit: MongoDB timeout exceeds configured maximum")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	unlink := context.AfterFunc(t.ctx, cancel)
	defer unlink()
	parsed, err := validateRequest(request.Operation, request.Body, t.limits, t.cfg.MongoDB, t.allowed)
	if err != nil {
		return nil, err
	}
	if err := t.ready(ctx); err != nil {
		return nil, err
	}
	class := admission.Interactive
	if request.Operation == "metadata" {
		class = admission.Metadata
	}
	permit, err := t.controller.Acquire(ctx, t.cfg.Name, class)
	if err != nil {
		return nil, err
	}
	defer permit.Release()
	var data json.RawMessage
	var count int
	var truncated bool
	switch request.Operation {
	case "metadata":
		data, count, truncated, err = t.metadata(ctx, request.Collection)
	case "find":
		data, count, truncated, err = t.find(ctx, request.Collection, parsed)
	case "count":
		data, err = t.count(ctx, request.Collection, parsed)
	case "aggregate":
		data, count, truncated, err = t.aggregate(ctx, request.Collection, parsed)
	default:
		err = errors.New("invalid_request: unsupported MongoDB operation")
	}
	if err != nil {
		return nil, err
	}
	return &core.MongoDBResult{Target: t.cfg.Name, Engine: config.EngineMongoDB, Database: t.cfg.Database, Collection: request.Collection, Operation: request.Operation, Data: data, Count: count, Truncated: truncated, DurationMS: time.Since(started).Milliseconds()}, nil
}

func extDocument(raw json.RawMessage) (bson.D, error) {
	if len(raw) == 0 {
		return bson.D{}, nil
	}
	var doc bson.D
	if err := bson.UnmarshalExtJSON(raw, false, &doc); err != nil {
		return nil, errors.New("invalid_request: MongoDB Extended JSON document is invalid")
	}
	return doc, nil
}

func extHint(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var name string
	if json.Unmarshal(raw, &name) == nil {
		if name == "" {
			return nil, errors.New("invalid_request: MongoDB hint cannot be empty")
		}
		return name, nil
	}
	return extDocument(raw)
}

func extCollation(raw json.RawMessage) (*options.Collation, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var collation options.Collation
	if json.Unmarshal(raw, &collation) != nil || collation.Locale == "" {
		return nil, errors.New("invalid_request: MongoDB collation needs a locale")
	}
	return &collation, nil
}

func (t *Target) find(ctx context.Context, collection string, request parsedRequest) (json.RawMessage, int, bool, error) {
	f := request.fields
	filter, err := extDocument(f["filter"])
	if err != nil {
		return nil, 0, false, err
	}
	opts := options.Find().SetLimit(int64(request.limit + 1)).SetSkip(int64(request.skip)).SetBatchSize(16).SetAllowDiskUse(false)
	for _, entry := range []struct {
		name string
		set  func(bson.D)
	}{
		{"projection", func(v bson.D) { opts.SetProjection(v) }}, {"sort", func(v bson.D) { opts.SetSort(v) }}, {"let", func(v bson.D) { opts.SetLet(v) }}, {"min", func(v bson.D) { opts.SetMin(v) }}, {"max", func(v bson.D) { opts.SetMax(v) }},
	} {
		if len(f[entry.name]) != 0 {
			doc, e := extDocument(f[entry.name])
			if e != nil {
				return nil, 0, false, e
			}
			entry.set(doc)
		}
	}
	if len(f["hint"]) != 0 {
		hint, e := extHint(f["hint"])
		if e != nil {
			return nil, 0, false, e
		}
		opts.SetHint(hint)
	}
	if len(f["collation"]) != 0 {
		collation, e := extCollation(f["collation"])
		if e != nil {
			return nil, 0, false, e
		}
		opts.SetCollation(collation)
	}
	cursor, err := t.database.Collection(collection).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, false, errors.New("upstream_error: MongoDB find failed")
	}
	return t.collect(ctx, cursor, request.limit)
}

func (t *Target) count(ctx context.Context, collection string, request parsedRequest) (json.RawMessage, error) {
	f := request.fields
	filter, err := extDocument(f["filter"])
	if err != nil {
		return nil, err
	}
	opts := options.Count()
	if len(f["hint"]) != 0 {
		hint, e := extHint(f["hint"])
		if e != nil {
			return nil, e
		}
		opts.SetHint(hint)
	}
	if len(f["collation"]) != 0 {
		collation, e := extCollation(f["collation"])
		if e != nil {
			return nil, e
		}
		opts.SetCollation(collation)
	}
	n, err := t.database.Collection(collection).CountDocuments(ctx, filter, opts)
	if err != nil {
		return nil, errors.New("upstream_error: MongoDB count failed")
	}
	return json.Marshal(map[string]any{"count": map[string]string{"$numberLong": strconv.FormatInt(n, 10)}})
}

func (t *Target) aggregate(ctx context.Context, collection string, request parsedRequest) (json.RawMessage, int, bool, error) {
	f := request.fields
	var pipeline mongo.Pipeline
	if err := bson.UnmarshalExtJSON(f["pipeline"], false, &pipeline); err != nil {
		return nil, 0, false, errors.New("invalid_request: MongoDB Extended JSON pipeline is invalid")
	}
	pipeline = append(pipeline, bson.D{{Key: "$limit", Value: int64(request.limit + 1)}})
	opts := options.Aggregate().SetAllowDiskUse(false).SetBatchSize(16)
	if len(f["hint"]) != 0 {
		hint, e := extHint(f["hint"])
		if e != nil {
			return nil, 0, false, e
		}
		opts.SetHint(hint)
	}
	if len(f["let"]) != 0 {
		let, e := extDocument(f["let"])
		if e != nil {
			return nil, 0, false, e
		}
		opts.SetLet(let)
	}
	if len(f["collation"]) != 0 {
		collation, e := extCollation(f["collation"])
		if e != nil {
			return nil, 0, false, e
		}
		opts.SetCollation(collation)
	}
	cursor, err := t.database.Collection(collection).Aggregate(ctx, pipeline, opts)
	if err != nil {
		return nil, 0, false, errors.New("upstream_error: MongoDB aggregation failed")
	}
	return t.collect(ctx, cursor, request.limit)
}

func (t *Target) metadata(ctx context.Context, collection string) (json.RawMessage, int, bool, error) {
	cursor, err := t.database.Collection(collection).Indexes().List(ctx)
	if err != nil {
		return nil, 0, false, errors.New("upstream_error: MongoDB collection index metadata failed")
	}
	data, count, truncated, err := t.collect(ctx, cursor, t.limits.MaxRows)
	if err != nil {
		return nil, 0, false, err
	}
	var payload struct {
		Documents json.RawMessage `json:"documents"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return nil, 0, false, errors.New("invalid_response: MongoDB index metadata is invalid")
	}
	wrapped, err := json.Marshal(struct {
		Name    string          `json:"name"`
		Type    string          `json:"type"`
		Indexes json.RawMessage `json:"indexes"`
	}{collection, "collection", payload.Documents})
	if err != nil || len(wrapped) > t.limits.MaxResultBytes {
		return nil, 0, false, errors.New("resource_limit: MongoDB index metadata exceeds max_result_bytes")
	}
	return wrapped, count, truncated, nil
}

func (t *Target) collect(ctx context.Context, cursor *mongo.Cursor, limit int) (json.RawMessage, int, bool, error) {
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = cursor.Close(cleanup)
		cancel()
	}()
	var output bytes.Buffer
	output.WriteString(`{"documents":[`)
	count := 0
	truncated := false
	for cursor.Next(ctx) {
		if count >= limit {
			truncated = true
			break
		}
		doc, err := bson.MarshalExtJSON(cursor.Current, true, false)
		if err != nil {
			return nil, 0, false, errors.New("invalid_response: cannot encode MongoDB document")
		}
		if output.Len()+len(doc)+128 > t.limits.MaxResultBytes {
			return nil, 0, false, errors.New("resource_limit: MongoDB result exceeds max_result_bytes")
		}
		if count > 0 {
			output.WriteByte(',')
		}
		output.Write(doc)
		count++
	}
	if cursor.Err() != nil || ctx.Err() != nil {
		return nil, 0, false, errors.New("deadline_exceeded: MongoDB cursor failed or was cancelled")
	}
	output.WriteString(`]}`)
	return json.RawMessage(output.Bytes()), count, truncated, nil
}

func (t *Target) Info() core.TargetInfo {
	t.mu.RLock()
	checked := t.checked
	t.mu.RUnlock()
	healthy := !checked.IsZero() && time.Since(checked) <= t.cfg.MongoDB.PrivilegeRecheck
	proofAt := ""
	if !checked.IsZero() {
		proofAt = checked.UTC().Format(time.RFC3339)
	}
	return core.TargetInfo{Name: t.cfg.Name, Engine: config.EngineMongoDB, Environment: t.cfg.Environment, Consistency: t.cfg.Consistency, Database: t.cfg.Database, Healthy: healthy, ReadOnlyUser: healthy, ServerReadOnly: false, ServerVersion: t.cfg.MongoDB.Version, AllowedCollections: append([]string(nil), t.cfg.MongoDB.Collections...), Capabilities: map[string]string{"document_read": "find,count,metadata", "aggregation": "reviewed_read_stages,scoped_lookups", "vector_search": "requires_separate_versioned_profile"}, ProofCheckedAt: proofAt}
}

func (t *Target) Close() error {
	t.cancel()
	t.wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return t.client.Disconnect(ctx)
}
