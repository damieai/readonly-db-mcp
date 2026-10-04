package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SQLiteConfig struct {
	Root string `yaml:"root"`
	File string `yaml:"file"`
}

func defaultSQLite(t *TargetConfig) {
	if t.SQLite == nil {
		t.SQLite = &SQLiteConfig{}
	}
}

// SQLitePath resolves both paths before each open, so an existing symlink cannot
// quietly redirect an operator's configured file outside the configured root.
func (t *TargetConfig) SQLitePath() (string, error) {
	if t.SQLite == nil || t.SQLite.Root == "" || t.SQLite.File == "" {
		return "", fmt.Errorf("sqlite.root and sqlite.file are required")
	}
	root, err := filepath.EvalSymlinks(t.SQLite.Root)
	if err != nil {
		return "", fmt.Errorf("resolve sqlite.root: %w", err)
	}
	file, err := filepath.EvalSymlinks(t.SQLite.File)
	if err != nil {
		return "", fmt.Errorf("resolve sqlite.file: %w", err)
	}
	rootInfo, err := os.Stat(root)
	if err != nil || !rootInfo.IsDir() {
		return "", fmt.Errorf("sqlite.root must be an existing directory")
	}
	fileInfo, err := os.Stat(file)
	if err != nil || !fileInfo.Mode().IsRegular() {
		return "", fmt.Errorf("sqlite.file must be an existing regular file")
	}
	rel, err := filepath.Rel(root, file)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("sqlite.file must resolve inside sqlite.root")
	}
	return file, nil
}

func validateSQLite(t *TargetConfig, limits Limits) []string {
	var problems []string
	if !safeName.MatchString(t.Environment) {
		problems = append(problems, "environment is required and must be a safe identifier")
	}
	if t.Consistency != ConsistencyCurrent {
		problems = append(problems, "SQLite requires current consistency")
	}
	if t.Host != "" || t.Port != 0 || t.Database != "" || t.Username != "" || t.PasswordFile != "" || t.PasswordEnv != "" {
		problems = append(problems, "SQLite rejects network and credential fields")
	}
	if len(t.AllowedSchemas) != 0 || len(t.DeniedTables) != 0 || t.Elasticsearch != nil || t.Qdrant != nil || t.MongoDB != nil || t.MySQL != (MySQLConfig{}) || t.PostgreSQL != (PostgreSQLConfig{}) || t.SQLServer != (SQLServerConfig{}) || !redisConfigEmpty(t.Redis) {
		problems = append(problems, "SQLite rejects other engine settings")
	}
	if t.TLS.Mode != TLSDisabled || t.TLS.CAFile != "" || t.TLS.CertFile != "" || t.TLS.KeyFile != "" || t.TLS.ServerName != "" || t.TLS.AllowInsecureRemote || t.TLS.AllowInsecureProduction {
		problems = append(problems, "SQLite does not use TLS settings")
	}
	if t.SQLite == nil || !filepath.IsAbs(t.SQLite.Root) || !filepath.IsAbs(t.SQLite.File) {
		problems = append(problems, "sqlite.root and sqlite.file must be absolute paths")
	} else if _, err := t.SQLitePath(); err != nil {
		problems = append(problems, err.Error())
	}
	if t.Connection.MaxOpen < limits.PerTargetConcurrency || t.Connection.MaxOpen > 16 || t.Connection.MaxIdle < 0 || t.Connection.MaxIdle > t.Connection.MaxOpen {
		problems = append(problems, "SQLite connection pool must cover per-target concurrency and stay within 16")
	}
	if t.Connection.ReadTimeout < limits.MaxTimeout {
		problems = append(problems, "SQLite read timeout must cover limits.max_timeout")
	}
	return problems
}
