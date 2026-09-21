package elasticsearch

import (
	"context"
	"encoding/json"
	"path"
	"strings"
	"testing"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

func TestGlobContainmentAndIntersection(t *testing.T) {
	for _, test := range []struct {
		left               string
		right              []string
		contained, overlap bool
	}{
		{"reports-202?", []string{"reports-*"}, true, true},
		{"reports-*", []string{"reports-202?"}, false, true},
		{"a?", []string{"aa", "ab"}, false, true},
		{"a*", []string{"a", "a?*"}, true, true},
		{"ab*cd", []string{"a*d"}, true, true},
		{"logs-*", []string{"events-*"}, false, false},
		{"reports-*", []string{"reports-secret*"}, false, true},
		{"日志-?", []string{"日志-*"}, true, true},
	} {
		for _, intersection := range []bool{false, true} {
			got, err := globRelation(context.Background(), []string{test.left}, test.right, intersection)
			want := test.contained
			if intersection {
				want = test.overlap
			}
			if err != nil || got != want {
				t.Fatalf("%s %v intersection=%v got=%v err=%v", test.left, test.right, intersection, got, err)
			}
		}
	}
}
func TestGlobProofAgreesWithEnumeratedNames(t *testing.T) {
	patterns := []string{"a", "b", "*", "?", "a*", "*b", "a?*", "*ab*", "a*b?"}
	names := []string{""}
	for length := 0; length < 5; length++ {
		var next []string
		for _, n := range names {
			if len(n) == length {
				for _, c := range "abc" {
					next = append(next, n+string(c))
				}
			}
		}
		names = append(names, next...)
	}
	for _, a := range patterns {
		for _, b := range patterns {
			subset, err := globRelation(context.Background(), []string{a}, []string{b}, false)
			if err != nil {
				t.Fatal(err)
			}
			overlap, err := globRelation(context.Background(), []string{a}, []string{b}, true)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range names {
				ma, _ := path.Match(a, name)
				mb, _ := path.Match(b, name)
				if subset && ma && !mb {
					t.Fatalf("false containment %q in %q", a, b)
				}
				if !overlap && ma && mb {
					t.Fatalf("false disjointness %q %q", a, b)
				}
			}
		}
	}
}
func TestStrictJSONPreservesDataAndRejectsAmbiguity(t *testing.T) {
	for _, raw := range []string{`{"n":9007199254740993,"field":"delete","script":"state.count += 1;"}`, `[null,false,1.1234567890123456789]`} {
		if err := strictJSON([]byte(raw), 16, 100); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`{"x":1,"x":2}`, `{"x":{"y":1,"y":2}}`, `{} {}`, `{"x":NaN}`, "{\"x\":\"\xff\"}", strings.Repeat("[", 20) + "0" + strings.Repeat("]", 20)} {
		if strictJSON([]byte(raw), 16, 100) == nil {
			t.Fatalf("ambiguous JSON accepted: %q", raw)
		}
	}
}

func TestProofCPUWorkHonorsCancellationAndScalarLimits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := globRelation(ctx, []string{"reports-*"}, []string{"reports-*"}, false); err == nil {
		t.Fatal("cancelled scope proof succeeded")
	}
	if err := strictJSONContext(ctx, []byte(`{}`), 10, 100, 100); err == nil {
		t.Fatal("cancelled JSON validation succeeded")
	}
	if err := strictJSONContext(context.Background(), []byte(`{"a":"oversized"}`), 10, 100, 3); err == nil {
		t.Fatal("oversized scalar accepted")
	}
	if ok, err := globRelation(context.Background(), []string{"Reports-Alias"}, []string{"Reports-*"}, false); err != nil || !ok {
		t.Fatal("case-sensitive alias scope rejected")
	}
}
func readonlyProof() []byte {
	return []byte(`{"cluster":["monitor"],"indices":[{"names":["reports-*"],"privileges":["read","view_index_metadata"],"allow_restricted_indices":false}],"applications":[],"run_as":[],"global":[]}`)
}
func TestEffectiveAuthorityRejectsWritesUnknownAndFutureScope(t *testing.T) {
	cfg := &config.ElasticsearchConfig{AllowedIndices: []string{"reports-*"}}
	if err := validatePrivileges(context.Background(), readonlyProof(), cfg); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		strings.Replace(string(readonlyProof()), `"monitor"`, `"manage"`, 1),
		strings.Replace(string(readonlyProof()), `"read"`, `"write"`, 1),
		strings.Replace(string(readonlyProof()), `"reports-*"`, `"*"`, 1),
		strings.Replace(string(readonlyProof()), `false`, `true`, 1),
		strings.Replace(string(readonlyProof()), `"run_as":[]`, `"run_as":["*"]`, 1),
		strings.Replace(string(readonlyProof()), `"global":[]`, `"global":[],"new_authority":["all"]`, 1),
		`{"cluster":[],"indices":[]}`,
	} {
		if validatePrivileges(context.Background(), []byte(raw), cfg) == nil {
			t.Fatal("unsafe effective authority accepted")
		}
	}
	cfg.DeniedIndices = []string{"reports-private-*"}
	if validatePrivileges(context.Background(), readonlyProof(), cfg) == nil {
		t.Fatal("future denied indices not covered by containment proof")
	}
}
func TestResolvedAliasesAndDataStreams(t *testing.T) {
	cfg := &config.ElasticsearchConfig{AllowedIndices: []string{"reports-*"}, MaxResolvedIndices: 10}
	good := []byte(`{"indices":[],"aliases":[{"name":"reports-alias","indices":["reports-2026"]}],"data_streams":[{"name":"reports-events","backing_indices":[".ds-reports-events-2026.09.21-000001"],"timestamp_field":"@timestamp"}]}`)
	physical, err := validateResolution(context.Background(), good, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !physical["reports-2026"] || !physical[".ds-reports-events-2026.09.21-000001"] {
		t.Fatal("lost alias or data-stream source")
	}
	if _, err := validateResolution(context.Background(), []byte(strings.Replace(string(good), `"reports-2026"`, `"private-2026"`, 1)), cfg); err == nil {
		t.Fatal("alias escape accepted")
	}
	if err := validateMappings([]byte(`{"private-2026":{}}`), physical); err == nil {
		t.Fatal("alias race escaped output scope")
	}
}

func TestResolvedBudgetCountsUniquePhysicalSources(t *testing.T) {
	cfg := &config.ElasticsearchConfig{AllowedIndices: []string{"reports-*"}, MaxResolvedIndices: 1}
	data := []byte(`{"indices":[{"name":"reports-2026"}],"aliases":[{"name":"reports-alias","indices":["reports-2026"]}],"data_streams":[]}`)
	if _, err := validateResolution(context.Background(), data, cfg); err != nil {
		t.Fatal(err)
	}
	data = []byte(`{"indices":[],"aliases":[],"data_streams":[{"name":"reports-stream","backing_indices":["reports-imported"],"timestamp_field":"@timestamp"}]}`)
	if _, err := validateResolution(context.Background(), data, cfg); err != nil {
		t.Fatal("scoped custom backing index rejected:", err)
	}
}
func TestOperationEffectsDoNotFollowHTTPMethods(t *testing.T) {
	for _, version := range []string{"8.19.21", "9.1.10"} {
		for _, name := range []string{"resolve", "mappings"} {
			op := catalog[version][name]
			if !op.Implemented || op.Effect != "read" || len(op.SHA256) != 64 {
				t.Fatal("unreviewed metadata route")
			}
		}
		if catalog[version]["search"].Effect != "read" || catalog[version]["close_point_in_time"].Effect != "owned_context" || catalog[version]["indices.refresh"].Effect != "mutation" {
			t.Fatal("HTTP method substituted for effect classification")
		}
	}
	for _, name := range []string{"search", "search_template", "open_point_in_time"} {
		err := metadataOperation("8.19.21", name)
		if err == nil || !strings.HasPrefix(err.Error(), "capability_unavailable:") {
			t.Fatalf("advanced reads mislabeled: %v", err)
		}
	}
	if err := metadataOperation("8.19.21", "bulk"); err == nil || !strings.HasPrefix(err.Error(), "mutation_forbidden:") {
		t.Fatal("bulk not classified as mutation")
	}
	data, _ := json.Marshal(capabilities("8.19.21"))
	if strings.Contains(string(data), "forbidden") {
		t.Fatal("unimplemented read capabilities labeled forbidden")
	}
}
