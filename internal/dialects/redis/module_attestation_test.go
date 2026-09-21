package redis

import (
	redisdriver "github.com/redis/go-redis/v9"
	modulepolicy "github.com/your-org/readonly-db-mcp/internal/dialects/redis/modules"
	"testing"
)

func TestBuiltinModuleRequiresExplicitSignedBuildBinding(t *testing.T) {
	live := installedModule{Version: 1}
	profile := modulepolicy.ModuleIdentity{ArtifactPath: "/opt/redis-server", VendorBuildID: "0123456789abcdef"}
	if moduleArtifactMatches(profile, live, "0123456789abcdef") {
		t.Fatal("pathless module accepted without builtin attestation")
	}
	profile.Builtin = true
	if !moduleArtifactMatches(profile, live, "0123456789abcdef") {
		t.Fatal("matching builtin profile rejected")
	}
	if moduleArtifactMatches(profile, live, "fedcba9876543210") {
		t.Fatal("server build drift accepted")
	}
	live.Path = "/opt/unexpected.so"
	if moduleArtifactMatches(profile, live, "0123456789abcdef") {
		t.Fatal("loadable module accepted as builtin")
	}
}

func TestRestorePinnedLoopbackNeverAdmitsNewDestination(t *testing.T) {
	pinned := map[string]string{"127.0.0.1:12345": "127.0.0.1:12345"}
	for _, test := range []struct{ addr, node, want string }{
		{":12345", "127.0.0.1:12345", "127.0.0.1:12345"},
		{":12346", "127.0.0.1:12345", ":12346"},
		{":12345", "unknown:12345", ":12345"},
		{"192.0.2.1:12345", "127.0.0.1:12345", "192.0.2.1:12345"},
	} {
		options := &redisdriver.Options{Addr: test.addr, NodeAddress: test.node}
		restorePinnedLoopback(options, pinned)
		if options.Addr != test.want {
			t.Fatalf("got %q want %q", options.Addr, test.want)
		}
	}
}
