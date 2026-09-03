package redis

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	modulepolicy "github.com/your-org/readonly-db-mcp/internal/dialects/redis/modules"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
)

type Target struct {
	cfg       *config.TargetConfig
	limits    config.Limits
	client    redisdriver.UniversalClient
	admission *admission.Controller
	auditor   audit.Auditor
	metrics   metrics.Recorder
	info      core.TargetInfo
	keySalt   []byte
	profiles  *modulepolicy.Set
	clientMu  sync.RWMutex

	policy           atomic.Pointer[Policy]
	healthy          atomic.Bool
	lastAttested     atomic.Int64
	lastTopology     atomic.Int64
	lastProfileCheck atomic.Int64
	stop             context.CancelFunc
	wg               sync.WaitGroup
	topologyRevision string
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, errors.New("initialize Redis audit salt")
	}
	profiles, err := modulepolicy.Load(cfg.Redis.ModuleProfiles, cfg.Redis.TrustedProfileKeys, time.Now())
	if err != nil {
		return nil, fmt.Errorf("target %q module profiles: %w", cfg.Name, err)
	}
	checkCtx, cancel := context.WithTimeout(ctx, startupTimeout(cfg))
	defer cancel()
	client, policy, version, revision, err := buildRuntime(checkCtx, cfg, salt, profiles)
	if err != nil {
		return nil, fmt.Errorf("target %q startup verification failed: %w", cfg.Name, err)
	}
	t := &Target{cfg: cfg, limits: limits, client: client, admission: controller, auditor: auditor, metrics: recorder, keySalt: salt, profiles: profiles,
		info: core.TargetInfo{Name: cfg.Name, Engine: cfg.Engine, Environment: cfg.Environment, Consistency: cfg.Consistency,
			Database: strconv.Itoa(cfg.Redis.Database), Healthy: true, ReadOnlyUser: true, ParameterStyle: "command arguments",
			ServerVersion: version, DeploymentMode: cfg.Redis.Mode, KeyPatterns: append([]string(nil), cfg.Redis.KeyPatterns...), PolicyRevision: revision}}
	t.policy.Store(policy)
	t.healthy.Store(true)
	t.lastAttested.Store(time.Now().UnixNano())
	t.lastTopology.Store(time.Now().UnixNano())
	t.lastProfileCheck.Store(time.Now().UnixNano())
	if cfg.Redis.Mode == "cluster" || cfg.Redis.Mode == "sentinel" {
		var topologyErr error
		if cfg.Redis.Mode == "cluster" {
			t.topologyRevision, topologyErr = clusterTopologyRevision(checkCtx, cfg)
		} else {
			t.topologyRevision, topologyErr = sentinelTopologyRevision(checkCtx, cfg)
		}
		if topologyErr != nil {
			t.topologyRevision = ""
		}
	}
	t.startMaintenance()
	return t, nil
}

func attest(ctx context.Context, client redisdriver.UniversalClient, cfg *config.TargetConfig, salt []byte, profiles *modulepolicy.Set) (*Policy, string, string, error) {
	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		return nil, "", "", errors.New("inspect Redis server")
	}
	version := infoField(info, "redis_version")
	if !supportedVersion(version) {
		return nil, "", "", fmt.Errorf("Redis version %q is not supported", version)
	}
	who, err := client.ACLWhoAmI(ctx).Result()
	if err != nil || who != cfg.Username {
		return nil, "", "", errors.New("connected Redis ACL identity does not match configuration")
	}
	commands, err := loadCommandCatalog(ctx, client)
	if err != nil {
		return nil, "", "", errors.New("inspect Redis command catalog")
	}
	trustedModules, moduleCommands, err := attestModules(ctx, client, profiles, commands, version)
	if err != nil {
		return nil, "", "", err
	}
	acl, err := client.Do(ctx, "ACL", "GETUSER", cfg.Username).Result()
	if err != nil {
		return nil, "", "", errors.New("inspect effective Redis ACL")
	}
	if err := attestACL(acl, cfg.Redis, commands); err != nil {
		return nil, "", "", err
	}
	revision := catalogRevision(version, commands) + ":" + profiles.Digest()
	return newPolicy(cfg.Redis, commands, salt, trustedModules, moduleCommands), version, revision, nil
}

func loadCommandCatalog(ctx context.Context, client redisdriver.UniversalClient) (map[string]*redisdriver.CommandInfo, error) {
	commands, err := client.Command(ctx).Result()
	if err != nil {
		return nil, err
	}
	names, err := client.CommandList(ctx, nil).Result()
	if err != nil {
		return nil, err
	}
	var subcommands []*redisdriver.CommandsInfoCmd
	_, err = client.Pipelined(ctx, func(pipe redisdriver.Pipeliner) error {
		for _, name := range names {
			if !strings.Contains(name, "|") {
				continue
			}
			cmd := redisdriver.NewCommandsInfoCmd(ctx, "command", "info", name)
			if err := pipe.Process(ctx, cmd); err != nil {
				return err
			}
			subcommands = append(subcommands, cmd)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, cmd := range subcommands {
		values, err := cmd.Result()
		if err != nil {
			return nil, err
		}
		for name, info := range values {
			commands[strings.ToLower(name)] = info
		}
	}
	return commands, nil
}

func (t *Target) Info() core.TargetInfo {
	t.clientMu.RLock()
	defer t.clientMu.RUnlock()
	info := t.info
	info.Healthy = t.requireHealthy() == nil
	info.KeyPatterns = append([]string(nil), info.KeyPatterns...)
	return info
}

func (t *Target) ValidateRedis(ctx context.Context, req core.RedisRequest) (*core.RedisValidation, error) {
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	permit, err := t.admission.Acquire(ctx, t.cfg.Name, admission.Interactive)
	if err != nil {
		return nil, fmt.Errorf("Redis concurrency limit: %w", err)
	}
	defer permit.Release()
	t.clientMu.RLock()
	defer t.clientMu.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	validation, _, err := t.policy.Load().validate(ctx, t.client, req)
	return validation, err
}

func (t *Target) RedisCommand(ctx context.Context, req core.RedisRequest) (*core.RedisResult, error) {
	started := time.Now()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = t.limits.DefaultTimeout
	}
	if timeout > t.limits.MaxTimeout {
		return nil, errors.New("requested timeout exceeds configured maximum")
	}
	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	permit, err := t.admission.Acquire(qctx, t.cfg.Name, admission.Interactive)
	if err != nil {
		return nil, fmt.Errorf("Redis concurrency limit: %w", err)
	}
	defer permit.Release()
	t.clientMu.RLock()
	defer t.clientMu.RUnlock()
	if err := t.requireHealthy(); err != nil {
		return nil, err
	}
	validation, args, err := t.policy.Load().validate(qctx, t.client, req)
	if err != nil {
		t.record(qctx, audit.Event{Target: t.cfg.Name, Operation: "redis_command", Decision: "rejected", Reason: err.Error()})
		return nil, err
	}
	value, err := t.client.Do(qctx, args...).Result()
	if err != nil {
		return nil, sanitizeRedis(err)
	}
	maxElements := req.MaxElements
	if maxElements <= 0 || maxElements > t.cfg.Redis.MaxReplyElements {
		maxElements = t.cfg.Redis.MaxReplyElements
	}
	cellLimit := normalizationCellLimit(t.limits.MaxResultBytes, maxElements, t.limits.MaxCellBytes)
	normalized, count, truncated, err := normalizeRedis(value, 0, &normalizer{maxDepth: t.cfg.Redis.MaxReplyDepth, maxElements: maxElements, maxCell: cellLimit})
	if err != nil {
		return nil, err
	}
	result := &core.RedisResult{RequestID: uuid.NewString(), Target: t.cfg.Name, Engine: t.cfg.Engine, Environment: t.cfg.Environment, Command: validation.Command, Value: normalized, ElementCount: count, Truncated: truncated, DurationMS: time.Since(started).Milliseconds()}
	if err := enforceRedisBudget(result, t.limits.MaxResultBytes); err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(result)
	t.record(qctx, audit.Event{QueryID: result.RequestID, Target: t.cfg.Name, Operation: "redis_command", Fingerprint: validation.Fingerprint, Tables: validation.KeyFingerprints, Decision: "allowed", Rows: count, Truncated: truncated, Duration: time.Since(started), ResponseBytes: len(encoded)})
	return result, nil
}

func (t *Target) requireHealthy() error {
	if !t.healthy.Load() {
		return errors.New("Redis target failed its latest ACL or command-catalog attestation")
	}
	if last := t.lastAttested.Load(); last == 0 || time.Since(time.Unix(0, last)) > t.cfg.Redis.CatalogMaxAge {
		return errors.New("Redis attestation or topology snapshot is stale")
	}
	if t.cfg.Redis.Mode == "cluster" {
		if last := t.lastTopology.Load(); last == 0 || time.Since(time.Unix(0, last)) > t.cfg.Redis.Cluster.TopologyMaxAge {
			return errors.New("Redis attestation or topology snapshot is stale")
		}
	}
	if t.cfg.Redis.Mode == "sentinel" {
		maxAge := 3 * t.cfg.Redis.Sentinel.RefreshInterval
		if last := t.lastTopology.Load(); last == 0 || time.Since(time.Unix(0, last)) > maxAge {
			return errors.New("Redis Sentinel discovery snapshot is stale")
		}
	}
	return nil
}

func (t *Target) startMaintenance() {
	ctx, cancel := context.WithCancel(context.Background())
	t.stop = cancel
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		interval := t.cfg.Redis.ACLRecheck
		if t.cfg.Redis.Mode == "sentinel" && t.cfg.Redis.Sentinel.RefreshInterval < interval {
			interval = t.cfg.Redis.Sentinel.RefreshInterval
		}
		if t.cfg.Redis.Mode == "cluster" && t.cfg.Redis.Cluster.TopologyRefresh < interval {
			interval = t.cfg.Redis.Cluster.TopologyRefresh
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkCtx, checkCancel := context.WithTimeout(ctx, startupTimeout(t.cfg))
				permit, err := t.admission.Acquire(checkCtx, t.cfg.Name, admission.Maintenance)
				if err == nil {
					topologyRevision := ""
					if t.cfg.Redis.Mode == "cluster" || t.cfg.Redis.Mode == "sentinel" {
						if t.cfg.Redis.Mode == "cluster" {
							topologyRevision, err = clusterTopologyRevision(checkCtx, t.cfg)
						} else {
							topologyRevision, err = sentinelTopologyRevision(checkCtx, t.cfg)
						}
						t.clientMu.RLock()
						profileErr := t.profiles.ValidateAt(time.Now())
						unchanged := err == nil && profileErr == nil && topologyRevision == t.topologyRevision
						t.clientMu.RUnlock()
						catalogFresh := time.Since(time.Unix(0, t.lastAttested.Load())) < t.cfg.Redis.ACLRecheck
						if unchanged && catalogFresh {
							t.lastTopology.Store(time.Now().UnixNano())
							permit.Release()
							checkCancel()
							continue
						}
					}
					var client redisdriver.UniversalClient
					var policy *Policy
					var version, revision string
					var profiles *modulepolicy.Set
					t.clientMu.RLock()
					profiles = t.profiles
					t.clientMu.RUnlock()
					profileReloaded := time.Since(time.Unix(0, t.lastProfileCheck.Load())) >= t.cfg.Redis.ACLRecheck
					if profileReloaded {
						profiles, err = modulepolicy.Load(t.cfg.Redis.ModuleProfiles, t.cfg.Redis.TrustedProfileKeys, time.Now())
					} else {
						err = profiles.ValidateAt(time.Now())
					}
					if err == nil {
						client, policy, version, revision, err = buildRuntime(checkCtx, t.cfg, t.keySalt, profiles)
					}
					if err == nil {
						t.clientMu.Lock()
						old := t.client
						t.client = client
						t.policy.Store(policy)
						t.profiles = profiles
						if profileReloaded {
							t.lastProfileCheck.Store(time.Now().UnixNano())
						}
						t.info.ServerVersion = version
						t.info.PolicyRevision = revision
						t.lastAttested.Store(time.Now().UnixNano())
						t.lastTopology.Store(time.Now().UnixNano())
						if topologyRevision != "" {
							t.topologyRevision = topologyRevision
						}
						t.healthy.Store(true)
						t.clientMu.Unlock()
						_ = old.Close()
					} else {
						t.clientMu.Lock()
						t.healthy.Store(false)
						t.clientMu.Unlock()
					}
					permit.Release()
				} else if checkCtx.Err() != context.Canceled {
					t.clientMu.Lock()
					t.healthy.Store(false)
					t.clientMu.Unlock()
				}
				checkCancel()
			}
		}
	}()
}

func (t *Target) Close() error {
	if t.stop != nil {
		t.stop()
		t.wg.Wait()
	}
	t.clientMu.Lock()
	defer t.clientMu.Unlock()
	return t.client.Close()
}

func (t *Target) record(ctx context.Context, event audit.Event) {
	if t.auditor != nil {
		t.auditor.Record(ctx, event)
	}
}

func redisTLS(cfg *config.TargetConfig) (*tls.Config, error) {
	if cfg.TLS.Mode == config.TLSDisabled {
		return nil, nil
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.TLS.ServerName}
	if cfg.TLS.Mode == config.TLSRequired {
		tlsConfig.InsecureSkipVerify = true
	} else {
		pem, err := os.ReadFile(cfg.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read TLS CA file: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("TLS CA file contains no certificates")
		}
		tlsConfig.RootCAs = roots
	}
	if cfg.TLS.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return tlsConfig, nil
}

func infoField(info, field string) string {
	for _, line := range strings.Split(info, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), field+":"); ok {
			return value
		}
	}
	return ""
}
func supportedVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major, e1 := strconv.Atoi(parts[0])
	minor, e2 := strconv.Atoi(parts[1])
	return e1 == nil && e2 == nil && ((major == 7 && minor >= 2) || major == 8)
}
func catalogRevision(version string, commands map[string]*redisdriver.CommandInfo) string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	// The map order must not influence an authorization revision.
	slicesSort(names)
	h := sha256.New()
	_, _ = h.Write([]byte(version))
	for _, name := range names {
		info := commands[name]
		flags := append([]string(nil), info.Flags...)
		aclFlags := append([]string(nil), info.ACLFlags...)
		slicesSort(flags)
		slicesSort(aclFlags)
		policy := ""
		if info.CommandPolicy != nil {
			tipNames := make([]string, 0, len(info.CommandPolicy.Tips))
			for tip := range info.CommandPolicy.Tips {
				tipNames = append(tipNames, tip)
			}
			slicesSort(tipNames)
			var tips strings.Builder
			for _, tip := range tipNames {
				tips.WriteString(tip)
				tips.WriteByte('=')
				tips.WriteString(info.CommandPolicy.Tips[tip])
				tips.WriteByte(0)
			}
			policy = fmt.Sprintf("%d:%d:%s", info.CommandPolicy.Request, info.CommandPolicy.Response, tips.String())
		}
		_, _ = h.Write([]byte(fmt.Sprintf("\x00%s\x00%t\x00%d\x00%d\x00%d\x00%d\x00%s\x00%s\x00%s", name, info.ReadOnly, info.Arity, info.FirstKeyPos, info.LastKeyPos, info.StepCount, strings.Join(flags, ","), strings.Join(aclFlags, ","), policy)))
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}
func slicesSort(values []string) {
	sort.Strings(values)
}

type normalizer struct{ maxDepth, maxElements, maxCell, elements int }

func normalizationCellLimit(byteBudget, elements, configured int) int {
	if elements < 1 {
		elements = 1
	}
	fair := byteBudget / elements
	if fair < 16 {
		fair = 16
	}
	if fair < configured {
		return fair
	}
	return configured
}

func normalizeRedis(value any, depth int, n *normalizer) (any, int, bool, error) {
	if depth > n.maxDepth {
		return nil, 0, false, errors.New("Redis reply exceeds nesting limit")
	}
	switch x := value.(type) {
	case nil:
		return nil, 0, false, nil
	case string:
		return normalizeBytes([]byte(x), n.maxCell)
	case []byte:
		return normalizeBytes(x, n.maxCell)
	case int64:
		if x > 1<<53-1 || x < -(1<<53-1) {
			return strconv.FormatInt(x, 10), 1, false, nil
		}
		return x, 1, false, nil
	case int:
		return x, 1, false, nil
	case float64:
		return x, 1, false, nil
	case bool:
		return x, 1, false, nil
	case []interface{}:
		out := make([]any, 0, min(len(x), n.maxElements-n.elements))
		count := 0
		truncated := false
		for _, item := range x {
			if n.elements >= n.maxElements {
				truncated = true
				break
			}
			n.elements++
			v, c, tr, err := normalizeRedis(item, depth+1, n)
			if err != nil {
				return nil, 0, false, err
			}
			out = append(out, v)
			count += c
			truncated = truncated || tr
		}
		return out, count, truncated, nil
	case map[interface{}]interface{}:
		pairs := make([]any, 0, min(len(x), max(0, n.maxElements-n.elements)))
		count := 0
		truncated := false
		for key, item := range x {
			if n.elements >= n.maxElements {
				truncated = true
				break
			}
			n.elements++
			k, _, kt, err := normalizeRedis(key, depth+1, n)
			if err != nil {
				return nil, 0, false, err
			}
			v, c, vt, err := normalizeRedis(item, depth+1, n)
			if err != nil {
				return nil, 0, false, err
			}
			pairs = append(pairs, []any{k, v})
			count += c
			truncated = truncated || kt || vt
		}
		return pairs, count, truncated, nil
	case map[string]interface{}:
		pairs := make([]any, 0, min(len(x), max(0, n.maxElements-n.elements)))
		count := 0
		truncated := false
		for key, item := range x {
			if n.elements >= n.maxElements {
				truncated = true
				break
			}
			n.elements++
			k, _, kt, err := normalizeBytes([]byte(key), n.maxCell)
			if err != nil {
				return nil, 0, false, err
			}
			v, c, tr, err := normalizeRedis(item, depth+1, n)
			if err != nil {
				return nil, 0, false, err
			}
			pairs = append(pairs, []any{k, v})
			count += c
			truncated = truncated || kt || tr
		}
		return pairs, count, truncated, nil
	default:
		return fmt.Sprint(x), 1, false, nil
	}
}
func normalizeBytes(value []byte, max int) (any, int, bool, error) {
	truncated := len(value) > max
	if truncated {
		value = value[:max]
	}
	if utf8.Valid(value) {
		return string(value), 1, truncated, nil
	}
	return map[string]string{"base64": base64.StdEncoding.EncodeToString(value)}, 1, truncated, nil
}
func enforceRedisBudget(result *core.RedisResult, max int) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return errors.New("encode Redis result")
	}
	if len(encoded) <= max {
		return nil
	}
	result.Value = nil
	result.Truncated = true
	encoded, _ = json.Marshal(result)
	if len(encoded) > max {
		return errors.New("Redis result metadata exceeds configured byte limit")
	}
	return nil
}
func sanitizeRedis(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("Redis command timed out")
	}
	if errors.Is(err, context.Canceled) {
		return errors.New("Redis command was canceled")
	}
	return errors.New("Redis rejected command")
}
