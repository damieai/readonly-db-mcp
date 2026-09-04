# Architecture

```text
MCP client over stdio
        |
        v
typed MCP tool schemas
        |
        v
exact target registry lookup
        |
        v
engine-specific SQL policy or Redis command policy
        |
        v
bounded fair admission by workload class
        |
        +----> target-local metadata/result caches
        |
        v
native read-only transaction or SQL Server SHOWPLAN proof / attested Redis command
        |
        v
dedicated SELECT-only SQL identity / read-key-only Redis ACL
```

## Packages

- `cmd/readonly-db-mcp`: flags, signal handling and process lifecycle.
- `internal/mcpserver`: MCP input/output contracts; no SQL implementation.
- `internal/registry`: exact-alias target lookup and lifecycle ownership.
- `internal/core`: engine-neutral target and result contracts.
- `internal/dialects/mysql`: MySQL connection, grant attestation, AST policy,
  metadata reads and bounded result collection.
- `internal/dialects/postgresql`: PostgreSQL connection, role/object privilege
  attestation, native AST policy and metadata reads.
- `internal/dialects/sqlserver`: SQL Server connection, effective-permission
  attestation, T-SQL safety scan, mandatory `SHOWPLAN_XML` proof, snapshot
  batches, and catalog metadata reads.
- `internal/dialects/redis`: Redis ACL and live command-catalog attestation,
  key-scope policy, RESP normalization and bounded command execution.
- `internal/config`: strict YAML decoding, hard ceilings and secret resolution.
- `internal/audit`: structured, non-content audit events.
- `internal/admission`: global/per-target bounded queues, workload fairness and
  cancel-safe permits.
- `internal/metrics`: content-free counters, phase durations and periodic stderr
  summaries.

Each MySQL target owns immutable normalized policy indexes and bounded caches.
Metadata, interactive, batch and maintenance work share one admission controller;
no runtime database operation bypasses it. Final query and batch structures are
JSON-sized before return, and batch execution stops when another minimal result
cannot fit the remaining response budget.

## Adding a database engine

A new engine gets its own `internal/dialects/<engine>` package. It implements the
minimal `core.Target` plus `core.SQLTarget` or `core.RedisTarget`, and an
appropriate batch capability. It must not reuse another engine's parser or
privilege assumptions.

An engine is considered complete only when it has:

1. A maintained dialect parser or live command capability catalog and a
   fail-closed side-effect policy.
2. Effective privilege attestation for the connected identity.
3. A database-native read-only transaction or read-only ACL/key enforcement.
4. Safe engine-specific metadata/introspection paths.
5. Cancellation and resource-limit integration tests.
6. A hostile query/command corpus demonstrating that public tools cannot mutate
   a disposable database.

## Why target selection is stateless

Every tool call contains `target`. A mutable `USE DATABASE` tool would make
concurrent requests and multiple clients capable of changing each other's
destination. Exact per-call target selection also keeps audit records complete
and makes a response self-describing.

## Why arbitrary DSNs are forbidden

Allowing a model to supply a host or DSN would turn the MCP server into an SSRF
and credential-routing primitive. Network destinations and credentials are
operator configuration, never tool input.
