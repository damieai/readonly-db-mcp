package redis

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func TestRedisIntegrationReadAndWriteBoundary(t *testing.T) {
	address := os.Getenv("READONLY_DB_MCP_REDIS_TEST_ADDR")
	if address == "" {
		t.Skip("READONLY_DB_MCP_REDIS_TEST_ADDR is not configured")
	}
	host, portText, ok := strings.Cut(address, ":")
	if !ok {
		t.Fatal("test address must be host:port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.TargetConfig{Name: "redis-integration", Engine: config.EngineRedis, Environment: "test", Consistency: config.ConsistencyCurrent, Host: host, Port: port, Username: os.Getenv("READONLY_DB_MCP_REDIS_TEST_USER"), PasswordEnv: "READONLY_DB_MCP_REDIS_TEST_PASSWORD", TLS: config.TLSConfig{Mode: config.TLSDisabled, AllowInsecureRemote: true}, Connection: config.ConnectionConfig{ConnectTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 3 * time.Second, MaxOpen: 2, MaxIdle: 1, MaxLifetime: 3 * time.Minute, MaxIdleTime: time.Minute}, Redis: config.RedisConfig{Mode: "standalone", KeyPatterns: []string{"mcp-test:*"}, Protocol: 3, ACLRecheck: 5 * time.Minute, CatalogMaxAge: 10 * time.Minute, AllowReadonlyScripts: true, MaxScriptBytes: 64 << 10, MaxKeysPerCommand: 32, MaxArgumentBytes: 64 << 10, MaxReplyDepth: 16, MaxReplyElements: 1000}}
	limits := config.Limits{GlobalConcurrency: 2, PerTargetConcurrency: 1, DefaultTimeout: 2 * time.Second, MaxTimeout: 5 * time.Second, MaxResultBytes: 1 << 20, MaxCellBytes: 64 << 10, MaxBatchQueries: 8, MaxQueuedRequests: 8, QueueTimeout: time.Second}
	controller := admission.New(admission.Config{Global: 2, PerTarget: 1, MaxQueued: 8, QueueTimeout: time.Second, BatchMax: 1, MaintenanceMax: 1})
	target, err := Open(context.Background(), cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	result, err := target.RedisCommand(context.Background(), core.RedisRequest{Command: "GET", Arguments: []core.RedisArgument{arg("mcp-test:fixture")}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "GET" {
		t.Fatalf("command=%s", result.Command)
	}
	if _, err := target.RedisCommand(context.Background(), core.RedisRequest{Command: "SET", Arguments: []core.RedisArgument{arg("mcp-test:forbidden"), arg("value")}}); err == nil {
		t.Fatal("write command passed application policy")
	}
}
