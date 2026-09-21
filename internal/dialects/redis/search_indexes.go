package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type moduleInspector interface {
	Do(context.Context, ...interface{}) *redisdriver.Cmd
}

func (p *Policy) spellcheckDictionariesAllowed(args []core.RedisArgument) bool {
	if len(args) < 2 {
		return false
	}
	for i := 2; i < len(args); {
		option, err := decodeArgument(args[i])
		if err != nil {
			return false
		}
		switch strings.ToUpper(string(option)) {
		case "DISTANCE", "DIALECT":
			if i+1 >= len(args) {
				return false
			}
			i += 2
		case "TERMS":
			if i+2 >= len(args) {
				return false
			}
			mode, err := decodeArgument(args[i+1])
			if err != nil || (!strings.EqualFold(string(mode), "INCLUDE") && !strings.EqualFold(string(mode), "EXCLUDE")) {
				return false
			}
			if !p.moduleObjectAllowed("FT.DICTDUMP", args[i+2:i+3]) {
				return false
			}
			i += 3
		default:
			return false
		}
	}
	return true
}

// searchIndex resolves aliases to the live canonical name and proves that every
// indexed key prefix is contained in the target's read-key scope. Filters are
// intentionally not used as an authorization proof.
func (p *Policy) searchIndex(ctx context.Context, client moduleInspector, command, name string) (string, error) {
	if !p.moduleObjectAllowed(strings.ToUpper(command), []core.RedisArgument{{String: &name}}) {
		return "", errors.New("Redis Search index is outside the configured logical scope")
	}
	raw, err := client.Do(ctx, "FT.INFO", name).Result()
	if err != nil {
		return "", errors.New("inspect Redis Search index prefixes")
	}
	return p.checkSearchIndex(raw, command)
}

func (p *Policy) checkSearchIndex(raw any, command string) (string, error) {
	info, err := strictModuleMap(raw)
	if err != nil {
		return "", err
	}
	canonical, ok := moduleString(info["index_name"])
	if !ok || canonical == "" || !p.moduleObjectAllowed(strings.ToUpper(command), []core.RedisArgument{{String: &canonical}}) {
		return "", errors.New("Redis Search canonical index is outside the configured logical scope")
	}
	definition, err := strictModuleMap(info["index_definition"])
	if err != nil {
		return "", err
	}
	kind, ok := moduleString(definition["key_type"])
	if !ok || (kind != "HASH" && kind != "JSON") {
		return "", errors.New("Redis Search index has an unverified key type")
	}
	prefixes, ok := definition["prefixes"].([]interface{})
	if !ok || len(prefixes) == 0 || len(prefixes) > p.cfg.MaxKeysPerCommand {
		return "", errors.New("Redis Search index prefixes are missing or exceed the resource limit")
	}
	for _, value := range prefixes {
		prefix, ok := moduleString(value)
		if !ok || len(prefix) > p.cfg.MaxArgumentBytes {
			return "", errors.New("Redis Search index prefix has an invalid shape")
		}
		inside := false
		for _, allowed := range p.cfg.KeyPatterns {
			// Config only admits literal prefixes followed by a single '*'.
			if strings.HasPrefix(prefix, strings.TrimSuffix(allowed, "*")) {
				inside = true
				break
			}
		}
		if !inside {
			return "", errors.New("Redis Search index reads keys outside the configured target")
		}
	}
	return canonical, nil
}

func (p *Policy) attestSearchIndexes(ctx context.Context, client moduleInspector) error {
	var commands []string
	for name, rule := range p.trustedModuleCommands {
		if rule.KeyModel == "index-prefix-attested" {
			commands = append(commands, name)
		}
	}
	if len(commands) == 0 {
		return nil
	}
	raw, err := client.Do(ctx, "FT._LIST").Result()
	if err != nil {
		return errors.New("enumerate Redis Search indexes for prefix attestation")
	}
	indexes, ok := raw.([]interface{})
	if !ok || len(indexes) > p.cfg.MaxKeysPerCommand {
		return errors.New("Redis Search index inventory is invalid or exceeds the resource limit")
	}
	for _, item := range indexes {
		name, ok := moduleString(item)
		if !ok || name == "" {
			return errors.New("Redis Search index inventory contains an invalid name")
		}
		for _, command := range commands {
			if p.moduleObjectAllowed(strings.ToUpper(command), []core.RedisArgument{{String: &name}}) {
				if _, err := p.searchIndex(ctx, client, command, name); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func moduleString(value any) (string, bool) {
	switch value := value.(type) {
	case string:
		return value, true
	case []byte:
		return string(value), true
	default:
		return "", false
	}
}

// FT.INFO is a RESP2 alternating array or a RESP3 map. Do not coerce arbitrary
// values to strings or accept duplicate fields in security metadata.
func strictModuleMap(raw any) (map[string]any, error) {
	result := make(map[string]any)
	add := func(key, value any) error {
		name, ok := moduleString(key)
		if !ok || name == "" {
			return errors.New("Redis module metadata has an invalid field name")
		}
		if _, exists := result[name]; exists {
			return errors.New("Redis module metadata has duplicate fields")
		}
		result[name] = value
		return nil
	}
	switch value := raw.(type) {
	case []interface{}:
		if len(value)%2 != 0 {
			return nil, errors.New("Redis module metadata has an odd field count")
		}
		for i := 0; i < len(value); i += 2 {
			if err := add(value[i], value[i+1]); err != nil {
				return nil, err
			}
		}
	case map[interface{}]interface{}:
		for k, v := range value {
			if err := add(k, v); err != nil {
				return nil, err
			}
		}
	case map[string]interface{}:
		for k, v := range value {
			if err := add(k, v); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("Redis module metadata has an unsupported shape")
	}
	return result, nil
}
