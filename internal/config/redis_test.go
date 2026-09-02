package config

import "testing"

func TestRedisDefaultsAndValidation(t *testing.T) {
	cfg := validConfig()
	target := cfg.Targets["test"]
	target.Engine = EngineRedis
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
	target.Database = ""
	target.AllowedSchemas = nil
	target.MetadataCache = MetadataCacheConfig{}
	target.ResultCache = ResultCacheConfig{}
	target.Redis = RedisConfig{KeyPatterns: []string{"*"}, AllowReadonlyScripts: true}
	applyDefaults(cfg)
	if err := cfg.Validate(); err != nil { t.Fatal(err) }
}
