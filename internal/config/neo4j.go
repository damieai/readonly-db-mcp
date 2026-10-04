package config

import (
	"net"
	"regexp"
	"strings"
	"time"
)

type Neo4jConfig struct {
	Version          string               `yaml:"version"`
	Attestor         *Neo4jAttestorConfig `yaml:"attestor"`
	PrivilegeRecheck time.Duration        `yaml:"privilege_recheck_interval"`
}

type Neo4jAttestorConfig struct {
	Username     string `yaml:"username"`
	PasswordFile string `yaml:"password_file"`
	PasswordEnv  string `yaml:"password_env"`
}

func (a *Neo4jAttestorConfig) Password() (string, error) {
	return readSecret(a.PasswordFile, a.PasswordEnv)
}

func defaultNeo4j(t *TargetConfig) {
	if t.Neo4j == nil {
		t.Neo4j = &Neo4jConfig{}
	}
	if t.Neo4j.PrivilegeRecheck == 0 {
		t.Neo4j.PrivilegeRecheck = time.Minute
	}
}

var neo4jVersion = regexp.MustCompile(`^20[0-9]{2}\.(0[1-9]|1[0-2])\.[0-9]+$`)
var neo4jName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,62}$`)
var neo4jHost = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]{0,252}$`)
var neo4jEnv = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,127}$`)

func validateNeo4j(t *TargetConfig, limits Limits) []string {
	var p []string
	add := func(s string) { p = append(p, s) }
	if t.Neo4j == nil {
		return []string{"neo4j configuration is required"}
	}
	if !safeName.MatchString(t.Environment) || t.Consistency != ConsistencyCurrent {
		add("Neo4j requires a named environment and current consistency")
	}
	if !(neo4jHost.MatchString(t.Host) || net.ParseIP(t.Host) != nil) || strings.Contains(t.Host, "..") || t.Port < 1 || t.Port > 65535 {
		add("Neo4j requires one exact host and valid Bolt port")
	}
	if !neo4jName.MatchString(t.Database) || strings.EqualFold(t.Database, "system") {
		add("Neo4j requires one exact non-system database")
	}
	if !neo4jName.MatchString(t.Username) || (t.PasswordFile == "") == (t.PasswordEnv == "") || (t.PasswordEnv != "" && !neo4jEnv.MatchString(t.PasswordEnv)) {
		add("Neo4j requires one native query identity and one password source")
	}
	a := t.Neo4j.Attestor
	if a == nil || !neo4jName.MatchString(a.Username) || a.Username == t.Username || (a.PasswordFile == "") == (a.PasswordEnv == "") || (a.PasswordEnv != "" && !neo4jEnv.MatchString(a.PasswordEnv)) {
		add("Neo4j requires a distinct system metadata attestor with one password source")
	}
	if !neo4jVersion.MatchString(t.Neo4j.Version) || t.Neo4j.Version < "2026.03.0" {
		add("neo4j.version must pin a 2026.03 or newer calendar-version patch")
	}
	if t.Neo4j.PrivilegeRecheck < time.Second || t.Neo4j.PrivilegeRecheck > 10*time.Minute {
		add("neo4j.privilege_recheck_interval must be between 1s and 10m")
	}
	if len(t.AllowedSchemas) != 0 || len(t.DeniedTables) != 0 || t.Elasticsearch != nil || t.Qdrant != nil || t.MongoDB != nil || t.SQLite != nil || t.DuckDB != nil || t.MySQL != (MySQLConfig{}) || t.PostgreSQL != (PostgreSQLConfig{}) || t.SQLServer != (SQLServerConfig{}) || !redisConfigEmpty(t.Redis) {
		add("Neo4j rejects other engine settings")
	}
	if t.TLS.Mode == TLSDisabled {
		if !isLoopbackHost(t.Host) || isProductionEnvironment(t.Environment) || t.TLS.CAFile != "" {
			add("Neo4j plaintext is permitted only on loopback outside production")
		}
	} else if t.TLS.Mode != TLSVerifyFull {
		add("Neo4j requires verified TLS or local loopback plaintext")
	}
	if t.TLS.CertFile != "" || t.TLS.KeyFile != "" || t.TLS.ServerName != "" || t.TLS.AllowInsecureRemote || t.TLS.AllowInsecureProduction {
		add("Neo4j TLS overrides and client certificates are unavailable")
	}
	if t.Connection.MaxOpen < limits.PerTargetConcurrency || t.Connection.MaxOpen > 64 || t.Connection.MaxIdle < 0 || t.Connection.MaxIdle > t.Connection.MaxOpen {
		add("Neo4j pool must cover per-target concurrency")
	}
	if t.Connection.ConnectTimeout <= 0 || t.Connection.ReadTimeout <= limits.MaxTimeout || t.Connection.MaxLifetime < limits.MaxTimeout {
		add("Neo4j connection timeout/lifetime settings are invalid")
	}
	if t.MetadataCache != (MetadataCacheConfig{}) || t.ResultCache != (ResultCacheConfig{}) {
		add("Neo4j generic SQL caches are unavailable")
	}
	return p
}
