package mcpserver

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	redisdriver "github.com/redis/go-redis/v9"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	modulepolicy "github.com/your-org/readonly-db-mcp/internal/dialects/redis/modules"
	"github.com/your-org/readonly-db-mcp/internal/registry"
)

// This fixture owns its process, role, indexes and documents. Its ephemeral
// signer is test-only and never creates a production trust profile.
func vectorRedisFixture(t *testing.T, protocol int) (*redisdriver.Client, *redisdriver.Client, *config.Config) {
	t.Helper()
	binaryPath, searchPath := os.Getenv("READONLY_DB_MCP_REDIS_SERVER"), os.Getenv("READONLY_DB_MCP_REDIS_SEARCH_MODULE")
	if binaryPath == "" || searchPath == "" {
		t.Skip("local Redis/Search artifacts are not configured")
	}
	jsonPath := os.Getenv("READONLY_DB_MCP_REDIS_JSON_MODULE")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		t.Fatal(err)
	}
	adminSecret := base64.RawURLEncoding.EncodeToString(secretBytes)
	if _, err := rand.Read(secretBytes); err != nil {
		t.Fatal(err)
	}
	readerSecret := base64.RawURLEncoding.EncodeToString(secretBytes)
	dir := t.TempDir()
	conf := fmt.Sprintf("bind 127.0.0.1\nport %d\nprotected-mode yes\nrequirepass %s\ndir %s\nsave \"\"\nappendonly no\n", port, adminSecret, dir)
	// JSON must be loaded first so Search can attach to its API.
	for _, path := range []string{jsonPath, searchPath} {
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) || strings.ContainsAny(path, "\r\n\"\\") {
			t.Fatal("invalid local module path")
		}
		conf += fmt.Sprintf("loadmodule \"%s\"\n", path)
	}
	path := filepath.Join(dir, "redis.conf")
	if err := os.WriteFile(path, []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binaryPath, path)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Redis fixture did not exit")
		}
	})
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	admin := redisdriver.NewClient(&redisdriver.Options{Addr: address, Password: adminSecret, Protocol: 2, DisableIdentity: true, MaxRetries: -1})
	t.Cleanup(func() { _ = admin.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for admin.Ping(ctx).Err() != nil {
		if ctx.Err() != nil {
			t.Fatal("Redis fixture did not become ready")
		}
		select {
		case <-done:
			t.Fatal("Redis fixture exited during startup")
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	acl := []any{"ACL", "SETUSER", "vector_reader", "reset", "on", ">" + readerSecret, "-@all", "%R~tenant:*", "~search-*", "+get", "+hget", "+ping", "+info", "+command", "+acl|getuser", "+acl|whoami", "+module|list", "+role", "+ft.search", "+ft.aggregate", "+ft.info", "+ft._list"}
	if err := admin.Do(ctx, acl...).Err(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCP_VECTOR_REDIS_PASSWORD", readerSecret)
	text := fmt.Sprintf(`server:
  strict_startup: true
targets:
  vectors:
    engine: redis
    environment: test
    host: 127.0.0.1
    port: %d
    username: vector_reader
    password_env: MCP_VECTOR_REDIS_PASSWORD
    tls:
      mode: disabled
      allow_insecure_remote: true
    redis:
      protocol: %d
      key_patterns: ["tenant:*"]
      module_object_patterns:
        FT.SEARCH: ["search-*"]
        FT.AGGREGATE: ["search-*"]
        FT.INFO: ["search-*"]
`, port, protocol)
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["vectors"]
	target.Redis.TrustedProfileKeys = map[string]string{"fixture": base64.StdEncoding.EncodeToString(public)}
	modules, err := admin.Do(ctx, "MODULE", "LIST").Slice()
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range modules {
		fields := redisTestMap(t, module, false)
		name, _ := fields["name"].(string)
		artifact, _ := fields["path"].(string)
		version, ok := fields["ver"].(int64)
		if !ok || (name != "search" && name != "ReJSON") {
			t.Fatal("unexpected fixture module identity")
		}
		data, err := os.ReadFile(artifact)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		profile := modulepolicy.Profile{ProfileVersion: 1, Module: modulepolicy.ModuleIdentity{Name: name, Version: int(version), RedisCompatibility: []string{"7.4"}, ArtifactPath: artifact, ArtifactSHA256: fmt.Sprintf("%x", sum), VendorBuildID: "disposable-vector-test"}, Commands: map[string]modulepolicy.CommandRule{}, IssuedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
		commands, err := admin.CommandList(ctx, &redisdriver.FilterBy{Module: name}).Result()
		if err != nil {
			t.Fatal(err)
		}
		for _, command := range commands {
			rule := modulepolicy.CommandRule{KeyModel: "ordinary-keys"}
			if name == "search" && modulepolicy.SearchIndexCommand(command) {
				rule.ReadOnly = true
				rule.KeyModel = "index-prefix-attested"
			}
			profile.Commands[strings.ToUpper(command)] = rule
		}
		payload, _ := json.Marshal(profile)
		envelope, _ := json.Marshal(modulepolicy.Envelope{Payload: base64.StdEncoding.EncodeToString(payload), KeyID: "fixture", Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))})
		profilePath := filepath.Join(dir, name+".json")
		if err := os.WriteFile(profilePath, envelope, 0600); err != nil {
			t.Fatal(err)
		}
		target.Redis.ModuleProfiles = append(target.Redis.ModuleProfiles, profilePath)
	}
	reader := redisdriver.NewClient(&redisdriver.Options{Addr: address, Username: target.Username, Password: readerSecret, Protocol: protocol, DisableIdentity: true, MaxRetries: -1})
	t.Cleanup(func() { _ = reader.Close() })
	return admin, reader, cfg
}

func redisTestMap(t *testing.T, raw any, pairs bool) map[string]any {
	t.Helper()
	if result, ok := raw.(map[string]any); ok {
		return result
	}
	if values, ok := raw.(map[any]any); ok {
		result := map[string]any{}
		for key, value := range values {
			result[key.(string)] = value
		}
		return result
	}
	values, ok := raw.([]any)
	if !ok {
		t.Fatalf("unexpected Redis map type %T", raw)
	}
	result := map[string]any{}
	if pairs {
		for _, value := range values {
			pair := value.([]any)
			result[pair[0].(string)] = pair[1]
		}
	} else {
		for i := 0; i < len(values); i += 2 {
			result[values[i].(string)] = values[i+1]
		}
	}
	return result
}

func vectorBytes(values []float64, dtype string) []byte {
	var blob []byte
	for _, value := range values {
		if dtype == "FLOAT32" {
			blob = binary.LittleEndian.AppendUint32(blob, math.Float32bits(float32(value)))
		} else {
			blob = binary.LittleEndian.AppendUint64(blob, math.Float64bits(value))
		}
	}
	return blob
}

type vectorHit struct {
	id    string
	score float64
}

func searchHits(t *testing.T, result core.RedisResult, protocol int) []vectorHit {
	t.Helper()
	if result.Truncated {
		t.Fatal("vector response was truncated")
	}
	var hits []vectorHit
	add := func(id string, fields map[string]any) {
		score, err := strconv.ParseFloat(fmt.Sprint(fields["distance"]), 64)
		if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
			t.Fatal("invalid vector distance")
		}
		hits = append(hits, vectorHit{id, score})
	}
	if protocol == 2 {
		values := result.Value.([]any)
		for i := 1; i < len(values); i += 2 {
			add(values[i].(string), redisTestMap(t, values[i+1], false))
		}
	} else {
		fields := redisTestMap(t, result.Value, true)
		for _, row := range fields["results"].([]any) {
			hit := redisTestMap(t, row, true)
			add(hit["id"].(string), redisTestMap(t, hit["extra_attributes"], true))
		}
	}
	return hits
}

func TestLocalRedisDenseVectorsThroughMCP(t *testing.T) {
	for _, protocol := range []int{2, 3} {
		t.Run(fmt.Sprintf("RESP%d", protocol), func(t *testing.T) {
			admin, reader, cfg := vectorRedisFixture(t, protocol)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			targets, err := registry.Open(ctx, cfg, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer targets.Close()
			server := New(targets, slog.Default(), "vector-test")
			a, b := mcp.NewInMemoryTransports()
			ss, err := server.mcp.Connect(ctx, a, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ss.Close()
			client := mcp.NewClient(&mcp.Implementation{Name: "vector-test", Version: "1"}, nil)
			cs, err := client.Connect(ctx, b, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()
			call := func(input RedisCommandInput) (core.RedisResult, error) {
				input.Target = "vectors"
				response, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "redis_command", Arguments: input})
				if err != nil {
					return core.RedisResult{}, err
				}
				if response.IsError {
					encoded, _ := json.Marshal(response.Content)
					return core.RedisResult{}, fmt.Errorf("MCP Redis command failed: %s", encoded)
				}
				var result core.RedisResult
				if len(response.Content) != 1 {
					return result, fmt.Errorf("missing MCP result")
				}
				err = json.Unmarshal([]byte(response.Content[0].(*mcp.TextContent).Text), &result)
				return result, err
			}
			stringArg := func(value string) core.RedisArgument { return core.RedisArgument{String: &value} }
			q := []float64{1, -0.5, 0.25}
			documents := [][]float64{q, {0.5, -0.125, 0.25}, {-1, 0.25, 0.75}, {0.25, 0.75, -0.5}}
			for _, storage := range []string{"HASH", "JSON"} {
				for _, algorithm := range []string{"FLAT", "HNSW"} {
					for _, metric := range []string{"L2", "IP", "COSINE"} {
						for _, dtype := range []string{"FLOAT32", "FLOAT64"} {
							t.Run(strings.Join([]string{storage, algorithm, metric, dtype}, "/"), func(t *testing.T) {
								if storage == "JSON" && os.Getenv("READONLY_DB_MCP_REDIS_JSON_MODULE") == "" {
									t.Skip("local JSON module is not configured")
								}
								name := strings.ToLower(strings.Join([]string{storage, algorithm, metric, dtype}, "-"))
								index, prefix := "search-"+name, "tenant:"+name+":"
								create := []any{"FT.CREATE", index, "ON", storage, "PREFIX", "1", prefix, "SCHEMA"}
								if storage == "HASH" {
									create = append(create, "category", "TAG", "embedding")
								} else {
									create = append(create, "$.category", "AS", "category", "TAG", "$.embedding", "AS", "embedding")
								}
								create = append(create, "VECTOR", algorithm, "6", "TYPE", dtype, "DIM", "3", "DISTANCE_METRIC", metric)
								if err := admin.Do(ctx, create...).Err(); err != nil {
									t.Fatal(err)
								}
								for i, document := range documents {
									category := "keep"
									if i == 0 {
										category = "omit"
									}
									var err error
									if storage == "HASH" {
										err = admin.HSet(ctx, prefix+strconv.Itoa(i), "embedding", vectorBytes(document, dtype), "category", category).Err()
									} else {
										data, _ := json.Marshal(map[string]any{"embedding": document, "category": category})
										err = admin.Do(ctx, "JSON.SET", prefix+strconv.Itoa(i), "$", string(data)).Err()
									}
									if err != nil {
										t.Fatal(err)
									}
								}
								info, err := call(RedisCommandInput{Command: "FT.INFO", Arguments: []core.RedisArgument{stringArg(index)}})
								if err != nil {
									t.Fatal(err)
								}
								if info.SearchIndex == nil || info.SearchIndex.Storage != storage || len(info.SearchIndex.Vectors) != 1 {
									t.Fatalf("missing vector metadata: %+v", info.SearchIndex)
								}
								field := info.SearchIndex.Vectors[0]
								if field.Dimensions != 3 || field.Algorithm != algorithm || field.DataType != dtype || field.DistanceMetric != metric {
									t.Fatalf("wrong metadata: %+v", field)
								}
								blob := base64.StdEncoding.EncodeToString(vectorBytes(q, dtype))
								makeInput := func(query string) RedisCommandInput {
									args := []core.RedisArgument{stringArg(index), stringArg(query), stringArg("PARAMS"), stringArg("2"), stringArg("v"), {Base64: &blob}}
									for _, value := range []string{"SORTBY", "distance", "ASC", "RETURN", "1", "distance", "LIMIT", "0", "10", "DIALECT", "2"} {
										args = append(args, stringArg(value))
									}
									return RedisCommandInput{Command: "FT.SEARCH", Arguments: args}
								}
								var expected []vectorHit
								for i, d := range documents {
									var dot, nq, nd, squared float64
									for j := range q {
										dot += q[j] * d[j]
										nq += q[j] * q[j]
										nd += d[j] * d[j]
										squared += (q[j] - d[j]) * (q[j] - d[j])
									}
									score := squared
									if metric == "IP" {
										score = 1 - dot
									}
									if metric == "COSINE" {
										score = 1 - dot/math.Sqrt(nq*nd)
									}
									expected = append(expected, vectorHit{prefix + strconv.Itoa(i), score})
								}
								sort.Slice(expected, func(i, j int) bool { return expected[i].score < expected[j].score })
								check := func(query string, want []vectorHit) {
									t.Helper()
									input := makeInput(query)
									result, err := call(input)
									if err != nil {
										t.Fatal(err)
									}
									hits := searchHits(t, result, protocol)
									if len(hits) != len(want) {
										t.Fatalf("got %v, want %v", hits, want)
									}
									for i := range hits {
										if hits[i].id != want[i].id || math.Abs(hits[i].score-want[i].score) > 1e-5 {
											t.Fatalf("got %v, want %v", hits, want)
										}
									}
									wire := []any{input.Command}
									for _, a := range input.Arguments {
										if a.String != nil {
											wire = append(wire, *a.String)
										} else {
											bytes, err := base64.StdEncoding.DecodeString(*a.Base64)
											if err != nil {
												t.Fatal(err)
											}
											wire = append(wire, bytes)
										}
									}
									native, err := reader.Do(ctx, wire...).Result()
									if err != nil {
										t.Fatal(err)
									}
									nativeHits := searchHits(t, core.RedisResult{Value: native}, protocol)
									if len(nativeHits) != len(hits) {
										t.Fatal("native/MCP hit count differs")
									}
									for i := range hits {
										if nativeHits[i] != hits[i] {
											t.Fatalf("native/MCP results differ: %v / %v", nativeHits, hits)
										}
									}
								}
								query := "*=>[KNN 4 @embedding $v AS distance]"
								if algorithm == "HNSW" {
									query = "*=>[KNN 4 @embedding $v EF_RUNTIME 20 AS distance]"
								}
								check(query, expected)
								if storage == "HASH" && algorithm == "FLAT" && metric == "L2" && dtype == "FLOAT32" {
									response, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "redis_batch", Arguments: RedisBatchInput{Target: "vectors", Commands: []RedisCommandInput{{Command: "FT.INFO", Arguments: []core.RedisArgument{stringArg(index)}}, makeInput(query)}}})
									if err != nil || response.IsError {
										t.Fatal("MCP vector batch failed", err)
									}
									var batch core.RedisBatchResult
									if err := json.Unmarshal([]byte(response.Content[0].(*mcp.TextContent).Text), &batch); err != nil {
										t.Fatal(err)
									}
									if batch.CompletedCommands != 2 || batch.Truncated || batch.Results[0].SearchIndex == nil || len(searchHits(t, *batch.Results[1], protocol)) != 4 {
										t.Fatal("vector batch incomplete")
									}
									for _, binaryArg := range []core.RedisArgument{{Base64: &blob, String: &index}, {Base64: func() *string { s := "!invalid!"; return &s }()}} {
										bad := makeInput(query)
										bad.Arguments[5] = binaryArg
										if _, err := call(bad); err == nil {
											t.Fatal("ambiguous or invalid vector encoding accepted")
										}
									}
									runtime, _ := targets.GetRedis("vectors")
									cancelled, stop := context.WithCancel(ctx)
									stop()
									input := makeInput(query)
									if _, err := runtime.RedisCommand(cancelled, core.RedisRequest{Command: input.Command, Arguments: input.Arguments}); err == nil {
										t.Fatal("cancelled vector request accepted")
									}
									if _, err := call(input); err != nil {
										t.Fatal("vector cancellation leaked admission permit", err)
									}
								}
								var filtered []vectorHit
								for _, hit := range expected {
									if hit.id != prefix+"0" {
										filtered = append(filtered, hit)
									}
								}
								check("(@category:{keep})=>[KNN 3 @embedding $v AS distance]", filtered)
								radius := (expected[1].score + expected[2].score) / 2
								check(fmt.Sprintf("@embedding:[VECTOR_RANGE %.12g $v]=>{$yield_distance_as: distance}", radius), expected[:2])
								if storage == "HASH" {
									result, err := call(RedisCommandInput{Command: "HGET", Arguments: []core.RedisArgument{stringArg(prefix + "0"), stringArg("embedding")}})
									if err != nil {
										t.Fatal(err)
									}
									var bytes []byte
									if value, ok := result.Value.(string); ok {
										bytes = []byte(value)
									} else {
										bytes, err = base64.StdEncoding.DecodeString(result.Value.(map[string]any)["base64"].(string))
									}
									if err != nil || result.Truncated || string(bytes) != string(vectorBytes(q, dtype)) {
										t.Fatal("MCP corrupted binary vector")
									}
								}
								bad := makeInput(query)
								short := base64.StdEncoding.EncodeToString(vectorBytes(q[:2], dtype))
								bad.Arguments[5] = core.RedisArgument{Base64: &short}
								if _, err := call(bad); err == nil {
									t.Fatal("wrong vector dimensions accepted")
								}
								if _, err := call(makeInput(query)); err != nil {
									t.Fatal("vector failure poisoned connection", err)
								}
								for _, mutation := range [][]any{{"HSET", prefix + "0", "category", "changed"}, {"FT.DROPINDEX", index}} {
									if err := reader.Do(ctx, mutation...).Err(); err == nil || !strings.Contains(err.Error(), "NOPERM") {
										t.Fatal("native write was not denied by ACL", err)
									}
									input := RedisCommandInput{Command: mutation[0].(string)}
									for _, value := range mutation[1:] {
										input.Arguments = append(input.Arguments, stringArg(value.(string)))
									}
									if _, err := call(input); err == nil {
										t.Fatal("MCP write accepted")
									}
								}
							})
						}
					}
				}
			}
		})
	}
}
