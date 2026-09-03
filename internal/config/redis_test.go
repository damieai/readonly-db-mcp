package config

import "testing"

func TestRedisDefaultsAndValidation(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineRedis
	target.MySQL = MySQLConfig{}
	target.Port = 0
	target.Database = ""
	target.AllowedSchemas = nil
	target.MetadataCache = MetadataCacheConfig{}
	target.ResultCache = ResultCacheConfig{}
	target.Redis = RedisConfig{KeyPatterns: []string{"analytics:*"}, AllowReadonlyScripts: true}
	applyDefaults(cfg)
	if target.Port != 6379 {
		t.Fatalf("Redis default port=%d", target.Port)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRedisRejectsComplexOrMissingKeyScope(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineRedis
	target.MySQL = MySQLConfig{}
	target.Database = ""
	target.AllowedSchemas = nil
	target.MetadataCache = MetadataCacheConfig{}
	target.ResultCache = ResultCacheConfig{}
	target.Redis = RedisConfig{KeyPatterns: []string{"analytics:[0-9]*"}}
	applyDefaults(cfg)
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected key pattern rejection")
	}
}

func TestRedisAllowsExplicitAllKeyScope(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineRedis
	target.MySQL = MySQLConfig{}
	target.Database = ""
	target.AllowedSchemas = nil
	target.MetadataCache = MetadataCacheConfig{}
	target.ResultCache = ResultCacheConfig{}
	target.Redis = RedisConfig{KeyPatterns: []string{"*"}, AllowReadonlyScripts: true}
	applyDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRedisSentinelAndClusterConfig(t *testing.T) {
	for _, mode := range []string{"sentinel", "cluster"} {
		t.Run(mode, func(t *testing.T) {
			cfg := validConfig()
			target := cfg.Targets["test"]
			target.Engine = EngineRedis
			target.MySQL = MySQLConfig{}
			target.Host = ""
			target.Port = 0
			target.Database = ""
			target.AllowedSchemas = nil
			target.MetadataCache = MetadataCacheConfig{}
			target.ResultCache = ResultCacheConfig{}
			target.TLS.AllowInsecureRemote = true
			target.Redis = RedisConfig{Mode: mode, KeyPatterns: []string{"analytics:*"}}
			if mode == "sentinel" {
				target.Redis.Sentinel = RedisSentinelConfig{ServiceName: "analytics", Addresses: []string{"s1.example:26379", "s2.example:26379"}, Username: "discovery", PasswordEnv: "REDIS_SENTINEL_PASSWORD", EndpointAllowlist: RedisEndpointAllowlist{DNSSuffixes: []string{".example"}, CIDRs: []string{"10.0.0.0/8"}}}
			} else {
				target.Redis.Cluster = RedisClusterConfig{SeedAddresses: []string{"c1.example:6379"}, EndpointAllowlist: RedisEndpointAllowlist{DNSSuffixes: []string{".example"}, CIDRs: []string{"10.0.0.0/8"}}}
			}
			applyDefaults(cfg)
			if err := cfg.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRedisClusterReplicaRequiresEventualConsistency(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineRedis
	target.MySQL = MySQLConfig{}
	target.Host = ""
	target.Port = 0
	target.Database = ""
	target.AllowedSchemas = nil
	target.MetadataCache = MetadataCacheConfig{}
	target.ResultCache = ResultCacheConfig{}
	target.TLS.AllowInsecureRemote = true
	target.Redis = RedisConfig{Mode: "cluster", KeyPatterns: []string{"analytics:*"}, Cluster: RedisClusterConfig{SeedAddresses: []string{"c1.example:6379"}, ReadRole: "replica", EndpointAllowlist: RedisEndpointAllowlist{DNSSuffixes: []string{".example"}, CIDRs: []string{"10.0.0.0/8"}}}}
	applyDefaults(cfg)
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected current-consistency replica rejection")
	}
	target.Consistency = ConsistencyEventual
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
