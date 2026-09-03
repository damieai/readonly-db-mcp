package redis

import (
	"context"
	"testing"

	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
)

func TestAgreedEndpoint(t *testing.T) {
	got, err := agreedEndpoint([]string{"a:1", "b:1", "a:1"}, 2)
	if err != nil || got != "a:1" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := agreedEndpoint([]string{"a:1", "b:1"}, 2); err == nil {
		t.Fatal("expected quorum failure")
	}
}

func TestReplicaQuorumAllowsMultipleAgreedCandidates(t *testing.T) {
	got := quorumEndpoints([]string{"a:1", "b:1", "a:1", "b:1"}, 2)
	if len(got) != 2 || got[0] != "a:1" || got[1] != "b:1" {
		t.Fatalf("candidates=%v", got)
	}
}

func TestValidateClusterSlots(t *testing.T) {
	allow := config.RedisEndpointAllowlist{CIDRs: []string{"10.0.0.0/8"}}
	slots := []redisdriver.ClusterSlot{{Start: 0, End: 16383, Nodes: []redisdriver.ClusterNode{{Addr: "10.1.2.3:6379"}}}}
	nodes, err := validateClusterSlots(context.Background(), slots, allow)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("nodes=%v err=%v", nodes, err)
	}
	slots[0].End = 100
	if _, err := validateClusterSlots(context.Background(), slots, allow); err == nil {
		t.Fatal("expected incomplete coverage rejection")
	}
}

func TestReplicaPrimaryHookAllowsOnlyInternalCommandIntrospection(t *testing.T) {
	hook := clusterPrimaryIntrospectionOnly{}
	next := func(context.Context, redisdriver.Cmder) error { return nil }
	process := hook.ProcessHook(next)
	if err := process(context.Background(), redisdriver.NewCmd(context.Background(), "get", "k")); err == nil {
		t.Fatal("expected primary data-read rejection")
	}
	if err := process(context.Background(), redisdriver.NewCmd(context.Background(), "command", "getkeysandflags", "get", "k")); err != nil {
		t.Fatal(err)
	}
}

func TestClusterReplicaCannotBelongToMultiplePrimaries(t *testing.T) {
	slots := []redisdriver.ClusterSlot{
		{Start: 0, End: 1, Nodes: []redisdriver.ClusterNode{{Addr: "10.0.0.1:6379"}, {Addr: "10.0.0.3:6379"}}},
		{Start: 2, End: 3, Nodes: []redisdriver.ClusterNode{{Addr: "10.0.0.2:6379"}, {Addr: "10.0.0.3:6379"}}},
	}
	if _, err := clusterReplicaAssignments(slots, "replica"); err == nil {
		t.Fatal("expected conflicting replica assignment rejection")
	}
}
