package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Profiles pin protocol/build identities, not a claim of live-server certification.
var ElasticsearchBuilds = map[string]string{
	"8.19.21": "4fe44c255c3d0da06779b921e132cdc555ed9aff",
	"9.1.10":  "f2e019bd8110088070638ca779ec1543188c0f43",
}

type ElasticsearchConfig struct {
	Endpoints          []string      `yaml:"endpoints"`
	Version            string        `yaml:"version"`
	ClusterUUID        string        `yaml:"cluster_uuid"`
	AllowedIndices     []string      `yaml:"allowed_indices"`
	DeniedIndices      []string      `yaml:"denied_indices"`
	PrivilegeRecheck   time.Duration `yaml:"privilege_recheck_interval"`
	MaxRequestBytes    int           `yaml:"max_request_bytes"`
	MaxJSONDepth       int           `yaml:"max_json_depth"`
	MaxJSONNodes       int           `yaml:"max_json_nodes"`
	MaxResolvedIndices int           `yaml:"max_resolved_indices"`
}

func defaultElasticsearch(t *TargetConfig) {
	if t.Elasticsearch == nil {
		t.Elasticsearch = &ElasticsearchConfig{}
	}
	e := t.Elasticsearch
	if e.PrivilegeRecheck == 0 {
		e.PrivilegeRecheck = time.Minute
	}
	if e.MaxRequestBytes == 0 {
		e.MaxRequestBytes = 1 << 20
	}
	if e.MaxJSONDepth == 0 {
		e.MaxJSONDepth = 128
	}
	if e.MaxJSONNodes == 0 {
		e.MaxJSONNodes = 100000
	}
	if e.MaxResolvedIndices == 0 {
		e.MaxResolvedIndices = 1024
	}
}

// ES glob selectors use only '*' and '?', never regexp/path/query syntax.
// This does not constrain Query DSL; it defines the authorization language.
func ValidElasticsearchPattern(s string) bool {
	if s == "" || len(s) > 255 || strings.HasPrefix(s, "_") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`/\,:#%&[]=<>"|`, r) {
			return false
		}
	}
	return true
}

func validateElasticsearch(t *TargetConfig, limits Limits) []string {
	var p []string
	add := func(s string) { p = append(p, s) }
	if !safeName.MatchString(t.Environment) {
		add("environment is required and must be a safe identifier")
	}
	if t.Consistency != ConsistencyEventual {
		add("Elasticsearch metadata/search targets require eventual consistency")
	}
	if t.Host != "" || t.Port != 0 || t.Database != "" || len(t.AllowedSchemas) != 0 || len(t.DeniedTables) != 0 {
		add("Elasticsearch uses endpoints and index scopes; omit SQL host/port/database/schema fields")
	}
	if t.MySQL != (MySQLConfig{}) || t.PostgreSQL != (PostgreSQLConfig{}) || t.SQLServer != (SQLServerConfig{}) || !redisConfigEmpty(t.Redis) {
		add("Elasticsearch rejects SQL/Redis settings")
	}
	if strings.TrimSpace(t.Username) == "" || strings.ContainsAny(t.Username, ":\r\n") {
		add("Elasticsearch requires a dedicated username without colon or line breaks")
	}
	if (t.PasswordFile == "") == (t.PasswordEnv == "") {
		add("configure exactly one of password_file or password_env")
	}
	if t.PasswordEnv != "" && !regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,127}$`).MatchString(t.PasswordEnv) {
		add("password_env must name an uppercase environment variable")
	}
	if t.Connection.MaxOpen < limits.PerTargetConcurrency || t.Connection.MaxOpen > 64 || t.Connection.MaxOpen < 1 || t.Connection.MaxIdle < 0 || t.Connection.MaxIdle > t.Connection.MaxOpen {
		add("Elasticsearch pool must cover per-target concurrency, with 1-64 open and at most max_open idle sockets")
	}
	if t.Connection.ConnectTimeout <= 0 || t.Connection.ConnectTimeout > time.Minute || t.Connection.WriteTimeout <= 0 || t.Connection.WriteTimeout > time.Minute || t.Connection.ReadTimeout < limits.MaxTimeout || t.Connection.ReadTimeout > 16*time.Minute || t.Connection.MaxLifetime <= 0 || t.Connection.MaxLifetime > 24*time.Hour || t.Connection.MaxIdleTime <= 0 || t.Connection.MaxIdleTime > time.Hour {
		add("Elasticsearch connection phase/lifetime limits are outside hard ceilings")
	}
	if t.ResultCache.Enabled {
		add("Elasticsearch query-result caching is not implemented")
	}
	if t.TLS.Mode != TLSVerifyFull || t.TLS.CAFile == "" || t.TLS.AllowInsecureRemote || t.TLS.AllowInsecureProduction {
		add("Elasticsearch requires verify-full TLS and a CA file")
	}
	if (t.TLS.CertFile == "") != (t.TLS.KeyFile == "") {
		add("tls.cert_file and tls.key_file must be configured together")
	}
	if t.TLS.ServerName != "" {
		add("Elasticsearch verifies each endpoint hostname; omit tls.server_name")
	}
	e := t.Elasticsearch
	if e == nil {
		return append(p, "elasticsearch configuration is required")
	}
	if _, ok := ElasticsearchBuilds[e.Version]; !ok {
		add("Elasticsearch version has no pinned implementation profile")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`).MatchString(e.ClusterUUID) {
		add("Elasticsearch cluster_uuid must pin a 16-64 character cluster identity")
	}
	if len(e.Endpoints) < 1 || len(e.Endpoints) > 32 {
		add("Elasticsearch requires 1-32 explicit HTTPS endpoints")
	}
	seen := map[string]bool{}
	for _, raw := range e.Endpoints {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") {
			add("Elasticsearch endpoints must be HTTPS origins without credentials, paths, queries or fragments")
			continue
		}
		if seen[u.Host] {
			add("duplicate Elasticsearch endpoint")
		}
		seen[u.Host] = true
	}
	if len(e.AllowedIndices) == 0 || len(e.AllowedIndices) > 64 || len(e.DeniedIndices) > 64 {
		add("Elasticsearch requires 1-64 allowed patterns and at most 64 denied patterns")
	}
	for _, group := range [][]string{e.AllowedIndices, e.DeniedIndices} {
		for _, pattern := range group {
			if !ValidElasticsearchPattern(pattern) {
				add("invalid Elasticsearch index scope pattern")
			}
		}
	}
	if e.PrivilegeRecheck < 10*time.Second || e.PrivilegeRecheck > 5*time.Minute {
		add("Elasticsearch privilege_recheck_interval must be between 10s and 5m")
	}
	for _, v := range []struct {
		name            string
		value, min, max int
	}{
		{"max_request_bytes", e.MaxRequestBytes, 1024, 16 << 20}, {"max_json_depth", e.MaxJSONDepth, 8, 512},
		{"max_json_nodes", e.MaxJSONNodes, 128, 1000000}, {"max_resolved_indices", e.MaxResolvedIndices, 1, 10000},
	} {
		if v.value < v.min || v.value > v.max {
			add(fmt.Sprintf("elasticsearch.%s is outside its resource ceiling", v.name))
		}
	}
	return p
}
