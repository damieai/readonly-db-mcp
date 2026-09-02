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

	policy  atomic.Pointer[Policy]
	healthy atomic.Bool
	stop    context.CancelFunc
	wg      sync.WaitGroup
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	password, err := cfg.Password()
	if err != nil {
		return nil, fmt.Errorf("target %q credentials: %w", cfg.Name, err)
	}
	tlsConfig, err := redisTLS(cfg)
	if err != nil {
		return nil, err
	}
	client := redisdriver.NewUniversalClient(&redisdriver.UniversalOptions{
		Addrs: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)}, Username: cfg.Username,
		Password: password, DB: cfg.Redis.Database, Protocol: cfg.Redis.Protocol,
		DialTimeout: cfg.Connection.ConnectTimeout, ReadTimeout: cfg.Connection.ReadTimeout,
		WriteTimeout: cfg.Connection.WriteTimeout, PoolSize: cfg.Connection.MaxOpen,
		MinIdleConns: cfg.Connection.MaxIdle, ConnMaxLifetime: cfg.Connection.MaxLifetime,
		ConnMaxIdleTime: cfg.Connection.MaxIdleTime, TLSConfig: tlsConfig, DisableIdentity: true,
	})
	password = ""
	checkCtx, cancel := context.WithTimeout(ctx, cfg.Connection.ConnectTimeout)
	defer cancel()
	if err := client.Ping(checkCtx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("target %q is unreachable", cfg.Name)
	}
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		client.Close()
		return nil, errors.New("initialize Redis audit salt")
	}
	policy, version, revision, err := attest(checkCtx, client, cfg, salt)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("target %q startup verification failed: %w", cfg.Name, err)
	}
	t := &Target{cfg: cfg, limits: limits, client: client, admission: controller, auditor: auditor, metrics: recorder, keySalt: salt,
		info: core.TargetInfo{Name: cfg.Name, Engine: cfg.Engine, Environment: cfg.Environment, Consistency: cfg.Consistency,
			Database: strconv.Itoa(cfg.Redis.Database), Healthy: true, ReadOnlyUser: true, ParameterStyle: "command arguments",
			ServerVersion: version, DeploymentMode: cfg.Redis.Mode, KeyPatterns: append([]string(nil), cfg.Redis.KeyPatterns...), PolicyRevision: revision}}
	t.policy.Store(policy)
	t.healthy.Store(true)
	t.startMaintenance()
	return t, nil
}

func attest(ctx context.Context, client redisdriver.UniversalClient, cfg *config.TargetConfig, salt []byte) (*Policy, string, string, error) {
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
	modules, err := client.Do(ctx, "MODULE", "LIST").Result()
	if err != nil {
		return nil, "", "", errors.New("inspect installed Redis modules")
	}
	if values, ok := modules.([]interface{}); !ok || len(values) != 0 {
		return nil, "", "", errors.New("Redis modules require a separately attested module policy")
	}
	acl, err := client.Do(ctx, "ACL", "GETUSER", cfg.Username).Result()
	if err != nil {
		return nil, "", "", errors.New("inspect effective Redis ACL")
	}
	if err := attestACL(acl, cfg.Redis, commands); err != nil {
		return nil, "", "", err
	}
	revision := catalogRevision(version, commands)
	return newPolicy(cfg.Redis, commands, salt), version, revision, nil
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
	info := t.info
	info.Healthy = t.healthy.Load()
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
	normalized, count, truncated, err := normalizeRedis(value, 0, &normalizer{maxDepth: t.cfg.Redis.MaxReplyDepth, maxElements: maxElements, maxCell: t.limits.MaxCellBytes})
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
	return nil
}

func (t *Target) startMaintenance() {
	ctx, cancel := context.WithCancel(context.Background())
	t.stop = cancel
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		ticker := time.NewTicker(t.cfg.Redis.ACLRecheck)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkCtx, checkCancel := context.WithTimeout(ctx, t.cfg.Connection.ConnectTimeout)
				permit, err := t.admission.Acquire(checkCtx, t.cfg.Name, admission.Maintenance)
				if err == nil {
					var policy *Policy
					policy, _, _, err = attest(checkCtx, t.client, t.cfg, t.keySalt)
					if err == nil {
						t.policy.Store(policy)
					}
					permit.Release()
				}
				t.healthy.Store(err == nil)
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
		_, _ = h.Write([]byte("\x00" + name + "\x00" + fmt.Sprint(info.ReadOnly) + "\x00" + strings.Join(info.Flags, ",")))
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}
func slicesSort(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

type normalizer struct{ maxDepth, maxElements, maxCell, elements int }

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
		pairs := make([]any, 0, len(x))
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
		out := make(map[string]any, len(x))
		count := 0
		truncated := false
		for key, item := range x {
			if n.elements >= n.maxElements {
				truncated = true
				break
			}
			n.elements++
			v, c, tr, err := normalizeRedis(item, depth+1, n)
			if err != nil {
				return nil, 0, false, err
			}
			out[key] = v
			count += c
			truncated = truncated || tr
		}
		return out, count, truncated, nil
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
