package redis

import (
	"strings"
	"testing"
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
