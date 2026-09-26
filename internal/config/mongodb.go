package config

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type MongoDBConfig struct {
	Version           string        `yaml:"version"`
	AuthSource        string        `yaml:"auth_source"`
	Collections       []string      `yaml:"collections"`
	PrivilegeRecheck  time.Duration `yaml:"privilege_recheck_interval"`
	MaxRequestBytes   int           `yaml:"max_request_bytes"`
	MaxJSONDepth      int           `yaml:"max_json_depth"`
	MaxJSONNodes      int           `yaml:"max_json_nodes"`
	MaxPipelineStages int           `yaml:"max_pipeline_stages"`
}

func defaultMongoDB(t *TargetConfig) {
	if t.MongoDB == nil {
		t.MongoDB = &MongoDBConfig{}
	}
	m := t.MongoDB
	if m.AuthSource == "" {
		m.AuthSource = "admin"
	}
	if m.PrivilegeRecheck == 0 {
		m.PrivilegeRecheck = time.Minute
	}
	if m.MaxRequestBytes == 0 {
		m.MaxRequestBytes = 1 << 20
	}
	if m.MaxJSONDepth == 0 {
		m.MaxJSONDepth = 128
	}
	if m.MaxJSONNodes == 0 {
		m.MaxJSONNodes = 100000
	}
	if m.MaxPipelineStages == 0 {
		m.MaxPipelineStages = 100
	}
}

var mongoVersion = regexp.MustCompile(`^8\.0\.([0-9]+)$`)
var mongoName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,119}$`)
var mongoHost = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]{0,252}$`)

func validateMongoDB(t *TargetConfig, limits Limits) []string {
	var p []string
	add := func(message string) { p = append(p, message) }
	m := t.MongoDB
	if m == nil {
		return []string{"mongodb configuration is required"}
	}
	if !safeName.MatchString(t.Environment) {
		add("environment is required and must be a safe identifier")
	}
	if t.Consistency != ConsistencyEventual {
		add("MongoDB target requires eventual consistency")
	}
	if !(mongoHost.MatchString(t.Host) || net.ParseIP(t.Host) != nil) || strings.Contains(t.Host, "..") {
		add("MongoDB host must be an exact DNS name or IP address")
	}
	if t.Port < 1 || t.Port > 65535 {
		add("MongoDB port is invalid")
	}
	if !mongoName.MatchString(t.Database) || strings.HasPrefix(t.Database, "system.") {
		add("MongoDB database is invalid")
	}
	if !mongoName.MatchString(m.AuthSource) {
		add("mongodb.auth_source is invalid")
	}
	if t.Username == "" || strings.ContainsAny(t.Username, "\x00\r\n") {
		add("MongoDB username is required")
	}
	if (t.PasswordFile == "") == (t.PasswordEnv == "") {
		add("MongoDB requires exactly one password_file or password_env")
	}
	if t.PasswordEnv != "" && !regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,127}$`).MatchString(t.PasswordEnv) {
		add("MongoDB password_env is invalid")
	}
	if len(t.AllowedSchemas) != 0 || len(t.DeniedTables) != 0 || t.Elasticsearch != nil || t.Qdrant != nil || t.MySQL != (MySQLConfig{}) || t.PostgreSQL != (PostgreSQLConfig{}) || t.SQLServer != (SQLServerConfig{}) || !redisConfigEmpty(t.Redis) {
		add("MongoDB rejects SQL, Redis, Qdrant and Elasticsearch settings")
	}
	parts := mongoVersion.FindStringSubmatch(m.Version)
	if len(parts) != 2 {
		add("mongodb.version must pin an exact MongoDB 8.0 patch version")
	} else if patch, _ := strconv.Atoi(parts[1]); patch < 32 {
		add("mongodb.version must be 8.0.32 or newer")
	}
	if len(m.Collections) == 0 || len(m.Collections) > 64 {
		add("mongodb.collections must contain 1-64 exact collection names")
	}
	seen := map[string]bool{}
	for _, name := range m.Collections {
		if !mongoName.MatchString(name) || strings.HasPrefix(name, "system.") || seen[name] {
			add(fmt.Sprintf("mongodb.collections contains an invalid or duplicate name %q", name))
		}
		seen[name] = true
	}
	if m.PrivilegeRecheck < time.Second || m.PrivilegeRecheck > 10*time.Minute {
		add("mongodb.privilege_recheck_interval must be between 1s and 10m")
	}
	if m.MaxRequestBytes < 1024 || m.MaxRequestBytes > 8<<20 {
		add("mongodb.max_request_bytes must be between 1 KiB and 8 MiB")
	}
	if m.MaxJSONDepth < 1 || m.MaxJSONDepth > 256 || m.MaxJSONNodes < 1 || m.MaxJSONNodes > 200000 || m.MaxPipelineStages < 1 || m.MaxPipelineStages > 200 {
		add("MongoDB JSON and pipeline structural ceilings are invalid")
	}
	if t.Connection.MaxOpen < limits.PerTargetConcurrency || t.Connection.MaxOpen > 64 || t.Connection.MaxIdle < 0 || t.Connection.MaxIdle > t.Connection.MaxOpen {
		add("MongoDB pool must cover per-target concurrency")
	}
	if t.Connection.ConnectTimeout <= 0 || t.Connection.ReadTimeout <= limits.MaxTimeout || t.Connection.WriteTimeout <= 0 || t.Connection.MaxLifetime < limits.MaxTimeout || t.Connection.MaxIdleTime <= 0 {
		add("MongoDB connection timeout/lifetime settings are invalid")
	}
	if t.TLS.CertFile != "" || t.TLS.KeyFile != "" || t.TLS.ServerName != "" || t.TLS.AllowInsecureRemote || t.TLS.AllowInsecureProduction {
		add("MongoDB TLS overrides and client certificates are unavailable")
	}
	if t.TLS.Mode == TLSDisabled {
		if !isLoopbackHost(t.Host) || isProductionEnvironment(t.Environment) || t.TLS.CAFile != "" {
			add("MongoDB plaintext is permitted only on loopback for non-production targets")
		}
	} else if t.TLS.Mode != TLSVerifyFull || t.TLS.CAFile == "" {
		add("MongoDB TLS requires verify-full and a pinned CA file")
	}
	if t.MetadataCache != (MetadataCacheConfig{}) || t.ResultCache != (ResultCacheConfig{}) {
		add("MongoDB metadata and result caches are unavailable")
	}
	return p
}
