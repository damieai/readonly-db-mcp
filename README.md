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
> verifiable module artifact. Search index prefixes are checked at startup and
> before each query. Production rollout still
> requires validation against your provisioned accounts and server builds.
> Elasticsearch implements pinned target/permission attestation, native metadata,
> and scoped `es_query` / `es_batch` / `es_cursor` tools with advanced DSL, aggregations, scripts,
> supplied-vector retrieval, templates, owned PIT/scroll/SQL pagination and synchronous EQL/SQL/ES|QL.
> ENRICH supports explicit cluster snapshot scope; finer isolation, inference and other advanced profiles remain tracked by
> [RFC-0006](docs/RFC-0006-elasticsearch-readonly-support.md); ES is not yet
> advertised as a complete or live-server-certified query adapter.

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
hatch logs a startup warning. Production targets additionally require
`tls.allow_insecure_production: true` to explicitly authorize a temporary
cleartext exception while retaining their production environment label.
Credentials, SQL and results are unencrypted, so use it only on a trusted private
network or VPN. Remove both exception flags when restoring `verify-full`.

For PostgreSQL, use a dedicated non-owning `LOGIN` role with no memberships,
`SUPERUSER`, `CREATEDB`, `CREATEROLE`, `REPLICATION`, `BYPASSRLS`, database
`TEMPORARY`, schema `CREATE`, sequence mutation, or executable user-function
privileges. Grant only database `CONNECT`, allowed-schema `USAGE`, and relation
`SELECT`. PostgreSQL targets require schema-qualified user relations and use
`$1`, `$2`, ... parameters. Revoke inherited `PUBLIC` privileges that violate
this scope, including `CONNECT` on other databases and `EXECUTE` on
non-system functions; startup attestation intentionally fails closed otherwise.

Redis Search dense retrieval has local native/MCP acceptance for HASH/JSON,
FLAT/HNSW, FLOAT32/FLOAT64 and L2/IP/COSINE over RESP2/RESP3. See
[vector setup and queries](docs/REDIS-VECTOR-RETRIEVAL.md) for signed-profile
requirements and the exact qualification boundary. PostgreSQL now supports an
opt-in PostgreSQL 16 / pgvector 0.8.2 profile for dense retrieval through the
existing SQL tools, including vector/halfvec results and transaction-local
HNSW/IVFFlat tuning. See the [setup and query guide](docs/POSTGRESQL-VECTOR-RETRIEVAL.md)
and [local MCP evidence](docs/qualification/2026-09-21-postgresql-vector-retrieval.md),
including [synthetic recall and recovery acceptance](docs/qualification/2026-09-21-postgresql-vector-resources.md).
The [TLS and long-read record](docs/qualification/2026-09-22-postgresql-vector-transport.md)
adds verified reconnects and concurrent query/metadata/privilege-recheck evidence
on PostgreSQL 16.15.
Installing the extension alone does not enable its permission exceptions.

For SQL Server, use a dedicated login and database user, grant `SELECT` only on
curated schemas, and grant database `SHOWPLAN`, and make `VIEW DEFINITION` available through the
runtime identity or the separate read-only catalog attestor below. Snapshot isolation is required
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
GRANT VIEW DEFINITION TO [finance_mcp_ro];
GRANT SELECT ON OBJECT::sys.sql_expression_dependencies TO [finance_mcp_ro];
GRANT SELECT ON SCHEMA::[reporting] TO [finance_mcp_ro];
```

Do not grant DML, DDL, ownership, impersonation, arbitrary procedure execution, bulk,
external-access, or broad control permissions. Startup walks effective server,
database, schema, and object permissions and fails closed on drift. SQL Server
parameters use `@p1`, `@p2`, and so on. See
[RFC-0005](docs/RFC-0005-sql-server-dialect-support.md) for the full model.

SQL Server now requires the pinned ScriptDom helper. Build it with
`make build-sqlserver-parser` using a .NET 8 SDK, then ship the complete
`bin/sqlserver-parser` directory beside the MCP binary. Its self-contained
release needs no installed .NET runtime. See the [helper build instructions](tools/sqlserver-parser/README.md).
Startup and each query verify reachable view/UDF definitions, dependency chains,
computed-column functions and RLS predicates. An optional `sqlserver.attestor`
uses a separate read-only identity for fixed catalog reads (including an exact
`SELECT` grant on `sys.sql_expression_dependencies`); one connection is
reserved from the configured target pool. Missing parser or catalog proof fails
closed. See the [qualification record](docs/qualification/2026-09-13-redis-sqlserver.md)
for the distinction between implemented checks and live-server qualification.

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

For Elasticsearch, configure explicit HTTPS origins, the cluster UUID and one
of the pinned build versions. The query identity may be a dedicated realm user
or an API key with a pinned ID. Give a realm query user cluster `monitor` and
only `read` / `view_index_metadata` on the configured index patterns. Startup
inspects authority on **every** endpoint; write, delegation, remote, unknown
and out-of-scope grants fail closed.
API-key mode requires a separate realm attestor whose only effective cluster
privilege is `read_security`; its credentials are used solely for key descriptor
proof. See [API-key qualification](docs/qualification/2026-09-23-elasticsearch-api-key.md).
Do not grant the query key security-management privileges.

Stored script/template inspection additionally accepts the narrowly scoped
`cluster:admin/script/get` action grant. Do not replace it with `manage` or
`cluster:admin/script/*`. Inline scripts/templates do not require this grant.
Query preflight checks all nodes against the pinned build and requires an empty
external-plugin inventory; custom plugin profiles remain pending.

`es_metadata` supports `resolve`, `mappings`, `field_mappings`, `aliases`,
`settings` and allowlisted `get_script`. The latter requires one exact script ID
in `names` and an operator `readable_script_ids` entry; see the
[stored-script qualification](docs/qualification/2026-09-23-elasticsearch-script-metadata.md).
`field_mappings` requires `fields` and reads selected native field
definitions, including wildcards. Alias and settings reads accept `names` to
select native alias or setting names. Mapping, alias and settings reads accept
pinned native `options`, such as `flat_settings` and `include_defaults`; returned
index and alias names are checked against configured scope. `es_query` supports `search`,
`count`, `get`, `exists`, `get_source`, `exists_source`, `mget`, `termvectors`, `mtermvectors`, `explain`, `field_caps`,
`search_shards`, `indices.validate_query`, `search_template` and
`render_search_template`, `eql.search`, `sql.query`, `sql.translate` and `esql.query`. IDs use the structured `id` field. Native JSON and
large integer values remain intact. Wildcards use ES `*` / `?` semantics, with containment
checked for future index names; aliases and data streams are resolved for each
call. Native date math index sources are supported in metadata, search and the
read-only languages: the server resolves the expression on the pinned endpoint,
checks every resulting index and logical source against the configured scope,
and rechecks before returning results. For example, `indices: ["<reports-{now/d{yyyy.MM.dd|+08:00}}>"]`
keeps the expression intact for Elasticsearch to evaluate. Custom plugins,
cross-cluster and finer ENRICH isolation profiles still need their respective
proof implementations. Missing capabilities are reported
by `inspect_target`, separately from mutation denial. See the
[native query qualification record](docs/qualification/2026-09-22-elasticsearch-native-queries.md)
for setup, tests and remaining gates.
See [date math qualification](docs/qualification/2026-09-23-elasticsearch-date-math.md)
for syntax, scope and live-test details.
See [alias/settings qualification](docs/qualification/2026-09-23-elasticsearch-metadata.md)
for native options and response checks.
See [field mapping qualification](docs/qualification/2026-09-23-elasticsearch-field-mappings.md)
for selected field paths and validation.
See [document read qualification](docs/qualification/2026-09-23-elasticsearch-document-reads.md)
for native source and existence operations.
ES|QL `pragma` is available when an operator configures `esql_pragmas` ceilings;
the adapter checks each recognized native setting and supplies the release-build
risk flag only after validation. See [pragma qualification](docs/qualification/2026-09-23-elasticsearch-esql-pragmas.md).

```json
{"target":"search-reporting","operation":"search","body":{"size":0,"runtime_mappings":{"taxed":{"type":"double","script":{"source":"emit(doc['amount'].value * params.factor)","params":{"factor":1.1}}}},"aggs":{"total":{"sum":{"field":"taxed"}}}}}
```

Search templates accept native `source` or stored `id`, `params`, `explain` and
`profile`. Envelope `explain/profile` flags default to false and override the
rendered search's same-named fields, matching native template execution. The
native `options.explain` override remains available. `render_search_template`
returns the validated rendered body without applying these execution flags.

Multi-search templates use `es_batch` with `search_template` members; each member
can use its own template, parameters, index scope, routing and diagnostic flags.
All templates render and pass source/effect checks before any search executes.
Frozen results go through `_msearch` or individual searches when native options
require them, preserving cancellation and avoiding a second template expansion.
See [template qualification](docs/qualification/2026-09-23-elasticsearch-templates.md).

Use `es_batch` with `requests: [{"operation":"search","body":{...}}, ...]` and
`consistency: "independent"`. All members are proved before execution and share
one deadline and one encoded result budget. Compatible searches use generated
`_msearch` framing; other combinations execute sequentially without dropping
native options. Set `consistency: "pit"` for compatible `search` / `search_template`
members with the same index scope and routing options: one service-owned PIT
is used by every member and closed before the response. Independent batches
do not share a snapshot. Oversized pages, buckets or
documents fail explicitly; aggregations are never silently truncated. Configure
`max_rows`, `max_result_bytes`, and ES `max_aggregation_buckets` for those limits.

`es_cursor` supports PIT, scroll and native SQL cursors with `action: "open" | "next" | "close"`.
Open returns the first page and an opaque handle bound to this MCP session and
configured target. Native PIT/scroll/SQL IDs cannot be supplied and are removed from
responses. For example:

```json
{"target":"search-reporting","action":"open","kind":"pit","indices":["reports-*"],"body":{"size":100,"sort":["_shard_doc"],"query":{"match_all":{}}},"keep_alive_ms":300000}
{"target":"search-reporting","action":"next","handle":"<handle returned by open>"}
{"target":"search-reporting","action":"close","handle":"<handle returned by open>"}
```

PIT `next` without a body uses the retained query and last hit's `search_after`,
preserving integer sort values. An explicit native body allows composite
`after_key`, changed aggregations, scripts and vector queries on the same snapshot.
PIT pages retain the original index snapshot across alias rollover; embedded
lookup sources are still validated live. Scroll fixes its query and page size at
open, including native `slice` options. Empty hit pages close ordinary hit cursors;
aggregation-only PITs remain open for explicit continuation or close.

Defaults are 32 contexts per target, 128 per process, 5 minutes idle, 30 minutes
absolute lifetime, and 64 KiB per native ID/sort continuation. Configuration can
raise target contexts to 128, idle to 15 minutes, lifetime to 2 hours, and ID bytes
to 1 MiB. Context state has a separate 64 MiB process budget and holds no query
permit while idle. Expiry, session disconnect, observed privilege drift and
shutdown trigger cleanup. Uncertain native outcomes disable the handle and keep
its reservation until cleanup or a conservative native lease horizon; queries
never silently restart a snapshot. See the
[cursor qualification record](docs/qualification/2026-09-22-elasticsearch-owned-contexts.md)
for fixture evidence and remaining live-server gates.

Use `es_query` with `operation: "eql.search"` for native synchronous EQL:

```json
{"target":"search-reporting","operation":"eql.search","indices":["reports-*"],"body":{"query":"sequence by host.id with maxspan=5m [process where process.name : \"cmd*\"] [network where destination.port in (80,443)]","size":10,"fetch_size":1000}}
```

The complete pinned upstream grammar supports event, sequence, sample and join
syntax, expressions, functions, comments and pipes. The native ES server decides
which semantic operations its version supports. The original query is preserved;
`filter`, `runtime_mappings`, field formats, time/category fields, ordering,
`fetch_size` and `max_samples_per_key` retain their native meaning. EQL event
categories and literals are data; index scope comes from `indices` and any
embedded filter/runtime lookups. Responses preserve events, sequence groups,
join keys, missing-event placeholders and exact JSON numbers.

`max_rows` bounds the total returned events, including missing placeholders,
and sequence count. An oversized sequence result fails as a whole. Native
`size` still means the number of events or sequence/sample groups; it is never
silently reduced. EQL can participate in independent batches, but not shared-PIT
batches or PIT/scroll cursors. HTTP cancellation shares the MCP deadline.
`wait_for_completion_timeout` must be omitted or `"-1"`; non-negative values
select stored asynchronous work and need a separate lifecycle profile.
`keep_on_completion: true` and partial results are not enabled.

Parser defaults: 256 KiB language text, 10,000 tokens, depth 128 and EQL fetch
size ceiling 10,000. Operator ceilings are 1 MiB, 100,000 tokens, depth 512 and
fetch size 100,000. A separate 128 MiB process parser pool is included in the
configuration forecast. Java is unnecessary at runtime; generated Go files are
checked in. The upstream EQL grammar and derived generated files retain Elastic
License 2.0, separately from the repository's MIT license. See
[parser packaging and regeneration](tools/es-language-parser/README.md) and the
[EQL qualification record](docs/qualification/2026-09-22-elasticsearch-eql.md).

Use `es_query` with `sql.query` for complete bounded SQL results, or
`sql.translate` for the native SQL-to-DSL plan. The two pinned SQL grammars are
compiled separately into Go; SQL text and native scalar/typed parameters remain
unchanged. Expression, aggregation, PIVOT, subquery, CTE, JOIN, EXPLAIN/DEBUG and
SHOW/SYS syntax is parsed completely; the native ES version decides semantic
availability. Every relation and metadata index pattern passes scope proof,
including nested branches and CTE definitions/references. Quoted aliases remain
in the original query. Local catalogs are checked against the cluster identity;
remote catalogs need the cross-cluster profile.

```json
{"target":"search-reporting","operation":"sql.query","body":{"query":"SELECT category, SUM(amount) FROM \"reports-*\" WHERE amount > ? GROUP BY category ORDER BY SUM(amount) DESC LIMIT 10","params":[100],"fetch_size":5},"max_rows":100}
{"target":"search-reporting","operation":"sql.translate","body":{"query":"SELECT * FROM \"reports-*\" WHERE amount > ?","params":[100]}}
{"target":"search-reporting","action":"open","kind":"sql","body":{"query":"SELECT * FROM \"reports-*\" ORDER BY amount DESC","fetch_size":100,"columnar":true},"keep_alive_ms":60000}
```

`sql.query` drains native pages within one deadline and total `max_rows` / result
byte budget. Exceeding a budget fails the whole query and cleans owned state;
use `es_cursor` for larger results. SQL cursor `next` uses only the returned local
handle (plus optional timeout/lease), preserving the initial parameters, page
size, mode and row/column orientation. Absence of a native cursor closes the
handle even when the final page contains rows. `close`, disconnect, expiry,
authority drift and shutdown use `/_sql/close` only for owned tokens.

JSON rows/columnar values and large integers are preserved; transport format is
JSON (`binary_format: false`). Explicit `fetch_size` must fit `max_rows`; when
omitted it defaults to `min(1000, max_rows)`. `page_timeout` or `keep_alive_ms`
selects the bounded cursor lease; specifying both is an error. Native
`request_timeout` is bounded by the remaining MCP deadline. Partial results are
disabled. Async waits must be omitted or `-1`, and retained async results remain
outside this profile. SQL can join independent batches; shared-PIT batches
remain native search operations. `SHOW TABLES` without a pattern means native
`*`, so use a permitted selector such as `SHOW TABLES LIKE 'reports-%'`.

SQL and EQL share the 128 MiB parser pool and configured language limits.
SQL grammars/generated files retain their upstream Apache 2.0 notices, separate
from the EQL grammar's Elastic License 2.0. No Java runtime is required.
See [SQL qualification and live-test setup](docs/qualification/2026-09-22-elasticsearch-sql.md).

Native synchronous ES|QL uses `es_query.operation=esql.query` or an independent
`es_batch` member. Complete version-specific upstream grammars cover pipelines,
expressions, aggregations, parameters, LOOKUP JOIN and the pinned release's
commands, including 9.1 FORK branches and 8.19 EXPLAIN. Native semantic/function
availability stays with ES; queries are never translated into a smaller dialect.
Every source, including JOINs and nested branches, must fit both the target and
any explicit request index scope. Original aliases, query text and native
anonymous/positional/named parameters are preserved.

```json
{"target":"search-reporting","operation":"esql.query","body":{"query":"FROM reports-* | LOOKUP JOIN reports-customers ON customer_id | WHERE amount > ?minimum | STATS total = SUM(amount) BY category | SORT total DESC | LIMIT 10","params":[{"minimum":100}],"columnar":false},"max_rows":100}
```

Results preserve native columns, rows/columnar values, profiling and exact JSON
numbers. `format=json` and `allow_partial_results=false` are fixed. `max_rows`
bounds the whole returned result: oversized responses fail without truncation;
use native `LIMIT` or aggregation to select the desired result. The adapter does
not append a limit or change the engine's native implicit limit. HTTP cancellation
uses the synchronous cancellable REST channel; ES|QL has no cursor in this profile.
`drop_null_columns` is supported, except when columnar output drops every column
and makes row cardinality unprovable; retain null columns or use row output then.

ENRICH supports existing native `match`, `range` and `geo_match` policies,
ON/WITH aliases, chains and nested branches. It is disabled by default. An
operator may authorize **all historical, current and future enrichment snapshots
in the pinned cluster**, independently of `allowed_indices` / `denied_indices`,
by configuring:

```yaml
elasticsearch:
  # Additional to the endpoint, version, cluster_uuid and ordinary index scope.
  enrich:
    scope: all_cluster_snapshots
```

This is a broad data grant, not a claim that source-index DLS/FLS or a policy
allowlist isolates snapshots. The runtime identity additionally needs explicit
`monitor_enrich`; `manage_enrich` and policy creation/execution/deletion remain
forbidden. Use a separately provisioned cluster when finer isolation is needed.
Native snapshot mappings, write blocks and active aliases are checked at startup
and on recheck; query preflight also checks the selected policies. Observed
metadata/policy drift fails the whole result, without an automatic retry. These
reads do not atomically lock distributed metadata. Normal FROM/JOIN source
checks remain in force. All native mode qualifiers work with proved local
sources; actual cross-cluster reads require a separate profile. See
[ENRICH qualification and provisioning](docs/qualification/2026-09-22-elasticsearch-enrich.md).

COMPLETION, semantic-text
matching and other inference require endpoint/egress/cost proof. Experimental
pragmas and persisted async results also need separate resource/lifecycle profiles.
These are reported as unavailable capabilities, not classified as writes merely
because a query is advanced. EQL, SQL and ES|QL share the 128 MiB parser pool;
ES|QL reserves an 8 MiB baseline per parse plus byte/token charges. Its vendored
grammar/generated code retains Elastic License 2.0; runtime remains Go-only.
See [ES|QL qualification and remaining gates](docs/qualification/2026-09-22-elasticsearch-esql.md).

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
| `es_metadata` | Resolves scoped Elasticsearch indices, aliases and data streams, or reads full/selected mappings, aliases, settings and allowlisted stored scripts. |
| `es_query` | Executes scoped native Elasticsearch reads, advanced DSL, scripts, aggregations, supplied vectors, templates and synchronous EQL/SQL/ES|QL, including SQL translation. |
| `es_batch` | Executes preflighted independent reads or compatible searches in one owned PIT, with a shared deadline and output budget. |
| `es_cursor` | Opens, advances and closes session-owned PIT/scroll/SQL handles with bounded lifetimes and cleanup. |
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
For the full production path, provision the disposable
[SQL fixture](tools/sqlserver-parser/acceptance-fixture.sql), then set
`READONLY_DB_MCP_SQLSERVER_CONFIG` and `READONLY_DB_MCP_SQLSERVER_TARGET` and run
`go test ./internal/dialects/sqlserver -run TestSQLServerConfiguredTargetIntegration -count=1 -v`.
This gate opens the real target, verifies curated views and functions, executes
advanced reads and snapshot batches, checks native UPDATE/DELETE denial, and
checks recovery after an already-cancelled request. Mid-execution cancellation
and the full engine/version matrix remain separate acceptance requirements.

Local disposable Redis tests use `READONLY_DB_MCP_REDIS_SERVER` and optionally
`READONLY_DB_MCP_REDIS_SEARCH_MODULE`. Run `make test-redis-local` to exercise
standalone, Sentinel failover, all-slot Cluster routing, and a signed Search
profile with index-prefix drift. These tests create and clean up their own
processes and fixture credentials; missing binaries cause explicit skips.

`make test-redis-vectors` additionally requires
`READONLY_DB_MCP_REDIS_JSON_MODULE` and runs the complete native dense-retrieval
matrix through MCP, including score comparison and native write denial.

## License

MIT
