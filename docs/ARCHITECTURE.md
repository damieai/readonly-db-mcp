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

## Agent workload latency budget

This server is an infrastructure tool for agents, so its defaults must tolerate
multi-step investigations and genuinely slow analytical reads without making
short deadlines an accidental availability boundary. Agent reasoning between
tool calls does not hold an admission permit, transaction, or database
connection. Only an active MCP tool call may lease those resources.

The common operational profile is a cross-engine compatibility contract:

| Control | Baseline |
| --- | ---: |
| Default MCP query or batch deadline | 5 minutes |
| Configured maximum request deadline | 10 minutes |
| Compiled request hard ceiling | 15 minutes |
| Default admission queue wait | 1 minute |
| Compiled queue-wait hard ceiling | 5 minutes |
| Global / per-target concurrency | 16 / 8 |
| Batch / maintenance concurrency | 4 / 2 |
| Open / idle connections per target | 8 / 4 |
| Connect / write timeout | 15 seconds |
| Read timeout | maximum request deadline plus 1 minute |
| Connection lifetime / idle time | 30 / 10 minutes |

`internal/config` and `configs/example.yaml` are the executable sources of truth
for these values. Tests must keep them aligned. An engine RFC inherits this
profile unless it documents a measured infrastructure constraint and retains an
operator-configurable path to the common request budget.

A request deadline starts before admission and covers queueing, policy or plan
preflight, transaction setup, execution, result collection, and cleanup. Batch
deadlines cover the entire batch unless the public contract explicitly says
otherwise. Driver socket read deadlines must exceed the maximum request
deadline. Shorter connect, write, lock, discovery, or cleanup deadlines are
permitted when they fail one phase quickly rather than reducing the execution
budget.

New engines and driver upgrades must not introduce an implicit timeout below
the remaining request deadline. Pool settings must map to actual driver hard
ceilings, not merely preferred or minimum pool sizes. Cancellation tests must
show that server work stops and the connection or permit becomes reusable.

Tightening the common profile is a compatibility and availability change. It
requires an explicit RFC, benchmark or production evidence, migration notes,
and tests for long-running requests, saturation, cancellation, and pool
recovery. Operators may always choose stricter values for a particular target.

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
7. A latency and pool budget conforming to the common agent workload profile.

## Why target selection is stateless

Every tool call contains `target`. A mutable `USE DATABASE` tool would make
concurrent requests and multiple clients capable of changing each other's
destination. Exact per-call target selection also keeps audit records complete
and makes a response self-describing.

## Why arbitrary DSNs are forbidden

Allowing a model to supply a host or DSN would turn the MCP server into an SSRF
and credential-routing primitive. Network destinations and credentials are
operator configuration, never tool input.
