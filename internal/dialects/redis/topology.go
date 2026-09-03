package redis

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
)

func sentinelTopologyRevision(ctx context.Context, cfg *config.TargetConfig) (string, error) {
	password, err := cfg.RedisSentinelPassword()
	if err != nil {
		return "", err
	}
	tlsConfig, err := redisTLS(cfg)
	if err != nil {
		return "", err
	}
	primaryAnswers := make([]string, len(cfg.Redis.Sentinel.Addresses))
	replicaAnswers := make([][]string, len(cfg.Redis.Sentinel.Addresses))
	var wg sync.WaitGroup
	for index, address := range cfg.Redis.Sentinel.Addresses {
		wg.Add(1)
		go func(index int, address string) {
			defer wg.Done()
			pinned, pinErr := pinEndpoint(ctx, address, cfg.Redis.Sentinel.EndpointAllowlist)
			if pinErr != nil {
				return
			}
			client := redisdriver.NewSentinelClient(&redisdriver.Options{Addr: pinned, Username: cfg.Redis.Sentinel.Username, Password: password, Protocol: cfg.Redis.Protocol, DialTimeout: cfg.Connection.ConnectTimeout, ReadTimeout: cfg.Redis.Sentinel.DiscoveryTimeout, WriteTimeout: cfg.Redis.Sentinel.DiscoveryTimeout, TLSConfig: tlsConfig, DisableIdentity: true})
			defer client.Close()
			master, masterErr := client.Master(ctx, cfg.Redis.Sentinel.ServiceName).Result()
			if epoch, epochErr := strconv.ParseUint(master["config-epoch"], 10, 64); masterErr == nil && epochErr == nil && master["ip"] != "" && master["port"] != "" {
				primaryAnswers[index] = net.JoinHostPort(master["ip"], master["port"]) + "\x00" + strconv.FormatUint(epoch, 10)
			}
			if cfg.Redis.Sentinel.ReadRole != "replica" {
				return
			}
			values, replicaErr := client.Replicas(ctx, cfg.Redis.Sentinel.ServiceName).Result()
			if replicaErr != nil {
				return
			}
			for _, value := range values {
				if (strings.EqualFold(value["master-link-status"], "ok") || strings.EqualFold(value["master-link-status"], "up")) && value["ip"] != "" && value["port"] != "" {
					replicaAnswers[index] = append(replicaAnswers[index], net.JoinHostPort(value["ip"], value["port"]))
				}
			}
		}(index, address)
	}
	wg.Wait()
	password = ""
	primaryAnswer, err := agreedEndpoint(primaryAnswers, cfg.Redis.Sentinel.MinAgreement)
	if err != nil {
		return "", err
	}
	primary, _, ok := strings.Cut(primaryAnswer, "\x00")
	if !ok {
		return "", fmt.Errorf("Sentinel primary agreement lacks a configuration epoch")
	}
	pinnedPrimary, err := pinEndpoint(ctx, primary, cfg.Redis.Sentinel.EndpointAllowlist)
	if err != nil {
		return "", err
	}
	selected := pinnedPrimary
	if cfg.Redis.Sentinel.ReadRole == "replica" {
		var votes []string
		for _, candidates := range replicaAnswers {
			seen := map[string]struct{}{}
			for _, candidate := range candidates {
				if _, exists := seen[candidate]; !exists {
					votes = append(votes, candidate)
					seen[candidate] = struct{}{}
				}
			}
		}
		candidates := quorumEndpoints(votes, cfg.Redis.Sentinel.MinAgreement)
		if len(candidates) == 0 {
			return "", fmt.Errorf("Sentinel quorum did not agree on a replica")
		}
		selected, err = pinEndpoint(ctx, candidates[0], cfg.Redis.Sentinel.EndpointAllowlist)
		if err != nil {
			return "", err
		}
	}
	sum := sha256.Sum256([]byte(primaryAnswer + "\x00" + selected))
	return hex.EncodeToString(sum[:16]), nil
}

func clusterTopologyRevision(ctx context.Context, cfg *config.TargetConfig) (string, error) {
	password, err := cfg.Password()
	if err != nil {
		return "", err
	}
	tlsConfig, err := redisTLS(cfg)
	if err != nil {
		return "", err
	}
	slots, err := discoverClusterSlots(ctx, cfg, password, tlsConfig)
	password = ""
	if err != nil {
		return "", err
	}
	pinned, err := validateClusterSlots(ctx, slots, cfg.Redis.Cluster.EndpointAllowlist)
	if err != nil {
		return "", err
	}
	var canonical strings.Builder
	sortedSlots := append([]redisdriver.ClusterSlot(nil), slots...)
	sort.Slice(sortedSlots, func(i, j int) bool {
		if sortedSlots[i].Start == sortedSlots[j].Start {
			return sortedSlots[i].End < sortedSlots[j].End
		}
		return sortedSlots[i].Start < sortedSlots[j].Start
	})
	for _, slot := range sortedSlots {
		fmt.Fprintf(&canonical, "%05d-%05d", slot.Start, slot.End)
		nodes := append([]redisdriver.ClusterNode(nil), slot.Nodes...)
		if len(nodes) > 2 {
			sort.Slice(nodes[1:], func(i, j int) bool {
				left, right := nodes[i+1], nodes[j+1]
				return pinned[left.Addr]+"\x00"+left.ID < pinned[right.Addr]+"\x00"+right.ID
			})
		}
		for _, node := range nodes {
			canonical.WriteByte(0)
			canonical.WriteString(pinned[node.Addr])
			canonical.WriteByte(0)
			canonical.WriteString(node.ID)
		}
		canonical.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(canonical.String()))
	return hex.EncodeToString(sum[:16]), nil
}

func discoverClusterSlots(ctx context.Context, cfg *config.TargetConfig, password string, tlsConfig *tls.Config) ([]redisdriver.ClusterSlot, error) {
	var lastErr error
	for _, seed := range cfg.Redis.Cluster.SeedAddresses {
		pinnedSeed, pinErr := pinEndpoint(ctx, seed, cfg.Redis.Cluster.EndpointAllowlist)
		if pinErr != nil {
			lastErr = pinErr
			continue
		}
		client := redisdriver.NewClient(dataOptions(cfg, pinnedSeed, password, tlsConfig))
		slots, err := client.ClusterSlots(ctx).Result()
		_ = client.Close()
		if err == nil {
			return slots, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func pinnedClusterSeeds(ctx context.Context, cfg *config.TargetConfig) ([]string, error) {
	seeds := make([]string, 0, len(cfg.Redis.Cluster.SeedAddresses))
	for _, seed := range cfg.Redis.Cluster.SeedAddresses {
		pinned, err := pinEndpoint(ctx, seed, cfg.Redis.Cluster.EndpointAllowlist)
		if err != nil {
			return nil, fmt.Errorf("Redis Cluster seed %q is outside the allowlist: %w", seed, err)
		}
		seeds = append(seeds, pinned)
	}
	return seeds, nil
}

func endpointAllowed(address string, allow config.RedisEndpointAllowlist) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if ip := net.ParseIP(host); ip != nil {
		for _, raw := range allow.CIDRs {
			_, network, err := net.ParseCIDR(raw)
			if err == nil && network.Contains(ip) {
				return true
			}
		}
		return false
	}
	for _, suffix := range allow.DNSSuffixes {
		suffix = strings.TrimSuffix(strings.ToLower(suffix), ".")
		if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
			return true
		}
	}
	return false
}

func pinEndpoint(ctx context.Context, address string, allow config.RedisEndpointAllowlist) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	if ip := net.ParseIP(host); ip != nil {
		if ipInAllowedCIDR(ip, allow.CIDRs) {
			return net.JoinHostPort(ip.String(), port), nil
		}
		return "", fmt.Errorf("endpoint IP is outside allowed CIDRs")
	}
	if !endpointAllowed(address, allow) {
		return "", fmt.Errorf("endpoint hostname is outside the DNS allowlist")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return "", fmt.Errorf("resolve endpoint")
	}
	var values []string
	for _, item := range addresses {
		if !ipInAllowedCIDR(item.IP, allow.CIDRs) {
			return "", fmt.Errorf("endpoint DNS answer is outside allowed CIDRs")
		}
		values = append(values, item.IP.String())
	}
	sort.Strings(values)
	return net.JoinHostPort(values[0], port), nil
}

func ipInAllowedCIDR(ip net.IP, cidrs []string) bool {
	for _, raw := range cidrs {
		_, network, err := net.ParseCIDR(raw)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func agreedEndpoint(answers []string, minimum int) (string, error) {
	counts := map[string]int{}
	for _, answer := range answers {
		if answer != "" {
			counts[answer]++
		}
	}
	type candidate struct {
		address string
		count   int
	}
	var values []candidate
	for address, count := range counts {
		values = append(values, candidate{address, count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].count == values[j].count {
			return values[i].address < values[j].address
		}
		return values[i].count > values[j].count
	})
	if len(values) == 0 || values[0].count < minimum {
		return "", fmt.Errorf("Sentinel quorum did not agree on an endpoint")
	}
	if len(values) > 1 && values[0].count == values[1].count {
		return "", fmt.Errorf("Sentinel quorum returned conflicting endpoints")
	}
	return values[0].address, nil
}

func quorumEndpoints(answers []string, minimum int) []string {
	counts := map[string]int{}
	for _, answer := range answers {
		if answer != "" {
			counts[answer]++
		}
	}
	var result []string
	for address, count := range counts {
		if count >= minimum {
			result = append(result, address)
		}
	}
	sort.Strings(result)
	return result
}

func validateClusterSlots(ctx context.Context, slots []redisdriver.ClusterSlot, allow config.RedisEndpointAllowlist) (map[string]string, error) {
	covered := make([]bool, 16384)
	nodes := map[string]string{}
	for _, slot := range slots {
		if slot.Start < 0 || slot.End > 16383 || slot.Start > slot.End || len(slot.Nodes) == 0 {
			return nil, fmt.Errorf("invalid Redis Cluster slot range")
		}
		for value := slot.Start; value <= slot.End; value++ {
			if covered[value] {
				return nil, fmt.Errorf("Redis Cluster slot %d has conflicting owners", value)
			}
			covered[value] = true
		}
		for _, node := range slot.Nodes {
			if _, checked := nodes[node.Addr]; !checked {
				pinned, err := pinEndpoint(ctx, node.Addr, allow)
				if err != nil {
					return nil, fmt.Errorf("Redis Cluster endpoint %q is outside the allowlist: %w", node.Addr, err)
				}
				nodes[node.Addr] = pinned
			}
		}
	}
	for value, ok := range covered {
		if !ok {
			return nil, fmt.Errorf("Redis Cluster slot %d is not covered", value)
		}
	}
	return nodes, nil
}
