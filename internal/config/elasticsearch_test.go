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
	if v.Port != 0 || v.Database != "" || v.Consistency != ConsistencyEventual || v.Elasticsearch.PrivilegeRecheck != time.Minute {
		t.Fatal("ES acquired SQL defaults")
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
