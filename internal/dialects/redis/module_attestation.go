package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	redisdriver "github.com/redis/go-redis/v9"
	modulepolicy "github.com/your-org/readonly-db-mcp/internal/dialects/redis/modules"
)

type installedModule struct {
	Version int
	Path    string
}

func attestModules(ctx context.Context, client redisdriver.UniversalClient, profiles *modulepolicy.Set, catalog map[string]*redisdriver.CommandInfo, redisVersion string) (map[string]modulepolicy.CommandRule, map[string]struct{}, error) {
	raw, err := client.Do(ctx, "MODULE", "LIST").Result()
	if err != nil {
		return nil, nil, fmt.Errorf("inspect installed Redis modules")
	}
	installed, err := parseModuleInventory(raw)
	if err != nil {
		return nil, nil, err
	}
	trusted := map[string]modulepolicy.CommandRule{}
	moduleCommands := map[string]struct{}{}
	if len(installed) == 0 {
		if profiles != nil && !profiles.Empty() {
			return nil, nil, fmt.Errorf("configured Redis module profiles do not match the empty live inventory")
		}
		return trusted, moduleCommands, nil
	}
	if profiles == nil || profiles.Empty() {
		return nil, nil, fmt.Errorf("Redis modules require signed module profiles")
	}
	if len(installed) != len(profiles.Profiles()) {
		return nil, nil, fmt.Errorf("Redis module inventory does not exactly match configured profiles")
	}
	for name, identity := range installed {
		profile, ok := profiles.Profile(name)
		if !ok || profile.Module.Version != identity.Version || profile.Module.ArtifactPath != identity.Path {
			return nil, nil, fmt.Errorf("Redis module %q identity does not match its signed profile", name)
		}
		if !compatibleRedisVersion(profile.Module.RedisCompatibility, redisVersion) {
			return nil, nil, fmt.Errorf("Redis module %q profile is incompatible with the live Redis version", name)
		}
		live, err := client.CommandList(ctx, &redisdriver.FilterBy{Module: name}).Result()
		if err != nil {
			return nil, nil, fmt.Errorf("enumerate commands for Redis module %q", name)
		}
		if len(live) != len(profile.Commands) {
			return nil, nil, fmt.Errorf("Redis module %q command set does not match its signed profile", name)
		}
		seenCommands := make(map[string]struct{}, len(live))
		for _, command := range live {
			lowerCommand := strings.ToLower(command)
			if _, duplicate := seenCommands[lowerCommand]; duplicate {
				return nil, nil, fmt.Errorf("Redis module %q command inventory contains duplicates", name)
			}
			seenCommands[lowerCommand] = struct{}{}
			moduleCommands[lowerCommand] = struct{}{}
			rule, exists := profile.Commands[strings.ToUpper(command)]
			info := catalog[strings.ToLower(command)]
			if !exists || info == nil {
				return nil, nil, fmt.Errorf("Redis module %q exposes an unprofiled command", name)
			}
			if rule.ReadOnly && !rule.ExternalSideEffects && info.ReadOnly && !containsFold(info.Flags, "write") {
				trusted[strings.ToLower(command)] = rule
			}
		}
		for command := range profile.Commands {
			if _, ok := seenCommands[strings.ToLower(command)]; !ok {
				return nil, nil, fmt.Errorf("Redis module %q is missing a profiled command", name)
			}
		}
	}
	return trusted, moduleCommands, nil
}

func parseModuleInventory(raw any) (map[string]installedModule, error) {
	rows, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("Redis module inventory has an unexpected shape")
	}
	result := map[string]installedModule{}
	for _, row := range rows {
		fields := map[string]string{}
		switch value := row.(type) {
		case []interface{}:
			for i := 0; i+1 < len(value); i += 2 {
				fields[strings.ToLower(fmt.Sprint(value[i]))] = fmt.Sprint(value[i+1])
			}
		case map[interface{}]interface{}:
			for key, item := range value {
				fields[strings.ToLower(fmt.Sprint(key))] = fmt.Sprint(item)
			}
		case map[string]interface{}:
			for key, item := range value {
				fields[strings.ToLower(key)] = fmt.Sprint(item)
			}
		default:
			return nil, fmt.Errorf("Redis module inventory entry has an unexpected shape")
		}
		name := strings.ToLower(fields["name"])
		version, err := strconv.Atoi(fields["ver"])
		if name == "" || err != nil || version < 1 {
			return nil, fmt.Errorf("Redis module inventory identity is incomplete")
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("duplicate Redis module inventory entry")
		}
		path := fields["path"]
		if path == "" {
			return nil, fmt.Errorf("Redis module inventory path is missing")
		}
		result[name] = installedModule{Version: version, Path: path}
	}
	return result, nil
}

func compatibleRedisVersion(allowed []string, live string) bool {
	for _, value := range allowed {
		if live == value || strings.HasPrefix(live, value+".") {
			return true
		}
	}
	return false
}
