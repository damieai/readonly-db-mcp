# readonly-db-mcp

A local-first Model Context Protocol server that lets AI clients run advanced,
read-only SQL against explicitly configured database targets.

The project is designed to be copied into its own GitHub repository. It is a
standalone Go module and does not import code or configuration from its parent
repository.

> Current status: MySQL 8, PostgreSQL 15-17, and SQL Server 2019-2025 are
> implemented. Redis 7.2/7.4
> and Redis 8.x standalone, Sentinel and Cluster read-only command support is
> implemented behind strict ACL, topology and live command-catalog attestation.
> Third-party modules require exact, Ed25519-signed module profiles and a locally
> verifiable module artifact. Module index-prefix introspection fails closed
> until a dedicated verifier is available. Production rollout still
> requires validation against your provisioned accounts and server builds.

## What it provides

- Multiple named database targets, selected explicitly on every tool call.
- Independent MySQL, PostgreSQL, and SQL Server dialect policies and privilege
  attestation.
- Redis command-vector tools with live read-only command, key-scope, ACL,
  Sentinel quorum, Cluster slot-map and signed module attestation.
- Full read-only analytical SQL: CTEs, joins, subqueries, unions, aggregates,
  window functions, JSON expressions and ordinary `USE INDEX`/`FORCE INDEX`
  clauses supported by MySQL/Vitess. Optimizer comment hints are rejected.
- Database-account privilege attestation during startup and periodic fail-closed
  rechecks for MySQL, PostgreSQL, and SQL Server.
- Dialect validation that rejects mutations and unsafe external capabilities.
  SQL Server additionally requires a server-compiled `SHOWPLAN_XML` proof before
  execution, while preserving advanced read-only T-SQL.
- Database-native read-only transactions where available. SQL Server uses a
  least-privilege account, mandatory compile-only preflight, and rollback-only
  SELECT transactions because it has no transaction-level read-only mode.
- Per-target and global concurrency limits, timeouts, row limits, SQL parameter
  byte limits, result-byte limits and cell-size limits.
- Bounded fair scheduling across metadata, interactive, batch and maintenance
  workloads, with overload rejected before a transaction is opened.
- Target-local bounded metadata caches and optional, explicit short-lived result
  caches for deterministic queries.
- Exact final-response byte enforcement and early batch stopping.
- Phase-level performance summaries on stderr when configured.
- Structured audit events that contain query fingerprints, never raw SQL,
  parameters, result values or credentials.
- Structured MCP outputs over stdio.

## Security boundary

The database account is the final write-protection boundary. Every target must
use a dedicated account whose effective grants contain only `USAGE` and
`SELECT` on the configured schemas. The process refuses to start when it sees
an extra privilege, a role it cannot verify, a global `SELECT`, or access to an
unconfigured schema.

Application validation is defense in depth. Do not connect this server using
an application account, administrator account, migration account, or an
account with `FILE`, `EXECUTE`, `LOCK TABLES`, temporary-table, DML, DDL or
grant-management privileges.

See [SECURITY.md](SECURITY.md) for the complete threat model.

## Quick start

### 1. Provision a dedicated account

Have a database administrator create the account out of band. Prefer a schema
containing curated, masked views for production data.

```sql
CREATE USER 'inventory_mcp_ro'@'127.0.0.1'
  IDENTIFIED BY RANDOM PASSWORD
  REQUIRE SSL
  WITH MAX_USER_CONNECTIONS 3
       MAX_QUERIES_PER_HOUR 1000
       MAX_CONNECTIONS_PER_HOUR 100;

GRANT SELECT ON `inventory`.*
  TO 'inventory_mcp_ro'@'127.0.0.1';
```

Do not add `MAX_UPDATES_PER_HOUR 0`: in MySQL, zero means unlimited rather than
"no updates". The absence of update privileges is what prevents writes.

### 2. Create local configuration

```bash
cp configs/example.yaml config.local.yaml
mkdir -m 700 secrets
printf '%s' 'replace-with-the-generated-password' > secrets/inventory-test.password
chmod 600 secrets/inventory-test.password
```

Edit `config.local.yaml`. The disposable local target encrypts without server
identity verification; the production example requires full certificate verification.

Never put a password in YAML, command arguments, an MCP client JSON file, or a
Git repository. On Windows, prefer `password_env` because POSIX file mode checks
are not available in the same form.

Remote databases are expected to use TLS. A non-production target whose server
cannot provide TLS can opt in to cleartext transport with both
`tls.mode: disabled` and `tls.allow_insecure_remote: true`. This explicit escape
hatch logs a startup warning and is still refused for production environments;
credentials, SQL and results are unencrypted, so use it only on a trusted private
network or VPN.

For PostgreSQL, use a dedicated non-owning `LOGIN` role with no memberships,
`SUPERUSER`, `CREATEDB`, `CREATEROLE`, `REPLICATION`, `BYPASSRLS`, database
`TEMPORARY`, schema `CREATE`, sequence mutation, or executable user-function
privileges. Grant only database `CONNECT`, allowed-schema `USAGE`, and relation
`SELECT`. PostgreSQL targets require schema-qualified user relations and use
`$1`, `$2`, ... parameters. Revoke inherited `PUBLIC` privileges that violate
this scope, including `CONNECT` on other databases and `EXECUTE` on
non-system functions; startup attestation intentionally fails closed otherwise.

For SQL Server, use a dedicated login and database user, grant `SELECT` only on
curated schemas, and grant database `SHOWPLAN`. Snapshot isolation is required
for consistent batches. A minimal shape is:

```sql
ALTER DATABASE [Finance] SET ALLOW_SNAPSHOT_ISOLATION ON;
GO
USE [Finance];
GO
CREATE USER [finance_mcp_ro] FOR LOGIN [finance_mcp_ro]
  WITH DEFAULT_SCHEMA = [reporting];
GRANT CONNECT TO [finance_mcp_ro];
GRANT SHOWPLAN TO [finance_mcp_ro];
GRANT SELECT ON SCHEMA::[reporting] TO [finance_mcp_ro];
```

Do not grant DML, DDL, ownership, impersonation, executable-module, bulk,
external-access, or broad control permissions. Startup walks effective server,
database, schema, and object permissions and fails closed on drift. SQL Server
parameters use `@p1`, `@p2`, and so on. See
[RFC-0005](docs/RFC-0005-sql-server-dialect-support.md) for the full model.

For Redis, create a password-protected ACL user from `reset`, grant `%R~pattern`
only for the configured key prefixes, start command permissions from `-@all`,
and grant `+@read`. Startup also needs the internal-only introspection commands
`+acl|whoami`, `+acl|getuser`, `+command`, `+command|info`,
`+command|list`, `+command|getkeysandflags`, `+ping`, `+info`, `+module|list`, `+role`, and—only when the
configured database is nonzero—`+select`. Do not grant write-key patterns,
channels, selectors, write/admin categories, or unprofiled module commands. A
profiled module also needs `COMMAND LIST FILTERBY MODULE` visibility and ACL
grants for only the approved read commands. These
introspection commands are never exposed through `redis_command`.

Sentinel mode uses separate discovery credentials, requires agreement from at
least two configured Sentinels, verifies the discovered node's `ROLE`, and
admits only endpoints whose resolved IPs remain inside configured CIDRs (and,
for hostnames, match configured DNS suffixes). Cluster mode
validates exact coverage of all 16,384 slots and attests every route-eligible
node before publishing a client generation. Unknown redirect endpoints are
refused. See [RFC-0004](docs/RFC-0004-redis-sentinel-cluster-modules.md) for the
configuration and signed module-profile format.

### 3. Build and verify

The module currently requires Go 1.26.6 or newer because of the maintained
MySQL parser dependency.

```bash
make test
make build
./bin/readonly-db-mcp -config ./config.local.yaml -check
```

`-check` opens every configured target, verifies its identity and grants, then
exits without starting MCP.

### 4. Configure an MCP client

Use absolute paths. Credentials stay in the password file or environment and
are not included here.

```json
{
  "mcpServers": {
    "readonly-db": {
      "command": "/absolute/path/readonly-db-mcp/bin/readonly-db-mcp",
      "args": [
        "-config",
        "/absolute/path/readonly-db-mcp/config.local.yaml"
      ]
    }
  }
}
```

Ask the client, for example:

```text
Use target inventory-test. Inspect the schema and verify whether transaction
90088 generated the expected stock movements.
```

## Tools

| Tool | Purpose |
| --- | --- |
| `list_targets` | Lists aliases and non-secret target safety metadata. |
| `inspect_target` | Shows one target's engine, environment and consistency. |
| `schema_list_tables` | Lists visible tables and views. |
| `schema_describe_table` | Shows columns and indexes. |
| `query_select` | Runs one validated SELECT. |
| `query_batch` | Runs several SELECTs in one read-only transaction snapshot. |
| `query_explain` | Returns an engine-native non-executing plan for a validated SELECT. |
| `redis_command` | Runs one attested advanced read-only Redis command vector. |
| `redis_batch` | Runs a bounded read-only Redis batch; non-atomic commands execute sequentially to bound retained reply memory. |

Every database tool requires an exact target alias. There is deliberately no
stateful `USE DATABASE` tool and no tool accepts a host, DSN, username or
password.

Example query input:

```json
{
  "target": "inventory-test",
  "sql": "SELECT id, status FROM stock_transactions WHERE company_id = ? AND id = ?",
  "parameters": [10001, 90088],
  "max_rows": 100,
  "purpose": "verify transaction processing"
}
```

Encode integers larger than JavaScript's safe integer range as strings.
MySQL parameters use `?`; PostgreSQL uses `$1`, `$2`; SQL Server uses `@p1`,
`@p2`, and so on. The selected target reports its `parameter_style`.

### Performance controls

Database-backed tools share one bounded admission controller. Metadata has a
small reserved capacity by default, so schema inspection remains responsive
while analytical queries are saturated. Queue length and wait time have hard
limits; overload returns a bounded error instead of accumulating goroutines.

Metadata caching is enabled by default with target-local TTL, entry and byte
limits. Result caching is disabled by default. Enable it only for targets where
the configured staleness is acceptable; volatile queries are always bypassed,
and current-consistency targets require an additional explicit opt-in.

Batch results report `truncated`, `truncation_reason` and `completed_queries`.
When the remaining response budget cannot represent another result, the server
does not execute that query.

Redis Sentinel discovery and Cluster topology are probed on their short refresh
intervals. An unchanged, fully allowed routing snapshot refreshes only its
freshness timestamp; node catalogs, ACLs, module artifacts and connection pools
are rebuilt on their slower attestation interval or immediately after topology
drift. This avoids periodic connection and artifact-hashing storms. Redis reply byte limits apply
to normalized MCP output; operators must also configure Redis/server and
container memory ceilings because the Redis client necessarily decodes one
server reply before application-level truncation.

Set `server.metrics_summary_interval` to a positive duration to emit bounded,
content-free performance summaries on stderr. Metrics never use SQL, parameters,
returned values, credentials, schemas, tables or fingerprints as labels.

Both `schema_list_tables` and `schema_describe_table` accept an optional
`fresh: true` argument. A fresh request bypasses the current entry, reads the
database, and atomically replaces the cached value. Concurrent refreshes for the
same key are coalesced, and a configurable cooldown prevents refresh storms.
Normal calls omit `fresh` and continue to use the cache.

```json
{
  "target": "inventory-test",
  "schema": "inventory",
  "table": "items",
  "fresh": true
}
```

Redis commands use an argument vector rather than a redis-cli string:

```json
{
  "target": "analytics-redis-test",
  "command": "ZINTER",
  "arguments": [
    {"string": "2"},
    {"string": "analytics:score:daily"},
    {"string": "analytics:score:verified"},
    {"string": "WITHSCORES"}
  ],
  "max_elements": 1000,
  "purpose": "compare verified daily scores"
}
```

Use `{"base64":"..."}` instead of `string` for binary arguments. Each argument
must contain exactly one representation.

## Adding targets

Targets are added only through operator-owned configuration. MCP tools cannot
create or change them. Each target has its own credentials, pool, schema scope,
fail-closed function policy and startup verification.

Use separate aliases when the same logical database has different freshness:

- `inventory-test-primary` with `consistency: current` for immediate tests.
- `inventory-production-replica` with `consistency: eventual` for lower-impact
  production investigation.

The selected target, environment and consistency are repeated in every query
result to reduce wrong-database mistakes.

## Publishing as a standalone GitHub repository

Copy this directory without its build output:

```bash
cp -R tools/readonly-db-mcp /path/to/readonly-db-mcp
cd /path/to/readonly-db-mcp
git init -b main
git add .
git commit -m "feat: initial readonly database MCP server"
git remote add origin git@github.com:OWNER/readonly-db-mcp.git
git push -u origin main
```

The placeholder Go module path does not prevent cloning and building the
command. Before publishing `go install` releases, replace
`github.com/your-org/readonly-db-mcp` in `go.mod` and Go imports with the final
repository path, then run `go mod tidy`.

## Development

```bash
make fmt
make test-race
make vet
make build
```

Unit tests cover the SQL security policy, privilege validation, secret-file
permissions, read-only transaction execution, batch output limits and result
truncation. Real integration tests should use disposable MySQL 8, PostgreSQL,
SQL Server 2019-2025, Redis standalone, Sentinel and Cluster deployments with
separately provisioned read-only accounts. Multi-node failover and third-party module certification
remain release-gate tests because a unit-test double cannot prove real routing,
replication or server-module behavior.

The Redis integration suite accepts operator-owned configurations through
`READONLY_DB_MCP_REDIS_SENTINEL_CONFIG`/`_TARGET`/`_KEY` and
`READONLY_DB_MCP_REDIS_CLUSTER_CONFIG`/`_TARGET`/`_KEY`. Tests are skipped when
these variables are absent and should be run against disposable deployments
during release qualification.

The SQL Server integration test accepts `READONLY_DB_MCP_SQLSERVER_DSN` and an
optional `READONLY_DB_MCP_SQLSERVER_SCHEMA` (default `reporting`). It performs
live permission attestation, an advanced SELECT `SHOWPLAN_XML`, and a
transactionally rolled-back DDL denial probe against a disposable database.

## License

MIT
