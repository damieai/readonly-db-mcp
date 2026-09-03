package redis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
	modulepolicy "github.com/your-org/readonly-db-mcp/internal/dialects/redis/modules"
)

func startupTimeout(cfg *config.TargetConfig) time.Duration {
	if cfg.Redis.Mode == "sentinel" {
		return cfg.Redis.Sentinel.DiscoveryTimeout + time.Duration(len(cfg.Redis.Sentinel.Addresses)+2)*cfg.Connection.ConnectTimeout
	}
	if cfg.Redis.Mode == "cluster" {
		return time.Duration(len(cfg.Redis.Cluster.SeedAddresses)+4) * cfg.Connection.ConnectTimeout
	}
	return cfg.Connection.ConnectTimeout
}

func buildRuntime(ctx context.Context, cfg *config.TargetConfig, salt []byte, profiles *modulepolicy.Set) (redisdriver.UniversalClient, *Policy, string, string, error) {
	if err := profiles.ValidateAt(time.Now()); err != nil {
		return nil, nil, "", "", fmt.Errorf("Redis module profile validity: %w", err)
	}
	password, err := cfg.Password()
	if err != nil {
		return nil, nil, "", "", err
	}
	tlsConfig, err := redisTLS(cfg)
	if err != nil {
		return nil, nil, "", "", err
	}
	switch cfg.Redis.Mode {
	case "sentinel":
		return buildSentinelRuntime(ctx, cfg, password, tlsConfig, salt, profiles)
	case "cluster":
		return buildClusterRuntime(ctx, cfg, password, tlsConfig, salt, profiles)
	default:
		client := redisdriver.NewClient(dataOptions(cfg, fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), password, tlsConfig))
		policy, version, revision, err := attestNode(ctx, client, cfg, salt, profiles, "", "")
		if err != nil {
			_ = client.Close()
			return nil, nil, "", "", err
		}
		return client, policy, version, revision, nil
	}
}

func dataOptions(cfg *config.TargetConfig, address, password string, tlsConfig *tls.Config) *redisdriver.Options {
	return &redisdriver.Options{Addr: address, Username: cfg.Username, Password: password, DB: cfg.Redis.Database, Protocol: cfg.Redis.Protocol,
		DialTimeout: cfg.Connection.ConnectTimeout, ReadTimeout: cfg.Connection.ReadTimeout, WriteTimeout: cfg.Connection.WriteTimeout,
		PoolSize: cfg.Connection.MaxOpen, MinIdleConns: cfg.Connection.MaxIdle, ConnMaxLifetime: cfg.Connection.MaxLifetime,
		ConnMaxIdleTime: cfg.Connection.MaxIdleTime, TLSConfig: tlsConfig, DisableIdentity: true}
}

func attestNode(ctx context.Context, client redisdriver.UniversalClient, cfg *config.TargetConfig, salt []byte, profiles *modulepolicy.Set, role string, expectedPrimary string) (*Policy, string, string, error) {
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, "", "", errors.New("Redis node is unreachable")
	}
	if role != "" {
		raw, err := client.Do(ctx, "ROLE").Result()
		if err != nil {
			return nil, "", "", errors.New("inspect Redis node role")
		}
		values, ok := raw.([]interface{})
		if !ok || len(values) == 0 {
			return nil, "", "", errors.New("Redis node returned an invalid role")
		}
		actual := strings.ToLower(fmt.Sprint(values[0]))
		if actual == "master" {
			actual = "primary"
		}
		if actual == "slave" {
			actual = "replica"
		}
		if actual != role {
			return nil, "", "", fmt.Errorf("Redis node role is %q, expected %q", actual, role)
		}
		if role == "replica" {
			if len(values) < 4 || !strings.EqualFold(fmt.Sprint(values[3]), "connected") {
				return nil, "", "", errors.New("Redis replica master link is not connected")
			}
			master := net.JoinHostPort(fmt.Sprint(values[1]), fmt.Sprint(values[2]))
			if expectedPrimary != "" {
				allow := cfg.Redis.Sentinel.EndpointAllowlist
				if cfg.Redis.Mode == "cluster" {
					allow = cfg.Redis.Cluster.EndpointAllowlist
				}
				pinnedMaster, pinErr := pinEndpoint(ctx, master, allow)
				if pinErr != nil || pinnedMaster != expectedPrimary {
					return nil, "", "", errors.New("Redis replica belongs to a different primary")
				}
			}
		}
	}
	return attest(ctx, client, cfg, salt, profiles)
}

func buildSentinelRuntime(ctx context.Context, cfg *config.TargetConfig, password string, tlsConfig *tls.Config, salt []byte, profiles *modulepolicy.Set) (redisdriver.UniversalClient, *Policy, string, string, error) {
	sentinelPassword, err := cfg.RedisSentinelPassword()
	if err != nil {
		return nil, nil, "", "", err
	}
	primaryAnswers := make([]string, len(cfg.Redis.Sentinel.Addresses))
	replicaAnswers := make([][]string, len(cfg.Redis.Sentinel.Addresses))
	var wg sync.WaitGroup
	for index, address := range cfg.Redis.Sentinel.Addresses {
		wg.Add(1)
		go func(index int, address string) {
			defer wg.Done()
			discoveryCtx, cancel := context.WithTimeout(ctx, cfg.Redis.Sentinel.DiscoveryTimeout)
			defer cancel()
			pinnedSentinel, pinErr := pinEndpoint(discoveryCtx, address, cfg.Redis.Sentinel.EndpointAllowlist)
			if pinErr != nil {
				return
			}
			client := redisdriver.NewSentinelClient(&redisdriver.Options{Addr: pinnedSentinel, Username: cfg.Redis.Sentinel.Username, Password: sentinelPassword, Protocol: cfg.Redis.Protocol, DialTimeout: cfg.Connection.ConnectTimeout, ReadTimeout: cfg.Redis.Sentinel.DiscoveryTimeout, WriteTimeout: cfg.Redis.Sentinel.DiscoveryTimeout, TLSConfig: tlsConfig, DisableIdentity: true})
			defer client.Close()
			master, masterErr := client.Master(discoveryCtx, cfg.Redis.Sentinel.ServiceName).Result()
			if epoch, epochErr := strconv.ParseUint(master["config-epoch"], 10, 64); masterErr == nil && epochErr == nil && master["ip"] != "" && master["port"] != "" {
				primaryAnswers[index] = net.JoinHostPort(master["ip"], master["port"]) + "\x00" + strconv.FormatUint(epoch, 10)
			}
			if cfg.Redis.Sentinel.ReadRole == "primary" {
				return
			}
			values, err := client.Replicas(discoveryCtx, cfg.Redis.Sentinel.ServiceName).Result()
			if err != nil {
				return
			}
			var candidates []string
			for _, value := range values {
				if strings.EqualFold(value["master-link-status"], "ok") || strings.EqualFold(value["master-link-status"], "up") {
					if value["ip"] != "" && value["port"] != "" {
						candidates = append(candidates, net.JoinHostPort(value["ip"], value["port"]))
					}
				}
			}
			sort.Strings(candidates)
			replicaAnswers[index] = candidates
		}(index, address)
	}
	wg.Wait()
	sentinelPassword = ""
	primaryAnswer, err := agreedEndpoint(primaryAnswers, cfg.Redis.Sentinel.MinAgreement)
	if err != nil {
		return nil, nil, "", "", err
	}
	primary, _, ok := strings.Cut(primaryAnswer, "\x00")
	if !ok {
		return nil, nil, "", "", errors.New("Sentinel primary agreement lacks a configuration epoch")
	}
	primaryPinned, err := pinEndpoint(ctx, primary, cfg.Redis.Sentinel.EndpointAllowlist)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("Sentinel primary endpoint: %w", err)
	}
	address := primaryPinned
	if cfg.Redis.Sentinel.ReadRole == "replica" {
		var votes []string
		for _, candidates := range replicaAnswers {
			seen := map[string]struct{}{}
			for _, candidate := range candidates {
				if _, ok := seen[candidate]; !ok {
					votes = append(votes, candidate)
					seen[candidate] = struct{}{}
				}
			}
		}
		candidates := quorumEndpoints(votes, cfg.Redis.Sentinel.MinAgreement)
		if len(candidates) == 0 {
			return nil, nil, "", "", errors.New("Sentinel quorum did not agree on a replica")
		}
		address = candidates[0]
	}
	address, err = pinEndpoint(ctx, address, cfg.Redis.Sentinel.EndpointAllowlist)
	if err != nil {
		return nil, nil, "", "", err
	}
	client := redisdriver.NewClient(dataOptions(cfg, address, password, tlsConfig))
	policy, version, revision, err := attestNode(ctx, client, cfg, salt, profiles, cfg.Redis.Sentinel.ReadRole, primaryPinned)
	if err != nil {
		_ = client.Close()
		return nil, nil, "", "", err
	}
	if cfg.Redis.Sentinel.ReadRole == "replica" {
		primaryClient := redisdriver.NewClient(dataOptions(cfg, primaryPinned, password, tlsConfig))
		lagErr := verifyReplicaLag(ctx, primaryClient, client, cfg.Redis.Sentinel.MaxReplicaLagBytes)
		_ = primaryClient.Close()
		if lagErr != nil {
			_ = client.Close()
			return nil, nil, "", "", lagErr
		}
	}
	return client, policy, version, revision, nil
}

func verifyReplicaLag(ctx context.Context, primary, replica redisdriver.UniversalClient, maximum int64) error {
	primaryRaw, err := primary.Do(ctx, "ROLE").Result()
	if err != nil {
		return errors.New("inspect Redis primary replication offset")
	}
	replicaRaw, err := replica.Do(ctx, "ROLE").Result()
	if err != nil {
		return errors.New("inspect Redis replica replication offset")
	}
	pv, pok := primaryRaw.([]interface{})
	rv, rok := replicaRaw.([]interface{})
	if !pok || !rok || len(pv) < 2 || len(rv) < 5 {
		return errors.New("Redis replication offsets have an invalid shape")
	}
	primaryOffset, e1 := strconv.ParseInt(fmt.Sprint(pv[1]), 10, 64)
	replicaOffset, e2 := strconv.ParseInt(fmt.Sprint(rv[4]), 10, 64)
	if e1 != nil || e2 != nil || replicaOffset > primaryOffset || primaryOffset-replicaOffset > maximum {
		return errors.New("Redis replica lag exceeds the configured ceiling")
	}
	return nil
}

func buildClusterRuntime(ctx context.Context, cfg *config.TargetConfig, password string, tlsConfig *tls.Config, salt []byte, profiles *modulepolicy.Set) (redisdriver.UniversalClient, *Policy, string, string, error) {
	slots, discoveryErr := discoverClusterSlots(ctx, cfg, password, tlsConfig)
	if discoveryErr != nil {
		return nil, nil, "", "", errors.New("discover Redis Cluster topology")
	}
	pinnedEndpoints, err := validateClusterSlots(ctx, slots, cfg.Redis.Cluster.EndpointAllowlist)
	if err != nil {
		return nil, nil, "", "", err
	}
	for slotIndex := range slots {
		for nodeIndex := range slots[slotIndex].Nodes {
			slots[slotIndex].Nodes[nodeIndex].Addr = pinnedEndpoints[slots[slotIndex].Nodes[nodeIndex].Addr]
		}
		if cfg.Redis.Cluster.ReadRole == "primary" {
			slots[slotIndex].Nodes = slots[slotIndex].Nodes[:1]
		} else if len(slots[slotIndex].Nodes) < 2 {
			return nil, nil, "", "", fmt.Errorf("Redis Cluster slot range %d-%d has no replica", slots[slotIndex].Start, slots[slotIndex].End)
		}
	}
	knownPrimaries := map[string]struct{}{}
	for _, slot := range slots {
		knownPrimaries[slot.Nodes[0].Addr] = struct{}{}
	}
	eligible := map[string]struct{}{}
	for _, slot := range slots {
		if cfg.Redis.Cluster.ReadRole == "primary" {
			eligible[slot.Nodes[0].Addr] = struct{}{}
		} else {
			for _, node := range slot.Nodes[1:] {
				eligible[node.Addr] = struct{}{}
			}
		}
	}
	if len(eligible) == 0 {
		return nil, nil, "", "", errors.New("Redis Cluster has no nodes for the configured read role")
	}
	verificationRoles := map[string]string{}
	expectedPrimaries, err := clusterReplicaAssignments(slots, cfg.Redis.Cluster.ReadRole)
	if err != nil {
		return nil, nil, "", "", err
	}
	for address := range eligible {
		verificationRoles[address] = cfg.Redis.Cluster.ReadRole
	}
	if cfg.Redis.Cluster.ReadRole == "replica" {
		for address := range knownPrimaries {
			verificationRoles[address] = "primary"
		}
	}
	if len(verificationRoles)*cfg.Connection.MaxOpen > 256 || len(verificationRoles)*cfg.Connection.MaxIdle > 64 {
		return nil, nil, "", "", errors.New("Redis Cluster discovered topology exceeds the per-target connection budget")
	}
	type nodeResult struct {
		address           string
		policy            *Policy
		version, revision string
		err               error
	}
	results := make(chan nodeResult, len(verificationRoles))
	var wg sync.WaitGroup
	for address, role := range verificationRoles {
		wg.Add(1)
		go func(address, role string) {
			defer wg.Done()
			node := redisdriver.NewClient(dataOptions(cfg, address, password, tlsConfig))
			defer node.Close()
			policy, version, revision, err := attestNode(ctx, node, cfg, salt, profiles, role, expectedPrimaries[address])
			results <- nodeResult{address, policy, version, revision, err}
		}(address, role)
	}
	wg.Wait()
	close(results)
	var policy *Policy
	var version, revision string
	for result := range results {
		if result.err != nil {
			return nil, nil, "", "", fmt.Errorf("attest Redis Cluster node %q: %w", result.address, result.err)
		}
		if policy == nil {
			policy, version, revision = result.policy, result.version, result.revision
		} else if revision != result.revision {
			return nil, nil, "", "", errors.New("Redis Cluster nodes have incompatible command or module policies")
		}
	}
	pinnedSeeds, err := pinnedClusterSeeds(ctx, cfg)
	if err != nil {
		return nil, nil, "", "", err
	}
	cluster := redisdriver.NewClusterClient(&redisdriver.ClusterOptions{Addrs: pinnedSeeds, Username: cfg.Username, Password: password, Protocol: cfg.Redis.Protocol, MaxRedirects: cfg.Redis.Cluster.RedirectLimit, ReadOnly: cfg.Redis.Cluster.ReadRole == "replica", DialTimeout: cfg.Connection.ConnectTimeout, ReadTimeout: cfg.Connection.ReadTimeout, WriteTimeout: cfg.Connection.WriteTimeout, PoolSize: cfg.Connection.MaxOpen, MinIdleConns: cfg.Connection.MaxIdle, ConnMaxLifetime: cfg.Connection.MaxLifetime, ConnMaxIdleTime: cfg.Connection.MaxIdleTime, TLSConfig: tlsConfig, ClusterSlots: func(context.Context) ([]redisdriver.ClusterSlot, error) { return slots, nil }, NewClient: func(options *redisdriver.Options) *redisdriver.Client {
		if _, ok := eligible[options.Addr]; !ok {
			if _, primary := knownPrimaries[options.Addr]; primary && cfg.Redis.Cluster.ReadRole == "replica" {
				client := redisdriver.NewClient(options)
				client.AddHook(clusterPrimaryIntrospectionOnly{})
				return client
			}
			options.Dialer = func(context.Context, string, string) (net.Conn, error) {
				return nil, errors.New("Redis Cluster redirect endpoint is not attested")
			}
		}
		return redisdriver.NewClient(options)
	}})
	if cfg.Redis.Cluster.ReadRole == "primary" {
		if err := cluster.Ping(ctx).Err(); err != nil {
			_ = cluster.Close()
			return nil, nil, "", "", errors.New("Redis Cluster is unreachable after topology admission")
		}
	}
	return cluster, policy, version, revision, nil
}

func clusterReplicaAssignments(slots []redisdriver.ClusterSlot, readRole string) (map[string]string, error) {
	assignments := map[string]string{}
	if readRole != "replica" {
		return assignments, nil
	}
	for _, slot := range slots {
		primary := slot.Nodes[0].Addr
		for _, replica := range slot.Nodes[1:] {
			if previous, exists := assignments[replica.Addr]; exists && previous != primary {
				return nil, errors.New("Redis Cluster replica is assigned to multiple primaries")
			}
			assignments[replica.Addr] = primary
		}
	}
	return assignments, nil
}

type clusterPrimaryIntrospectionOnly struct{}

func (clusterPrimaryIntrospectionOnly) DialHook(next redisdriver.DialHook) redisdriver.DialHook {
	return next
}
func (clusterPrimaryIntrospectionOnly) ProcessHook(next redisdriver.ProcessHook) redisdriver.ProcessHook {
	return func(ctx context.Context, cmd redisdriver.Cmder) error {
		if !strings.EqualFold(cmd.Name(), "command") {
			return errors.New("Redis Cluster replica target refused primary fallback")
		}
		return next(ctx, cmd)
	}
}
func (clusterPrimaryIntrospectionOnly) ProcessPipelineHook(next redisdriver.ProcessPipelineHook) redisdriver.ProcessPipelineHook {
	return func(context.Context, []redisdriver.Cmder) error {
		return errors.New("Redis Cluster replica target refused a primary pipeline")
	}
}
