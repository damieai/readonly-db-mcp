package redis

import (
	"context"
	"errors"
	"testing"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	modulepolicy "github.com/your-org/readonly-db-mcp/internal/dialects/redis/modules"
)

type searchInspector struct {
	fakeResolver
	reply any
	err   error
	calls int
}

func (f *searchInspector) Do(ctx context.Context, args ...interface{}) *redisdriver.Cmd {
	f.calls++
	c := redisdriver.NewCmd(ctx, args...)
	c.SetVal(f.reply)
	c.SetErr(f.err)
	return c
}
func searchInfo(name string, prefixes ...any) any {
	return []any{"index_name", name, "index_definition", []any{"key_type", "HASH", "prefixes", prefixes}}
}
func searchPolicy() *Policy {
	cfg := config.RedisConfig{KeyPatterns: []string{"tenant:*"}, MaxArgumentBytes: 4096, MaxKeysPerCommand: 32, ModuleObjectPatterns: map[string][]string{"FT.SEARCH": {"tenant-*"}, "FT.AGGREGATE": {"tenant-*"}}}
	catalog := map[string]*redisdriver.CommandInfo{}
	rules := map[string]modulepolicy.CommandRule{}
	moduleCommands := map[string]struct{}{}
	for _, command := range []string{"ft.search", "ft.aggregate"} {
		catalog[command] = &redisdriver.CommandInfo{ReadOnly: true, FirstKeyPos: 1, Flags: []string{"readonly"}}
		rules[command] = modulepolicy.CommandRule{ReadOnly: true, KeyModel: "index-prefix-attested"}
		moduleCommands[command] = struct{}{}
	}
	return newPolicy(cfg, catalog, nil, rules, moduleCommands)
}
func TestSearchPrefixProofPreservesAdvancedQueryAndFreezesAlias(t *testing.T) {
	p := searchPolicy()
	f := &searchInspector{reply: searchInfo("tenant-index", "tenant:doc:", "tenant:other:")}
	request := core.RedisRequest{Command: "FT.SEARCH", Arguments: []core.RedisArgument{arg("tenant-alias"), arg("(@category:{a|b})=>[KNN 5 @vector $v]"), arg("PARAMS"), arg("2"), arg("v"), arg("binary vector"), arg("DIALECT"), arg("2")}}
	_, wire, err := p.validate(context.Background(), f, request)
	if err != nil {
		t.Fatal(err)
	}
	if string(wire[1].([]byte)) != "tenant-index" || string(wire[2].([]byte)) != *request.Arguments[1].String {
		t.Fatalf("unexpected wire: %v", wire)
	}
	f.reply = searchInfo("tenant-index", "private:")
	if _, _, err := p.validate(context.Background(), f, request); err == nil {
		t.Fatal("index drift was not rechecked")
	}
}
func TestSearchPrefixProofRejectsAmbiguousAndEscapingMetadata(t *testing.T) {
	for name, reply := range map[string]any{
		"all keys":           searchInfo("tenant-index", ""),
		"mixed":              searchInfo("tenant-index", "tenant:", "private:"),
		"missing":            searchInfo("tenant-index"),
		"typed prefix":       searchInfo("tenant-index", 42),
		"alias escape":       searchInfo("private-index", "tenant:"),
		"odd fields":         []any{"index_name"},
		"duplicate":          []any{"index_name", "tenant-index", "index_name", "private-index"},
		"missing definition": map[string]any{"index_name": "tenant-index"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := searchPolicy().searchIndex(context.Background(), &searchInspector{reply: reply}, "ft.search", "tenant-index"); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
	if _, err := searchPolicy().searchIndex(context.Background(), &searchInspector{err: errors.New("no permission")}, "ft.search", "tenant-index"); err == nil {
		t.Fatal("inspection failure accepted")
	}
}
func TestSearchPrefixProofRESP3AndAggregate(t *testing.T) {
	f := &searchInspector{reply: map[any]any{"index_name": "tenant-index", "index_definition": map[any]any{"key_type": "JSON", "prefixes": []any{"tenant:"}}}}
	request := core.RedisRequest{Command: "FT.AGGREGATE", Arguments: []core.RedisArgument{arg("tenant-index"), arg("*"), arg("GROUPBY"), arg("1"), arg("@region"), arg("REDUCE"), arg("SUM"), arg("1"), arg("@amount"), arg("AS"), arg("total")}}
	if _, _, err := searchPolicy().validate(context.Background(), f, request); err != nil {
		t.Fatal(err)
	}
}

func TestSpellcheckDictionariesRequireTheirOwnScope(t *testing.T) {
	p := searchPolicy()
	p.cfg.ModuleObjectPatterns["FT.DICTDUMP"] = []string{"tenant-dictionary:*"}
	if !p.spellcheckDictionariesAllowed([]core.RedisArgument{arg("tenant-index"), arg("complex query"), arg("DISTANCE"), arg("2"), arg("TERMS"), arg("INCLUDE"), arg("tenant-dictionary:terms")}) {
		t.Fatal("scoped dictionary rejected")
	}
	if p.spellcheckDictionariesAllowed([]core.RedisArgument{arg("tenant-index"), arg("query"), arg("TERMS"), arg("EXCLUDE"), arg("private-dictionary")}) {
		t.Fatal("foreign dictionary accepted")
	}
}
