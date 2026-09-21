package redis

import (
	"testing"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
	modulepolicy "github.com/your-org/readonly-db-mcp/internal/dialects/redis/modules"
)

func TestAttestACLRequiresExactReadScope(t *testing.T) {
	catalog := map[string]*redisdriver.CommandInfo{"get": {ReadOnly: true, Flags: []string{"readonly"}}}
	value := []interface{}{"flags", []interface{}{"on"}, "commands", "-@all +@read +command|getkeysandflags +acl|getuser +acl|whoami", "keys", "%R~analytics:*", "channels", "", "selectors", []interface{}{}}
	if err := attestACL(value, config.RedisConfig{KeyPatterns: []string{"analytics:*"}}, catalog); err != nil {
		t.Fatal(err)
	}
	bad := []interface{}{"flags", []interface{}{"on"}, "commands", "-@all +@read +set", "keys", "%R~analytics:*", "channels", "", "selectors", []interface{}{}}
	if err := attestACL(bad, config.RedisConfig{KeyPatterns: []string{"analytics:*"}}, catalog); err == nil {
		t.Fatal("expected write grant rejection")
	}
	wide := []interface{}{"flags", []interface{}{"on"}, "commands", "-@all +@read", "keys", "%R~*", "channels", "", "selectors", []interface{}{}}
	if err := attestACL(wide, config.RedisConfig{KeyPatterns: []string{"analytics:*"}}, catalog); err == nil {
		t.Fatal("expected broad key scope rejection")
	}
}

func TestLegacyModuleIndexACLRequiresDisjointSignedScopeAndNoWrites(t *testing.T) {
	cfg := config.RedisConfig{KeyPatterns: []string{"tenant:*"}, ModuleObjectPatterns: map[string][]string{"FT.SEARCH": {"search-*"}}}
	rules := map[string]modulepolicy.CommandRule{"ft.search": {ReadOnly: true, KeyModel: "index-prefix-attested"}}
	catalog := map[string]*redisdriver.CommandInfo{"get": {ReadOnly: true}, "ft.search": {ReadOnly: true}, "set": {Flags: []string{"write"}}}
	value := map[string]any{"flags": []any{"on"}, "commands": "-@all +get +ft.search", "keys": "%R~tenant:* ~search-*", "channels": "", "selectors": []any{}}
	if err := attestACL(value, cfg, catalog, rules); err != nil {
		t.Fatal(err)
	}
	if err := attestACL(value, cfg, catalog); err == nil {
		t.Fatal("unsigned logical key rule accepted")
	}
	value["commands"] = "-@all +get +ft.search +set"
	if err := attestACL(value, cfg, catalog, rules); err == nil {
		t.Fatal("logical key rule enabled write authority")
	}
	cfg.ModuleObjectPatterns["FT.SEARCH"] = []string{"tenant:*"}
	if moduleIndexACLRule("~tenant:*", cfg, rules) {
		t.Fatal("overlapping document key rule promoted to RW")
	}
	if moduleIndexACLRule("~*", cfg, rules) {
		t.Fatal("unbounded logical key rule accepted")
	}
}
