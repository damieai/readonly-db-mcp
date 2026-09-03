package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	EngineMySQL         = "mysql"
	EnginePostgreSQL    = "postgresql"
	EngineRedis         = "redis"
	TransportStdio      = "stdio"
	TLSDisabled         = "disabled"
	TLSRequired         = "required"
	TLSVerifyFull       = "verify-full"
	ConsistencyCurrent  = "current"
	ConsistencyEventual = "eventual"
)

var (
	safeName          = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)
	safeIdentifier    = regexp.MustCompile(`^[a-zA-Z0-9_$]{1,64}$`)
	systemSchemaNames = map[string]struct{}{
		"information_schema": {},
		"mysql":              {},
		"performance_schema": {},
		"sys":                {},
	}
)

type Config struct {
	Server  ServerConfig             `yaml:"server"`
	Limits  Limits                   `yaml:"limits"`
	Targets map[string]*TargetConfig `yaml:"targets"`
}

type ServerConfig struct {
	Transport              string        `yaml:"transport"`
	StrictStartup          bool          `yaml:"strict_startup"`
	MetricsSummaryInterval time.Duration `yaml:"metrics_summary_interval"`
}

type Limits struct {
	GlobalConcurrency      int             `yaml:"global_concurrency"`
	PerTargetConcurrency   int             `yaml:"per_target_concurrency"`
	DefaultTimeout         time.Duration   `yaml:"default_timeout"`
	MaxTimeout             time.Duration   `yaml:"max_timeout"`
	MaxRows                int             `yaml:"max_rows"`
	MaxResultBytes         int             `yaml:"max_result_bytes"`
	MaxCellBytes           int             `yaml:"max_cell_bytes"`
	MaxSQLBytes            int             `yaml:"max_sql_bytes"`
	MaxBatchQueries        int             `yaml:"max_batch_queries"`
	MaxParameters          int             `yaml:"max_parameters"`
	MaxParameterBytes      int             `yaml:"max_parameter_bytes"`
	MaxParameterValueBytes int             `yaml:"max_parameter_value_bytes"`
	MaxQueuedRequests      int             `yaml:"max_queued_requests"`
	QueueTimeout           time.Duration   `yaml:"queue_timeout"`
	WorkloadClasses        WorkloadClasses `yaml:"workload_classes"`
}

type WorkloadClasses struct {
	MetadataReserved          int `yaml:"metadata_reserved"`
	BatchMaxConcurrency       int `yaml:"batch_max_concurrency"`
	MaintenanceMaxConcurrency int `yaml:"maintenance_max_concurrency"`
}

type TargetConfig struct {
	Name           string              `yaml:"-"`
	Engine         string              `yaml:"engine"`
	Environment    string              `yaml:"environment"`
	Consistency    string              `yaml:"consistency"`
	Host           string              `yaml:"host"`
	Port           int                 `yaml:"port"`
	Database       string              `yaml:"database"`
	Username       string              `yaml:"username"`
	PasswordFile   string              `yaml:"password_file"`
	PasswordEnv    string              `yaml:"password_env"`
	AllowedSchemas []string            `yaml:"allowed_schemas"`
	DeniedTables   []string            `yaml:"denied_tables"`
	Connection     ConnectionConfig    `yaml:"connection"`
	TLS            TLSConfig           `yaml:"tls"`
	MySQL          MySQLConfig         `yaml:"mysql"`
	PostgreSQL     PostgreSQLConfig    `yaml:"postgresql"`
	Redis          RedisConfig         `yaml:"redis"`
	MetadataCache  MetadataCacheConfig `yaml:"metadata_cache"`
	ResultCache    ResultCacheConfig   `yaml:"result_cache"`
}

type MySQLConfig struct {
	PrivilegeRecheck time.Duration `yaml:"privilege_recheck_interval"`
}

type PostgreSQLConfig struct {
	ApplicationName        string        `yaml:"application_name"`
	StatementTimeoutMargin time.Duration `yaml:"statement_timeout_margin"`
	BatchIsolation         string        `yaml:"batch_isolation"`
	RequireHotStandby      bool          `yaml:"require_hot_standby"`
	PrivilegeRecheck       time.Duration `yaml:"privilege_recheck_interval"`
}

type RedisConfig struct {
	Mode                 string              `yaml:"mode"`
	Database             int                 `yaml:"database"`
	KeyPatterns          []string            `yaml:"key_patterns"`
	Protocol             int                 `yaml:"protocol"`
	ACLRecheck           time.Duration       `yaml:"acl_recheck_interval"`
	CatalogMaxAge        time.Duration       `yaml:"command_catalog_max_age"`
	AllowReadonlyScripts bool                `yaml:"allow_readonly_scripts"`
	MaxScriptBytes       int                 `yaml:"max_script_bytes"`
	MaxKeysPerCommand    int                 `yaml:"max_keys_per_command"`
	MaxArgumentBytes     int                 `yaml:"max_argument_bytes"`
	MaxReplyDepth        int                 `yaml:"max_reply_depth"`
	MaxReplyElements     int                 `yaml:"max_reply_elements"`
	Sentinel             RedisSentinelConfig `yaml:"sentinel"`
	Cluster              RedisClusterConfig  `yaml:"cluster"`
	ModuleProfiles       []string            `yaml:"module_profiles"`
	TrustedProfileKeys   map[string]string   `yaml:"trusted_profile_keys"`
	ModuleObjectPatterns map[string][]string `yaml:"module_object_patterns"`
}

type RedisSentinelConfig struct {
	ServiceName         string                 `yaml:"service_name"`
	Addresses           []string               `yaml:"addresses"`
	Username            string                 `yaml:"username"`
	PasswordFile        string                 `yaml:"password_file"`
	PasswordEnv         string                 `yaml:"password_env"`
	MinAgreement        int                    `yaml:"min_agreement"`
	DiscoveryTimeout    time.Duration          `yaml:"discovery_timeout"`
	RefreshInterval     time.Duration          `yaml:"refresh_interval"`
	ReadRole            string                 `yaml:"read_role"`
	EndpointAllowlist   RedisEndpointAllowlist `yaml:"endpoint_allowlist"`
	RequireMasterLinkUp *bool                  `yaml:"require_master_link_up"`
	MaxReplicaLagBytes  int64                  `yaml:"max_replica_lag_bytes"`
}

type RedisClusterConfig struct {
	SeedAddresses           []string               `yaml:"seed_addresses"`
	ReadRole                string                 `yaml:"read_role"`
	TopologyRefresh         time.Duration          `yaml:"topology_refresh_interval"`
	TopologyMaxAge          time.Duration          `yaml:"topology_max_age"`
	RedirectLimit           int                    `yaml:"redirect_limit"`
	RequireFullSlotCoverage *bool                  `yaml:"require_full_slot_coverage"`
	EndpointAllowlist       RedisEndpointAllowlist `yaml:"endpoint_allowlist"`
}

type RedisEndpointAllowlist struct {
	DNSSuffixes []string `yaml:"dns_suffixes"`
	CIDRs       []string `yaml:"cidrs"`
}

type MetadataCacheConfig struct {
	Enabled             *bool         `yaml:"enabled"`
	AllowFresh          *bool         `yaml:"allow_fresh"`
	FreshCooldown       time.Duration `yaml:"fresh_cooldown"`
	TableListTTL        time.Duration `yaml:"table_list_ttl"`
	TableDescriptionTTL time.Duration `yaml:"table_description_ttl"`
	NegativeTTL         time.Duration `yaml:"negative_ttl"`
	MaxEntries          int           `yaml:"max_entries"`
	MaxBytes            int           `yaml:"max_bytes"`
}

type ResultCacheConfig struct {
	Enabled                 bool          `yaml:"enabled"`
	TTL                     time.Duration `yaml:"ttl"`
	MaxEntries              int           `yaml:"max_entries"`
	MaxBytes                int           `yaml:"max_bytes"`
	MaxEntryBytes           int           `yaml:"max_entry_bytes"`
	AllowCurrentConsistency bool          `yaml:"allow_current_consistency"`
}

func (c MetadataCacheConfig) IsEnabled() bool      { return c.Enabled == nil || *c.Enabled }
func (c MetadataCacheConfig) IsFreshAllowed() bool { return c.AllowFresh == nil || *c.AllowFresh }

type ConnectionConfig struct {
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	MaxOpen        int           `yaml:"max_open"`
	MaxIdle        int           `yaml:"max_idle"`
	MaxLifetime    time.Duration `yaml:"max_lifetime"`
	MaxIdleTime    time.Duration `yaml:"max_idle_time"`
}

type TLSConfig struct {
	Mode                string `yaml:"mode"`
	AllowInsecureRemote bool   `yaml:"allow_insecure_remote"`
	CAFile              string `yaml:"ca_file"`
	CertFile            string `yaml:"cert_file"`
	KeyFile             string `yaml:"key_file"`
	ServerName          string `yaml:"server_name"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("configuration path is required")
	}
	b, err := readFileLimited(path, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("configuration must contain exactly one YAML document")
		}
		return nil, fmt.Errorf("decode configuration: %w", err)
	}

	applyDefaults(&cfg)
	configDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("resolve configuration directory: %w", err)
	}
	for name, target := range cfg.Targets {
		if target != nil {
			target.Name = name
			resolveRelativePaths(target, configDir)
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Transport == "" {
		cfg.Server.Transport = TransportStdio
	}
	if cfg.Limits.GlobalConcurrency == 0 {
		cfg.Limits.GlobalConcurrency = 4
	}
	if cfg.Limits.PerTargetConcurrency == 0 {
		cfg.Limits.PerTargetConcurrency = 2
	}
	if cfg.Limits.DefaultTimeout == 0 {
		cfg.Limits.DefaultTimeout = 3 * time.Second
	}
	if cfg.Limits.MaxTimeout == 0 {
		cfg.Limits.MaxTimeout = 10 * time.Second
	}
	if cfg.Limits.MaxRows == 0 {
		cfg.Limits.MaxRows = 500
	}
	if cfg.Limits.MaxResultBytes == 0 {
		cfg.Limits.MaxResultBytes = 1 << 20
	}
	if cfg.Limits.MaxCellBytes == 0 {
		cfg.Limits.MaxCellBytes = 64 << 10
	}
	if cfg.Limits.MaxSQLBytes == 0 {
		cfg.Limits.MaxSQLBytes = 32 << 10
	}
	if cfg.Limits.MaxBatchQueries == 0 {
		cfg.Limits.MaxBatchQueries = 10
	}
	if cfg.Limits.MaxParameters == 0 {
		cfg.Limits.MaxParameters = 100
	}
	if cfg.Limits.MaxParameterBytes == 0 {
		cfg.Limits.MaxParameterBytes = 1 << 20
	}
	if cfg.Limits.MaxParameterValueBytes == 0 {
		cfg.Limits.MaxParameterValueBytes = 256 << 10
	}
	if cfg.Limits.MaxQueuedRequests == 0 {
		cfg.Limits.MaxQueuedRequests = 32
	}
	if cfg.Limits.QueueTimeout == 0 {
		cfg.Limits.QueueTimeout = 500 * time.Millisecond
	}
	if cfg.Limits.WorkloadClasses.MetadataReserved == 0 && cfg.Limits.GlobalConcurrency >= 3 {
		cfg.Limits.WorkloadClasses.MetadataReserved = 1
	}
	if cfg.Limits.WorkloadClasses.BatchMaxConcurrency == 0 {
		cfg.Limits.WorkloadClasses.BatchMaxConcurrency = (cfg.Limits.GlobalConcurrency + 3) / 4
	}
	if cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency == 0 {
		cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency = 1
	}

	for _, target := range cfg.Targets {
		if target == nil {
			continue
		}
		if target.Engine == "" {
			target.Engine = EngineMySQL
		}
		if target.Consistency == "" {
			target.Consistency = ConsistencyCurrent
		}
		if target.Port == 0 {
			if target.Engine == EnginePostgreSQL {
				target.Port = 5432
			} else if target.Engine == EngineRedis {
				if target.Redis.Mode != "sentinel" && target.Redis.Mode != "cluster" {
					target.Port = 6379
				}
			} else {
				target.Port = 3306
			}
		}
		if target.Connection.ConnectTimeout == 0 {
			target.Connection.ConnectTimeout = 3 * time.Second
		}
		if target.Connection.ReadTimeout == 0 {
			target.Connection.ReadTimeout = cfg.Limits.MaxTimeout + 2*time.Second
		}
		if target.Connection.WriteTimeout == 0 {
			target.Connection.WriteTimeout = 3 * time.Second
		}
		if target.Connection.MaxOpen == 0 {
			target.Connection.MaxOpen = 2
		}
		if target.Connection.MaxIdle == 0 {
			target.Connection.MaxIdle = 1
		}
		if target.Connection.MaxLifetime == 0 {
			target.Connection.MaxLifetime = 3 * time.Minute
		}
		if target.Connection.MaxIdleTime == 0 {
			target.Connection.MaxIdleTime = time.Minute
		}
		if target.TLS.Mode == "" {
			target.TLS.Mode = TLSVerifyFull
		}
		if target.Engine == EnginePostgreSQL {
			if target.PostgreSQL.ApplicationName == "" {
				target.PostgreSQL.ApplicationName = "readonly-db-mcp"
			}
			if target.PostgreSQL.StatementTimeoutMargin == 0 {
				target.PostgreSQL.StatementTimeoutMargin = 250 * time.Millisecond
			}
			if target.PostgreSQL.BatchIsolation == "" {
				target.PostgreSQL.BatchIsolation = "repeatable-read"
			}
			if target.PostgreSQL.PrivilegeRecheck == 0 {
				target.PostgreSQL.PrivilegeRecheck = 5 * time.Minute
			}
		}
		if target.Engine == EngineMySQL && target.MySQL.PrivilegeRecheck == 0 {
			target.MySQL.PrivilegeRecheck = 5 * time.Minute
		}
		if target.Engine == EngineRedis {
			if target.Redis.Mode == "" {
				target.Redis.Mode = "standalone"
			}
			if target.Redis.Protocol == 0 {
				target.Redis.Protocol = 3
			}
			if target.Redis.ACLRecheck == 0 {
				target.Redis.ACLRecheck = 5 * time.Minute
			}
			if target.Redis.CatalogMaxAge == 0 {
				target.Redis.CatalogMaxAge = 10 * time.Minute
			}
			if target.Redis.MaxScriptBytes == 0 {
				target.Redis.MaxScriptBytes = 64 << 10
			}
			if target.Redis.MaxKeysPerCommand == 0 {
				target.Redis.MaxKeysPerCommand = 256
			}
			if target.Redis.MaxArgumentBytes == 0 {
				target.Redis.MaxArgumentBytes = 256 << 10
			}
			if target.Redis.MaxReplyDepth == 0 {
				target.Redis.MaxReplyDepth = 32
			}
			if target.Redis.MaxReplyElements == 0 {
				target.Redis.MaxReplyElements = 10_000
			}
			if target.Redis.Sentinel.MinAgreement == 0 {
				target.Redis.Sentinel.MinAgreement = 2
			}
			if target.Redis.Sentinel.DiscoveryTimeout == 0 {
				target.Redis.Sentinel.DiscoveryTimeout = 750 * time.Millisecond
			}
			if target.Redis.Sentinel.RefreshInterval == 0 {
				target.Redis.Sentinel.RefreshInterval = 5 * time.Second
			}
			if target.Redis.Sentinel.ReadRole == "" {
				target.Redis.Sentinel.ReadRole = "primary"
			}
			if target.Redis.Sentinel.RequireMasterLinkUp == nil {
				value := true
				target.Redis.Sentinel.RequireMasterLinkUp = &value
			}
			if target.Redis.Sentinel.MaxReplicaLagBytes == 0 {
				target.Redis.Sentinel.MaxReplicaLagBytes = 16 << 20
			}
			if target.Redis.Cluster.ReadRole == "" {
				target.Redis.Cluster.ReadRole = "primary"
			}
			if target.Redis.Cluster.TopologyRefresh == 0 {
				target.Redis.Cluster.TopologyRefresh = 5 * time.Second
			}
			if target.Redis.Cluster.TopologyMaxAge == 0 {
				target.Redis.Cluster.TopologyMaxAge = 30 * time.Second
			}
			if target.Redis.Cluster.RedirectLimit == 0 {
				target.Redis.Cluster.RedirectLimit = 3
			}
			if target.Redis.Cluster.RequireFullSlotCoverage == nil {
				value := true
				target.Redis.Cluster.RequireFullSlotCoverage = &value
			}
		}
		if target.Engine != EngineRedis {
			if target.MetadataCache.TableListTTL == 0 {
				target.MetadataCache.TableListTTL = 20 * time.Minute
			}
			if target.MetadataCache.FreshCooldown == 0 {
				target.MetadataCache.FreshCooldown = time.Second
			}
			if target.MetadataCache.TableDescriptionTTL == 0 {
				target.MetadataCache.TableDescriptionTTL = 20 * time.Minute
			}
			if target.MetadataCache.NegativeTTL == 0 {
				target.MetadataCache.NegativeTTL = 5 * time.Second
			}
			if target.MetadataCache.MaxEntries == 0 {
				target.MetadataCache.MaxEntries = 256
			}
			if target.MetadataCache.MaxBytes == 0 {
				target.MetadataCache.MaxBytes = 8 << 20
			}
		}
		if target.ResultCache.Enabled {
			if target.ResultCache.TTL == 0 {
				target.ResultCache.TTL = 10 * time.Second
			}
			if target.ResultCache.MaxEntries == 0 {
				target.ResultCache.MaxEntries = 128
			}
			if target.ResultCache.MaxBytes == 0 {
				target.ResultCache.MaxBytes = 16 << 20
			}
			if target.ResultCache.MaxEntryBytes == 0 {
				target.ResultCache.MaxEntryBytes = 256 << 10
			}
		}
	}
}

func resolveRelativePaths(target *TargetConfig, configDir string) {
	for _, path := range []*string{&target.PasswordFile, &target.TLS.CAFile, &target.TLS.CertFile, &target.TLS.KeyFile, &target.Redis.Sentinel.PasswordFile} {
		if *path != "" && !filepath.IsAbs(*path) {
			*path = filepath.Join(configDir, *path)
		}
	}
	for i := range target.Redis.ModuleProfiles {
		if !filepath.IsAbs(target.Redis.ModuleProfiles[i]) {
			target.Redis.ModuleProfiles[i] = filepath.Join(configDir, target.Redis.ModuleProfiles[i])
		}
	}
}

func (cfg *Config) Validate() error {
	var problems []string
	if cfg.Server.Transport != TransportStdio {
		problems = append(problems, "server.transport must be stdio")
	}
	if !cfg.Server.StrictStartup {
		problems = append(problems, "server.strict_startup must be true")
	}
	if cfg.Server.MetricsSummaryInterval < 0 || (cfg.Server.MetricsSummaryInterval > 0 && (cfg.Server.MetricsSummaryInterval < time.Second || cfg.Server.MetricsSummaryInterval > time.Hour)) {
		problems = append(problems, "server.metrics_summary_interval must be disabled or between 1s and 1h")
	}
	if len(cfg.Targets) == 0 {
		problems = append(problems, "at least one target is required")
	}
	if len(cfg.Targets) > 64 {
		problems = append(problems, "targets must not contain more than 64 entries")
	}
	if cfg.Limits.GlobalConcurrency < 1 || cfg.Limits.GlobalConcurrency > 32 {
		problems = append(problems, "limits.global_concurrency must be between 1 and 32")
	}
	if cfg.Limits.PerTargetConcurrency < 1 || cfg.Limits.PerTargetConcurrency > cfg.Limits.GlobalConcurrency {
		problems = append(problems, "limits.per_target_concurrency must be positive and no greater than global_concurrency")
	}
	if cfg.Limits.DefaultTimeout <= 0 || cfg.Limits.MaxTimeout <= 0 || cfg.Limits.DefaultTimeout > cfg.Limits.MaxTimeout || cfg.Limits.MaxTimeout > time.Minute {
		problems = append(problems, "query timeouts are invalid or exceed the one-minute hard ceiling")
	}
	if cfg.Limits.MaxRows < 1 || cfg.Limits.MaxRows > 10_000 {
		problems = append(problems, "limits.max_rows must be between 1 and 10000")
	}
	if cfg.Limits.MaxResultBytes < 1024 || cfg.Limits.MaxResultBytes > 16<<20 {
		problems = append(problems, "limits.max_result_bytes must be between 1 KiB and 16 MiB")
	}
	if cfg.Limits.MaxCellBytes < 256 || cfg.Limits.MaxCellBytes > cfg.Limits.MaxResultBytes {
		problems = append(problems, "limits.max_cell_bytes must be between 256 bytes and max_result_bytes")
	}
	if cfg.Limits.MaxSQLBytes < 256 || cfg.Limits.MaxSQLBytes > 1<<20 {
		problems = append(problems, "limits.max_sql_bytes must be between 256 bytes and 1 MiB")
	}
	if cfg.Limits.MaxBatchQueries < 1 || cfg.Limits.MaxBatchQueries > 100 {
		problems = append(problems, "limits.max_batch_queries must be between 1 and 100")
	}
	if cfg.Limits.MaxParameters < 1 || cfg.Limits.MaxParameters > 10_000 {
		problems = append(problems, "limits.max_parameters must be between 1 and 10000")
	}
	if cfg.Limits.MaxParameterValueBytes < 1024 || cfg.Limits.MaxParameterValueBytes > 16<<20 || cfg.Limits.MaxParameterBytes < cfg.Limits.MaxParameterValueBytes || cfg.Limits.MaxParameterBytes > 64<<20 {
		problems = append(problems, "SQL parameter byte limits are invalid")
	}
	if cfg.Limits.MaxQueuedRequests < 1 || cfg.Limits.MaxQueuedRequests > 1024 {
		problems = append(problems, "limits.max_queued_requests must be between 1 and 1024")
	}
	if cfg.Limits.QueueTimeout < time.Millisecond || cfg.Limits.QueueTimeout > 30*time.Second || cfg.Limits.QueueTimeout > cfg.Limits.MaxTimeout {
		problems = append(problems, "limits.queue_timeout must be between 1ms and 30s and no greater than max_timeout")
	}
	wc := cfg.Limits.WorkloadClasses
	if wc.MetadataReserved < 0 || wc.MetadataReserved > cfg.Limits.GlobalConcurrency {
		problems = append(problems, "metadata_reserved is outside global concurrency")
	}
	if wc.BatchMaxConcurrency < 1 || wc.BatchMaxConcurrency > cfg.Limits.GlobalConcurrency {
		problems = append(problems, "batch_max_concurrency is outside global concurrency")
	}
	if wc.MaintenanceMaxConcurrency < 1 || wc.MaintenanceMaxConcurrency > cfg.Limits.GlobalConcurrency {
		problems = append(problems, "maintenance_max_concurrency is outside global concurrency")
	}

	names := make([]string, 0, len(cfg.Targets))
	totalMaxOpen, totalMaxIdle := 0, 0
	for name := range cfg.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := cfg.Targets[name]
		if target == nil {
			problems = append(problems, fmt.Sprintf("target %q is null", name))
			continue
		}
		totalMaxOpen += target.Connection.MaxOpen
		totalMaxIdle += target.Connection.MaxIdle
		for _, problem := range validateTarget(name, target, cfg.Limits) {
			problems = append(problems, fmt.Sprintf("target %q: %s", name, problem))
		}
	}
	if totalMaxOpen > 256 || totalMaxIdle > 128 {
		problems = append(problems, "aggregate target connection pools exceed 256 open or 128 idle connections")
	}
	if cfg.ResourceForecastBytes() > 1<<30 {
		problems = append(problems, "configured cache and concurrent response forecast exceeds 1 GiB hard ceiling")
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid configuration:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

// ResourceForecastBytes conservatively estimates configured cache storage and
// concurrent response object/encoding copies. It excludes the Go runtime,
// parser state, driver buffers, and database server memory.
func (cfg *Config) ResourceForecastBytes() int64 {
	total := int64(cfg.Limits.GlobalConcurrency) * (int64(cfg.Limits.MaxResultBytes)*3 + int64(cfg.Limits.MaxParameterBytes))
	for _, target := range cfg.Targets {
		if target != nil {
			if target.Engine != EngineRedis {
				total += int64(target.MetadataCache.MaxBytes) + int64(target.ResultCache.MaxBytes)
			}
			if target.Engine == EngineRedis && target.Redis.Mode == "cluster" {
				total += 2 * 256 * 64 << 10
			}
		}
	}
	return total
}

func validateTarget(name string, target *TargetConfig, limits Limits) []string {
	var problems []string
	if !safeName.MatchString(name) {
		problems = append(problems, "name must match [a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}")
	}
	if target.Engine != EngineMySQL && target.Engine != EnginePostgreSQL && target.Engine != EngineRedis {
		problems = append(problems, "engine must be mysql, postgresql, or redis")
	}
	if !safeName.MatchString(target.Environment) {
		problems = append(problems, "environment is required and must be a safe identifier")
	}
	if target.Consistency != ConsistencyCurrent && target.Consistency != ConsistencyEventual {
		problems = append(problems, "consistency must be current or eventual")
	}
	if target.Engine != EngineRedis || target.Redis.Mode == "standalone" {
		if strings.TrimSpace(target.Host) == "" {
			problems = append(problems, "host is required")
		}
		if target.Port < 1 || target.Port > 65535 {
			problems = append(problems, "port is invalid")
		}
	} else if target.Host != "" || target.Port != 0 {
		problems = append(problems, "top-level host/port must be omitted for Redis Sentinel and Cluster targets")
	}
	if target.Engine != EngineRedis && !safeIdentifier.MatchString(target.Database) {
		problems = append(problems, "database is required and must be a safe identifier")
	}
	if _, system := systemSchemaNames[strings.ToLower(target.Database)]; system && target.Engine != EngineRedis {
		problems = append(problems, "database must not be a system schema")
	}
	if strings.TrimSpace(target.Username) == "" {
		problems = append(problems, "username is required")
	}
	if strings.EqualFold(target.Username, "root") || strings.Contains(strings.ToLower(target.Username), "admin") {
		problems = append(problems, "privileged-looking usernames are refused")
	}
	if (target.PasswordFile == "") == (target.PasswordEnv == "") {
		problems = append(problems, "configure exactly one of password_file or password_env")
	}
	if target.PasswordEnv != "" && !regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,127}$`).MatchString(target.PasswordEnv) {
		problems = append(problems, "password_env must name an uppercase environment variable")
	}
	if target.Engine != EngineRedis && len(target.AllowedSchemas) == 0 {
		problems = append(problems, "allowed_schemas must not be empty")
	}
	seenSchemas := make(map[string]struct{}, len(target.AllowedSchemas))
	databaseAllowed := false
	for _, schema := range target.AllowedSchemas {
		if !safeIdentifier.MatchString(schema) {
			problems = append(problems, fmt.Sprintf("allowed schema %q is invalid", schema))
		}
		lower := strings.ToLower(schema)
		if _, system := systemSchemaNames[lower]; system {
			problems = append(problems, fmt.Sprintf("system schema %q cannot be allowed", schema))
		}
		if _, duplicate := seenSchemas[lower]; duplicate {
			problems = append(problems, fmt.Sprintf("allowed schema %q is duplicated", schema))
		}
		seenSchemas[lower] = struct{}{}
		if strings.EqualFold(schema, target.Database) {
			databaseAllowed = true
		}
	}
	if target.Engine == EngineMySQL && !databaseAllowed {
		problems = append(problems, "database must be included in allowed_schemas")
	}
	for _, selector := range target.DeniedTables {
		parts := strings.Split(selector, ".")
		valid := len(parts) == 1 || len(parts) == 2
		for _, part := range parts {
			valid = valid && safeIdentifier.MatchString(part)
		}
		if !valid {
			problems = append(problems, fmt.Sprintf("denied table %q must be table or schema.table", selector))
		}
	}
	if target.Connection.MaxOpen < 1 || target.Connection.MaxOpen > 16 {
		problems = append(problems, "connection.max_open must be between 1 and 16")
	}
	if target.Connection.MaxIdle < 0 || target.Connection.MaxIdle > target.Connection.MaxOpen {
		problems = append(problems, "connection.max_idle must be between 0 and max_open")
	}
	if target.Connection.MaxOpen < limits.PerTargetConcurrency {
		problems = append(problems, "connection.max_open must be at least per_target_concurrency")
	}
	if target.Connection.ReadTimeout < limits.MaxTimeout {
		problems = append(problems, "connection.read_timeout must be at least limits.max_timeout")
	}
	switch target.TLS.Mode {
	case TLSDisabled:
		if !redisTargetIsLoopback(target) && !target.TLS.AllowInsecureRemote {
			problems = append(problems, "TLS may be disabled for a remote database only when tls.allow_insecure_remote is true")
		}
		if isProductionEnvironment(target.Environment) {
			problems = append(problems, "TLS cannot be disabled for production")
		}
	case TLSRequired:
		if target.TLS.AllowInsecureRemote {
			problems = append(problems, "tls.allow_insecure_remote is valid only when tls.mode is disabled")
		}
		if isProductionEnvironment(target.Environment) {
			problems = append(problems, "production requires tls.mode verify-full")
		}
	case TLSVerifyFull:
		if target.TLS.AllowInsecureRemote {
			problems = append(problems, "tls.allow_insecure_remote is valid only when tls.mode is disabled")
		}
		if target.TLS.CAFile == "" {
			problems = append(problems, "tls.ca_file is required for verify-full")
		}
		if target.TLS.ServerName == "" {
			problems = append(problems, "tls.server_name is required for verify-full")
		}
	default:
		problems = append(problems, "tls.mode must be disabled, required, or verify-full")
	}
	if (target.TLS.CertFile == "") != (target.TLS.KeyFile == "") {
		problems = append(problems, "tls.cert_file and tls.key_file must be configured together")
	}
	if target.Engine == EngineMySQL {
		if target.MySQL.PrivilegeRecheck < 10*time.Second || target.MySQL.PrivilegeRecheck > time.Hour {
			problems = append(problems, "mysql.privilege_recheck_interval must be between 10s and 1h")
		}
		if target.PostgreSQL != (PostgreSQLConfig{}) {
			problems = append(problems, "postgresql settings are valid only for postgresql targets")
		}
		if !redisConfigEmpty(target.Redis) {
			problems = append(problems, "redis settings are valid only for redis targets")
		}
	} else if target.Engine == EnginePostgreSQL {
		if target.MySQL != (MySQLConfig{}) {
			problems = append(problems, "mysql settings are valid only for mysql targets")
		}
		if !redisConfigEmpty(target.Redis) {
			problems = append(problems, "redis settings are valid only for redis targets")
		}
		pg := target.PostgreSQL
		if !safeName.MatchString(pg.ApplicationName) {
			problems = append(problems, "postgresql.application_name must be a safe identifier")
		}
		if pg.StatementTimeoutMargin < 0 || pg.StatementTimeoutMargin > 5*time.Second || pg.StatementTimeoutMargin >= limits.MaxTimeout {
			problems = append(problems, "postgresql.statement_timeout_margin must be non-negative, below 5s and below max_timeout")
		}
		if pg.BatchIsolation != "repeatable-read" {
			problems = append(problems, "postgresql.batch_isolation must be repeatable-read")
		}
		if pg.PrivilegeRecheck < 10*time.Second || pg.PrivilegeRecheck > time.Hour {
			problems = append(problems, "postgresql.privilege_recheck_interval must be between 10s and 1h")
		}
	} else if target.Engine == EngineRedis {
		if target.MySQL != (MySQLConfig{}) {
			problems = append(problems, "mysql settings are valid only for mysql targets")
		}
		if target.PostgreSQL != (PostgreSQLConfig{}) {
			problems = append(problems, "postgresql settings are valid only for postgresql targets")
		}
		r := target.Redis
		if strings.EqualFold(target.Username, "default") {
			problems = append(problems, "Redis targets require a dedicated ACL user, not default")
		}
		if target.MetadataCache != (MetadataCacheConfig{}) || target.ResultCache != (ResultCacheConfig{}) {
			problems = append(problems, "SQL metadata/result cache settings are not valid for Redis targets")
		}
		if r.Mode != "standalone" && r.Mode != "sentinel" && r.Mode != "cluster" {
			problems = append(problems, "redis.mode must be standalone, sentinel, or cluster")
		}
		if r.Database < 0 || r.Database > 15 {
			problems = append(problems, "redis.database must be between 0 and 15")
		}
		if r.Protocol != 3 {
			problems = append(problems, "redis.protocol must be 3")
		}
		if len(r.KeyPatterns) == 0 {
			problems = append(problems, "redis.key_patterns must not be empty")
		}
		for _, pattern := range r.KeyPatterns {
			if !validRedisKeyPattern(pattern) {
				problems = append(problems, fmt.Sprintf("redis key pattern %q must be a non-empty literal prefix ending in *", pattern))
			}
		}
		if r.ACLRecheck < 10*time.Second || r.ACLRecheck > time.Hour || r.CatalogMaxAge < r.ACLRecheck || r.CatalogMaxAge > 2*time.Hour {
			problems = append(problems, "redis attestation intervals are invalid")
		}
		if r.MaxScriptBytes < 1024 || r.MaxScriptBytes > 1<<20 || r.MaxKeysPerCommand < 1 || r.MaxKeysPerCommand > 10_000 || r.MaxArgumentBytes < 1024 || r.MaxArgumentBytes > 16<<20 {
			problems = append(problems, "redis request ceilings are invalid")
		}
		if r.MaxReplyDepth < 1 || r.MaxReplyDepth > 128 || r.MaxReplyElements < 1 || r.MaxReplyElements > 1_000_000 {
			problems = append(problems, "redis reply ceilings are invalid")
		}
		switch r.Mode {
		case "standalone":
			if len(r.Sentinel.Addresses) != 0 || r.Sentinel.ServiceName != "" || len(r.Cluster.SeedAddresses) != 0 {
				problems = append(problems, "Sentinel/Cluster endpoints are not valid in standalone mode")
			}
		case "sentinel":
			if !safeName.MatchString(r.Sentinel.ServiceName) {
				problems = append(problems, "redis.sentinel.service_name is invalid")
			}
			if len(r.Sentinel.Addresses) < 2 || len(r.Sentinel.Addresses) > 16 {
				problems = append(problems, "redis.sentinel.addresses must contain 2-16 endpoints")
			}
			for _, address := range r.Sentinel.Addresses {
				if !validHostPort(address) {
					problems = append(problems, fmt.Sprintf("invalid Redis Sentinel address %q", address))
				}
			}
			if (r.Sentinel.PasswordFile == "") == (r.Sentinel.PasswordEnv == "") {
				problems = append(problems, "configure exactly one Redis Sentinel password source")
			}
			if r.Sentinel.MinAgreement < 2 || r.Sentinel.MinAgreement > len(r.Sentinel.Addresses) {
				problems = append(problems, "redis.sentinel.min_agreement is invalid")
			}
			if r.Sentinel.DiscoveryTimeout < 100*time.Millisecond || r.Sentinel.DiscoveryTimeout > 5*time.Second || r.Sentinel.RefreshInterval < time.Second || r.Sentinel.RefreshInterval > time.Minute {
				problems = append(problems, "Redis Sentinel discovery intervals are invalid")
			}
			if r.Sentinel.ReadRole != "primary" && r.Sentinel.ReadRole != "replica" {
				problems = append(problems, "redis.sentinel.read_role must be primary or replica")
			}
			if r.Sentinel.ReadRole == "replica" && target.Consistency != ConsistencyEventual {
				problems = append(problems, "Redis Sentinel replica reads require eventual consistency")
			}
			if len(r.Sentinel.EndpointAllowlist.CIDRs) == 0 {
				problems = append(problems, "redis.sentinel.endpoint_allowlist.cidrs must not be empty")
			}
			problems = append(problems, validateEndpointAllowlist(r.Sentinel.EndpointAllowlist)...)
			if r.Sentinel.RequireMasterLinkUp == nil || !*r.Sentinel.RequireMasterLinkUp {
				problems = append(problems, "Redis Sentinel requires master link verification")
			}
			if r.Sentinel.MaxReplicaLagBytes < 1 || r.Sentinel.MaxReplicaLagBytes > 1<<30 {
				problems = append(problems, "redis.sentinel.max_replica_lag_bytes must be between 1 byte and 1 GiB")
			}
			if len(r.Cluster.SeedAddresses) != 0 {
				problems = append(problems, "Redis Cluster settings are not valid in Sentinel mode")
			}
		case "cluster":
			if r.Database != 0 {
				problems = append(problems, "Redis Cluster database must be zero")
			}
			if len(r.Cluster.SeedAddresses) < 1 || len(r.Cluster.SeedAddresses) > 32 {
				problems = append(problems, "redis.cluster.seed_addresses must contain 1-32 endpoints")
			}
			for _, address := range r.Cluster.SeedAddresses {
				if !validHostPort(address) {
					problems = append(problems, fmt.Sprintf("invalid Redis Cluster seed address %q", address))
				}
			}
			if r.Cluster.ReadRole != "primary" && r.Cluster.ReadRole != "replica" {
				problems = append(problems, "redis.cluster.read_role must be primary or replica")
			}
			if r.Cluster.ReadRole == "replica" && target.Consistency != ConsistencyEventual {
				problems = append(problems, "Redis Cluster replica reads require eventual consistency")
			}
			if r.Cluster.TopologyRefresh < time.Second || r.Cluster.TopologyRefresh > time.Minute || r.Cluster.TopologyMaxAge < r.Cluster.TopologyRefresh || r.Cluster.TopologyMaxAge > 5*time.Minute {
				problems = append(problems, "Redis Cluster topology intervals are invalid")
			}
			if r.Cluster.RedirectLimit < 1 || r.Cluster.RedirectLimit > 8 || r.Cluster.RequireFullSlotCoverage == nil || !*r.Cluster.RequireFullSlotCoverage {
				problems = append(problems, "Redis Cluster requires 1-8 redirects and full slot coverage")
			}
			if len(r.Cluster.EndpointAllowlist.CIDRs) == 0 {
				problems = append(problems, "redis.cluster.endpoint_allowlist.cidrs must not be empty")
			}
			problems = append(problems, validateEndpointAllowlist(r.Cluster.EndpointAllowlist)...)
			if len(r.Sentinel.Addresses) != 0 || r.Sentinel.ServiceName != "" {
				problems = append(problems, "Redis Sentinel settings are not valid in Cluster mode")
			}
		}
		if len(r.ModuleProfiles) > 0 && len(r.TrustedProfileKeys) == 0 {
			problems = append(problems, "Redis module profiles require trusted_profile_keys")
		}
		for command, patterns := range r.ModuleObjectPatterns {
			if command == "" || strings.ToUpper(command) != command || len(patterns) == 0 {
				problems = append(problems, "redis.module_object_patterns requires uppercase command names and non-empty scopes")
			}
			for _, pattern := range patterns {
				if !validRedisKeyPattern(pattern) {
					problems = append(problems, fmt.Sprintf("Redis module object pattern %q is invalid", pattern))
				}
			}
		}
	}
	if target.Engine != EngineRedis {
		mc := target.MetadataCache
		if mc.TableListTTL < time.Second || mc.TableListTTL > 24*time.Hour || mc.TableDescriptionTTL < time.Second || mc.TableDescriptionTTL > 24*time.Hour || mc.NegativeTTL < time.Second || mc.NegativeTTL > time.Minute {
			problems = append(problems, "metadata cache TTLs are outside hard ceilings")
		}
		if mc.FreshCooldown < 100*time.Millisecond || mc.FreshCooldown > time.Minute {
			problems = append(problems, "metadata fresh_cooldown must be between 100ms and 1m")
		}
		if mc.MaxEntries < 1 || mc.MaxEntries > 10_000 || mc.MaxBytes < 1024 || mc.MaxBytes > 256<<20 {
			problems = append(problems, "metadata cache capacity is outside hard ceilings")
		}
	}
	rc := target.ResultCache
	if rc.Enabled {
		if target.Consistency == ConsistencyCurrent && !rc.AllowCurrentConsistency {
			problems = append(problems, "result cache on current consistency requires allow_current_consistency")
		}
		if rc.TTL <= 0 || rc.TTL > time.Hour || rc.MaxEntries < 1 || rc.MaxEntries > 10_000 || rc.MaxBytes < 1024 || rc.MaxBytes > 512<<20 || rc.MaxEntryBytes < 1024 || rc.MaxEntryBytes > rc.MaxBytes {
			problems = append(problems, "result cache settings are outside hard ceilings")
		}
	}
	return problems
}

func redisConfigEmpty(r RedisConfig) bool {
	return r.Mode == "" && r.Database == 0 && len(r.KeyPatterns) == 0 && r.Protocol == 0 && r.ACLRecheck == 0 && r.CatalogMaxAge == 0 && !r.AllowReadonlyScripts && r.MaxScriptBytes == 0 && r.MaxKeysPerCommand == 0 && r.MaxArgumentBytes == 0 && r.MaxReplyDepth == 0 && r.MaxReplyElements == 0 && len(r.Sentinel.Addresses) == 0 && r.Sentinel.ServiceName == "" && len(r.Cluster.SeedAddresses) == 0 && len(r.ModuleProfiles) == 0 && len(r.TrustedProfileKeys) == 0 && len(r.ModuleObjectPatterns) == 0
}

func validHostPort(value string) bool {
	host, port, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(host) == "" {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n > 0 && n <= 65535
}

func validateEndpointAllowlist(value RedisEndpointAllowlist) []string {
	var problems []string
	for _, suffix := range value.DNSSuffixes {
		if len(suffix) < 2 || suffix[0] != '.' || strings.ContainsAny(suffix, " /\\\t\r\n") {
			problems = append(problems, fmt.Sprintf("invalid Redis endpoint DNS suffix %q", suffix))
		}
	}
	for _, cidr := range value.CIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			problems = append(problems, fmt.Sprintf("invalid Redis endpoint CIDR %q", cidr))
		}
	}
	return problems
}

func redisTargetIsLoopback(target *TargetConfig) bool {
	if target.Engine != EngineRedis || target.Redis.Mode == "standalone" {
		return isLoopbackHost(target.Host)
	}
	var addresses []string
	if target.Redis.Mode == "sentinel" {
		addresses = target.Redis.Sentinel.Addresses
	} else {
		addresses = target.Redis.Cluster.SeedAddresses
	}
	for _, address := range addresses {
		host, _, err := net.SplitHostPort(address)
		if err != nil || !isLoopbackHost(host) {
			return false
		}
	}
	return len(addresses) > 0
}

func validRedisKeyPattern(pattern string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) < 2 || !strings.HasSuffix(pattern, "*") || strings.ContainsAny(strings.TrimSuffix(pattern, "*"), "*?[]\\") {
		return false
	}
	return !strings.ContainsAny(pattern, "\x00\r\n")
}

func isProductionEnvironment(value string) bool {
	value = strings.ToLower(value)
	return value == "prod" || value == "production" ||
		strings.HasPrefix(value, "prod-") || strings.HasPrefix(value, "production-") ||
		strings.HasSuffix(value, "-prod") || strings.HasSuffix(value, "-production")
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (target *TargetConfig) Password() (string, error) {
	return readSecret(target.PasswordFile, target.PasswordEnv)
}

func (target *TargetConfig) RedisSentinelPassword() (string, error) {
	return readSecret(target.Redis.Sentinel.PasswordFile, target.Redis.Sentinel.PasswordEnv)
}

func readSecret(passwordFile, passwordEnv string) (string, error) {
	if passwordEnv != "" {
		value, ok := os.LookupEnv(passwordEnv)
		if !ok || value == "" {
			return "", fmt.Errorf("password environment variable %q is empty", passwordEnv)
		}
		return value, nil
	}
	file, err := os.Open(passwordFile)
	if err != nil {
		return "", fmt.Errorf("open password file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat password file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return "", errors.New("password file must be a regular file no larger than 64 KiB")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("password file %q must not be accessible by group or others", passwordFile)
	}
	b, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil {
		return "", fmt.Errorf("read password file: %w", err)
	}
	if len(b) > 64<<10 {
		return "", errors.New("password file exceeds 64 KiB")
	}
	password := strings.TrimRight(string(b), "\r\n")
	if password == "" {
		return "", errors.New("password file is empty")
	}
	return password, nil
}

func readFileLimited(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, fmt.Errorf("file must be regular and no larger than %d bytes", maximum)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	return data, nil
}
