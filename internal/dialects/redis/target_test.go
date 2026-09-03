package redis

import (
	"strings"
	"testing"
	"time"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
)

func TestNormalizeRedisBoundsDepthElementsAndBinary(t *testing.T) {
	value := []interface{}{[]byte("ok"), []byte{0xff, 0x00}, []interface{}{int64(1), int64(2)}}
	normalized, count, truncated, err := normalizeRedis(value, 0, &normalizer{maxDepth: 4, maxElements: 3, maxCell: 16})
	if err != nil {
		t.Fatal(err)
	}
	if normalized == nil || count == 0 || !truncated {
		t.Fatalf("normalized=%#v count=%d truncated=%v", normalized, count, truncated)
	}
	_, _, _, err = normalizeRedis([]interface{}{[]interface{}{1}}, 0, &normalizer{maxDepth: 0, maxElements: 10, maxCell: 16})
	if err == nil || !strings.Contains(err.Error(), "nesting") {
		t.Fatal("expected nesting rejection")
	}
}

func TestNormalizeRedisCapsMapPreallocation(t *testing.T) {
	input := make(map[interface{}]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = i
	}
	normalized, _, truncated, err := normalizeRedis(input, 0, &normalizer{maxDepth: 4, maxElements: 1, maxCell: 16})
	if err != nil {
		t.Fatal(err)
	}
	pairs := normalized.([]any)
	if !truncated || len(pairs) != 1 || cap(pairs) != 1 {
		t.Fatalf("len=%d cap=%d truncated=%v", len(pairs), cap(pairs), truncated)
	}
}

func TestNormalizeRedisBoundsStringMapKeys(t *testing.T) {
	normalized, _, truncated, err := normalizeRedis(map[string]interface{}{strings.Repeat("k", 100): "value"}, 0, &normalizer{maxDepth: 4, maxElements: 4, maxCell: 8})
	if err != nil {
		t.Fatal(err)
	}
	pairs := normalized.([]any)
	key := pairs[0].([]any)[0].(string)
	if !truncated || len(key) != 8 {
		t.Fatalf("key length=%d truncated=%v", len(key), truncated)
	}
}

func TestCatalogRevisionIncludesKeyExtractionMetadata(t *testing.T) {
	a := map[string]*redisdriver.CommandInfo{"get": {Name: "get", ReadOnly: true, FirstKeyPos: 1, LastKeyPos: 1, StepCount: 1, Flags: []string{"readonly"}}}
	b := map[string]*redisdriver.CommandInfo{"get": {Name: "get", ReadOnly: true, FirstKeyPos: 0, LastKeyPos: 0, StepCount: 0, Flags: []string{"readonly"}}}
	if catalogRevision("8.0.0", a) == catalogRevision("8.0.0", b) {
		t.Fatal("key extraction drift was not detected")
	}
}

func TestTargetRejectsStaleAttestation(t *testing.T) {
	target := &Target{cfg: &config.TargetConfig{Redis: config.RedisConfig{CatalogMaxAge: time.Second}}}
	target.healthy.Store(true)
	target.lastAttested.Store(time.Now().Add(-2 * time.Second).UnixNano())
	if err := target.requireHealthy(); err == nil {
		t.Fatal("expected stale attestation rejection")
	}
}

func TestSupportedRedisVersions(t *testing.T) {
	for _, version := range []string{"7.2.0", "7.4.1", "8.0.0", "8.2.3"} {
		if !supportedVersion(version) {
			t.Fatalf("version %s rejected", version)
		}
	}
	for _, version := range []string{"6.2.0", "7.0.15", "9.0.0", "bad"} {
		if supportedVersion(version) {
			t.Fatalf("version %s accepted", version)
		}
	}
}
