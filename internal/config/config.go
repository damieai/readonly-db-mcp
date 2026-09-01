package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	EngineMySQL         = "mysql"
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
	Transport     string `yaml:"transport"`
	StrictStartup bool   `yaml:"strict_startup"`
}

type Limits struct {
	GlobalConcurrency    int           `yaml:"global_concurrency"`
	PerTargetConcurrency int           `yaml:"per_target_concurrency"`
	DefaultTimeout       time.Duration `yaml:"default_timeout"`
	MaxTimeout           time.Duration `yaml:"max_timeout"`
	MaxRows              int           `yaml:"max_rows"`
	MaxResultBytes       int           `yaml:"max_result_bytes"`
	MaxCellBytes         int           `yaml:"max_cell_bytes"`
	MaxSQLBytes          int           `yaml:"max_sql_bytes"`
	MaxBatchQueries      int           `yaml:"max_batch_queries"`
	MaxParameters        int           `yaml:"max_parameters"`
}

type TargetConfig struct {
	Name           string           `yaml:"-"`
	Engine         string           `yaml:"engine"`
	Environment    string           `yaml:"environment"`
	Consistency    string           `yaml:"consistency"`
	Host           string           `yaml:"host"`
	Port           int              `yaml:"port"`
	Database       string           `yaml:"database"`
	Username       string           `yaml:"username"`
	PasswordFile   string           `yaml:"password_file"`
	PasswordEnv    string           `yaml:"password_env"`
	AllowedSchemas []string         `yaml:"allowed_schemas"`
	DeniedTables   []string         `yaml:"denied_tables"`
	Connection     ConnectionConfig `yaml:"connection"`
	TLS            TLSConfig        `yaml:"tls"`
}

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
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
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
			target.Port = 3306
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
	}
}

func resolveRelativePaths(target *TargetConfig, configDir string) {
	for _, path := range []*string{&target.PasswordFile, &target.TLS.CAFile, &target.TLS.CertFile, &target.TLS.KeyFile} {
		if *path != "" && !filepath.IsAbs(*path) {
			*path = filepath.Join(configDir, *path)
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
	if len(cfg.Targets) == 0 {
		problems = append(problems, "at least one target is required")
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

	names := make([]string, 0, len(cfg.Targets))
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
		for _, problem := range validateTarget(name, target, cfg.Limits) {
			problems = append(problems, fmt.Sprintf("target %q: %s", name, problem))
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid configuration:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func validateTarget(name string, target *TargetConfig, limits Limits) []string {
	var problems []string
	if !safeName.MatchString(name) {
		problems = append(problems, "name must match [a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}")
	}
	if target.Engine != EngineMySQL {
		problems = append(problems, "engine is not supported; currently only mysql is available")
	}
	if !safeName.MatchString(target.Environment) {
		problems = append(problems, "environment is required and must be a safe identifier")
	}
	if target.Consistency != ConsistencyCurrent && target.Consistency != ConsistencyEventual {
		problems = append(problems, "consistency must be current or eventual")
	}
	if strings.TrimSpace(target.Host) == "" {
		problems = append(problems, "host is required")
	}
	if target.Port < 1 || target.Port > 65535 {
		problems = append(problems, "port is invalid")
	}
	if !safeIdentifier.MatchString(target.Database) {
		problems = append(problems, "database is required and must be a safe identifier")
	}
	if _, system := systemSchemaNames[strings.ToLower(target.Database)]; system {
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
	if len(target.AllowedSchemas) == 0 {
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
	if !databaseAllowed {
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
	if target.Connection.ReadTimeout < limits.MaxTimeout {
		problems = append(problems, "connection.read_timeout must be at least limits.max_timeout")
	}
	switch target.TLS.Mode {
	case TLSDisabled:
		if !isLoopbackHost(target.Host) && !target.TLS.AllowInsecureRemote {
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
	return problems
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
	if target.PasswordEnv != "" {
		value, ok := os.LookupEnv(target.PasswordEnv)
		if !ok || value == "" {
			return "", fmt.Errorf("password environment variable %q is empty", target.PasswordEnv)
		}
		return value, nil
	}
	info, err := os.Stat(target.PasswordFile)
	if err != nil {
		return "", fmt.Errorf("stat password file: %w", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("password file %q must not be accessible by group or others", target.PasswordFile)
	}
	b, err := os.ReadFile(target.PasswordFile)
	if err != nil {
		return "", fmt.Errorf("read password file: %w", err)
	}
	password := strings.TrimRight(string(b), "\r\n")
	if password == "" {
		return "", errors.New("password file is empty")
	}
	return password, nil
}
