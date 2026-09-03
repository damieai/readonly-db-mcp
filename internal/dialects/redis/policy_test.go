package redis

import (
	"context"
	"encoding/base64"
	"testing"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	modulepolicy "github.com/your-org/readonly-db-mcp/internal/dialects/redis/modules"
)

type fakeResolver struct{ flags []redisdriver.KeyFlags }

func (f fakeResolver) CommandGetKeysAndFlags(ctx context.Context, args ...interface{}) *redisdriver.KeyFlagsCmd {
	cmd := redisdriver.NewKeyFlagsCmd(ctx, args...)
	cmd.SetVal(f.flags)
	return cmd
}

func TestPolicyRequiresSignedModuleRuleAndLogicalScope(t *testing.T) {
	commands := map[string]*redisdriver.CommandInfo{"ft.search": {Name: "ft.search", ReadOnly: true, Flags: []string{"readonly"}}}
	cfg := config.RedisConfig{KeyPatterns: []string{"analytics:*"}, MaxArgumentBytes: 1024, MaxKeysPerCommand: 8, ModuleObjectPatterns: map[string][]string{"FT.SEARCH": {"tenant-42:*"}}}
	moduleCommands := map[string]struct{}{"ft.search": {}}
	p := newPolicy(cfg, commands, []byte("salt"), nil, moduleCommands)
	request := core.RedisRequest{Command: "FT.SEARCH", Arguments: []core.RedisArgument{arg("tenant-42:index"), arg("*")}}
	if _, _, err := p.validate(context.Background(), fakeResolver{}, request); err == nil {
		t.Fatal("expected unsigned module command rejection")
	}
	p = newPolicy(cfg, commands, []byte("salt"), map[string]modulepolicy.CommandRule{"ft.search": {ReadOnly: true, KeyModel: "index-name"}}, moduleCommands)
	if _, _, err := p.validate(context.Background(), fakeResolver{}, request); err != nil {
		t.Fatal(err)
	}
	request.Arguments[0] = arg("other:index")
	if _, _, err := p.validate(context.Background(), fakeResolver{}, request); err == nil {
		t.Fatal("expected logical object scope rejection")
	}
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
	return newPolicy(config.RedisConfig{KeyPatterns: []string{"analytics:*"}, MaxArgumentBytes: 1024, MaxKeysPerCommand: 8, MaxScriptBytes: 1024, MaxReplyElements: 1000, AllowReadonlyScripts: true}, commands, []byte("test-salt"), nil, nil)
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
		{"duplicate scan match", fakeResolver{}, core.RedisRequest{Command: "SCAN", Arguments: []core.RedisArgument{arg("0"), arg("MATCH"), arg("analytics:*"), arg("MATCH"), arg("*")}}},
		{"unknown scan option", fakeResolver{}, core.RedisRequest{Command: "SCAN", Arguments: []core.RedisArgument{arg("0"), arg("MATCH"), arg("analytics:*"), arg("SURPRISE"), arg("1")}}},
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

func TestPolicyAllowsStrictScopedScanOptions(t *testing.T) {
	p := testPolicy()
	request := core.RedisRequest{Command: "SCAN", Arguments: []core.RedisArgument{arg("0"), arg("MATCH"), arg("analytics:daily:*"), arg("COUNT"), arg("100"), arg("TYPE"), arg("hash")}}
	if _, _, err := p.validate(context.Background(), fakeResolver{}, request); err != nil {
		t.Fatal(err)
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

func TestPolicyRejectsOversizedEncodedArgumentBeforeDecode(t *testing.T) {
	p := testPolicy()
	p.cfg.MaxArgumentBytes = 4
	encoded := base64.StdEncoding.EncodeToString([]byte("12345"))
	if _, _, err := p.validate(context.Background(), fakeResolver{}, core.RedisRequest{Command: "GET", Arguments: []core.RedisArgument{{Base64: &encoded}}}); err == nil {
		t.Fatal("expected oversized encoded argument rejection")
	}
	arguments := make([]core.RedisArgument, 10_001)
	for i := range arguments {
		arguments[i] = arg("")
	}
	if _, _, err := p.validate(context.Background(), fakeResolver{}, core.RedisRequest{Command: "GET", Arguments: arguments}); err == nil {
		t.Fatal("expected argument-count rejection")
	}
}

func TestRedisKeySlotHonorsHashTags(t *testing.T) {
	if redisKeySlot("analytics:{tenant-42}:a") != redisKeySlot("other:{tenant-42}:b") {
		t.Fatal("matching hash tags must map to the same Redis Cluster slot")
	}
	if redisKeySlot("analytics:{tenant-42}:a") == redisKeySlot("analytics:{tenant-43}:a") {
		t.Fatal("different hash tags unexpectedly mapped to the same test slot")
	}
}

func TestAtomicSlotValidationRejectsKeylessAndCrossSlotCommands(t *testing.T) {
	slot := -1
	first := redisKeySlot("analytics:{tenant-42}:a")
	if err := extendAtomicSlot(&slot, []int{first, first}); err != nil {
		t.Fatal(err)
	}
	if err := extendAtomicSlot(&slot, nil); err == nil {
		t.Fatal("expected keyless command rejection")
	}
	if err := extendAtomicSlot(&slot, []int{redisKeySlot("analytics:{tenant-43}:a")}); err == nil {
		t.Fatal("expected cross-slot rejection")
	}
}
