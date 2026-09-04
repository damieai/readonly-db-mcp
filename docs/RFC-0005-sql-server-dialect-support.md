# RFC-0005: Microsoft SQL Server Dialect Support

- Status: Partially implemented
- Authors: readonly-db-mcp maintainers
- Created: 2026-09-04
- Target release: staged
- Scope: Microsoft SQL Server targets over TDS and the existing stdio MCP transport

Implementation note (2026-09-04): the first runnable slice now includes
`go-mssqldb`, strict configuration, effective permission attestation, native
`@pN` binding, catalog metadata, snapshot batches, a T-SQL safety scanner, and
mandatory server-compiled `SHOWPLAN_XML` validation before execution. The
planned ScriptDom helper remains a release-hardening phase: the build
environment has no managed runtime, so the current boundary deliberately pairs
the Go scanner with SQL Server's own non-executing compiler proof instead of
shipping an unverified grammar substitute. Exact-version integration matrices,
transitive module-definition attestation, and the optional separate attestor
identity remain open release gates.

## Summary

This RFC adds Microsoft SQL Server as a first-class database engine without
weakening the project's primary security objective: no public MCP tool may
persistently modify a configured database, even when the caller supplies
hostile T-SQL.

SQL Server receives an independent driver, T-SQL parser policy, effective
permission attestation, object resolver, metadata implementation, result
normalizer, estimated-plan path and hostile-query corpus under
`internal/dialects/sqlserver`. Existing MySQL, PostgreSQL and Redis behavior is
unchanged. Engine-neutral admission control, response budgets, caches, audit
events and MCP tools are reused.

The SQL policy is deny-by-persistent-effect, not allow-by-query-complexity. It
does not reject a query because it uses advanced T-SQL. Recursive CTEs, APPLY,
PIVOT, grouping sets, window frames, JSON/XML expressions, full-text search,
spatial and hierarchy types, temporal queries, graph MATCH, table sampling,
ordinary table hints and query hints remain available when their resolved data
sources stay within target policy and their operations cannot persist a change.

The policy explicitly distinguishes three concerns:

1. **Persistent mutation:** DML, DDL, `SELECT INTO` of a persistent object,
   sequence consumption and executable modules that may change durable state
   are rejected.
2. **Target authorization:** cross-database/server access, external rowsets,
   unconfigured schemas and denied relations are rejected even when they only
   read, because they escape the configured target.
3. **Resource safety:** deadlines, admission limits, lock timeouts and response
   budgets bound availability risk. Query complexity and locking hints are not
   treated as data mutation.

SQL Server has no transaction mode equivalent to PostgreSQL `BEGIN READ ONLY`.
`ApplicationIntent=ReadOnly` is a routing declaration for availability groups,
not an authorization boundary, and a primary replica can accept a read-intent
connection. Consequently, the dedicated database principal and its attested
effective permissions are the final write-protection boundary. Static and
semantic validation, explicit transactions and replica routing are defense in
depth.

## Motivation

SQL Server is a common system of record for small and medium businesses using
.NET applications, ERP, finance, retail, manufacturing and Windows-centered
infrastructure. Supporting it closes the largest relational-engine gap after
MySQL and PostgreSQL.

The existing SQL abstraction is reusable, but MySQL or PostgreSQL parsing and
privilege assumptions are not. T-SQL differs in important ways:

- `SELECT ... INTO` creates and populates a table;
- `NEXT VALUE FOR` changes a sequence even inside a `SELECT`;
- three- and four-part names can cross databases or linked servers;
- synonyms, views, functions, ownership chains and module signing can change
  the effective authority of apparently simple queries;
- SQLCLR and external data features can execute code or contact other systems;
- server, database, schema, object and column permissions form an implication
  hierarchy with fixed and user-defined roles;
- `ApplicationIntent=ReadOnly` and readable secondaries are routing and
  availability features, not substitutes for permissions;
- estimated plans use session-scoped `SET SHOWPLAN_XML`, not an `EXPLAIN`
  statement.

An engine-specific design is therefore required to preserve both broad query
expressiveness and the persistent-mutation guarantee.

## Goals

- Support SQL Server through `engine: sqlserver` and the existing SQL MCP tools.
- Support SQL Server 2019, 2022 and 2025 at database compatibility levels 150,
  160 and 170 after each exact combination passes integration tests.
- Preserve advanced, result-producing T-SQL rather than maintaining a small
  positive list of familiar query forms.
- Reject only persistent mutation capabilities, target-scope escapes,
  unprovable executable capabilities and explicit resource-limit violations.
- Prove the connected login/user has no effective durable-write authority.
- Accept both schema-qualified and safely resolvable unqualified local object
  names; advanced SQL must not require unnatural rewriting merely for policy.
- Support native `@p1`, `@p2`, ... parameters without string interpolation.
- Reuse query, batch, schema and estimated-plan tools with additive,
  engine-neutral result changes only.
- Support ordinary nondeterministic but non-mutating expressions for execution;
  determinism affects caching, not query admission.
- Periodically re-attest permission, role and executable-module drift and fail
  closed when the latest proof is stale or unhealthy.
- Provide a path to explicitly attested read-only T-SQL functions and reporting
  procedures without allowing arbitrary `EXEC`.

## Non-goals

- Treating SQL Server as MySQL-compatible or PostgreSQL-compatible.
- Using regexes, leading keywords or `database/sql` transaction flags as the
  T-SQL security boundary.
- Allowing the model to supply a server, instance, DSN, database, credential,
  encryption option or application intent.
- Providing DML, schema migration, administration, backup/restore, DBCC,
  Service Broker, replication or SQL Agent tools.
- Guaranteeing confidentiality for columns the configured identity may select.
  Column grants, row-level security, dynamic data masking and curated views
  remain operator responsibilities.
- Supporting arbitrary stored procedures, dynamic SQL, SQLCLR or extended
  stored procedures merely because their names suggest they are read-only.
- Supporting linked servers, external tables, PolyBase, `OPENROWSET`,
  `OPENDATASOURCE`, `OPENQUERY`, bulk/file rowsets or external REST/AI calls in
  the first release.
- Enabling Always Encrypted key providers, enclave access or caller-selectable
  Microsoft Entra credentials in the first release.
- Claiming Azure Synapse, Fabric Warehouse or third-party TDS products as SQL
  Server-compatible without separate RFCs and test matrices.
- Guaranteeing that an allowed analytical query cannot consume substantial
  server CPU, memory, tempdb or I/O before its deadline.

An unsupported operational statement is not automatically classified as a
mutation. Some non-mutating administrative statements are outside the public
query API because they have different result, permission and operational
contracts.

## Supported platforms and versions

The first production matrix is:

| Platform | Engine/version | Compatibility level | Initial status |
| --- | --- | --- | --- |
| SQL Server on Windows/Linux | 2019 (15.x) | 150 | required |
| SQL Server on Windows/Linux | 2022 (16.x) | 150 or 160 | required |
| SQL Server on Windows/Linux | 2025 (17.x) | 150, 160 or 170 | required |
| Azure SQL Database | current service | 150, 160 or 170 | staged profile |
| Azure SQL Managed Instance | current service | 150, 160 or 170 | staged profile |

The server product version, `EngineEdition`, database compatibility level and
edition are inspected independently. Azure SQL reports versions differently
from boxed SQL Server, so it is never admitted solely by parsing
`SERVERPROPERTY('ProductVersion')`.

The parser version is selected from database compatibility level, not just
server major. A SQL Server 2025 instance hosting a level-150 database uses the
level-150 grammar policy. Unknown compatibility levels, preview-only grammar or
unverified engine editions fail startup.

Support means that the exact server/compatibility pair has passed the parser,
driver, permission, cancellation and hostile corpus. A newer server is not
silently accepted on the assumption that T-SQL is backward compatible.

## Security invariants

The implementation maintains the following invariants:

1. Public calls cannot persistently change user data, schema, permissions,
   sequences, configuration, jobs, queues, files or external systems.
2. Every referenced user relation resolves to the configured database and an
   allowed entry object; denied entry objects cannot be named directly.
3. The runtime identity has no effective permission that can persistently
   modify the instance or configured database.
4. Ownership, impersonation, role membership, module signing and ownership
   chains cannot give the runtime identity an unmeasured write capability.
5. Every executable user-defined function or optional reporting module used by
   a public call has a complete, current, fail-closed capability proof.
6. Parser, resolver or catalog uncertainty rejects the request or marks the
   target unhealthy; it never falls back to sending the SQL to the server.
7. Parameters are sent through native TDS binding. User values are never
   interpolated into T-SQL.
8. Cancellation is followed by transaction rollback and connection-state
   verification; an uncertain connection is discarded.
9. Internal catalog and session-control statements are fixed by the binary and
   cannot contain caller-supplied identifiers or SQL fragments.
10. Result, plan and error handling obey the same row, cell and exact JSON byte
    limits as existing engines.

## Policy principle: block effects, not sophistication

A query is not rejected for being nested, expensive-looking, uncommon or
optimizer-specific. The AST and resolved capabilities determine admission.

For example, all of the following are intended to be valid:

```sql
WITH sales_tree AS (
    SELECT parent_id, child_id, 0 AS depth
    FROM reporting.sales_hierarchy
    WHERE parent_id = @p1
    UNION ALL
    SELECT h.parent_id, h.child_id, t.depth + 1
    FROM reporting.sales_hierarchy AS h
    JOIN sales_tree AS t ON h.parent_id = t.child_id
)
SELECT TOP (@p2)
       t.depth,
       s.region,
       SUM(s.amount) OVER (
           PARTITION BY s.region
           ORDER BY s.booked_at
           ROWS BETWEEN 6 PRECEDING AND CURRENT ROW
       ) AS rolling_amount
FROM sales_tree AS t
CROSS APPLY reporting.sales_for_node(t.child_id) AS s
ORDER BY rolling_amount DESC
OPTION (MAXRECURSION 1000, RECOMPILE);
```

It remains valid because CTE recursion, APPLY, a table-valued function, a
window frame and query hints are not themselves mutations. Admission depends on
the relations and the attested capability of `reporting.sales_for_node`.

Conversely, this simple-looking statement is rejected:

```sql
SELECT NEXT VALUE FOR reporting.invoice_number;
```

`NEXT VALUE FOR` allocates and persists sequence state even if the surrounding
transaction later rolls back. Syntax simplicity is irrelevant.

Pure expressions whose names contain words such as `MODIFY`, `WRITE` or
`DELETE` are not rejected by string matching. For example, `JSON_MODIFY` returns
a transformed value and does not persist it, so it is allowed in a `SELECT`.

## Final boundary: dedicated SQL Server identity

### Runtime login and database user

Every boxed SQL Server target uses a dedicated SQL login mapped to exactly one
dedicated database user. Windows or Microsoft Entra identities may be added by a
later authentication profile, but they must satisfy the same effective
permission proof.

The identity must:

- not be `sysadmin`, the server owner, database owner or `dbo`;
- not be a member of any fixed server role other than implicit `public`;
- not have `CONTROL SERVER`, `ALTER ANY LOGIN`, `ALTER ANY SERVER ROLE`,
  `IMPERSONATE ANY LOGIN`, `CONNECT ANY DATABASE`,
  `SELECT ALL USER SECURABLES`, external-access, unsafe-assembly, credential,
  endpoint, trace, state-alteration, shutdown or equivalent implied authority;
- have no impersonation permission on a login, user, role or application role;
- have no database ownership, `CONTROL`, `ALTER`, `TAKE OWNERSHIP`, DDL, DML,
  `REFERENCES`, `RECEIVE`, queue, assembly, certificate, key, credential,
  external script, external endpoint or grant-option authority;
- have `CONNECT` only where required by the configured target and SQL Server
  infrastructure;
- receive `SELECT` only on allowed schemas, relations or columns;
- receive optional `VIEW DEFINITION`, `SHOWPLAN` and
  `VIEW CHANGE TRACKING` only as explicitly configured and only at the narrowest
  supported scope;
- receive `EXECUTE` only on an exactly named module whose capability proof is
  enabled by configuration and currently healthy;
- not own a schema, relation, function, procedure, synonym, sequence, type,
  assembly, certificate, key or security policy;
- have an operator-controlled default schema that is included in target policy;
- have no usable write authority through fixed roles, user-defined nested roles,
  `guest`, `public`, ownership chains, certificates or asymmetric keys.

Role membership is not categorically forbidden because many small-business
deployments provision shared read roles. The verifier expands the complete
server and database role graph, includes `public`, detects cycles, accounts for
GRANT/DENY and permission implication, and evaluates the final effective
capability. If the graph cannot be completely observed, production startup
fails. A strict option may require no explicit memberships.

### Why DENY alone is insufficient

Provisioning may add explicit `DENY INSERT`, `DENY UPDATE`, `DENY DELETE` and
similar defense-in-depth rules, but the service does not infer safety from those
rows alone. `sysadmin` can bypass ordinary permission checks, ownership and
permission implication matter, object/column rules can interact, and new SQL
Server versions add permissions.

The verifier computes effective permissions and rejects every unknown
permission. It does not maintain only a short forbidden-name list.

### Readable replicas

`application_intent: read-only` is sent only when configured. A
`require_read_only_replica` option additionally verifies that the connected
database reports non-updateable/read-only replica state using fixed queries.
This is recommended for production analytics, but never replaces the dedicated
identity because read intent can route to a primary and replica topology can
change.

## Defense in depth

1. Exact configured target lookup; the caller cannot supply connection data.
2. TLS, product, database, identity and effective-permission verification at
   startup.
3. A Microsoft T-SQL parser accepting a single supported read statement and
   emitting a complete policy-fact document.
4. Local name binding and catalog resolution for every relation, synonym,
   temporal history object and user-defined callable.
5. AST rejection of durable mutation targets and persistent state allocators.
6. A regular explicit transaction that is always rolled back. SQL Server has no
   native read-only transaction, so this is cleanup rather than authorization.
7. Fixed session initialization, deadlines, lock timeout, admission limits and
   exact response budgets.
8. Periodic permission and module re-attestation with fail-closed health.
9. Query-content-free audit records and low-cardinality metrics.

## Dependency decisions

### Driver

Use `github.com/microsoft/go-mssqldb` through `database/sql`, with driver name
`sqlserver` rather than the deprecated compatibility driver name `mssql`.

Reasons:

- it is Microsoft's maintained pure-Go TDS driver;
- it supports SQL Server and Azure SQL without ODBC or CGO;
- it uses native named `@` parameters without client-side token replacement;
- it supports TLS, context cancellation and availability-group read routing;
- it fits the current pool and engine-neutral interfaces.

The dependency is pinned to an exact reviewed version. Driver upgrades execute
the full real-server, cancellation, TLS and hostile-query matrix.

The application constructs a driver configuration object from validated fields.
It does not concatenate a DSN, and it never accepts arbitrary connection-string
properties from YAML or MCP input.

### T-SQL parser

Use Microsoft's open-source `Microsoft.SqlServer.TransactSql.ScriptDom`, pinned
to an exact NuGet version and artifact hash, behind a small maintained parser
helper named `readonly-db-mcp-tsql-parser`.

ScriptDom is selected because grammar fidelity is necessary to support advanced
T-SQL without broad false rejections. The community ANTLR T-SQL grammar is useful
for differential testing but is not the production authorization parser.

The helper:

- runs as a long-lived child process started from a trusted path adjacent to the
  main binary;
- communicates only over inherited, length-prefixed stdin/stdout pipes;
- has no database credentials, network configuration or writable working
  directory;
- selects `TSql150Parser`, `TSql160Parser` or `TSql170Parser` from the attested
  database compatibility level;
- accepts one bounded UTF-8 SQL payload and returns a versioned bounded policy
  document, never generated SQL;
- reports every statement, relation name, callable, sequence expression,
  external rowset, `INTO` target, hint, variable and parser error;
- fails the request on any unhandled ScriptDom fragment relevant to execution
  or object resolution;
- is supervised with bounded restarts and health checks.

NativeAOT self-contained builds are preferred if the pinned ScriptDom version
passes compatibility and parser-corpus tests. Otherwise release artifacts must
ship a framework-dependent helper and explicitly declare the required .NET
runtime. SQL Server support is not compiled as silently degraded keyword
validation when the helper is absent.

A bounded helper pool and a Go-side CPU/admission semaphore prevent parse
storms. Helper crashes reject the current request; no SQL reaches SQL Server.

The service executes the original validated SQL. Neither ScriptDom's generator
nor a custom rewriter is used on untrusted statements.

### Parser-helper protocol

The first protocol is logically:

```json
{
  "protocol": 1,
  "compatibility_level": 170,
  "sql": "SELECT ..."
}
```

and returns:

```json
{
  "protocol": 1,
  "parser_build": "pinned-build-id",
  "statement_count": 1,
  "statement_kind": "select",
  "relations": [],
  "functions": [],
  "parameters": ["p1"],
  "global_variables": [],
  "into_targets": [],
  "sequence_references": [],
  "external_sources": [],
  "hints": [],
  "unknown_fragments": []
}
```

Wire frames have independent request and response byte ceilings. The Go process
validates every field and does not trust the helper to apply target policy. The
parser build ID is part of the policy revision and result-cache key.

## Configuration

Example:

```yaml
targets:
  finance-sqlserver-production:
    engine: sqlserver
    environment: production
    consistency: eventual
    host: finance-ag-listener.internal.example
    port: 1433
    database: Finance
    username: finance_mcp_ro
    password_file: /run/secrets/finance-sqlserver.password
    allowed_schemas:
      - reporting
      - dimensions
    denied_tables:
      - reporting.raw_payroll
    connection:
      connect_timeout: 3s
      read_timeout: 12s
      write_timeout: 3s
      max_open: 2
      max_idle: 1
      max_lifetime: 3m
      max_idle_time: 1m
    sqlserver:
      application_name: readonly-db-mcp
      application_intent: read-only
      require_read_only_replica: true
      lock_timeout: 1500ms
      batch_isolation: snapshot
      require_snapshot_isolation: true
      privilege_recheck_interval: 5m
      allow_change_tracking: false
      allow_readonly_modules: false
      strict_role_membership: false
    tls:
      mode: verify-full
      ca_file: /etc/readonly-db-mcp/sqlserver-ca.pem
      server_name: finance-ag-listener.internal.example
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

Optional attested reporting modules use exact names and definition hashes:

```yaml
    sqlserver:
      allow_readonly_modules: true
      readonly_functions:
        - name: reporting.sales_for_node
          definition_sha256: 7ad1...c204
      readonly_procedures:
        - name: reporting.month_end_report
          definition_sha256: 822f...91aa
          max_result_sets: 2
```

Hashes are deployment assertions, not the safety proof. The parser still
classifies the module and its complete static dependency/call graph. A matching
hash cannot make a mutating body safe.

### Configuration rules

- Add `EngineSQLServer = "sqlserver"`; the default port is 1433.
- A `sqlserver` block is rejected for other engines, and other engine-specific
  blocks are rejected for SQL Server targets.
- Production targets require authenticated encryption. `verify-full` validates
  the configured host or explicit server name against the certificate.
- Instance discovery through SQL Browser and caller-provided instance names is
  not supported initially; configure an exact host and port.
- `application_intent` is `read-write` or `read-only`; production defaults to
  `read-only`, but the value is never described as a permission control.
- `lock_timeout` must be positive, below the request maximum and below a hard
  ceiling. Caller SQL cannot change it.
- `batch_isolation: snapshot` requires database option
  `ALLOW_SNAPSHOT_ISOLATION = ON`. No configuration change is attempted.
- Arbitrary session initialization SQL, connection properties, failover
  partners, packet sizes, workstation IDs and access tokens are forbidden.
- Allowed/denied identifiers are stored as operator text but resolved through
  SQL Server catalog comparison under the database collation. Go-side lowercase
  comparison is not an authorization decision.

## Provisioning model

Provisioning occurs out of band. The following is an illustrative baseline, not
a universal script:

```sql
USE [master];
CREATE LOGIN [finance_mcp_ro]
    WITH PASSWORD = 'generated-out-of-band',
         CHECK_POLICY = ON,
         CHECK_EXPIRATION = ON,
         DEFAULT_DATABASE = [Finance];

USE [Finance];
CREATE USER [finance_mcp_ro]
    FOR LOGIN [finance_mcp_ro]
    WITH DEFAULT_SCHEMA = [reporting];

GRANT CONNECT TO [finance_mcp_ro];
GRANT SELECT ON SCHEMA::[reporting] TO [finance_mcp_ro];
GRANT SELECT ON SCHEMA::[dimensions] TO [finance_mcp_ro];
GRANT VIEW DEFINITION TO [finance_mcp_ro];
GRANT SHOWPLAN TO [finance_mcp_ro];
```

Prefer granting `SELECT` on curated views or exact relations when a whole schema
contains sensitive or mixed-purpose objects. `db_datareader` is not the
documented default because it grants reads across all user tables and views and
automatically expands to future objects outside configured schemas.

Do not add `db_owner`, `db_ddladmin`, `db_securityadmin`, `db_datawriter`,
`EXECUTE` at database/schema scope, `VIEW SERVER STATE`, `CONNECT ANY DATABASE`,
`SELECT ALL USER SECURABLES`, `IMPERSONATE`, ownership or grant option.

`VIEW DEFINITION` and `SHOWPLAN` are optional. If omitted, metadata or plan
features that require them are reported unavailable; the runtime never asks the
operator to grant write authority to restore those features.

## Startup attestation

Attestation uses fixed, parameterized catalog queries and permission functions.
It never parses textual output from SSMS or generated GRANT scripts.

### Server, transport and identity

Verify at minimum:

- `ORIGINAL_LOGIN()`, `SUSER_SNAME()`, `USER_NAME()` and database principal IDs
  match the configured identity model;
- `DB_NAME()` equals the configured database;
- product version, engine edition and compatibility level are supported;
- the database is online and accessible, and optional replica requirements hold;
- TLS is active when required and certificate validation is enforced by the
  client configuration;
- the login is not disabled, sysadmin, a server owner or reachable through an
  unsafe credential type;
- default database and default schema are exactly known;
- database owner is not the runtime login and the runtime user is not `dbo`;
- `TRUSTWORTHY`, cross-database ownership chaining and containment state do not
  invalidate the selected executable-module policy.

### Server permissions and roles

Use `sys.fn_my_permissions(NULL, 'SERVER')`, token views, fixed-role membership
checks and, when fully visible, `sys.server_permissions`,
`sys.server_role_members` and `sys.server_principals`.

Allowed effective server authority is restricted to the minimum connection and
metadata baseline for the selected platform. All other effective permission
names fail closed, including new names introduced by a future engine version.

SQL Server 2022+ security-definition permissions and pre-2022 metadata
visibility differ. If the runtime identity cannot completely prove its own role
and permission closure, production configuration requires one of:

1. a separate, operator-configured attestation identity that can only execute
   the binary's fixed catalog checks; or
2. a DBA-installed, certificate-signed attestation procedure with a pinned
   definition and result contract.

The attestation identity is never placed in a query pool and never receives
caller SQL. Failure to refresh its proof makes the target unhealthy. The first
implementation should prefer self-attestation where complete and treat the
fallback as a Phase-0 feasibility gate rather than silently accepting incomplete
catalog visibility.

### Database, schema, object and column permissions

Evaluate effective permissions at all scopes, including implication,
memberships, `public`, `guest`, ownership and column exceptions.

Allowed capabilities are:

- `CONNECT` on the configured database;
- `SELECT` on allowed schemas, relations or columns;
- metadata visibility required by enabled metadata tools;
- `SHOWPLAN` only when estimated plans are enabled;
- `VIEW CHANGE TRACKING` only on configured objects when enabled;
- exact function/procedure permissions only for attested read-only modules.

Reject:

- database- or schema-level DML/DDL authority;
- write, alter, control, ownership, impersonation, reference, receive, queue,
  assembly, key, certificate, credential or security-policy permissions;
- grant option on any securable;
- direct SELECT access to user objects outside allowed schemas or explicit
  object policy;
- direct SELECT on a denied relation or denied column;
- UPDATE on any sequence;
- effective access supplied by an unobservable token or role edge.

Catalog visibility is asymmetric: invisible objects normally indicate no
permission, but invisibility must not be used to prove that no hidden role or
server grant exists. The version-specific proof documents which facts come from
effective permission functions and which require security-catalog visibility.

### Ownership chains, views and row-level security

A curated view may legitimately read a base table that the runtime identity
cannot select directly. Therefore, policy applies `allowed_schemas` and
`denied_tables` to entry objects named by caller SQL, while startup attestation
records the transitive dependencies of views and inline functions separately.

Rules:

- direct access to a denied base table remains rejected;
- an explicitly allowed curated view may use an ordinary same-database ownership
  chain to read protected base data;
- the view/module itself must be T-SQL, visible, unencrypted and non-mutating;
- cross-database ownership chains, synonyms or dependencies are rejected;
- `EXECUTE AS`, certificate/asymmetric-key signing and unexpected owners are
  rejected unless a future capability profile models them completely;
- active row-level security policies and predicate functions are inventoried;
- the runtime principal must not have `ALTER ANY SECURITY POLICY`, impersonation
  or ownership that bypasses the intended filter;
- `UNMASK` is rejected unless a target explicitly treats unmasked values as its
  approved data boundary.

The service does not claim that dependency inspection reproduces SQL Server's
confidentiality model. SQL Server permissions, RLS and curated views remain the
data-disclosure boundary.

### Synonyms, temporal, graph and ledger objects

- Local synonyms may be admitted only after their exact base object resolves
  inside the configured database and current policy.
- Synonyms with three/four-part, linked-server, external or unresolved targets
  are rejected.
- A temporal query is allowed only when the current table is an allowed entry
  object and its catalog-linked history table remains local and policy-valid.
- Graph node/edge and ledger tables are readable like tables; their hidden and
  generated columns are normalized and bounded.
- A ledger verification procedure is not implied by SELECT support and would
  require a separately attested reporting-module profile.

### User-defined executable capabilities

Built-in scalar operations are classified by exact ScriptDom node/name and
server compatibility. Nondeterminism alone is not mutation: `GETDATE`,
`NEWID`, `RAND` and similar value-producing functions are executable, though
normally not cacheable.

T-SQL user-defined scalar and table-valued functions may be allowed because SQL
Server prevents ordinary T-SQL UDFs from modifying database state. That server
rule is necessary but not sufficient. The service additionally requires:

- T-SQL module type, not SQLCLR;
- visible, non-encrypted definition;
- no `EXECUTE AS` or module signing;
- no dynamic SQL, extended procedure call, external source, sequence allocation
  or cross-database reference;
- a fully parsed transitive UDF/view dependency graph;
- all entry and dependency objects to meet the configured capability policy;
- an optional configured definition hash and a live computed hash;
- periodic re-attestation on `modify_date`, definition hash, permissions and
  dependency graph.

SQLCLR functions, aggregates and user-defined types are rejected initially.
Microsoft-shipped spatial, hierarchy and other system types are admitted by
exact tested identity rather than a blanket CLR exception.

Unknown built-ins introduced by a newer compatibility level are rejected until
classified. The correct remediation is to classify their actual effect, not to
ban the surrounding advanced query form.

## Session and connection lifecycle

Every physical connection is created with fixed driver-controlled properties:

- configured database;
- bounded connect and command deadlines;
- application name;
- authenticated encryption settings;
- optional read-only application intent;
- no caller-controlled failover or session options.

On checkout, the executor applies fixed session settings such as:

```sql
SET NOCOUNT ON;
SET XACT_ABORT ON;
SET LOCK_TIMEOUT 1500;
SET DEADLOCK_PRIORITY LOW;
```

`LOCK_TIMEOUT` is connection-scoped and persists for the connection. The
implementation must therefore initialize and verify every new physical
connection and restore constants before reuse. Public SQL cannot contain `SET`.
If cancellation, estimated-plan cleanup or driver behavior leaves session state
uncertain, the connection is discarded rather than returned to the pool.

Language, date format, date first, quoted identifier, ANSI settings and
arithmetic behavior that affect parsing or results are pinned where the driver
and server permit. Policy never depends on a user-modifiable ambient setting.

The executor starts an explicit transaction for each query and always rolls it
back. The transaction does not provide a read-only guarantee; it scopes locks,
snapshot state and cleanup.

## T-SQL policy

### Accepted top-level form

The default `query_select` contract accepts exactly one ScriptDom batch
containing exactly one result-producing `SelectStatement`, with optional leading
CTEs as part of that statement. Empty batches, `GO`, multiple statements and
trailing injected statements are rejected.

This single-statement shape is an MCP result contract, not a claim that every
other T-SQL statement mutates data.

### Explicitly supported SELECT features

The policy intends to support all parser-recognized, non-mutating local SELECT
features for the attested compatibility level, including:

- ordinary and recursive CTEs;
- nested and correlated subqueries;
- inner, outer, cross and full joins;
- `CROSS APPLY` and `OUTER APPLY`;
- `UNION`, `INTERSECT` and `EXCEPT`;
- `TOP`, `WITH TIES`, `OFFSET` and `FETCH`;
- aggregate, analytic, ranking and window functions with full frame syntax;
- `GROUPING SETS`, `ROLLUP`, `CUBE`, `PIVOT` and `UNPIVOT`;
- derived tables, table value constructors and local table-valued functions;
- JSON construction/query/transformation and the native JSON type when tested;
- XML construction/query methods that do not update stored XML;
- string, regular expression, date/time, mathematical and conversion functions;
- full-text predicates and rowsets;
- spatial, geography and hierarchy system operations;
- `FOR SYSTEM_TIME` temporal queries;
- graph `MATCH` queries;
- `TABLESAMPLE`;
- change-tracking reads when explicitly enabled and permission-scoped;
- ordinary collations and casts;
- `FOR JSON` and `FOR XML` result formatting;
- safe global-variable reads such as attested `@@VERSION`-class values;
- documented table and query hints, including optimizer and locking hints, when
  they do not introduce an external/cross-target capability;
- safe nondeterministic and session-observation functions.

Locking hints such as `UPDLOCK`, `HOLDLOCK`, `XLOCK` and `TABLOCKX` do not
persistently alter data and are therefore not classified as mutation. They may
cause blocking, so every request remains subject to a short operator-controlled
lock timeout, total deadline and admission limits. Deployments that cannot
tolerate such transient locks should use a readable secondary or an optional
operational profile that rejects write-intent hints; that profile is an
availability policy, not the default semantic mutation policy.

Query hints such as `RECOMPILE`, `MAXDOP`, `MAXRECURSION`, join hints, memory
grant hints and recognized `USE HINT` values are likewise allowed when the
server accepts them for the runtime identity. The service does not maintain a
small performance-opinion allowlist.

### Persistent mutation rejection

Reject every top-level or nested construct capable of persisting a target or
instance change, including:

- `INSERT`, `UPDATE`, `DELETE`, `MERGE` and their `OUTPUT INTO` targets;
- `TRUNCATE`, bulk insert and all DDL;
- `SELECT ... INTO` when the target is persistent or global temporary;
- `NEXT VALUE FOR` and any sequence range allocation;
- transaction commit/savepoint manipulation supplied by the caller;
- permission, role, login, ownership, signing or impersonation changes;
- configuration, statistics, index, maintenance, backup, restore, attach,
  detach, shrink and DBCC operations;
- Service Broker sends/receives with state effects, SQL Agent operations and
  replication/change-data-capture administration;
- persisted session/server context setters and application locks;
- arbitrary `EXEC`, dynamic SQL and module calls without a current read-only
  capability proof;
- external scripts, external REST/AI endpoints and CLR code without a future
  explicit capability model.

`SELECT INTO` is inspected structurally, not by tokens. A comment, nested query,
bracketed identifier or alternate whitespace cannot hide it.

`SELECT ... INTO #local_temp` is also rejected by the initial `query_select`
tool, but for a different reason: it creates no result set for that tool and
belongs to the separately gated local-workspace script capability. It is not
misreported as a persistent mutation of the configured database.

### Target-scope rejection

The following reads are rejected by default even if they do not mutate data:

- three- or four-part references outside the configured database;
- linked-server names and remote synonyms;
- `OPENROWSET`, `OPENDATASOURCE`, `OPENQUERY`, `OPENXML` handles supplied by
  external session setup, BULK/file rowsets and PolyBase/external tables;
- unresolved synonyms or modules;
- ad hoc access to other databases, `master`, `msdb`, `model` or `tempdb`;
- caller-selected catalog/DMV access outside fixed metadata tools;
- user-defined external language, SQLCLR or extended-procedure calls.

These are authorization and trust-boundary decisions, not assertions that an
advanced SQL feature is inherently unsafe.

### Unqualified-name resolution

Unlike the first PostgreSQL implementation, SQL Server does not require every
user relation to be schema-qualified. The resolver distinguishes CTE/derived
aliases from base objects and resolves unqualified names using the attested
database user default schema followed by `dbo`, matching tested SQL Server
behavior.

Resolution uses catalog object IDs and database collation. If more than one
interpretation remains possible, metadata is stale, or the resolved object
changes during validation/execution setup, the request fails closed. Resolved
object IDs and relevant schema version material enter the policy proof.

Two-part local names are preferred in documentation. Three-part names naming
the configured database may be accepted only after exact resolution; they do
not authorize another database. Four-part names are rejected.

### Parameters and variables

SQL Server requests use `@p1`, `@p2`, ... with array positions from the existing
MCP contract.

Validation proves:

- parameter names are case-insensitively exactly `p1` through `pN`;
- positions are contiguous and the highest position equals parameter count;
- `@@` global variables are not confused with input parameters;
- no local variable declaration or assignment appears in `query_select`;
- bound values satisfy count, per-value and aggregate byte ceilings;
- all parameters use `sql.Named` with the preferred driver and are never
  interpolated;
- unsupported structured parameters and table-valued parameters fail clearly.

Scalar JSON values map conservatively. Large integers and exact decimals may be
bound as strings plus an explicit T-SQL cast where caller intent requires an
exact SQL type.

### Comments and batch separators

Ordinary parser-recognized comments are inert and allowed. `GO` is a client-side
batch separator, not T-SQL, and is rejected by the one-batch API. Query size is
bounded before parser-helper invocation.

## Read-only reporting modules

SQL Server deployments often place reports in stored procedures. Blanket
`EXECUTE` would be needlessly restrictive for proven read-only reports but is
unsafe for arbitrary modules.

A later staged capability adds a SQL Server-specific
`sqlserver_readonly_module` tool. It accepts an exact configured module name and
typed parameters, never raw `EXEC` text.

A procedure is eligible only when:

- it is explicitly configured with an exact object ID/name and optional hash;
- it is a visible, unencrypted T-SQL procedure;
- ScriptDom proves every branch contains no persistent mutation target;
- dynamic SQL, unknown nested execution, `EXECUTE AS`, signatures, SQLCLR,
  extended procedures and external access are absent;
- every static nested module has the same complete proof;
- temporary/local workspace mutations are either absent or admitted by the
  separate rule below;
- result-set count and schemas can be described and bounded;
- its EXECUTE grant is exact rather than database/schema-wide;
- its definition, dependency graph and permissions are periodically re-attested.

Module result sets are consumed with `Rows.NextResultSet`, with per-set and total
row/cell/byte limits. Output parameters and return codes are represented
explicitly and bounded.

This staged feature allows genuinely read-only procedures instead of assuming
all stored procedures are dangerous. It also refuses to pretend that an
uninspected procedure is safe.

## Optional local-workspace scripts

Complex SQL Server reports sometimes use `#local` temporary tables or table
variables. These do not persist changes in the configured database, but they do
mutate session-local tempdb state and can amplify resource consumption.

They are not part of the initial `query_select` contract. A future
`sqlserver_readonly_script` capability may allow them only when:

- every statement is parsed in one bounded batch;
- dataflow proves every DML/DDL target is a local `#` temp table or local table
  variable created in that batch;
- global `##` temporary objects are forbidden;
- no persistent object, sequence, external target or dynamic SQL is reachable;
- execution uses a dedicated connection that is rolled back, cleaned and
  discarded rather than pooled;
- deployment uses a dedicated reporting replica or Resource Governor workload
  group with tested tempdb controls;
- all result sets obey aggregate response limits.

This extension is consistent with the persistent-mutation objective, but its
resource model requires a separate RFC or acceptance gate. It must not delay
ordinary advanced SELECT support.

## Query execution

### Single query

1. Validate request, SQL, parameter and response bounds.
2. Parse with the compatibility-specific ScriptDom helper.
3. Apply static mutation and scope policy.
4. Resolve local objects and callable capabilities from the current attested
   catalog snapshot.
5. Recheck target health and acquire an interactive admission permit.
6. Check out an initialized physical connection.
7. Begin a regular transaction at configured isolation.
8. Execute the original SQL with native named parameters and context deadline.
9. Collect rows through exact row, column, cell, nesting and JSON byte budgets.
10. Roll back, verify cleanup, release resources, audit and record metrics.

No production commit path exists in the SQL Server dialect package.

### Batch snapshot

`query_batch` validates every statement before acquiring a database permit.

For a transaction-consistent batch, use SQL Server `SNAPSHOT` isolation and
require `ALLOW_SNAPSHOT_ISOLATION = ON`. The service does not enable the option.
All queries execute sequentially in one transaction that is rolled back.

If snapshot isolation is unavailable, the target either:

- does not implement `core.BatchTarget`; or
- explicitly selects an operator-configured weaker mode whose response reports
  its actual snapshot semantics.

It must not claim that ordinary READ COMMITTED is one stable batch snapshot.
SERIALIZABLE or REPEATABLE READ may be evaluated as an explicit locking profile,
but are not silent fallbacks because of their production blocking impact.

### Estimated execution plan

`query_explain` returns an estimated XML plan using internal
`SET SHOWPLAN_XML ON`; it never enables actual-plan execution.

Flow:

1. Fully validate the SELECT exactly as for execution.
2. Acquire the explain admission class and a dedicated connection.
3. Enable `SHOWPLAN_XML` with fixed internal SQL.
4. send only the already validated SELECT and collect bounded XML.
5. Disable `SHOWPLAN_XML` and verify session state.
6. Discard the connection on any error, timeout or uncertain cleanup.

`SHOWPLAN` permission is required only when this feature is enabled. Plan XML is
treated as untrusted database output, bounded as a cell and never logged.
Arbitrary SHOWPLAN/STATISTICS options and actual execution plans are not caller
controlled.

## Metadata implementation

Metadata tools use fixed catalog queries and never route caller SQL to system
views.

### List tables

Read bounded rows from `sys.schemas`, `sys.tables`, `sys.views` and related
catalogs. Return only entry objects permitted by target policy, excluding system
and internal objects.

Represent at least:

- table or view kind;
- schema and exact identifier spelling;
- temporal current/history relationship;
- graph node/edge kind;
- ledger kind;
- memory-optimized status where applicable.

Apply allowed and denied policy again after the fixed catalog query.

### Describe table

Use fixed queries over `sys.columns`, `sys.types`, `sys.default_constraints`,
`sys.computed_columns`, `sys.identity_columns`, `sys.indexes`,
`sys.index_columns`, `sys.key_constraints`, `sys.foreign_keys`, temporal, graph
and ledger catalogs.

Return additive fields for:

- exact SQL type, length, precision, scale and collation;
- nullability, identity, computed, sparse, hidden, generated and masked state;
- primary, unique, clustered, nonclustered, columnstore and hash indexes;
- key order, included columns and filtered predicates;
- temporal period/history relationship;
- graph and ledger generated columns.

Definitions and predicates are untrusted text, bounded before return and never
executed. Metadata cache keys include target policy revision, database ID,
object ID, schema modification marker, parser build and exact identifier.

`fresh: true` remains subject to metadata admission and cooldown and atomically
replaces cached entries.

## Result representation

Reuse `core.QueryResult` and exact encoded-response budgeting.

SQL Server normalization rules include:

- `tinyint`, `smallint`, `int` and safe `bigint` become JSON numbers;
- integers outside JavaScript's safe range become decimal strings;
- `decimal`, `numeric`, `money` and `smallmoney` remain exact decimal strings
  unless an engine-neutral exact-number representation is introduced;
- `real` and `float` handle non-finite values as bounded strings or explicit
  tagged values rather than invalid JSON;
- `date`, `datetime`, `smalldatetime`, `datetime2`, `time` and `datetimeoffset`
  use explicit documented encodings; timezone-less values are not falsely
  labeled UTC;
- `uniqueidentifier` becomes canonical text;
- `binary`, `varbinary`, `image`, rowversion and unknown byte values become
  `base64:<payload>`;
- JSON is returned as bounded text unless safe lossless decoding fits all
  numeric and byte rules;
- XML and estimated-plan XML are bounded text;
- `sql_variant` is normalized from its concrete driver type with an optional
  type tag if ambiguity would otherwise lose information;
- built-in geography, geometry, hierarchyid and vector values use tested,
  bounded representations;
- unknown SQLCLR or extension values fail closed rather than bypassing budgets;
- duplicate column labels use the existing deterministic suffix convention.

The collector closes rows promptly on truncation and drains only when the driver
requires it for safe connection reuse. Otherwise the connection is discarded.

## Result-cache eligibility

SQL Server result caching is disabled by default.

Execution safety and cache determinism are separate decisions. A query using
`GETDATE()`, `NEWID()`, `RAND()`, session observations, temporal relative time
or ordinary nondeterministic values may execute but normally cannot be cached.

A result is eligible only when:

- caching is explicitly configured with an accepted freshness tradeoff;
- every relation/object/module and its policy revision are current;
- no volatile, time-, random-, identity-, context-, change-tracking-, temporal-
  current-time or replica-state dependency is present;
- no table/query hint makes reuse semantically surprising;
- no external, temporary, cross-database or user-defined unproven capability is
  involved;
- the result succeeds and is not truncated.

SQL Server determinism metadata is used only as evidence, not as a complete
security oracle. The first release may conservatively cache only simple audited
built-in expressions and direct local relations.

Cache keys include engine/platform, server major, compatibility level, database
ID, parser build, policy revision, resolved object/module revisions, SQL
fingerprint, typed parameters, row limit and result schema version.

## Error handling

Map driver and SQL Server error numbers/classes to stable sanitized categories:

- authentication/TLS failure;
- permission or scope rejection;
- syntax/compatibility failure;
- deadlock;
- lock timeout;
- statement timeout/cancellation;
- snapshot isolation conflict;
- unavailable/failover;
- resource limit;
- generic database rejection.

Do not expose raw server messages, procedure names, SQL fragments, parameter
values, hostnames, linked-server names or catalog details. A stable numeric SQL
Server error code may be returned only after review proves it does not encode
sensitive content.

Cancellation must send the driver's attention/cancel path, roll back, and prove
the server request stopped. Pool capacity must recover within the acceptance
gate. An uncertain TDS session is discarded.

## Audit and metrics

Audit events retain:

- query/batch/module ID;
- target, engine and operation;
- non-reversible SQL fingerprint;
- statically named and resolved allowed entry objects;
- policy decision and stable reason class;
- parser/policy revision without raw SQL;
- rows, bytes, truncation, cache status and duration.

Never record raw SQL, parameters, returned values, plans, credentials or module
definitions.

Metrics use bounded labels such as engine, operation, outcome, cache status and
phase. Database, schema, object, login, SQL fingerprint and error-message text
are not metric labels.

Useful phase timings are parser IPC, AST policy, catalog resolution,
transaction setup, server execution, row normalization and rollback/cleanup.

## Package and interface changes

```text
internal/dialects/sqlserver/
    config.go
    connect.go
    attestation.go
    permissions.go
    resolver.go
    policy.go
    modules.go
    metadata.go
    execute.go
    batch.go
    explain.go
    normalize.go
    errors.go

cmd/readonly-db-mcp-tsql-parser/
    Program.cs
    Protocol.cs
    PolicyFactVisitor.cs
```

Engine-neutral changes:

- registry construction for `EngineSQLServer`;
- `TargetInfo.parameter_style = "@p1"`;
- additive target capability fields for batch snapshot, estimated plan and
  optional reporting modules;
- additive column/index metadata needed by SQL Server;
- shared collection code only where byte and type semantics are genuinely
  common;
- parser-helper supervision as a bounded shared service whose policy results
  remain target-specific.

The SQL Server package does not import MySQL or PostgreSQL dialect packages.
Duplication is preferable to conditionals that obscure permission or transaction
semantics.

## Performance and resource design

- SQL length is bounded before parser IPC.
- Parser work occurs before database admission but behind its own CPU and queue
  ceilings.
- Successful static policy facts may be cached by parser build, compatibility
  level and SQL fingerprint; mutable catalog resolution is never assumed from
  that cache.
- Catalog snapshots and module proofs are immutable generations atomically
  swapped after complete re-attestation.
- Database concurrency remains globally and per-target bounded.
- Lock timeout is always shorter than request deadline.
- Query cancellation, TDS attention and transaction rollback are independent
  controls.
- Result collection enforces max columns, rows, per-cell bytes, total JSON bytes
  and nested value depth.
- Prepared statement caching is disabled initially because object/schema and
  failover invalidation require separate evidence.
- Estimated plans use a dedicated connection path because SHOWPLAN changes
  session behavior.
- Production documentation recommends a readable replica and an operator-side
  Resource Governor workload group where available.

Initial performance gates:

| Path | Gate |
| --- | --- |
| Warm simple SELECT parse and policy | p95 below 2 ms |
| Advanced 32 KiB T-SQL parse/policy | p95 below 15 ms |
| Warm metadata hit | within 10% of RFC-0001 baseline |
| Parser-helper crash recovery | rejects in-flight work; healthy within 2 s |
| Scalar result normalization | at least 75 MiB/s |
| Cancellation | server request stops and pool capacity recovers within 1 s |
| Mixed-engine saturation | no target exceeds configured capacity |

Gates may be revised from recorded benchmarks, but revisions cannot remove
correctness or boundedness requirements.

## Testing strategy

### Parser and policy unit tests

- one statement, multiple statements, comments and `GO` handling;
- every ScriptDom statement class and unknown-fragment fail-closed behavior;
- advanced SELECT acceptance across compatibility levels 150/160/170;
- CTE, APPLY, PIVOT, windows, grouping sets, JSON/XML, full text, spatial,
  temporal, graph, ledger, sampling and hints;
- structural `SELECT INTO` rejection;
- `NEXT VALUE FOR` in projections, defaults, expressions and nested queries;
- DML/DDL hidden by CTEs, OUTPUT, comments or unusual quoting;
- local, two-part, same-database three-part and forbidden cross-target names;
- CTE/alias/base-object disambiguation under case-sensitive and insensitive
  collations;
- `@pN` parameter gaps, duplicates, case and `@@` globals;
- pure but nondeterministic execution versus result-cache ineligibility;
- JSON_MODIFY and other pure transformation functions are not rejected by name;
- parser-helper protocol size, version, malformed response, crash and timeout.

### Permission and capability tests

- direct, role-derived, nested-role, `public`, `guest`, GRANT, DENY and implied
  permissions;
- all fixed server and database roles;
- sysadmin, dbo, database owner, schema/object owner and grant option;
- `CONNECT ANY DATABASE`, `SELECT ALL USER SECURABLES`, impersonation and
  metadata/security-definition permissions;
- column-level SELECT and exceptional column permissions;
- permitted curated views over directly denied base tables;
- ownership chains, cross-database chaining, synonyms and signed modules;
- RLS predicates, masking and rejected UNMASK;
- T-SQL UDF dependency closure, encrypted definitions, SQLCLR and extended
  procedures;
- permission/module drift after startup and stale-proof failure.

### Hostile T-SQL corpus

At minimum include:

- all INSERT/UPDATE/DELETE/MERGE forms and OUTPUT destinations;
- `SELECT INTO` with quoted, bracketed, temp, global-temp and schema targets;
- sequence allocation through `NEXT VALUE FOR` and module indirection;
- DDL, permissions, ownership, impersonation and transaction control;
- `EXEC`, `sp_executesql`, dynamic SQL and procedure/function call chains;
- linked servers, synonyms, OPENROWSET/OPENDATASOURCE/OPENQUERY and BULK access;
- SQLCLR, assemblies, external scripts, external REST/AI and extended procedures;
- persisted context, application locks, Service Broker and job/agent paths;
- view/UDF ownership chains that reach mutating or cross-database capabilities;
- Unicode confusables, escaped identifiers, nested comments and semicolon
  injection;
- deeply nested ASTs, pathological literals and parser resource exhaustion;
- cancellation during result streaming and during blocked locks.

Every hostile case asserts unchanged durable state across:

- allowed and denied tables;
- schemas and object counts;
- sequences;
- permissions and principals;
- database/server configuration visible to the test administrator;
- external-test sentinels where an external capability is exercised.

### Real-server integration matrix

Run disposable SQL Server 2019, 2022 and 2025 containers/instances for supported
host platforms and compatibility levels. Azure SQL uses a separate opt-in CI
matrix with short-lived credentials.

Verify:

- documented safe principals start and every added write capability fails;
- SQL authentication, TLS verification and invalid certificate/name behavior;
- read-intent routing and `require_read_only_replica` truthfulness;
- snapshot batches observe one snapshot and do not silently downgrade;
- estimated plans never execute the query and SHOWPLAN state never leaks;
- cancellation stops server work and restores pool capacity;
- metadata covers temporal, graph, ledger, memory-optimized and all index kinds;
- case-sensitive/case-insensitive identifier resolution is correct;
- curated views and RLS behave as the database defines;
- privilege/module changes make the target unhealthy at the next recheck;
- mixed MySQL/PostgreSQL/SQL Server/Redis load preserves fairness.

### Fuzz, differential, race and soak tests

- Fuzz ScriptDom helper framing and Go policy-document validation.
- Fuzz T-SQL parser/walker with bounded input and assert no unclassified
  executable node reaches resolution.
- Differentially compare ScriptDom parsing with the target SQL Server compiler
  and the pinned ANTLR grammar for corpus expansion; disagreements fail tests or
  become documented unsupported syntax, never automatic acceptance.
- Race-test catalog generation swaps, policy caches, health transitions,
  helper restarts and shutdown.
- Soak mixed query/metadata/batch/plan workloads for at least 30 minutes.
- Track Go heap, helper heap, goroutines, helper processes, connections,
  tempdb/version-store effects, parser CPU and cancellation latency.

## Implementation phases

### Phase 0: Feasibility and security proof

- Pin `go-mssqldb` and ScriptDom versions.
- Prove ScriptDom helper packaging on supported release platforms.
- Inventory every ScriptDom SELECT child node for levels 150/160/170.
- Prove complete effective-permission visibility for SQL Server 2019/2022/2025
  and define the attestation fallback where self-observation is incomplete.
- Validate unqualified-name resolution, synonyms, UDFs and ownership chains.
- Verify cancellation and session reset behavior of the driver.
- Record dependency and parser benchmarks under `docs/benchmarks/`.

Exit gate: implementation does not proceed if permission closure, parser
coverage or connection cleanup cannot fail closed without broadly rejecting
advanced SELECT.

### Phase 1: Configuration, driver and identity

- Add strict engine/config models and validation.
- Construct driver configuration without arbitrary DSNs.
- Implement TLS, pool lifecycle, version/edition/compatibility gates and fixed
  session initialization.
- Add target identity and optional readable-replica checks.

Exit gate: safe disposable targets connect; unsupported platforms and identity
mismatches fail startup.

### Phase 2: Effective permission attestation

- Implement server/database role closure and effective permission evaluation.
- Add schema/object/column, ownership, guest/public and grant-option checks.
- Add periodic immutable attestation generations and health gating.
- Document safe provisioning for boxed SQL Server and Azure profiles.

Exit gate: each tested write/ownership/impersonation capability independently
prevents startup or makes a running target unhealthy.

### Phase 3: Parser helper and static policy

- Implement bounded parser-helper protocol and supervision.
- Add exhaustive mutation, external-source, object and parameter fact extraction.
- Implement compatibility-specific policy and parser-fact caching.
- Add advanced acceptance and hostile corpora plus fuzzing.

Exit gate: advanced SELECT corpus is accepted; durable-mutation corpus is
rejected without contacting SQL Server.

### Phase 4: Semantic resolution

- Resolve local and unqualified objects under database collation.
- Implement allowed/denied entry policy, synonyms, temporal history and graph
  objects.
- Prove T-SQL UDF and view dependency capabilities.
- Atomically bind resolutions to the current catalog/attestation generation.

Exit gate: no unresolved, cross-target or unproven callable reaches execution.

### Phase 5: Execution, batch and estimated plans

- Implement query transactions, native parameters, deadlines and cancellation.
- Implement snapshot batches without silent isolation downgrade.
- Implement dedicated-connection SHOWPLAN_XML.
- Reuse exact response budgets, audit and metrics.

Exit gate: real-server hostile tests leave durable state unchanged and all
connections recover or are discarded safely.

### Phase 6: Metadata and normalization

- Implement bounded table/view/column/index metadata.
- Cover temporal, graph, ledger and SQL Server-specific types.
- Integrate target-local metadata cache and forced refresh.

Exit gate: metadata and normalization tests pass the full server matrix.

### Phase 7: Production hardening

- Complete mixed-engine load, race, fuzz and soak tests.
- Validate permission and module drift under concurrent requests.
- Publish provisioning, TLS, replica and Resource Governor guidance.
- Keep SQL Server result cache disabled by default.

Exit gate: all acceptance criteria pass and rollback can disable SQL Server
targets without affecting existing engines.

### Phase 8: Attested reporting modules

- Add configured T-SQL UDF and reporting-procedure proofs.
- Add structured procedure invocation and multi-result bounding.
- Re-attest module definitions, signatures, execution context and dependencies.

Exit gate: every accepted module is non-mutating across all branches and every
definition/permission drift fails closed.

The optional local-workspace script capability requires its own go/no-go review
after the core engine ships.

## Rollout and rollback

- SQL Server support is selected only by explicit target configuration.
- Existing engines and configurations remain byte-for-byte compatible.
- Initial production rollout uses one target, low concurrency, result caching
  off, short deadlines and preferably a readable secondary.
- Observe policy rejection classes, parser-helper health, cancellation, locks,
  tempdb/version-store usage and pool recovery before expanding.
- A feature flag may disable reporting modules independently of SELECT support.
- Rollback removes/disables SQL Server target aliases and parser helpers; it does
  not migrate data or modify database grants automatically.
- Credentials and grants are removed out of band by the database administrator.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| No native read-only transaction | Effective least-privilege identity is final boundary; all transactions roll back |
| T-SQL grammar breadth | Official ScriptDom parser, compatibility-level selection, exhaustive visitor and hostile corpus |
| Parser helper deployment complexity | Pinned self-contained artifact where possible; hard failure when unavailable |
| Incomplete permission visibility | Effective permission functions plus version-specific complete catalog proof or isolated attestor |
| Ownership/module escalation | Reject ownership, execution context, signing and unproven call graphs |
| SQLCLR/external side effects | Reject by default; exact future capability profile required |
| Read intent mistaken for security | Documentation and runtime metadata state explicitly that it is routing only |
| Locking hints block production | Short lock timeout/deadline, bounded concurrency, replica recommendation |
| Snapshot version-store pressure | Explicit operator opt-in, monitoring and bounded batch duration |
| SHOWPLAN session leakage | Dedicated connection, fixed cleanup, discard on uncertainty |
| New SQL Server permissions/features | Exact version gates and unknown-permission/unknown-node fail closed |
| Broad false positives hurt analytics | Classify effects and resolve capabilities instead of query-shape allowlists |

## Alternatives considered

### Rely only on a SELECT-only login

Rejected as the sole design. It is the final boundary, but parser and target
scope validation reduce damage from provisioning mistakes and prevent attempts
to reach external systems or consume sequences.

### Use `sql.TxOptions{ReadOnly: true}`

Rejected as a security claim. SQL Server/go-mssqldb does not provide a native
read-only transaction guarantee equivalent to PostgreSQL. Unsupported driver
flags must not create a false assurance.

### Treat `ApplicationIntent=ReadOnly` as enforcement

Rejected. It participates in availability-group routing and can connect to a
primary depending on topology/configuration.

### Reuse the MySQL or PostgreSQL parser

Rejected. Neither grammar models T-SQL `SELECT INTO`, hints, multi-part names,
modules or compatibility levels correctly.

### Use regexes or a SELECT-prefix check

Rejected. They cannot structurally detect nested syntax, comments, sequence
allocation, alternate quoting, executable modules or multi-statements.

### Use the community ANTLR T-SQL grammar as the authorization parser

Not selected initially. It is valuable for differential tests and may later
replace the helper after independent completeness evidence, but ScriptDom gives
better Microsoft-dialect fidelity for the broad-query requirement.

### Embed .NET in the Go process

Rejected initially. An isolated helper keeps runtime and failure boundaries
explicit and prevents parser runtime faults from corrupting the Go process.

### Reject all UDFs and stored procedures forever

Rejected as inconsistent with the effect-based policy. T-SQL UDFs and reporting
procedures that have a complete non-mutation proof should be usable. Unknown or
dynamic executable code remains rejected because its effect is unprovable.

### Reject every locking or optimizer hint

Rejected as an unnecessary advanced-query restriction. Hints that do not
persist state are governed by deadlines and operational profiles, not labeled
as data mutation.

### Require every object name to be schema-qualified

Rejected as the permanent design. Qualification is recommended, but SQL Server
name resolution can be reproduced from an attested default schema and catalog.
Ambiguity fails closed.

### Allow arbitrary multi-statement scripts with temp tables immediately

Deferred. Local temp mutation does not violate the durable-data objective, but
safe dataflow classification, connection disposal and tempdb resource controls
require a separate acceptance gate.

## Open decisions

Before implementation, maintainers must record decisions for:

1. Whether NativeAOT can package the pinned ScriptDom build on every supported
   platform or a .NET runtime remains an explicit SQL Server dependency.
2. The exact self-attestation proof for each SQL Server major and the minimum
   metadata/security-definition permission it needs.
3. Whether Azure SQL Database ships with the boxed-server release or remains a
   separate staged platform profile.
4. Whether same-database three-part names are admitted initially or after the
   local two-part resolver is proven.
5. The initial set of safe system CLR types and SQL Server 2025 vector/JSON
   representations.
6. Whether an availability-only profile rejects write-intent locking hints for
   primary production targets.
7. Whether reporting procedures require configured definition hashes in all
   environments or only production.

No open decision permits a keyword fallback, incomplete privilege proof or
silent compatibility/version admission.

## Acceptance criteria

SQL Server support is complete only when:

- all supported server/compatibility combinations pass real integration tests;
- advanced SELECT fixtures listed in this RFC are accepted and execute correctly;
- every hostile durable-mutation fixture is rejected before execution or denied
  by the independently verified final permission boundary;
- the runtime principal's effective server/database/object/column authority is
  completely attested and periodically refreshed;
- unqualified and qualified identifiers resolve correctly under tested
  collations without cross-target escape;
- all accepted user-defined functions have complete, current capability proofs;
- native parameter binding, cancellation and pool recovery are verified;
- snapshot batches provide the snapshot semantics advertised in MCP output;
- estimated plans cannot execute statements and SHOWPLAN state cannot leak;
- result normalization cannot exceed cell or exact response budgets;
- audit logs contain no SQL, parameters, values, plans, credentials or DSNs;
- mixed-engine admission, shutdown and health isolation pass race and soak tests;
- production documentation states clearly that permissions, not read intent or
  transaction flags, are the final write-protection boundary.

Optional reporting modules are not part of core completion, but once enabled
they must meet their independent Phase-8 gates.

## References

- Microsoft, SELECT (Transact-SQL):
  https://learn.microsoft.com/en-us/sql/t-sql/queries/select-transact-sql
- Microsoft, Permissions (Database Engine):
  https://learn.microsoft.com/en-us/sql/relational-databases/security/permissions-database-engine
- Microsoft, Server-level roles:
  https://learn.microsoft.com/en-us/sql/relational-databases/security/authentication-access/server-level-roles
- Microsoft, Availability-group client connection access:
  https://learn.microsoft.com/en-us/sql/database-engine/availability-groups/windows/about-client-connection-access-to-availability-replicas-sql-server
- Microsoft, Sequence numbers:
  https://learn.microsoft.com/en-us/sql/relational-databases/sequence-numbers/sequence-numbers
- Microsoft, Create user-defined functions:
  https://learn.microsoft.com/en-us/sql/relational-databases/user-defined-functions/create-user-defined-functions-database-engine
- Microsoft, SET SHOWPLAN_XML:
  https://learn.microsoft.com/en-us/sql/t-sql/statements/set-showplan-xml-transact-sql
- Microsoft, SET LOCK_TIMEOUT:
  https://learn.microsoft.com/en-us/sql/t-sql/statements/set-lock-timeout-transact-sql
- Microsoft, go-mssqldb driver:
  https://learn.microsoft.com/en-us/sql/connect/golang/microsoft-go-mssqldb-driver
- Microsoft, go-mssqldb source:
  https://github.com/microsoft/go-mssqldb
- Microsoft, ScriptDom source:
  https://github.com/microsoft/SqlScriptDOM
- Microsoft, SQL Server version updates:
  https://learn.microsoft.com/en-us/troubleshoot/sql/releases/download-and-install-latest-updates
