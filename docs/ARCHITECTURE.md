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
dialect-specific AST policy
        |
        v
global + target concurrency gates
        |
        v
READ ONLY transaction
        |
        v
dedicated SELECT-only database account
```

## Packages

- `cmd/readonly-db-mcp`: flags, signal handling and process lifecycle.
- `internal/mcpserver`: MCP input/output contracts; no SQL implementation.
- `internal/registry`: exact-alias target lookup and lifecycle ownership.
- `internal/core`: engine-neutral target and result contracts.
- `internal/dialects/mysql`: MySQL connection, grant attestation, AST policy,
  metadata reads and bounded result collection.
- `internal/config`: strict YAML decoding, hard ceilings and secret resolution.
- `internal/audit`: structured, non-content audit events.

## Adding a database engine

A new engine gets its own `internal/dialects/<engine>` package. It must implement
`core.Target` and, when possible, `core.BatchTarget`. It must not reuse the MySQL
parser or MySQL privilege assumptions.

An engine is considered complete only when it has:

1. A maintained dialect parser and fail-closed statement policy.
2. Effective privilege attestation for the connected identity.
3. A database-native read-only transaction.
4. Fixed metadata queries for schemas, tables, columns and indexes.
5. Cancellation and resource-limit integration tests.
6. A hostile SQL corpus demonstrating that public tools cannot mutate a
   disposable database.

## Why target selection is stateless

Every tool call contains `target`. A mutable `USE DATABASE` tool would make
concurrent requests and multiple clients capable of changing each other's
destination. Exact per-call target selection also keeps audit records complete
and makes a response self-describing.

## Why arbitrary DSNs are forbidden

Allowing a model to supply a host or DSN would turn the MCP server into an SSRF
and credential-routing primitive. Network destinations and credentials are
operator configuration, never tool input.
