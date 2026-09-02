package redis

import (
	"context"
	"testing"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

type fakeResolver struct{ flags []redisdriver.KeyFlags }

func (f fakeResolver) CommandGetKeysAndFlags(ctx context.Context, args ...interface{}) *redisdriver.KeyFlagsCmd {
	cmd := redisdriver.NewKeyFlagsCmd(ctx, args...)
	cmd.SetVal(f.flags)
	return cmd
}
func str(value string) *string            { return &value }
func arg(value string) core.RedisArgument { return core.RedisArgument{String: str(value)} }

func testPolicy() *Policy {
	commands := map[string]*redisdriver.CommandInfo{
		"get":          {Name: "get", ReadOnly: true, FirstKeyPos: 1, Flags: []string{"readonly"}},
		"set":          {Name: "set", ReadOnly: false, FirstKeyPos: 1, Flags: []string{"write"}},
		"scan":         {Name: "scan", ReadOnly: true, Flags: []string{"readonly"}},
		"keys":         {Name: "keys", ReadOnly: true, Flags: []string{"readonly"}},
		"eval_ro":      {Name: "eval_ro", ReadOnly: true, Flags: []string{"readonly"}},
		"xread":        {Name: "xread", ReadOnly: true, Flags: []string{"readonly", "movablekeys"}},
		"memory":       {Name: "memory", ReadOnly: false},
		"memory|usage": {Name: "memory|usage", ReadOnly: true, FirstKeyPos: 2, Flags: []string{"readonly"}},
	}
	return newPolicy(config.RedisConfig{KeyPatterns: []string{"analytics:*"}, MaxArgumentBytes: 1024, MaxKeysPerCommand: 8, MaxScriptBytes: 1024, AllowReadonlyScripts: true}, commands, []byte("test-salt"))
}

func TestPolicyAllowsAdvancedScopedReads(t *testing.T) {
	p := testPolicy()
	v, _, err := p.validate(context.Background(), fakeResolver{flags: []redisdriver.KeyFlags{{Key: "analytics:item:1", Flags: []string{"RO", "access"}}}}, core.RedisRequest{Command: "GET", Arguments: []core.RedisArgument{arg("analytics:item:1")}})
	if err != nil {
		t.Fatal(err)
	}
	if v.KeyCount != 1 || len(v.KeyFingerprints) != 1 {
		t.Fatalf("validation=%#v", v)
	}
	if _, _, err := p.validate(context.Background(), fakeResolver{}, core.RedisRequest{Command: "SCAN", Arguments: []core.RedisArgument{arg("0"), arg("MATCH"), arg("analytics:daily:*")}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.validate(context.Background(), fakeResolver{}, core.RedisRequest{Command: "KEYS", Arguments: []core.RedisArgument{arg("analytics:*")}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.validate(context.Background(), fakeResolver{flags: []redisdriver.KeyFlags{{Key: "analytics:events", Flags: []string{"RO", "access"}}}}, core.RedisRequest{Command: "XREAD", Arguments: []core.RedisArgument{arg("STREAMS"), arg("analytics:events"), arg("0")}}); err != nil {
		t.Fatal(err)
	}
	validation, wire, err := p.validate(context.Background(), fakeResolver{flags: []redisdriver.KeyFlags{{Key: "analytics:item:1", Flags: []string{"RO"}}}}, core.RedisRequest{Command: "MEMORY", Arguments: []core.RedisArgument{arg("USAGE"), arg("analytics:item:1")}})
	if err != nil {
		t.Fatal(err)
	}
	if validation.Command != "MEMORY|USAGE" || wire[0] != "MEMORY" {
		t.Fatalf("validation=%#v wire=%#v", validation, wire)
	}
}

func TestPolicyRejectsWritesAndScopeEscape(t *testing.T) {
	p := testPolicy()
	tests := []struct {
		name     string
		resolver fakeResolver
		request  core.RedisRequest
	}{
		{"catalog write", fakeResolver{}, core.RedisRequest{Command: "SET", Arguments: []core.RedisArgument{arg("analytics:x"), arg("v")}}},
		{"write key flag", fakeResolver{flags: []redisdriver.KeyFlags{{Key: "analytics:x", Flags: []string{"RW", "update"}}}}, core.RedisRequest{Command: "GET", Arguments: []core.RedisArgument{arg("analytics:x")}}},
		{"outside key", fakeResolver{flags: []redisdriver.KeyFlags{{Key: "private:x", Flags: []string{"RO"}}}}, core.RedisRequest{Command: "GET", Arguments: []core.RedisArgument{arg("private:x")}}},
		{"scan escape", fakeResolver{}, core.RedisRequest{Command: "SCAN", Arguments: []core.RedisArgument{arg("0")}}},
		{"keys escape", fakeResolver{}, core.RedisRequest{Command: "KEYS", Arguments: []core.RedisArgument{arg("*")}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := p.validate(context.Background(), test.resolver, test.request); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestPolicyAcceptsReadOnlyScriptAndRejectsBinaryAmbiguity(t *testing.T) {
	p := testPolicy()
	request := core.RedisRequest{Command: "EVAL_RO", Arguments: []core.RedisArgument{arg("return redis.call('GET', KEYS[1])"), arg("1"), arg("analytics:x")}}
	if _, _, err := p.validate(context.Background(), fakeResolver{}, request); err == nil {
		t.Fatal("expected scoped script rejection")
	}
	p.cfg.KeyPatterns = []string{"*"}
	_, _, err := p.validate(context.Background(), fakeResolver{flags: []redisdriver.KeyFlags{{Key: "analytics:x", Flags: []string{"RO"}}}}, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.validate(context.Background(), fakeResolver{}, core.RedisRequest{Command: "GET", Arguments: []core.RedisArgument{{}}}); err == nil {
		t.Fatal("expected ambiguous argument rejection")
	}
}
