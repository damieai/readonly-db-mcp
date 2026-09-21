package redis

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func waitFixture(t *testing.T, description string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatal(description)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func copyFixtureACL(t *testing.T, admin *redisdriver.Client, cfg *config.TargetConfig) {
	t.Helper()
	args := []any{"ACL", "SETUSER", cfg.Username, "reset", "on", ">" + os.Getenv(cfg.PasswordEnv), "%R~tenant:*", "-@all", "+@read", "+ping", "+info", "+command", "+acl|getuser", "+acl|whoami", "+module|list", "+role", "+cluster|slots", "+readonly"}
	if err := admin.Do(context.Background(), args...).Err(); err != nil {
		t.Fatal("copy fixture reader ACL")
	}
}

func TestLocalRedisSentinelFailover(t *testing.T) {
	ctx := context.Background()
	primary, adminPassword, port := startLocalRedis(t, "", false)
	replica, _, replicaPort := startLocalRedis(t, fmt.Sprintf("replicaof 127.0.0.1 %d\nmasterauth %s", port, adminPassword), false, adminPassword)
	cfg, limits, controller := localRedisConfig(t, primary, port)
	copyFixtureACL(t, replica, cfg)
	// Built-in module inventory is identical because every fixture uses the
	// same executable. Its signed profile is also rechecked on promotion.
	writeBuiltinFixtureProfiles(t, primary, cfg)
	if err := primary.Set(ctx, "tenant:sentinel", "stable", 0).Err(); err != nil {
		t.Fatal(err)
	}
	waitFixture(t, "replication did not catch up", func() bool { return replica.Get(ctx, "tenant:sentinel").Val() == "stable" })
	settings := fmt.Sprintf("sentinel monitor fixture 127.0.0.1 %d 2\nsentinel auth-pass fixture %s\nsentinel down-after-milliseconds fixture 2000\nsentinel failover-timeout fixture 10000", port, adminPassword)
	sentinel1, sentinelPassword, sentinelPort1 := startLocalRedis(t, settings, true)
	_, _, sentinelPort2 := startLocalRedis(t, settings, true, sentinelPassword)
	t.Setenv("READONLY_SENTINEL_FIXTURE_PASSWORD", sentinelPassword)
	cfg.Redis.Mode = "sentinel"
	cfg.Redis.Sentinel = config.RedisSentinelConfig{ServiceName: "fixture", Addresses: []string{net.JoinHostPort("127.0.0.1", strconv.Itoa(sentinelPort1)), net.JoinHostPort("127.0.0.1", strconv.Itoa(sentinelPort2))}, Username: "default", PasswordEnv: "READONLY_SENTINEL_FIXTURE_PASSWORD", MinAgreement: 2, ReadRole: "primary", DiscoveryTimeout: 5 * time.Second, RefreshInterval: time.Second, EndpointAllowlist: config.RedisEndpointAllowlist{CIDRs: []string{"127.0.0.0/8"}}}
	waitFixture(t, "Sentinels did not discover replica", func() bool {
		r, err := sentinel1.Do(ctx, "SENTINEL", "REPLICAS", "fixture").Slice()
		if err != nil || len(r) == 0 {
			return false
		}
		fields, ok := stringMap(r[0])
		if !ok {
			return false
		}
		return fmt.Sprint(fields["master-link-status"]) == "ok" && !strings.Contains(fmt.Sprint(fields["flags"]), "down") && !strings.Contains(fmt.Sprint(fields["flags"]), "disconnected")
	})
	target, err := Open(ctx, cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	request := core.RedisRequest{Command: "GET", Arguments: []core.RedisArgument{arg("tenant:sentinel")}}
	if _, err := target.RedisCommand(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := sentinel1.Do(ctx, "SENTINEL", "FAILOVER", "fixture").Err(); err != nil {
		t.Fatal(err)
	}
	waitFixture(t, "Sentinel did not promote replica", func() bool {
		v, err := sentinel1.Do(ctx, "SENTINEL", "GET-MASTER-ADDR-BY-NAME", "fixture").StringSlice()
		return err == nil && len(v) == 2 && v[1] == strconv.Itoa(replicaPort)
	})
	waitFixture(t, "target did not attest and adopt new primary", func() bool {
		if !target.Info().Healthy {
			return false
		}
		_, err := target.RedisCommand(ctx, request)
		return err == nil && func() bool {
			target.clientMu.RLock()
			defer target.clientMu.RUnlock()
			client, ok := target.client.(*redisdriver.Client)
			return ok && client.Options().Addr == net.JoinHostPort("127.0.0.1", strconv.Itoa(replicaPort))
		}()
	})
	if err := target.client.Set(ctx, "tenant:sentinel", "must-not-write", 0).Err(); err == nil {
		t.Fatal("promoted runtime can write")
	}
}

func TestLocalRedisClusterAllSlots(t *testing.T) {
	if os.Getenv("READONLY_DB_MCP_REDIS_SERVER") == "" {
		t.Skip("READONLY_DB_MCP_REDIS_SERVER is not configured")
	}
	ctx := context.Background()
	var nodes []*redisdriver.Client
	var ports []int
	var busPorts []int
	for i := 0; i < 3; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		busPort := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		node, _, port := startLocalRedis(t, fmt.Sprintf("cluster-enabled yes\ncluster-config-file nodes.conf\ncluster-node-timeout 1000\ncluster-port %d", busPort), false)
		nodes = append(nodes, node)
		ports = append(ports, port)
		busPorts = append(busPorts, busPort)
	}
	cfg, limits, controller := localRedisConfig(t, nodes[0], ports[0])
	writeBuiltinFixtureProfiles(t, nodes[0], cfg)
	for i, node := range nodes {
		if i > 0 {
			copyFixtureACL(t, node, cfg)
		}
		if err := node.ClusterAddSlotsRange(ctx, i*16384/3, (i+1)*16384/3-1).Err(); err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			if err := nodes[0].Do(ctx, "CLUSTER", "MEET", "127.0.0.1", ports[i], busPorts[i]).Err(); err != nil {
				t.Fatal(err)
			}
		}
	}
	waitFixture(t, "cluster did not cover all slots", func() bool {
		for _, node := range nodes {
			info, err := node.ClusterInfo(ctx).Result()
			if err != nil || !strings.Contains(info, "cluster_state:ok") {
				return false
			}
		}
		return true
	})
	cfg.Redis.Mode = "cluster"
	full := true
	cfg.Redis.Cluster = config.RedisClusterConfig{SeedAddresses: []string{nodes[0].Options().Addr}, ReadRole: "primary", TopologyRefresh: time.Second, TopologyMaxAge: 10 * time.Second, RedirectLimit: 3, RequireFullSlotCoverage: &full, EndpointAllowlist: config.RedisEndpointAllowlist{CIDRs: []string{"127.0.0.0/8"}}}
	target, err := Open(ctx, cfg, limits, controller, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	seen := map[int]bool{}
	for i := 0; len(seen) < 3 && i < 10000; i++ {
		key := fmt.Sprintf("tenant:cluster:%d", i)
		slot := redisKeySlot(key)
		node := slot * 3 / 16384
		// Integer range boundaries use floor(i*16384/3), so resolve exactly.
		for n := 0; n < 3; n++ {
			if slot >= n*16384/3 && slot < (n+1)*16384/3 {
				node = n
			}
		}
		if seen[node] {
			continue
		}
		seen[node] = true
		if err := nodes[node].Set(ctx, key, "stable", 0).Err(); err != nil {
			t.Fatal(err)
		}
		if _, err := target.RedisCommand(ctx, core.RedisRequest{Command: "GET", Arguments: []core.RedisArgument{arg(key)}}); err != nil {
			t.Fatal(err)
		}
		if err := target.client.Set(ctx, key, "must-not-write", 0).Err(); err == nil {
			t.Fatal("cluster runtime can write")
		}
	}
	if len(seen) != 3 {
		t.Fatal("did not exercise every primary")
	}
}
