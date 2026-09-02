package redis

import (
	"errors"
	"fmt"
	"strings"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
)

func attestACL(value any, cfg config.RedisConfig, catalog map[string]*redisdriver.CommandInfo) error {
	m, ok := stringMap(value)
	if !ok {
		return errors.New("Redis ACL response has an unsupported shape")
	}
	flags := stringSlice(m["flags"])
	if !containsFold(flags, "on") || containsFold(flags, "nopass") {
		return errors.New("Redis ACL user must be enabled and password protected")
	}
	if selectors := anySlice(m["selectors"]); len(selectors) != 0 {
		return errors.New("Redis ACL selectors are not supported because they can broaden scope")
	}
	channels := stringSlice(m["channels"])
	if len(channels) != 0 && !(len(channels) == 1 && channels[0] == "") {
		return errors.New("Redis ACL user must not have Pub/Sub channel access")
	}
	keys := stringSlice(m["keys"])
	if len(keys) == 1 && strings.Contains(keys[0], " ") {
		keys = strings.Fields(keys[0])
	}
	want := make(map[string]struct{}, len(cfg.KeyPatterns))
	for _, pattern := range cfg.KeyPatterns {
		want["%R~"+pattern] = struct{}{}
	}
	for _, rule := range keys {
		if _, ok := want[rule]; !ok {
			return fmt.Errorf("Redis ACL key rule exceeds configured read scope")
		}
		delete(want, rule)
	}
	if len(want) != 0 {
		return errors.New("Redis ACL is missing a configured read-key pattern")
	}
	commands, _ := m["commands"].(string)
	if commands == "" {
		parts := stringSlice(m["commands"])
		commands = strings.Join(parts, " ")
	}
	if err := attestCommandRules(commands, catalog); err != nil {
		return err
	}
	return nil
}

func attestCommandRules(rules string, catalog map[string]*redisdriver.CommandInfo) error {
	fields := strings.Fields(strings.ToLower(rules))
	hasReset := false
	internal := map[string]struct{}{"acl|getuser": {}, "acl|whoami": {}, "command": {}, "command|info": {}, "command|list": {}, "command|getkeysandflags": {}, "info": {}, "module|list": {}, "ping": {}, "select": {}}
	for _, rule := range fields {
		if rule == "-@all" {
			hasReset = true
			continue
		}
		if strings.HasPrefix(rule, "-") {
			continue
		}
		if rule == "+@read" {
			continue
		}
		if strings.HasPrefix(rule, "+@") {
			return errors.New("Redis ACL grants a command category other than @read")
		}
		if !strings.HasPrefix(rule, "+") {
			continue
		}
		name := strings.TrimPrefix(rule, "+")
		if _, ok := internal[name]; ok {
			continue
		}
		info := catalog[name]
		if info == nil {
			info = catalog[strings.Split(name, "|")[0]]
		}
		if info == nil || !info.ReadOnly || containsFold(info.Flags, "write") {
			return fmt.Errorf("Redis ACL explicitly grants unproven command %q", name)
		}
	}
	if !hasReset {
		return errors.New("Redis ACL command rules must start from -@all")
	}
	return nil
}

func stringMap(v any) (map[string]any, bool) {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]any, len(x))
		for k, value := range x {
			out[fmt.Sprint(k)] = value
		}
		return out, true
	case map[string]interface{}:
		return x, true
	case []interface{}:
		if len(x)%2 != 0 {
			return nil, false
		}
		out := make(map[string]any, len(x)/2)
		for i := 0; i < len(x); i += 2 {
			out[fmt.Sprint(x[i])] = x[i+1]
		}
		return out, true
	default:
		return nil, false
	}
}

func stringSlice(v any) []string {
	if value, ok := v.(string); ok {
		if value == "" {
			return nil
		}
		return []string{value}
	}
	var out []string
	for _, value := range anySlice(v) {
		out = append(out, fmt.Sprint(value))
	}
	return out
}
func anySlice(v any) []any { x, _ := v.([]interface{}); return x }
func containsFold(values []string, value string) bool {
	for _, current := range values {
		if strings.EqualFold(current, value) {
			return true
		}
	}
	return false
}
