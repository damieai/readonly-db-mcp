package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validESConfig() *Config {
	c := validConfig()
	t := c.Targets["test"]
	t.Engine = EngineElasticsearch
	t.MySQL = MySQLConfig{}
	t.Host = ""
	t.Port = 0
	t.Database = ""
	t.AllowedSchemas = nil
	t.Consistency = ""
	t.TLS = TLSConfig{Mode: TLSVerifyFull, CAFile: "ca.pem"}
	t.Elasticsearch = &ElasticsearchConfig{Version: "8.19.21", ClusterUUID: "abcdefghijklmnopqrstuv", Endpoints: []string{"https://es.example.invalid:9200"}, AllowedIndices: []string{"reports-*"}}
	applyDefaults(c)
	return c
}

func TestElasticsearchDefaultsAndForecast(t *testing.T) {
	c := validESConfig()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	v := c.Targets["test"]
	if v.Elasticsearch.Enrich != nil || v.Elasticsearch.EnrichEnabled() {
		t.Fatal("ENRICH data scope was implicitly authorized")
	}
	if v.Elasticsearch.ESQLPragmas != nil {
		t.Fatal("experimental ES|QL pragmas were implicitly enabled")
	}
	if v.Port != 0 || v.Database != "" || v.Consistency != ConsistencyEventual || v.Elasticsearch.PrivilegeRecheck != time.Minute {
		t.Fatal("ES acquired SQL defaults")
	}
	if v.Elasticsearch.MaxOpenContexts != 32 || v.Elasticsearch.ContextKeepAlive != 5*time.Minute || v.Elasticsearch.MaxContextLifetime != 30*time.Minute || v.Elasticsearch.MaxContextIDBytes != 64<<10 {
		t.Fatal("context defaults differ from resource contract")
	}
	if v.Elasticsearch.MaxLanguageBytes != 256<<10 || v.Elasticsearch.MaxLanguageTokens != 10000 || v.Elasticsearch.MaxLanguageDepth != 128 || v.Elasticsearch.MaxEQLFetchSize != 10000 {
		t.Fatal("language defaults differ from resource contract")
	}
	resolveRelativePaths(v, "/srv/config")
	if v.TLS.CAFile != filepath.Join("/srv/config", "ca.pem") {
		t.Fatal("CA path not resolved")
	}
	before := c.ResourceForecastBytes()
	v.Elasticsearch.MaxJSONNodes *= 2
	if c.ResourceForecastBytes() <= before {
		t.Fatal("ES parser budget omitted from forecast")
	}
}

func TestElasticsearchEnrichRequiresExplicitSeparateDataScope(t *testing.T) {
	for _, scope := range []string{"", "reports-*", "policy_allowlist", ElasticsearchEnrichScope} {
		c := validESConfig()
		c.Targets["test"].Elasticsearch.Enrich = &ElasticsearchEnrichConfig{Scope: scope}
		err := c.Validate()
		if (err == nil) != (scope == ElasticsearchEnrichScope) {
			t.Fatalf("scope %q: %v", scope, err)
		}
	}
}

func TestElasticsearchStandaloneExampleLoads(t *testing.T) {
	if _, err := Load("../../configs/elasticsearch.example.yaml"); err != nil {
		t.Fatal(err)
	}
}
func TestElasticsearchRejectsConflictingOrUnboundedConfiguration(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*TargetConfig)
	}{
		{"http", func(t *TargetConfig) { t.Elasticsearch.Endpoints = []string{"http://localhost:9200"} }},
		{"url credentials", func(t *TargetConfig) { t.Elasticsearch.Endpoints = []string{"https://user:password@example.invalid"} }},
		{"url path", func(t *TargetConfig) { t.Elasticsearch.Endpoints = []string{"https://example.invalid/_search"} }},
		{"url query", func(t *TargetConfig) { t.Elasticsearch.Endpoints = []string{"https://example.invalid?x=1"} }},
		{"fake sql", func(t *TargetConfig) { t.Database = "fake" }},
		{"redis settings", func(t *TargetConfig) { t.Redis.Mode = "standalone" }},
		{"wrong version", func(t *TargetConfig) { t.Elasticsearch.Version = "9.99.0" }},
		{"unpinned uuid", func(t *TargetConfig) { t.Elasticsearch.ClusterUUID = "" }},
		{"long proof lease", func(t *TargetConfig) { t.Elasticsearch.PrivilegeRecheck = 10 * time.Minute }},
		{"scope path", func(t *TargetConfig) { t.Elasticsearch.AllowedIndices = []string{"reports-*/_search"} }},
		{"scope regex", func(t *TargetConfig) { t.Elasticsearch.AllowedIndices = []string{"/reports-.*/"} }},
		{"TLS bypass", func(t *TargetConfig) { t.TLS.Mode = TLSRequired }},
		{"common servername", func(t *TargetConfig) { t.TLS.ServerName = "other.invalid" }},
		{"excess connections", func(t *TargetConfig) { t.Connection.MaxOpen = 65 }},
		{"two secrets", func(t *TargetConfig) { t.PasswordEnv = "ANOTHER_SECRET"; t.PasswordFile = "password" }},
		{"result cache", func(t *TargetConfig) { t.ResultCache.Enabled = true }},
		{"context count", func(t *TargetConfig) { t.Elasticsearch.MaxOpenContexts = 129 }},
		{"context bytes", func(t *TargetConfig) { t.Elasticsearch.MaxContextIDBytes = (1 << 20) + 1 }},
		{"idle lifetime", func(t *TargetConfig) { t.Elasticsearch.ContextKeepAlive = 16 * time.Minute }},
		{"absolute lifetime", func(t *TargetConfig) { t.Elasticsearch.MaxContextLifetime = 3 * time.Hour }},
		{"absolute shorter than idle", func(t *TargetConfig) { t.Elasticsearch.MaxContextLifetime = time.Second }},
		{"language byte ceiling", func(t *TargetConfig) { t.Elasticsearch.MaxLanguageBytes = (1 << 20) + 1 }},
		{"language token ceiling", func(t *TargetConfig) { t.Elasticsearch.MaxLanguageTokens = 100001 }},
		{"language depth ceiling", func(t *TargetConfig) { t.Elasticsearch.MaxLanguageDepth = 513 }},
		{"EQL fetch ceiling", func(t *TargetConfig) { t.Elasticsearch.MaxEQLFetchSize = 100001 }},
		{"EQL native default fetch", func(t *TargetConfig) { t.Elasticsearch.MaxEQLFetchSize = 999 }},
		{"bucket ceiling", func(t *TargetConfig) { t.Elasticsearch.MaxAggregationBuckets = 65537 }},
		{"pragma negative ceiling", func(t *TargetConfig) {
			t.Elasticsearch.ESQLPragmas = &ElasticsearchESQLPragmaConfig{MaxTaskConcurrency: -1}
		}},
		{"pragma native shard ceiling", func(t *TargetConfig) {
			t.Elasticsearch.ESQLPragmas = &ElasticsearchESQLPragmaConfig{MaxConcurrentShardsPerNode: 101}
		}},
		{"pragma fold percentage", func(t *TargetConfig) {
			t.Elasticsearch.ESQLPragmas = &ElasticsearchESQLPragmaConfig{MaxFoldPercent: 101}
		}},
		{"duplicate readable script", func(t *TargetConfig) {
			t.Elasticsearch.ReadableScriptIDs = []string{"report", "report"}
		}},
		{"invalid readable script", func(t *TargetConfig) {
			t.Elasticsearch.ReadableScriptIDs = []string{"report\nsecret"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := validESConfig()
			test.change(c.Targets["test"])
			if c.Validate() == nil {
				t.Fatal("invalid ES configuration accepted")
			}
		})
	}
	c := validConfig()
	c.Targets["test"].Elasticsearch = &ElasticsearchConfig{}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "elasticsearch settings") {
		t.Fatalf("foreign block accepted: %v", err)
	}
}
