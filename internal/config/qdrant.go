package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

// Qdrant's version is pinned to the Query API shape implemented by this adapter.
const QdrantVersion = "1.19.1"

type QdrantConfig struct {
	Endpoint         string        `yaml:"endpoint"`
	Version          string        `yaml:"version"`
	Collections      []string      `yaml:"collections"`
	KeyFile          string        `yaml:"key_file"`
	KeyEnv           string        `yaml:"key_env"`
	PrivilegeRecheck time.Duration `yaml:"privilege_recheck_interval"`
	MaxRequestBytes  int           `yaml:"max_request_bytes"`
	MaxJSONDepth     int           `yaml:"max_json_depth"`
	MaxJSONNodes     int           `yaml:"max_json_nodes"`
}

func (q *QdrantConfig) Secret() (string, error) { return readSecret(q.KeyFile, q.KeyEnv) }

func defaultQdrant(t *TargetConfig) {
	if t.Qdrant == nil {
		t.Qdrant = &QdrantConfig{}
	}
	q := t.Qdrant
	if q.PrivilegeRecheck == 0 {
		q.PrivilegeRecheck = time.Minute
	}
	if q.MaxRequestBytes == 0 {
		q.MaxRequestBytes = 1 << 20
	}
	if q.MaxJSONDepth == 0 {
		q.MaxJSONDepth = 128
	}
	if q.MaxJSONNodes == 0 {
		q.MaxJSONNodes = 100000
	}
}

var qdrantCollectionName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,254}$`)

func validateQdrant(t *TargetConfig, limits Limits) []string {
	var p []string
	add := func(s string) { p = append(p, s) }
	q := t.Qdrant
	if q == nil {
		return []string{"qdrant configuration is required"}
	}
	if !safeName.MatchString(t.Environment) {
		add("environment is required and must be a safe identifier")
	}
	if t.Consistency != ConsistencyEventual {
		add("Qdrant targets require eventual consistency")
	}
	if t.Host != "" || t.Port != 0 || t.Database != "" || t.Username != "" || t.PasswordFile != "" || t.PasswordEnv != "" || len(t.AllowedSchemas) != 0 || len(t.DeniedTables) != 0 {
		add("Qdrant uses endpoint, collections and scoped key; omit SQL identity and scope fields")
	}
	if t.Elasticsearch != nil || t.MySQL != (MySQLConfig{}) || t.PostgreSQL != (PostgreSQLConfig{}) || t.SQLServer != (SQLServerConfig{}) || !redisConfigEmpty(t.Redis) {
		add("Qdrant rejects other engine settings")
	}
	u, err := url.Parse(q.Endpoint)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && u.Scheme != "http") {
		add("qdrant.endpoint must be an absolute HTTP(S) origin without path, query, userinfo or fragment")
	} else {
		if u.Scheme == "http" {
			if t.TLS.Mode != TLSDisabled || !isLoopbackHost(u.Hostname()) || isProductionEnvironment(t.Environment) {
				add("Qdrant HTTP is permitted only for a loopback, non-production target with tls.mode disabled")
			}
		} else if t.TLS.Mode != TLSVerifyFull {
			add("Qdrant HTTPS requires tls.mode verify-full")
		}
		if port := u.Port(); port != "" {
			n, e := strconv.Atoi(port)
			if e != nil || n < 1 || n > 65535 {
				add("qdrant.endpoint port is invalid")
			}
		}
	}
	if q.Version != QdrantVersion {
		add(fmt.Sprintf("qdrant.version must be pinned to %s", QdrantVersion))
	}
	if len(q.Collections) == 0 || len(q.Collections) > 64 {
		add("qdrant.collections must contain 1-64 exact collection names")
	}
	seen := map[string]bool{}
	for _, name := range q.Collections {
		if !qdrantCollectionName.MatchString(name) || seen[name] {
			add("qdrant.collections contains an invalid or duplicate name")
		}
		seen[name] = true
	}
	if (q.KeyFile == "") == (q.KeyEnv == "") || q.KeyEnv != "" && !regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,127}$`).MatchString(q.KeyEnv) {
		add("Qdrant requires exactly one protected key_file or key_env")
	}
	if q.PrivilegeRecheck < time.Second || q.PrivilegeRecheck > 10*time.Minute {
		add("qdrant.privilege_recheck_interval must be between 1s and 10m")
	}
	if q.MaxRequestBytes < 1024 || q.MaxRequestBytes > 8<<20 {
		add("qdrant.max_request_bytes must be between 1 KiB and 8 MiB")
	}
	if q.MaxJSONDepth < 1 || q.MaxJSONDepth > 256 || q.MaxJSONNodes < 1 || q.MaxJSONNodes > 200000 {
		add("Qdrant JSON depth/node ceilings are invalid")
	}
	if t.Connection.MaxOpen < limits.PerTargetConcurrency || t.Connection.MaxOpen > 64 || t.Connection.MaxIdle < 0 || t.Connection.MaxIdle > t.Connection.MaxOpen {
		add("Qdrant connection pool must cover per-target concurrency")
	}
	if t.Connection.ConnectTimeout <= 0 || t.Connection.ReadTimeout <= limits.MaxTimeout || t.Connection.WriteTimeout <= 0 || t.Connection.MaxLifetime < limits.MaxTimeout || t.Connection.MaxIdleTime <= 0 {
		add("Qdrant connection timeout/lifetime settings are invalid")
	}
	if t.TLS.CertFile != "" || t.TLS.KeyFile != "" || t.TLS.ServerName != "" || t.TLS.AllowInsecureRemote || t.TLS.AllowInsecureProduction {
		add("Qdrant client certificates and TLS overrides are unavailable")
	}
	if t.TLS.Mode == TLSVerifyFull && t.TLS.CAFile == "" {
		add("Qdrant HTTPS requires a pinned CA file")
	}
	if t.TLS.Mode == TLSDisabled && t.TLS.CAFile != "" {
		add("Qdrant HTTP must omit CA file")
	}
	if t.MetadataCache != (MetadataCacheConfig{}) || t.ResultCache != (ResultCacheConfig{}) {
		add("Qdrant metadata and result caches are unavailable")
	}
	return p
}
