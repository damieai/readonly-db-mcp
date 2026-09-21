# Redis and SQL Server qualification record

Implementation started 2026-09-12/13; follow-up review 2026-09-21.
This record separates implemented controls, observed test results and release
gates that still need a provisioned environment. Elasticsearch remains an RFC.

## Implemented in this change

- Redis Search canonical index and HASH/JSON prefix checks at startup and before
  dispatch; advanced query/aggregation arguments are preserved. Alias targets,
  outside prefixes and out-of-scope spellcheck dictionaries are rejected.
- Signed Redis built-in module identities bind the local server artifact SHA-256
  and declared `redis_build_id`. Signed native module profiles remain operator
  trust, not remote cryptographic attestation or a native-code sandbox.
- Legacy Search logical-index ACL patterns are permitted only when separately
  scoped, disjoint from document key prefixes, and backed by signed read-only
  commands. Document keys retain read-only ACLs. Unprofiled commands fail closed.
- Redis Cluster handles the driver's empty-host loopback rewrite only for an
  already attested loopback endpoint with exactly the same port.
- SQL Server requires ScriptDom 170.191.0 and a self-contained .NET 8.0.31 helper.
  No production lexical-scanner fallback remains. The original SQL is executed.
- View/UDF definitions, synonyms and transitive catalog dependencies are checked
  before execution, including computed-column and RLS dependencies. Unqualified
  module sources use native dependency IDs; unqualified query sources use native
  runtime name resolution before scope checks, preserving the `dbo` fallback.
- Catalog visibility (`VIEW DEFINITION` and exact `SELECT` on
  `sys.sql_expression_dependencies`) is required through the runtime identity or
  a separate read-only attestor. Dependency classes keep object IDs separate from
  type/index/XML/partition metadata; native CLR type capabilities are checked. One attestor connection is reserved from `max_open`.
- Parser work shares request admission/deadlines. SHOWPLAN cleanup failure
  discards the physical connection. Snapshot batches retain rollback semantics.

## Observed evidence

| Check | Result | Evidence boundary |
| --- | --- | --- |
| Redis 8.0.5 local standalone | Passed during the initial implementation | Native GET, application/native write denial, cancellation/recovery, signed built-in inventory |
| Redis 7.4.7 + Search 2.10.20 local suite | Four tests passed in 11.195 s during the initial implementation | Standalone; real FT.AGGREGATE; native DROPINDEX denial; prefix replacement rejection; two-Sentinel promotion; three-primary Cluster covering 16,384 slots |
| ScriptDom self-contained Linux x64 helper | Passed again 2026-09-21, 22.381 s | Advanced/hostile corpus at grammar levels 150/160/170, module bodies, source extraction, malformed framing and cancellation |
| Go repository checks | Passed 2026-09-21 | `go test -race ./...`, `go vet ./...`, Go binary build and `git diff --check`; environment-gated tests skip when not configured |
| SQL Server production target integration | Added, not run | Requires operator-provisioned disposable instance and read-only account |

The temporary Redis and SDK installations used for the earlier runs were gone
on 2026-09-21. The retained self-contained parser artifact was executable and
was retested. The Windows Docker client could not reach its Linux engine; the
WSL Docker command reported unavailable integration. No live SQL Server instance
or test configuration was supplied. Tests skipped for missing integration
environment variables do **not** count as database qualification.

## Reproduce

General checks:

```sh
go test -race ./...
go vet ./...
make build
```

Parser packaging and corpus (requires the .NET 8 SDK and package restore):

```sh
make build-sqlserver-parser
make test-sqlserver-parser
```

The project locks the NuGet package and runtime version. Ship the entire
`bin/sqlserver-parser/` directory beside the Go executable. Parsing has a 1 MiB
frame limit, 100,000-node/256-depth AST limits and a 256 MiB CLR heap ceiling per
active helper, plus native/runtime overhead. Catalog proofs cap object/edge
counts and definition bytes; these limits bound resources, not query complexity
categories such as window functions, APPLY, JSON, CTEs or search aggregations.

Owned Redis fixtures (absolute artifact paths, no external database credentials):

```sh
export READONLY_DB_MCP_REDIS_SERVER=/absolute/path/to/redis-server
export READONLY_DB_MCP_REDIS_SEARCH_MODULE=/absolute/path/to/redisearch.so
make test-redis-local
```

The fixture signer is ephemeral and test-only; a successful fixture run does not
issue or approve a production module profile. With no Search module path, the
Search-specific test skips. Set `LD_LIBRARY_PATH` if the chosen server artifact
needs separately extracted shared libraries.

SQL Server production path:

1. Provision a new disposable database with `ALLOW_SNAPSHOT_ISOLATION ON`.
2. Apply [the fixture](../../tools/sqlserver-parser/acceptance-fixture.sql) as its
   administrator, then create the dedicated reader using the documented grants.
3. Configure the target with `allowed_schemas: [reporting]`, the helper path and
   file/environment-sourced reader credentials and `environment: test`. Keep admin credentials out of
   the test process. Test both runtime visibility and separate-attestor profiles.
4. Run:

```sh
export READONLY_DB_MCP_SQLSERVER_CONFIG=/absolute/path/to/disposable.yaml
export READONLY_DB_MCP_SQLSERVER_TARGET=acceptance
go test ./internal/dialects/sqlserver -run TestSQLServerConfiguredTargetIntegration -count=1 -v
```

This gate covers production startup, curated private-base-table access through
views/functions, CTE/window/APPLY/JSON reads, parameter binding, Explain, snapshot
batches, application rejection, direct database UPDATE/DELETE denial, and recovery
after an already-cancelled request. The older `READONLY_DB_MCP_SQLSERVER_DSN` test
remains a separate limited identity/SHOWPLAN/DDL probe.

## Still required before full release acceptance

- SQL Server 2019/2022/2025 real engine and supported compatibility combinations;
  ScriptDom grammar compatibility is not engine qualification.
- Effective-permission/metadata completeness across supported editions and
  collations; separate attestor; hidden/encrypted/signed/CLR/external dependencies;
  live permission and definition drift. Case-folding collisions currently fail
  closed; complete collation-specific binding evidence is still required.
- Mid-execution cancellation, network loss, SHOWPLAN cleanup failure and pool
  recovery against the real driver/server; concurrent DDL and permission drift;
  long-running mixed-engine fairness and helper memory/latency measurements.
- Redis production TLS/ACL/topology combinations, replica routing, unsafe
  redirects and topology changes under concurrent load; module certification for
  each exact production build. The local Sentinel/Cluster runs cover primary
  routing and promotion, not every deployment matrix entry.
- Release packaging and helper tests for platforms other than Linux x64.

The RFCs remain the wider acceptance specification. This change closes concrete
implementation gaps and adds executable gates; it does not declare every RFC
acceptance item complete.

Catalog class identifiers and required catalog grants follow the
[Microsoft dependency catalog documentation](https://learn.microsoft.com/en-us/sql/relational-databases/system-catalog-views/sys-sql-expression-dependencies-transact-sql?view=sql-server-ver17).
