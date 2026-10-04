package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type DuckDBConfig struct {
	Root       string `yaml:"root"`
	File       string `yaml:"file"`
	WorkerPath string `yaml:"worker_path"`
	Version    string `yaml:"version"`
	MemoryMB   int    `yaml:"memory_mb"`
	Threads    int    `yaml:"threads"`
}

func defaultDuckDB(t *TargetConfig) {
	if t.DuckDB == nil {
		t.DuckDB = &DuckDBConfig{}
	}
	if t.DuckDB.MemoryMB == 0 {
		t.DuckDB.MemoryMB = 256
	}
	if t.DuckDB.Threads == 0 {
		t.DuckDB.Threads = 2
	}
}

// DuckDBPath pins one existing database file beneath one operator-owned data
// root. The entire root is exposed read-only to the sandbox for Parquet reads.
func (t *TargetConfig) DuckDBPath() (root, file, worker string, err error) {
	if t.DuckDB == nil {
		return "", "", "", errors.New("duckdb settings are required")
	}
	d := t.DuckDB
	for _, path := range []string{d.Root, d.File, d.WorkerPath} {
		if path == "" || !filepath.IsAbs(path) {
			return "", "", "", errors.New("duckdb paths must be absolute")
		}
	}
	root, err = filepath.EvalSymlinks(d.Root)
	if err != nil {
		return
	}
	file, err = filepath.EvalSymlinks(d.File)
	if err != nil {
		return
	}
	worker, err = filepath.EvalSymlinks(d.WorkerPath)
	if err != nil {
		return
	}
	rinfo, e := os.Stat(root)
	if e != nil || !rinfo.IsDir() {
		err = errors.New("duckdb.root must be an existing directory")
		return
	}
	if root == string(filepath.Separator) {
		err = errors.New("duckdb.root must be a dedicated data directory")
		return
	}
	finfo, e := os.Stat(file)
	if e != nil || !finfo.Mode().IsRegular() {
		err = errors.New("duckdb.file must be an existing regular file")
		return
	}
	winfo, e := os.Stat(worker)
	if e != nil || !winfo.Mode().IsRegular() || winfo.Mode().Perm()&0111 == 0 {
		err = errors.New("duckdb.worker_path must be an executable regular file")
		return
	}
	rel, e := filepath.Rel(root, file)
	if e != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		err = errors.New("duckdb.file must resolve inside duckdb.root")
		return
	}
	return
}

func validateDuckDB(t *TargetConfig, limits Limits) []string {
	var p []string
	add := func(s string) { p = append(p, s) }
	if !safeName.MatchString(t.Environment) {
		add("environment is required and must be a safe identifier")
	}
	if t.Consistency != ConsistencyCurrent {
		add("DuckDB requires current consistency")
	}
	if t.Host != "" || t.Port != 0 || t.Database != "" || t.Username != "" || t.PasswordFile != "" || t.PasswordEnv != "" {
		add("DuckDB rejects network and credential fields")
	}
	if len(t.AllowedSchemas) != 0 || len(t.DeniedTables) != 0 || t.Elasticsearch != nil || t.Qdrant != nil || t.MongoDB != nil || t.SQLite != nil || t.MySQL != (MySQLConfig{}) || t.PostgreSQL != (PostgreSQLConfig{}) || t.SQLServer != (SQLServerConfig{}) || !redisConfigEmpty(t.Redis) {
		add("DuckDB rejects other engine settings")
	}
	if t.TLS.Mode != TLSDisabled || t.TLS.CAFile != "" || t.TLS.CertFile != "" || t.TLS.KeyFile != "" || t.TLS.ServerName != "" || t.TLS.AllowInsecureRemote || t.TLS.AllowInsecureProduction {
		add("DuckDB does not use TLS settings")
	}
	if t.DuckDB == nil {
		add("duckdb settings are required")
		return p
	}
	if _, _, _, err := t.DuckDBPath(); err != nil {
		add(err.Error())
	}
	if t.DuckDB.Version != "1.5.6" {
		add("duckdb.version must pin 1.5.6")
	}
	if t.DuckDB.MemoryMB < 128 || t.DuckDB.MemoryMB > 2048 {
		add("duckdb.memory_mb must be between 128 and 2048")
	}
	if t.DuckDB.Threads < 1 || t.DuckDB.Threads > 8 {
		add("duckdb.threads must be between 1 and 8")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		add("DuckDB requires bubblewrap (bwrap) in PATH")
	}
	if t.Connection.MaxOpen < limits.PerTargetConcurrency || t.Connection.MaxOpen > 16 {
		add("DuckDB connection.max_open must cover per-target concurrency and stay within 16")
	}
	if t.Connection.MaxIdle < 0 || t.Connection.MaxIdle > t.Connection.MaxOpen {
		add("DuckDB connection.max_idle must be between 0 and max_open")
	}
	if int64(t.DuckDB.MemoryMB)*int64(limits.PerTargetConcurrency) > 8192 {
		add(fmt.Sprintf("DuckDB worker memory budget exceeds 8192 MiB for target %q", t.Name))
	}
	return p
}
