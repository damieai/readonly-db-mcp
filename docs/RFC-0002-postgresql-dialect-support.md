# RFC-0002: PostgreSQL Dialect Support

- Status: Proposed
- Authors: readonly-db-mcp maintainers
- Created: 2026-09-01
- Target release: staged
- Scope: PostgreSQL targets over TCP and the existing stdio MCP transport

## Summary

This RFC adds PostgreSQL as a first-class database engine without weakening the
project's security objective: no public MCP tool may persistently modify a
configured database, even when the caller supplies hostile SQL.

PostgreSQL receives its own parser policy, privilege attestation, connection
implementation, metadata queries, explain path, function policy and integration
test corpus under `internal/dialects/postgresql`. MySQL behavior remains
unchanged. The common scheduler, caches, response budgets, audit recorder and MCP
tools are reused through engine-neutral interfaces.

The database role remains the final write-protection boundary. PostgreSQL
`READ ONLY` transactions are defense in depth, not the final boundary: they are
a high-level restriction and do not make every possible operation physically
read-only. PostgreSQL roles, inherited memberships, object ownership, function
execution and sequence privileges must therefore be attested explicitly.

## Motivation

The current engine boundary was designed to permit a second dialect, but only
MySQL is implemented. PostgreSQL support is requested for the same workflows:

- inspect schemas, tables, views, columns and indexes;
- run advanced analytical `SELECT` queries;
- run several queries in a consistent read-only snapshot;
- obtain non-executing JSON query plans;
- preserve target isolation, output limits, caching and audit behavior.

Reusing MySQL parsing or privilege assumptions would be unsafe. PostgreSQL has
different role inheritance, schema resolution, object ownership, function and
operator resolution, sequence behavior, row security and system catalogs.

## Goals

- Support PostgreSQL as `engine: postgresql` through the existing MCP tools.
- Preserve the current exact-target and SELECT-only security objective.
- Fail closed when effective privileges, role memberships, functions, operators
  or object resolution cannot be proven safe.
- Support CTEs, recursive CTEs, joins, subqueries, set operations, aggregates,
  window functions, JSON/array expressions and ordinary read-only PostgreSQL
  expressions.
- Support `$1`, `$2`, ... parameters with strict placeholder validation.
- Reuse unified admission control, metadata caches including `fresh: true`, exact
  response budgets, result caching, metrics and structured audit events.
- Keep MySQL and PostgreSQL semantics isolated behind dialect packages.
- Provide real PostgreSQL integration, hostile-query and privilege-mutation
  tests before production support is declared.

## Non-goals

- Reusing the MySQL/Vitess parser for PostgreSQL.
- Supporting arbitrary SQL, procedures, `DO`, `CALL`, `COPY`, DDL or DML.
- Allowing MCP callers to set `search_path`, role, session variables or GUCs.
- Supporting `EXPLAIN ANALYZE`, which executes the statement.
- Acting as a PostgreSQL administration or migration interface.
- Automatically creating roles, grants, schemas, views or RLS policies.
- Guaranteeing confidentiality for data the configured role can legitimately
  select. Curated views and column privileges remain the confidentiality boundary.
- Supporting logical/physical replication protocol connections.
- Sharing pools or cache entries between MySQL and PostgreSQL targets.

## Supported versions

The first release supports PostgreSQL 15, 16 and 17. PostgreSQL 18 is admitted
only after the selected parser version, driver tests and hostile corpus are
validated against PostgreSQL 18. Unsupported major versions fail startup rather
than running in an unverified compatibility mode.

Parser and server majors need not be identical for every accepted query, but the
server version must be in the tested matrix. A parser may reject a valid newer
query; it must never accept an unsafe construct because it misunderstood newer
grammar.

## Security model

### Policy principle: semantic mutation blocking, not SQL feature reduction

The PostgreSQL policy is deny-by-side-effect, not allow-by-query-complexity.
Complexity, nesting depth within configured resource ceilings, uncommon syntax
and planner sophistication are not rejection reasons. A recursive CTE with
LATERAL joins, grouping sets, window frames, JSONPath, arrays, ordered-set
aggregates and nested set operations is acceptable when its resolved operations
are non-mutating and its relations remain inside target scope.

The policy rejects a construct only when it can mutate persistent state, acquire
write-intent/advisory locks, change session/server state, escape configured target
scope, invoke an unprovable executable capability, or violate explicit resource
ceilings. Target/schema/denied-table boundaries remain authorization constraints,
not SQL-language limitations.

Whenever the parser encounters a new read-only PostgreSQL grammar node, the
preferred response is to classify and support it. A broad positive list of
"common query shapes" is explicitly forbidden because it would unnecessarily
degrade PostgreSQL's analytical expressiveness.

### Final boundary

Every target uses a dedicated `LOGIN` role that:

- is not superuser;
- does not own the database, allowed schemas, queried relations, functions,
  operators or types;
- has no `CREATEDB`, `CREATEROLE`, `REPLICATION` or `BYPASSRLS` attribute;
- has a finite connection limit appropriate for the MCP target;
- cannot `SET ROLE` to any other role;
- receives no inherited privilege-bearing membership;
- has `CONNECT` only on the configured database;
- has no `TEMPORARY` privilege on the database;
- has `USAGE` only on configured schemas;
- has only `SELECT` or column-level `SELECT` on allowed relations;
- has no `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE`, `REFERENCES`, `TRIGGER`,
  `CREATE`, `MAINTAIN`, sequence `USAGE`/`UPDATE`, large-object write, foreign
  server, language, tablespace, parameter `SET`, or grant option;
- is not a member of `pg_read_server_files`, `pg_write_server_files`,
  `pg_execute_server_program`, `pg_signal_backend`, monitoring roles with
  unacceptable disclosure, or future predefined roles not explicitly audited.

The implementation refuses ownership because owners retain powers that ordinary
ACL inspection cannot safely model as a SELECT-only grant.

### Defense in depth

1. Exact configured target lookup; callers cannot supply DSNs or credentials.
2. Startup identity, role, ownership and effective-privilege attestation.
3. A PostgreSQL-native parser accepting exactly one top-level `SelectStmt`.
4. AST rejection of modifying CTEs, row locks, `SELECT INTO`, utility commands,
   session state and resolved operations that can create persistent side effects.
5. Explicit deterministic session setup and transaction-local safety GUCs.
6. `BEGIN READ ONLY` for every select/explain and `REPEATABLE READ READ ONLY` for
   batches.
7. Unified admission, server-side statement timeout, row/cell/byte/column limits
   and exact final-response accounting.
8. Query-content-free audit and low-cardinality metrics.

## Dependency decisions

### Driver

Use `github.com/jackc/pgx/v5/stdlib` through `database/sql`.

Reasons:

- integrates with the existing engine-neutral pool and transaction contracts;
- supports context cancellation, TLS, PostgreSQL parameter encoding and rich
  PostgreSQL error codes;
- avoids exposing pgx-specific types through `core`;
- permits a later direct-pgx optimization without changing MCP contracts.

The dependency is pinned to an audited release. Driver upgrades run the full
integration, cancellation and hostile corpus matrix.

### Parser

Use `github.com/pganalyze/pg_query_go/v6`, pinned to an exact release, for the
initial implementation. It embeds the PostgreSQL parser and returns PostgreSQL
parse-tree protobufs. This is chosen for grammar fidelity and parsing speed.

Consequences:

- builds require CGO and a supported C compiler;
- release artifacts require an explicit OS/architecture build matrix;
- parser compilation increases cold build time;
- fuzzing must include parser crashes and resource limits;
- `CGO_ENABLED=0` is not a supported PostgreSQL-enabled build initially.

An optional future build may use `github.com/wasilibs/go-pgquery`, which exposes
compatible parse-tree types through a pure-Go WebAssembly runtime. Its upstream
benchmarks show a material parse-time cost, so it is not the performance-default.
Adopting it requires separate benchmarks and supply-chain review.

The deparser is not used on untrusted SQL. The server executes the original
validated query with bound parameters; it never executes a rewritten string.

## Configuration

Example target:

```yaml
targets:
  analytics-postgresql-production:
    engine: postgresql
    environment: production
    consistency: eventual
    host: analytics-replica.internal.example
    port: 5432
    database: analytics
    username: analytics_mcp_ro
    password_file: /run/secrets/analytics-postgresql.password
    allowed_schemas:
      - reporting
      - dimensions
    denied_tables:
      - reporting.raw_customer_export
    connection:
      connect_timeout: 3s
      read_timeout: 12s
      write_timeout: 3s
      max_open: 2
      max_idle: 1
      max_lifetime: 3m
      max_idle_time: 1m
    postgresql:
      application_name: readonly-db-mcp
      statement_timeout_margin: 250ms
      batch_isolation: repeatable-read
      require_hot_standby: true
      privilege_recheck_interval: 5m
    tls:
      mode: verify-full
      ca_file: /etc/readonly-db-mcp/postgresql-ca.pem
      server_name: analytics-replica.internal.example
    metadata_cache:
      enabled: true
      allow_fresh: true
      fresh_cooldown: 1s
      table_list_ttl: 20m
      table_description_ttl: 20m
      negative_ttl: 5s
      max_entries: 256
      max_bytes: 8388608
```

### Configuration changes

- Add `EnginePostgreSQL = "postgresql"`.
- Default port is engine-specific: MySQL 3306, PostgreSQL 5432.
- Add an optional `postgresql` target block. It is rejected for non-PostgreSQL
  targets; PostgreSQL-specific fields outside it are forbidden.
- Existing TLS modes retain their operator-facing meaning. The PostgreSQL driver
  maps them without constructing a caller-controlled connection string.
- Production PostgreSQL targets require `verify-full` exactly as MySQL does.
- `require_hot_standby` verifies `pg_is_in_recovery()` at startup. It is strongly
  recommended for production investigation targets but is not a substitute for
  a SELECT-only role.
- `application_name` is operator configuration, length bounded and sanitized.
- Arbitrary runtime parameters, DSN fragments and `options` are forbidden.

### Provisioning example

Provisioning is performed out of band by an administrator. Exact commands depend
on ownership and existing `PUBLIC` privileges; the documentation generator must
not imply that the following abbreviated example is sufficient for every cluster.

```sql
CREATE ROLE analytics_mcp_ro
  LOGIN
  NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS
  CONNECTION LIMIT 3
  PASSWORD 'generated-out-of-band';

REVOKE ALL ON DATABASE analytics FROM analytics_mcp_ro;
GRANT CONNECT ON DATABASE analytics TO analytics_mcp_ro;
REVOKE TEMPORARY ON DATABASE analytics FROM PUBLIC;

GRANT USAGE ON SCHEMA reporting, dimensions TO analytics_mcp_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA reporting, dimensions
  TO analytics_mcp_ro;
```

Operators must also address future objects using reviewed default privileges or
an explicit deployment grant step. Broad `PUBLIC EXECUTE` on user-defined
functions is not accepted merely because PostgreSQL commonly grants it by
default.

## Startup verification

Startup verification runs through fixed, parameterized catalog queries. It does
not parse textual `\dp` or `GRANT` output.

### Identity and server checks

Verify:

- `current_user = session_user = configured username`;
- `current_database() = configured database`;
- `server_version_num` is in the tested matrix;
- `transaction_read_only` can be enabled and observed as `on`;
- TLS is active when required and certificate verification is configured by the
  client, not inferred from a server value;
- `pg_is_in_recovery()` satisfies `require_hot_standby`;
- `search_path`, `role`, `session_authorization`, `row_security`,
  `statement_timeout`, `lock_timeout`, `idle_in_transaction_session_timeout` and
  relevant role/database defaults contain no unsafe operator-controlled surprise.

### Role attributes and memberships

Inspect `pg_roles`, `pg_auth_members` and predefined-role membership.

Fail when:

- any forbidden role attribute is set;
- the login role owns an object in the configured database;
- any membership is inherited or settable, even when the referenced role appears
  harmless today;
- a grant has admin option;
- an unknown predefined role is reachable;
- `PUBLIC` or an inherited role supplies an effective forbidden privilege.

Rejecting all memberships is intentionally stricter than attempting to flatten
role graphs. A future RFC may admit a precisely attested SELECT-only group role.

### Database and schema privileges

Use `has_database_privilege`, `has_schema_privilege` and ACL catalog expansion to
prove the effective role has:

- `CONNECT` on exactly the configured database;
- no `CREATE` or `TEMPORARY` there;
- `USAGE` but not `CREATE` on exactly the allowed schemas;
- no usable access to unconfigured user schemas.

System catalog visibility is handled separately. Merely being able to read
ordinary catalog metadata does not authorize free-form queries against catalog
schemas.

### Relation, sequence and large-object privileges

For every visible relation in non-system schemas, compute effective privileges
including direct, `PUBLIC` and ownership-derived rights.

Allowed:

- table/view/materialized-view/foreign-table `SELECT` within allowed schemas;
- column-level `SELECT` within allowed schemas.

Rejected:

- any modifying, ownership or grant-option capability;
- SELECT on a relation outside allowed schemas;
- sequence `USAGE` or `UPDATE`;
- large-object ownership or write capability;
- ownership of any relation, sequence, schema, function, procedure, operator,
  type, database, foreign server, wrapper or tablespace in the database.

Foreign tables are disabled by default because a SELECT may reach an external
system with separate availability and confidentiality consequences. A later
option may admit explicitly enumerated foreign tables after server/user-mapping
attestation.

### Function and operator privileges

PostgreSQL expressions invoke functions both explicitly and through operators,
casts, aggregates and type I/O. A name-only allowlist is insufficient because
overloading and `search_path` affect resolution.

The initial policy therefore has two layers:

1. AST rejects explicitly dangerous or unresolved constructs before execution.
2. At startup, build an immutable allowlist keyed by function OID and operator
   OID from audited `pg_catalog` objects for the connected server major.

The policy is capability-oriented rather than syntax-oriented. It must not reject
an advanced SQL expression merely because it is complex, overloaded or uncommon.
Built-in functions, aggregates, window functions, operators and casts are allowed
when their resolved identity is known and the audited capability catalog proves
they cannot persistently mutate data under the attested role and transaction.

User-defined executable objects are rejected by default, including
`SECURITY DEFINER` functions, because PostgreSQL has no trustworthy catalog bit
proving arbitrary code is free of side effects. Optional support requires exact
configured signatures/OIDs plus verification of owner, language, volatility,
security-definer flag, configuration, dependency closure, effective EXECUTE
privileges and an operator assertion that the body is read-only. This restricts
an unprovable executable capability, not advanced SQL syntax.

At minimum reject:

- every procedure and `CALL`;
- `nextval`, `setval`, advisory locks, backend signaling/termination, large-object
  mutation, file access, dblink, extension management, GUC mutation and snapshot
  export/import functions;
- non-`pg_catalog` executable objects unless explicitly and completely attested;
- functions with `prosecdef = true`;
- untrusted or procedural-language functions;
- unknown function/operator/cast/aggregate OIDs.

Volatility is not itself a rejection rule. Nondeterministic but non-mutating
built-ins remain usable when their exact OID is classified as non-mutating.
Volatility is still used for result-cache eligibility.

Because raw parsing does not perform semantic name resolution, AST-only OID
allowlisting is not sufficient. The implementation uses a two-stage design:

- static AST rejects unsafe statement shapes and schema qualifications;
- a fixed server-side validation query prepares/describes the statement in a
  read-only, rolled-back transaction and inspects the resolved dependency plan
  where reliably available;
- if complete resolved provenance cannot be obtained without executing the query,
  syntactic built-ins with unambiguous audited behavior may be admitted, but
  support cannot be declared complete until ordinary resolved built-in calls are
  supported without weakening the mutation guarantee.

This resolution proof is an implementation gate. PostgreSQL support cannot ship
by copying MySQL's function-name allowlist.

## Session initialization

Each newly opened physical connection is initialized with driver-controlled,
constant settings. Pool reuse must not permit state leakage.

Required settings include:

```sql
SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;
SET search_path = pg_catalog;
SET row_security = on;
SET client_min_messages = warning;
SET idle_in_transaction_session_timeout = '<bounded operator value>';
SET lock_timeout = '<bounded operator value>';
```

`statement_timeout` is set transaction-locally from the already validated MCP
deadline using `set_config('statement_timeout', $1, true)` with a bound value, or
an equivalent driver-safe mechanism. Caller SQL cannot contain `SET`.

The query policy requires every user relation to be schema-qualified in the
first release. This eliminates dependence on mutable `search_path`. CTE names
and aliases remain unqualified by definition. A later compatibility option may
permit unqualified relations by resolving them against exactly one configured
default schema.

Connections that fail initialization are discarded. Connection-return hooks or
`DISCARD ALL` are evaluated in benchmarks; correctness is mandatory, but an
extra round trip after every request is not accepted without evidence. Since
public SQL cannot mutate session state, constant initialization plus driver
reset semantics should normally suffice.

## SQL policy

### Accepted top-level shape

Exactly one parsed statement whose root is `SelectStmt`.

Accepted SELECT features include:

- ordinary and recursive CTEs;
- joins, subqueries and lateral subqueries;
- `UNION`, `INTERSECT` and `EXCEPT`;
- grouping, aggregates and window expressions;
- `VALUES` only when represented inside an otherwise accepted read-only SELECT;
- JSON, JSONB, array and range expressions whose resolved operations are audited;
- `DISTINCT ON`, `FILTER`, ordered-set aggregates, grouping sets, rollup and cube;
- `LATERAL`, safe set-returning built-ins, `TABLESAMPLE`, collations and casts;
- full-text search, regular expressions, JSONPath, XML and geometric operations
  whose resolved built-in capabilities are non-mutating;
- ordering, offset and limit;
- parameter references `$1` through `$N`.

### Rejected statement and AST shapes

- multiple statements, including trailing injected statements;
- modifying CTEs containing INSERT/UPDATE/DELETE/MERGE;
- `SELECT INTO`;
- `FOR UPDATE`, `FOR NO KEY UPDATE`, `FOR SHARE`, `FOR KEY SHARE` and locking
  options such as `NOWAIT`/`SKIP LOCKED`;
- `COPY`, `CALL`, `DO`, `SET`, `RESET`, `SHOW`, `DISCARD`, `LISTEN`, `NOTIFY`,
  `UNLISTEN`, transaction commands and every utility statement;
- DDL, DML, maintenance, privilege, security-label and extension commands;
- resolved executable objects whose mutation capability cannot be proven safe;
- system catalog relations in free-form SQL;
- unconfigured schemas and denied relations;
- temporary schemas and temporary relations;
- foreign tables unless explicitly enabled in a future policy;
- parameter numbers with gaps, `$0`, or numbers exceeding configured limits;
- parser nodes unknown to the audited walker.

Comments are accepted only as inert parser-recognized comments. PostgreSQL has no
MySQL-style executable comments, but query-size limits apply before parsing.

### System schemas

Free-form SQL rejects relations in:

- `pg_catalog`;
- `information_schema`;
- `pg_toast` and all `pg_toast_temp_*` schemas;
- `pg_temp_*` schemas;
- other schemas beginning with `pg_` unless a future RFC explicitly admits one.

Metadata tools use fixed catalog queries and return only objects within allowed
schemas after denied-table filtering.

### Parameter semantics

PostgreSQL requests use `$1`, `$2`, ... placeholders. Question-mark placeholders
remain MySQL-only.

Validation proves:

- the highest parameter number equals the parameter count;
- every position is referenced or gaps are rejected;
- parameters are JSON scalar values accepted by the current core contract;
- large integers encoded as strings remain strings unless PostgreSQL can infer or
  the query explicitly casts them;
- no string interpolation is performed by the server.

Tool descriptions become engine-neutral and tell callers to use the selected
target's placeholder syntax. `inspect_target` exposes `parameter_style`.

## Transaction execution

### Single query

1. Validate request bounds and SQL policy.
2. Check optional result-cache eligibility.
3. Acquire an interactive admission permit.
4. Begin a driver read-only transaction.
5. Set transaction-local statement/lock timeout from validated limits.
6. Verify `transaction_read_only = on` in integration tests; avoid a production
   round trip per query unless operational evidence requires it.
7. Execute the original SQL with bound parameters.
8. Collect through the shared row/cell/column/response budget.
9. Roll back, release permit, audit and record metrics.

### Batch

Use `REPEATABLE READ READ ONLY` by default so all batch items observe one snapshot.
`SERIALIZABLE READ ONLY DEFERRABLE` may be offered later for long reports, but its
initial snapshot wait complicates MCP deadlines and is not the default.

Batch retains current early response-budget stopping and
`completed_queries` semantics.

### Explain

Use:

```sql
EXPLAIN (FORMAT JSON, ANALYZE FALSE, VERBOSE FALSE, COSTS TRUE) <validated SELECT>
```

`ANALYZE`, arbitrary explain options and utility statements are never caller
controlled. Explain executes inside a read-only transaction with the same
timeouts and admission class as a query.

## Metadata implementation

### List tables

Use a fixed query against `pg_catalog.pg_class` and `pg_namespace`, limited to
allowed schemas and relation kinds:

- ordinary table;
- partitioned table;
- view;
- materialized view;
- optionally foreign table only when policy permits it.

Exclude temporary, TOAST and system schemas. Apply denied-table policy after the
fixed query as defense in depth. Preserve identifier case exactly.

### Describe table

Return existing engine-neutral fields from fixed catalog queries:

- columns from `pg_attribute`, `pg_type`, `pg_attrdef` and safe formatting
  helpers;
- nullability, generated/identity state and default expression;
- indexes from `pg_index`, `pg_class`, `pg_attribute` and bounded
  `pg_get_indexdef` output;
- included columns, expression indexes and predicates represented explicitly
  rather than pretending every index is a list of simple columns.

This requires additive engine-neutral model changes:

```go
type IndexDescription struct {
    Name       string   `json:"name"`
    Unique     bool     `json:"unique"`
    Primary    bool     `json:"primary,omitempty"`
    Method     string   `json:"method,omitempty"`
    Columns    []string `json:"columns,omitempty"`
    Includes   []string `json:"includes,omitempty"`
    Expressions []string `json:"expressions,omitempty"`
    Predicate  string   `json:"predicate,omitempty"`
}
```

MySQL populates only fields it supports. Existing clients remain compatible.

Metadata cache keys include engine, target policy revision and exact identifier
case. Existing TTL, negative-cache, singleflight and `fresh: true` behavior is
shared. A forced refresh always queries PostgreSQL and atomically replaces the
entry; it remains subject to metadata admission and refresh cooldown.

## Result representation

Reuse `core.QueryResult` and exact JSON response budgeting.

PostgreSQL normalization rules:

- signed integers outside JavaScript's safe range become decimal strings;
- `numeric` values are returned as strings unless lossless JSON-number handling
  is proven end to end;
- timestamps are RFC3339Nano; `timestamp without time zone` is not falsely tagged
  with UTC and must carry an explicit representation policy;
- `bytea` becomes `base64:<payload>`;
- UUID, inet/cidr, interval, enum, range and unknown extension values use bounded
  textual encodings;
- JSON/JSONB is decoded only when doing so preserves the byte budget and numeric
  precision; otherwise return bounded JSON text;
- arrays use bounded recursive normalization with depth and element ceilings;
- duplicate output column names use the existing deterministic suffix scheme.

The implementation must not permit a driver-returned type to bypass cell or
response limits.

## Result-cache eligibility

PostgreSQL result caching remains disabled by default.

An entry is eligible only when:

- target configuration enables caching and accepts the consistency tradeoff;
- the resolved statement uses no `VOLATILE` function;
- it uses no sequence, current/session identity, transaction/snapshot, advisory
  lock, catalog, temporary or foreign object;
- all relations and resolved operations are covered by the policy revision;
- the result succeeds and is not truncated.

`STABLE` and safe `VOLATILE` functions remain valid for execution but usually make
cross-transaction result caching surprising. The first release therefore
requires `IMMUTABLE` resolved functions for cached expressions, with explicitly
audited exceptions for simple operators and aggregates. Cache eligibility must
never be confused with execution eligibility. Cache keys include engine, server
major, target policy revision, SQL fingerprint, typed parameters, row limit and
result schema version.

## Errors, audit and metrics

Map PostgreSQL SQLSTATE into stable sanitized classes. Never expose server error
detail, hint, internal query, schema/table names derived from error messages, SQL
text or parameters.

Audit events retain:

- query/batch ID;
- target and operation;
- content-free fingerprint;
- statically identified allowed relations;
- decision and stable reason class;
- cache status, rows, bytes, truncation and duration.

Metrics reuse controlled operation/engine/outcome labels. Never use database,
schema, table, role, query ID, SQLSTATE detail or fingerprint as metric labels.

New phase timings may distinguish PostgreSQL parse, semantic resolution,
transaction setup and server execution.

## Package and interface changes

```text
internal/dialects/postgresql/
    config.go
    connect.go
    grants.go
    policy.go
    functions.go
    metadata.go
    execute.go
    batch.go
    normalize.go
```

Engine-neutral changes:

- registry constructs the engine selected by target configuration;
- `TargetInfo` gains `parameter_style` and `server_version` or a non-sensitive
  major-version field;
- index descriptions gain additive PostgreSQL fields;
- shared response collection is extracted only where semantics are truly common;
- admission, metrics, audit and cache primitives remain engine-neutral;
- engine packages never import each other.

Do not force MySQL and PostgreSQL into one large conditional executor. Duplication
is preferable where transaction, metadata or normalization semantics differ.

## Performance design

- Parser work occurs before admission so invalid requests consume no DB slot, but
  parser concurrency receives a bounded CPU semaphore to prevent parse storms.
- Cache policy validations by `(engine, policy revision, SQL fingerprint)` only
  after proving AST objects immutable; parameters do not affect AST validity.
- PostgreSQL prepared statement caching is disabled initially to avoid per-session
  schema/search-path and invalidation complexity. Benchmark before enabling it.
- Metadata uses existing target-local caches and miss coalescing.
- Catalog attestation runs at startup and periodically in the maintenance class;
  it is not repeated per query.
- Per-connection initialization avoids caller-controlled round trips.
- `statement_timeout`, context cancellation and driver read deadlines overlap but
  are retained as independent protections.
- Benchmark parser, parameter encoding, row normalization, catalog metadata and
  mixed-engine fairness independently.

Initial performance gates:

| Path | Gate |
| --- | --- |
| PostgreSQL policy validation, 32 KiB SQL | p95 below 10 ms |
| Simple SELECT policy validation | p95 below 500 microseconds |
| Warm metadata hit | unchanged from RFC-0001 within 10% |
| Scheduler overhead | unchanged from RFC-0001 within 10% |
| Scalar row normalization | at least 75 MiB/s |
| Mixed MySQL/PostgreSQL saturation | no target exceeds configured capacity |
| Cancellation | server work terminates and pool capacity recovers within 1 s |

## Testing strategy

### Unit tests

- PostgreSQL configuration defaults and cross-engine field rejection;
- AST acceptance for advanced read-only SELECTs;
- rejection of every utility/DML/DDL/locking/modifying-CTE shape while retaining
  broad advanced read-only SELECT coverage;
- placeholder gaps, counts and limits;
- schema, CTE, identifier-case and denied-table resolution;
- function/operator/cast/aggregate capability policy including overloads and
  safe volatile built-ins;
- role graph, PUBLIC grants, ownership and predefined-role attestation;
- SQLSTATE sanitization;
- all PostgreSQL value normalization and budget boundaries;
- metadata mapping, cache and forced refresh behavior;
- result-cache deterministic/volatile eligibility.

### Hostile corpus

At minimum include:

- data-modifying CTEs hidden inside SELECT;
- `SELECT INTO`, all row-lock clauses and nested set operations;
- `COPY ... TO/FROM`, `COPY PROGRAM`, large-object and file functions;
- `nextval`, `setval`, advisory locks and backend termination;
- `SECURITY DEFINER`, overloaded and search-path-shadowed functions/operators;
- malicious casts, domains, aggregates and type I/O functions;
- `dblink`, foreign tables and extension functions;
- temp schema/object attempts;
- catalog and information-schema exfiltration;
- comments, dollar quoting, Unicode identifiers and multi-statements;
- deeply nested AST and parser resource-exhaustion inputs;
- privilege changes and role membership changes after startup.

### Disposable PostgreSQL integration matrix

Run PostgreSQL 15, 16 and 17 with a dedicated SELECT-only role. PostgreSQL 18 is
added before it becomes supported.

Verify:

- startup accepts the documented safe role and refuses each forbidden privilege,
  ownership attribute and membership individually;
- every hostile public tool call leaves persistent tables, sequences, large
  objects, roles, settings and filesystem-visible effects unchanged;
- `BEGIN READ ONLY` and batch snapshot semantics are real;
- RLS is honored and BYPASSRLS/ownership is rejected;
- timeouts cancel server work and return pool capacity;
- TLS verify-full succeeds and invalid CA/name fails;
- metadata, case-sensitive identifiers, partitions, materialized views and
  expression/partial indexes are correct;
- `fresh: true` observes DDL immediately and replaces the old entry;
- grants or role membership changed after startup cause periodic attestation to
  mark the target unhealthy and reject new queries;
- mixed MySQL/PostgreSQL targets respect global fairness and isolation.

Integration tests use environment-provided secrets and never log DSNs.

### Fuzz, race and soak tests

- fuzz parser/walker with a bounded input size and no panics;
- fuzz incremental/final result budgeting for PostgreSQL values;
- race-test cache refresh, policy cache, health transitions and shutdown;
- soak mixed metadata/query/batch workloads for at least 30 minutes;
- track heap, goroutines, CGO memory, pool stats, parser CPU and cancellation.

## Implementation phases

### Phase 0: Dependency and semantic spike

- Pin and benchmark pgx and `pg_query_go`.
- Prove parser behavior on the hostile corpus.
- Prototype function/operator resolution and document its completeness.
- Validate CGO builds on all release platforms.
- Record baseline under `docs/benchmarks/`.

Exit gate: no implementation proceeds if resolved callable provenance cannot be
made fail-closed.

### Phase 1: Configuration, connection and identity

- Add engine/config models and strict validation.
- Implement TLS, pool setup, connection initialization and server version gate.
- Add identity, role-attribute, membership and recovery-state verification.

Exit gate: safe disposable targets start; every forbidden role attribute fails.

### Phase 2: Effective privilege attestation

- Implement database/schema/relation/column/sequence/ownership/PUBLIC checks.
- Reject foreign tables and unverified functions/operators.
- Add catalog fixtures and real-server mutation tests.

Exit gate: no tested privilege or ownership escalation passes startup.

### Phase 3: Parser and static policy

- Implement exact-one-SelectStmt and exhaustive fail-closed AST walking.
- Enforce qualified relations, schema scope, denied tables and placeholders.
- Add hostile corpus, fuzzing and parser CPU bounds.

Exit gate: public validation accepts the supported SELECT corpus and rejects the
hostile corpus without database access.

### Phase 4: Execution, batch and explain

- Implement read-only transactions, timeouts and SQLSTATE sanitization.
- Implement repeatable-read batches and non-analyzing JSON explain.
- Reuse admission and exact response budgets.

Exit gate: persistent database state is byte/logically unchanged by hostile calls.

### Phase 5: Metadata and normalization

- Implement table/column/index catalogs and PostgreSQL value normalization.
- Integrate bounded caches and forced refresh.
- Extend index response fields additively.

Exit gate: metadata fixtures and real DDL refresh tests pass across the matrix.

### Phase 6: Observability, health and performance

- Add PostgreSQL phase metrics, pool stats and sanitized audit coverage.
- Add periodic maintenance-class privilege re-attestation.
- Complete mixed-engine load, race and soak testing.

Exit gate: overload is bounded, unhealthy targets fail closed, and performance
gates pass.

### Phase 7: Optional result cache

- Enable only after callable volatility and dependency proofs are complete.
- Keep disabled by default and require explicit staleness configuration.

Exit gate: no volatile, session-dependent, foreign, temporary or policy-changing
query becomes cache-eligible.

## Compatibility and migration

- Existing MySQL configurations and tool calls remain valid.
- `engine` selects placeholder syntax and dialect; no auto-detection.
- New response fields are additive.
- Tool descriptions become dialect-neutral; target inspection reports syntax.
- PostgreSQL result caching remains off unless explicitly enabled.
- A build that cannot include the required parser fails at build time; it does
  not silently ship PostgreSQL configuration support without validation.

## Rollout and rollback

1. Land engine-neutral additive model changes with no behavior change.
2. Ship PostgreSQL behind an explicit experimental build/version marker.
3. Enable test targets, then non-sensitive replicas.
4. Complete privilege mutation, mixed-load and soak evidence.
5. Mark PostgreSQL production-supported only after all exit gates pass.

Rollback removes PostgreSQL targets from operator configuration. MySQL target
behavior and persisted state are unaffected. Cache data is process-local and is
dropped when targets close.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Function/operator side effects through SELECT | Resolved OID capability policy; allow advanced safe built-ins and reject mutating/unprovable callables |
| Inherited or SET ROLE privilege escalation | Reject all memberships initially |
| Ownership bypasses ACL/RLS expectations | Reject ownership and BYPASSRLS |
| Sequence mutation is not rolled back | Reject sequence USAGE/UPDATE and sequence-mutating functions |
| READ ONLY is mistaken for a complete boundary | Dedicated role remains final boundary; hostile state tests |
| `COPY PROGRAM` or server file access | Reject COPY AST and predefined file/program roles |
| `search_path` shadowing | Constant path plus mandatory qualified user relations |
| Parser/server grammar mismatch | Tested major matrix and startup version gate |
| CGO build/release complexity | Pinned parser, explicit build matrix, optional later WASI variant |
| Foreign data source side effects/load | Foreign tables rejected initially |
| RLS silently omitted or bypassed | Reject owner/BYPASSRLS; test RLS behavior |
| Privileges change after startup | Periodic maintenance-class re-attestation; fail closed |
| Metadata cache hides DDL | Existing `fresh: true`, TTL and atomic replacement |

## Alternatives considered

### Send SQL directly and rely only on a read-only role

Rejected. The role is the final boundary, but AST policy, transaction mode,
timeouts and resource controls materially reduce blast radius and parser/driver
surprises.

### Reuse the MySQL/Vitess parser

Rejected. Grammar, semantics, functions, operators, casts, placeholders and
locking constructs differ.

### Validate with regexes or first keywords

Rejected. Modifying CTEs, comments, quoting, nested statements and PostgreSQL
grammar make lexical allowlisting unsafe.

### Decide execution safety only from volatility labels

Rejected. Volatility is not a complete security proof. Safe volatile built-ins
should remain available, while malicious or misdeclared user code must not become
executable merely because it claims `IMMUTABLE`.

### Accept SELECT-only ownership

Rejected. Owners have implicit powers and may bypass RLS; ownership is not a
normal revocable SELECT grant.

### Use a pure-Go/WASI parser by default

Deferred. It simplifies builds but currently costs substantial parse throughput.
The project prioritizes performance and will benchmark it as an optional build.

### Require PostgreSQL to be a hot standby

Not universally required. It is an excellent production defense but excludes
legitimate read-only primary/test use. The per-target option can enforce it.

## Open decisions

1. Exact technique for complete resolved function/operator/cast provenance
   without executing caller expressions.
2. Whether PostgreSQL 18 support waits for a new `pg_query_go` major or uses the
   tested PostgreSQL 17 grammar subset temporarily.
3. Whether foreign tables can ever meet the project's availability/security
   expectations.
4. Representation of `timestamp without time zone` across MCP clients.
5. Whether parser CPU gets a separate global semaphore or a weighted admission
   class shared with database work.
6. Frequency and cost budget for periodic effective-privilege re-attestation.

The first item is a release blocker, not an implementation detail.

## Acceptance criteria

PostgreSQL support is complete only when:

- configuration, TLS, identity and server version fail closed;
- effective role attributes, memberships, ownership, PUBLIC grants and object
  privileges are proven SELECT-only;
- exactly one supported SELECT is accepted and the hostile corpus is rejected;
- resolved functions/operators/casts/aggregates are proven safe or rejected;
- single, batch and explain operations use native read-only transactions;
- metadata, normalization, exact budgets, caches and `fresh` work correctly;
- privilege changes after startup make the target reject new work;
- MySQL behavior and performance remain within existing regression gates;
- unit, fuzz, integration, race, mixed-load and soak suites pass on every
  supported PostgreSQL major and release platform;
- documentation clearly states provisioning, CGO and known limitations.

## References

- PostgreSQL role membership and inheritance:
  <https://www.postgresql.org/docs/current/role-membership.html>
- PostgreSQL privileges and ownership:
  <https://www.postgresql.org/docs/current/ddl-priv.html>
- PostgreSQL read-only transaction semantics:
  <https://www.postgresql.org/docs/current/sql-set-transaction.html>
- PostgreSQL row security:
  <https://www.postgresql.org/docs/current/ddl-rowsecurity.html>
- PostgreSQL COPY security:
  <https://www.postgresql.org/docs/current/sql-copy.html>
- `pg_query_go` parser:
  <https://github.com/pganalyze/pg_query_go>
- Pure-Go/WASI parser alternative:
  <https://github.com/wasilibs/go-pgquery>
