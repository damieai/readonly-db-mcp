package redis

import (
	"testing"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
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
