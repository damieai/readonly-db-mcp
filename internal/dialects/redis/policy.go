package redis

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

var forbiddenCommands = map[string]struct{}{
	"acl": {}, "asking": {}, "auth": {}, "bgrewriteaof": {}, "bgsave": {},
	"client": {}, "cluster": {}, "command": {}, "config": {}, "debug": {},
	"discard": {}, "eval": {}, "evalsha": {}, "exec": {}, "failover": {},
	"fcall": {}, "flushall": {}, "flushdb": {}, "function": {}, "hello": {},
	"latency": {}, "migrate": {}, "module": {}, "monitor": {}, "multi": {},
	"psubscribe": {}, "publish": {}, "pubsub": {}, "punsubscribe": {}, "quit": {},
	"readonly": {}, "readwrite": {}, "replicaof": {}, "restore": {}, "role": {},
	"save": {}, "script": {}, "select": {}, "sentinel": {}, "shutdown": {},
	"slowlog": {}, "spublish": {}, "ssubscribe": {}, "subscribe": {},
	"swapdb": {}, "sync": {}, "unsubscribe": {}, "watch": {},
}

type catalogEntry struct {
	name     string
	readonly bool
	flags    map[string]struct{}
	firstKey int8
}

type Policy struct {
	cfg     config.RedisConfig
	catalog map[string]catalogEntry
	keySalt []byte
}

type keyResolver interface {
	CommandGetKeysAndFlags(context.Context, ...interface{}) *redisdriver.KeyFlagsCmd
}

func newPolicy(cfg config.RedisConfig, commands map[string]*redisdriver.CommandInfo, salt []byte) *Policy {
	catalog := make(map[string]catalogEntry, len(commands))
	for name, info := range commands {
		flags := make(map[string]struct{}, len(info.Flags))
		for _, flag := range info.Flags {
			flags[strings.ToLower(flag)] = struct{}{}
		}
		catalog[strings.ToLower(name)] = catalogEntry{name: strings.ToUpper(name), readonly: info.ReadOnly, flags: flags, firstKey: info.FirstKeyPos}
	}
	return &Policy{cfg: cfg, catalog: catalog, keySalt: append([]byte(nil), salt...)}
}

func (p *Policy) validate(ctx context.Context, client keyResolver, req core.RedisRequest) (*core.RedisValidation, []any, error) {
	command := strings.ToLower(strings.TrimSpace(req.Command))
	if command == "" || strings.ContainsAny(command, " \t\r\n\x00") {
		return nil, nil, errors.New("Redis command must be one command name")
	}
	entry, ok := p.catalog[command]
	if len(req.Arguments) > 0 {
		if subcommand, err := decodeArgument(req.Arguments[0]); err == nil {
			if candidate, exists := p.catalog[command+"|"+strings.ToLower(string(subcommand))]; exists {
				entry, ok = candidate, true
			}
		}
	}
	if !ok {
		return nil, nil, errors.New("Redis command is not present in the attested catalog")
	}
	if _, forbidden := forbiddenCommands[command]; forbidden {
		return nil, nil, errors.New("Redis command changes forbidden data or server state")
	}
	if !entry.readonly {
		return nil, nil, errors.New("Redis command is not proven read-only")
	}
	if _, write := entry.flags["write"]; write {
		return nil, nil, errors.New("Redis command may modify data")
	}
	if (command == "eval_ro" || command == "evalsha_ro" || command == "fcall_ro") && !p.cfg.AllowReadonlyScripts {
		return nil, nil, errors.New("read-only Redis programmability is disabled for this target")
	}
	if (command == "eval_ro" || command == "evalsha_ro" || command == "fcall_ro") && !p.hasAllKeyScope() {
		return nil, nil, errors.New("read-only Redis programmability requires an all-key target because nested keyless reads cannot be prefix-scoped")
	}
	args := make([]any, 0, len(req.Arguments)+1)
	args = append(args, strings.ToUpper(command))
	argumentBytes := 0
	for _, argument := range req.Arguments {
		value, err := decodeArgument(argument)
		if err != nil {
			return nil, nil, err
		}
		argumentBytes += len(value)
		if argumentBytes > p.cfg.MaxArgumentBytes {
			return nil, nil, errors.New("Redis arguments exceed the configured byte limit")
		}
		args = append(args, value)
	}
	if (command == "eval_ro" || command == "evalsha_ro") && len(req.Arguments) > 0 {
		value, _ := decodeArgument(req.Arguments[0])
		if len(value) > p.cfg.MaxScriptBytes {
			return nil, nil, errors.New("Redis script exceeds the configured byte limit")
		}
	}
	var keyFlags []redisdriver.KeyFlags
	if needsKeyPreflight(command, entry) {
		var err error
		keyFlags, err = client.CommandGetKeysAndFlags(ctx, args...).Result()
		if err != nil {
			return nil, nil, errors.New("Redis could not prove command key access")
		}
	}
	if len(keyFlags) > p.cfg.MaxKeysPerCommand {
		return nil, nil, errors.New("Redis command references too many keys")
	}
	if len(keyFlags) == 0 && !p.keylessInvocationAllowed(command, req.Arguments) {
		return nil, nil, errors.New("keyless Redis command cannot be proven inside the configured target")
	}
	keyFingerprints := make([]string, 0, len(keyFlags))
	for _, key := range keyFlags {
		if !readOnlyFlags(key.Flags) {
			return nil, nil, errors.New("Redis command may modify key data or metadata")
		}
		if !p.keyAllowed(key.Key) {
			return nil, nil, errors.New("Redis key is outside the configured target")
		}
		keyFingerprints = append(keyFingerprints, p.fingerprintKey(key.Key))
	}
	fingerprintInput := entry.name + "\x00" + fmt.Sprint(len(req.Arguments)) + "\x00" + fmt.Sprint(argumentBytes)
	sum := sha256.Sum256([]byte(fingerprintInput))
	return &core.RedisValidation{Command: entry.name, Fingerprint: hex.EncodeToString(sum[:12]), KeyFingerprints: keyFingerprints, KeyCount: len(keyFlags), ArgumentBytes: argumentBytes}, args, nil
}

func needsKeyPreflight(command string, entry catalogEntry) bool {
	switch command {
	case "ping", "echo", "time", "dbsize", "randomkey", "keys", "scan":
		return false
	case "eval_ro", "evalsha_ro", "fcall_ro":
		return true
	default:
		_, movable := entry.flags["movablekeys"]
		return entry.firstKey != 0 || movable
	}
}

func (p *Policy) keylessInvocationAllowed(command string, args []core.RedisArgument) bool {
	allKeys := p.hasAllKeyScope()
	switch command {
	case "ping", "echo", "time":
		return true
	case "dbsize", "randomkey":
		return allKeys
	case "keys":
		if len(args) != 1 {
			return false
		}
		pattern, err := decodeArgument(args[0])
		return err == nil && p.patternInsideScope(string(pattern))
	case "scan":
		for i := 1; i+1 < len(args); i++ {
			name, err := decodeArgument(args[i])
			if err == nil && strings.EqualFold(string(name), "MATCH") {
				pattern, err := decodeArgument(args[i+1])
				return err == nil && p.patternInsideScope(string(pattern))
			}
		}
		return allKeys
	case "eval_ro", "evalsha_ro", "fcall_ro":
		return true
	default:
		return false
	}
}

func (p *Policy) hasAllKeyScope() bool {
	return len(p.cfg.KeyPatterns) == 1 && p.cfg.KeyPatterns[0] == "*"
}

func (p *Policy) patternInsideScope(pattern string) bool {
	for _, allowed := range p.cfg.KeyPatterns {
		prefix := strings.TrimSuffix(allowed, "*")
		if strings.HasPrefix(pattern, prefix) && !strings.ContainsAny(pattern[:len(prefix)], "?[]\\") {
			return true
		}
	}
	return false
}

func decodeArgument(argument core.RedisArgument) ([]byte, error) {
	if (argument.String == nil) == (argument.Base64 == nil) {
		return nil, errors.New("each Redis argument must contain exactly one of string or base64")
	}
	if argument.String != nil {
		return []byte(*argument.String), nil
	}
	value, err := base64.StdEncoding.DecodeString(*argument.Base64)
	if err != nil {
		return nil, errors.New("Redis argument contains invalid base64")
	}
	return value, nil
}

func readOnlyFlags(flags []string) bool {
	hasRO := false
	for _, flag := range flags {
		switch strings.ToLower(flag) {
		case "ro", "access":
			if strings.EqualFold(flag, "ro") {
				hasRO = true
			}
		case "rw", "ow", "rm", "update", "insert", "delete", "write":
			return false
		}
	}
	return hasRO
}

func (p *Policy) keyAllowed(key string) bool {
	for _, pattern := range p.cfg.KeyPatterns {
		if strings.HasPrefix(key, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	return false
}

func (p *Policy) fingerprintKey(key string) string {
	h := sha256.New()
	_, _ = h.Write(p.keySalt)
	_, _ = h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil)[:12])
}
