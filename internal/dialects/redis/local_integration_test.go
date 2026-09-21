package redis

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	modulepolicy "github.com/your-org/readonly-db-mcp/internal/dialects/redis/modules"
)

// These tests own every process/account/dataset they mutate. They never connect
// to an operator database and do not require Docker or fixed TCP ports.
func startLocalRedis(t *testing.T, extra string, sentinel bool, secrets ...string) (*redisdriver.Client, string, int) {
	t.Helper()
	binary := os.Getenv("READONLY_DB_MCP_REDIS_SERVER")
	if binary == "" {
		t.Skip("READONLY_DB_MCP_REDIS_SERVER is not configured")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	password := make([]byte, 24)
	if _, err := rand.Read(password); err != nil {
		t.Fatal(err)
	}
	secret := base64.RawURLEncoding.EncodeToString(password)
	if len(secrets) > 0 {
		secret = secrets[0]
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "redis.conf")
	configuration := fmt.Sprintf("bind 127.0.0.1\nport %d\nprotected-mode yes\nrequirepass %s\ndir %s\n", port, secret, dir)
	if !sentinel {
		configuration += "save \"\"\nappendonly no\n"
	}
	configuration += extra + "\n"
	if err := os.WriteFile(path, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{path}
	if sentinel {
		args = append(args, "--sentinel")
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Redis fixture failed to exit")
		}
	})
	client := redisdriver.NewClient(&redisdriver.Options{Addr: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), Password: secret, DisableIdentity: true, MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	deadline := time.Now().Add(10 * time.Second)
	for client.Ping(context.Background()).Err() != nil {
		select {
		case <-done:
			t.Fatal("Redis fixture exited during startup")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Redis fixture did not start")
		}
		time.Sleep(25 * time.Millisecond)
	}
	return client, secret, port
}

func localRedisConfig(t *testing.T, admin *redisdriver.Client, port int) (*config.TargetConfig, config.Limits, *admission.Controller) {
	t.Helper()
	secretBytes := make([]byte, 24)
	_, _ = rand.Read(secretBytes)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	t.Setenv("READONLY_REDIS_FIXTURE_PASSWORD", secret)
	args := []any{"ACL", "SETUSER", "mcp_reader", "reset", "on", ">" + secret, "%R~tenant:*", "-@all", "+@read", "+ping", "+info", "+command", "+acl|getuser", "+acl|whoami", "+module|list", "+role", "+cluster|slots", "+readonly"}
	if err := admin.Do(context.Background(), args...).Err(); err != nil {
		t.Fatal("provision fixture reader ACL")
	}
	cfg := &config.TargetConfig{Name: "local-fixture", Engine: config.EngineRedis, Environment: "test", Consistency: config.ConsistencyCurrent, Host: "127.0.0.1", Port: port, Username: "mcp_reader", PasswordEnv: "READONLY_REDIS_FIXTURE_PASSWORD", TLS: config.TLSConfig{Mode: config.TLSDisabled, AllowInsecureRemote: true},
		Connection: config.ConnectionConfig{ConnectTimeout: 15 * time.Second, ReadTimeout: 11 * time.Minute, WriteTimeout: 15 * time.Second, MaxOpen: 8, MaxIdle: 4, MaxLifetime: 30 * time.Minute, MaxIdleTime: 10 * time.Minute},
		Redis:      config.RedisConfig{Mode: "standalone", KeyPatterns: []string{"tenant:*"}, Protocol: 3, ACLRecheck: time.Minute, CatalogMaxAge: 2 * time.Minute, MaxScriptBytes: 64 << 10, MaxKeysPerCommand: 1024, MaxArgumentBytes: 1 << 20, MaxReplyDepth: 64, MaxReplyElements: 10000}}
	limits := config.Limits{GlobalConcurrency: 16, PerTargetConcurrency: 8, DefaultTimeout: 5 * time.Minute, MaxTimeout: 10 * time.Minute, MaxResultBytes: 8 << 20, MaxCellBytes: 1 << 20, MaxBatchQueries: 50, MaxQueuedRequests: 256, QueueTimeout: time.Minute}
	controller := admission.New(admission.Config{Global: 16, PerTarget: 8, MaxQueued: 256, QueueTimeout: time.Minute, BatchMax: 4, MaintenanceMax: 2})
	return cfg, limits, controller
}

func TestLocalRedisNativeReadOnlyAndRecovery(t *testing.T) {
	admin, _, port := startLocalRedis(t, "", false)
	cfg, limits, controller := localRedisConfig(t, admin, port)
	writeBuiltinFixtureProfiles(t, admin, cfg)
	ctx := context.Background()
	if err := admin.Set(ctx, "tenant:fixture", "before", 0).Err(); err != nil {
		t.Fatal(err)
	}
	target, err := Open(ctx, cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	read := core.RedisRequest{Command: "GET", Arguments: []core.RedisArgument{arg("tenant:fixture")}}
	if _, err := target.RedisCommand(ctx, read); err != nil {
		t.Fatal(err)
	}
	for _, request := range []core.RedisRequest{
		{Command: "SET", Arguments: []core.RedisArgument{arg("tenant:fixture"), arg("after")}},
		{Command: "GET", Arguments: []core.RedisArgument{arg("private:fixture")}},
	} {
		if _, err := target.RedisCommand(ctx, request); err == nil {
			t.Fatal("hostile request accepted")
		}
	}
	// Deliberately bypass application validation to prove the server ACL itself.
	if err := target.client.Set(ctx, "tenant:fixture", "after", 0).Err(); err == nil {
		t.Fatal("runtime identity can write")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := target.RedisCommand(cancelled, read); err == nil {
		t.Fatal("cancelled request accepted")
	}
	if _, err := target.RedisCommand(ctx, read); err != nil {
		t.Fatal("permit/pool did not recover")
	}
	if value, err := admin.Get(ctx, "tenant:fixture").Result(); err != nil || value != "before" {
		t.Fatal("fixture was modified")
	}
}

func writeBuiltinFixtureProfiles(t *testing.T, admin *redisdriver.Client, cfg *config.TargetConfig) {
	t.Helper()
	ctx := context.Background()
	raw, err := admin.Do(ctx, "MODULE", "LIST").Result()
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := parseModuleInventory(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory) == 0 {
		return
	}
	artifact, err := filepath.EvalSymlinks(os.Getenv("READONLY_DB_MCP_REDIS_SERVER"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	info, err := admin.Info(ctx, "server").Result()
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Redis.TrustedProfileKeys = map[string]string{"fixture": base64.StdEncoding.EncodeToString(public)}
	for name, identity := range inventory {
		if identity.Path != "" {
			t.Fatal("native fixture expects only built-in modules")
		}
		commands, err := admin.CommandList(ctx, &redisdriver.FilterBy{Module: name}).Result()
		if err != nil {
			t.Fatal(err)
		}
		profile := modulepolicy.Profile{ProfileVersion: 1, Module: modulepolicy.ModuleIdentity{Name: name, Version: identity.Version, Builtin: true, RedisCompatibility: []string{infoField(info, "redis_version")}, ArtifactPath: artifact, ArtifactSHA256: fmt.Sprintf("%x", sum), VendorBuildID: infoField(info, "redis_build_id")}, Commands: map[string]modulepolicy.CommandRule{}, IssuedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
		// This fixture does not certify built-in module commands; it proves that
		// an explicitly signed inventory permits ordinary GET/ACL operation.
		for _, command := range commands {
			profile.Commands[strings.ToUpper(command)] = modulepolicy.CommandRule{KeyModel: "ordinary-keys"}
		}
		payload, _ := json.Marshal(profile)
		envelope, _ := json.Marshal(modulepolicy.Envelope{Payload: base64.StdEncoding.EncodeToString(payload), KeyID: "fixture", Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))})
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, envelope, 0600); err != nil {
			t.Fatal(err)
		}
		cfg.Redis.ModuleProfiles = append(cfg.Redis.ModuleProfiles, path)
	}
}

func TestLocalRedisSearchSignedProfileAndPrefixDrift(t *testing.T) {
	artifact := os.Getenv("READONLY_DB_MCP_REDIS_SEARCH_MODULE")
	if artifact == "" {
		t.Skip("READONLY_DB_MCP_REDIS_SEARCH_MODULE is not configured")
	}
	if strings.ContainsAny(artifact, "\r\n\"") {
		t.Fatal("invalid module fixture path")
	}
	admin, _, port := startLocalRedis(t, "loadmodule \""+artifact+"\"", false)
	cfg, limits, controller := localRedisConfig(t, admin, port)
	ctx := context.Background()
	for _, command := range []string{"FT.SEARCH", "FT.AGGREGATE", "FT.INFO", "FT._LIST"} {
		if err := admin.Do(ctx, "ACL", "SETUSER", cfg.Username, "+"+command).Err(); err != nil {
			t.Fatal(err)
		}
	}
	if err := admin.Do(ctx, "FT.CREATE", "search-index", "ON", "HASH", "PREFIX", "1", "tenant:", "SCHEMA", "region", "TAG", "amount", "NUMERIC").Err(); err != nil {
		t.Fatal(err)
	}
	if err := admin.HSet(ctx, "tenant:one", "region", "east", "amount", "10").Err(); err != nil {
		t.Fatal(err)
	}
	writeSearchFixtureProfile(t, admin, cfg, artifact)
	if err := admin.Do(ctx, "ACL", "SETUSER", cfg.Username, "~search-*").Err(); err != nil {
		t.Fatal(err)
	}
	reader := redisdriver.NewClient(&redisdriver.Options{Addr: admin.Options().Addr, Username: cfg.Username, Password: os.Getenv(cfg.PasswordEnv), Protocol: 3, DisableIdentity: true})
	t.Cleanup(func() { _ = reader.Close() })
	if err := reader.Do(ctx, "FT.INFO", "search-index").Err(); err != nil {
		t.Fatalf("fixture FT.INFO under reader ACL: %v", err)
	}
	for _, command := range []string{"FT.SEARCH", "FT.AGGREGATE", "FT.INFO"} {
		cfg.Redis.ModuleObjectPatterns[command] = []string{"search-*"}
	}
	target, err := Open(ctx, cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	request := core.RedisRequest{Command: "FT.AGGREGATE", Arguments: []core.RedisArgument{arg("search-index"), arg("*"), arg("GROUPBY"), arg("1"), arg("@region"), arg("REDUCE"), arg("SUM"), arg("1"), arg("@amount"), arg("AS"), arg("total")}}
	if _, err := target.RedisCommand(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := target.client.Do(ctx, "FT.DROPINDEX", "search-index").Err(); err == nil {
		t.Fatal("runtime can delete an index")
	}
	if err := admin.Do(ctx, "FT.DROPINDEX", "search-index").Err(); err != nil {
		t.Fatal(err)
	}
	if err := admin.Do(ctx, "FT.CREATE", "search-index", "ON", "HASH", "PREFIX", "1", "private:", "SCHEMA", "region", "TAG").Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := target.RedisCommand(ctx, request); err == nil {
		t.Fatal("changed index prefix accepted")
	}
}

func writeSearchFixtureProfile(t *testing.T, admin *redisdriver.Client, cfg *config.TargetConfig, artifact string) {
	t.Helper()
	ctx := context.Background()
	raw, err := admin.Do(ctx, "MODULE", "LIST").Result()
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := parseModuleInventory(raw)
	if err != nil {
		t.Fatal(err)
	}
	identity, ok := inventory["search"]
	if !ok || len(inventory) != 1 {
		t.Fatal("fixture must load exactly the Search module")
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	live, err := admin.CommandList(ctx, &redisdriver.FilterBy{Module: "search"}).Result()
	if err != nil {
		t.Fatal(err)
	}
	profile := modulepolicy.Profile{ProfileVersion: 1, Module: modulepolicy.ModuleIdentity{Name: "search", Version: identity.Version, RedisCompatibility: []string{"7.4", "8.0"}, ArtifactPath: artifact, ArtifactSHA256: fmt.Sprintf("%x", sum), VendorBuildID: "disposable-test-artifact"}, Commands: map[string]modulepolicy.CommandRule{}, IssuedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
	for _, name := range live {
		rule := modulepolicy.CommandRule{KeyModel: "keyless-safe"}
		if modulepolicy.SearchIndexCommand(name) {
			rule.ReadOnly = true
			rule.KeyModel = "index-prefix-attested"
		}
		profile.Commands[strings.ToUpper(name)] = rule
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(profile)
	envelope, _ := json.Marshal(modulepolicy.Envelope{Payload: base64.StdEncoding.EncodeToString(payload), KeyID: "fixture", Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))})
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, envelope, 0600); err != nil {
		t.Fatal(err)
	}
	cfg.Redis.ModuleProfiles = []string{path}
	cfg.Redis.TrustedProfileKeys = map[string]string{"fixture": base64.StdEncoding.EncodeToString(public)}
	cfg.Redis.ModuleObjectPatterns = map[string][]string{}
}
