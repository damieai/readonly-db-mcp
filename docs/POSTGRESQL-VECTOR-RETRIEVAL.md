# PostgreSQL dense vector retrieval

An opt-in `pg16-vector0.8.2` profile now enables native pgvector queries through
`query_select`, `query_batch` and `query_explain`. It supports text vector
parameters with explicit casts, decoded `vector`/`halfvec` results, and optional
transaction-local search controls. The profile is default-off.

The [local qualification record](qualification/2026-09-21-postgresql-vector-retrieval.md)
states the exact tested builds and limits. Installing pgvector alone does not
enable this path. Database operators provision the extension, indexes and native
analysis helper; MCP does not install them or load embeddings.

## Provision the deployment

Use a trusted deployment owner, different from the runtime reader, to install
pgvector 0.8.2 in a dedicated schema such as `vectors`. Build the native helper
against the deployed PostgreSQL 16 headers with
[these instructions](../tools/pgvector-proof/README.md), and put the library at a
stable absolute path on the database server. Do not install the test fixture
library. The extension, its routines/types/operators and both schemas must be
owned by the configured deployment owner. Schema CREATE cannot be delegated to
other ordinary roles.

As that trusted deployment owner, provision the two helper functions:

```sql
CREATE SCHEMA mcp_proof;
CREATE FUNCTION mcp_proof.bind(text) RETURNS text
  AS '/opt/readonly-db-mcp/pgvector-proof.so', 'mcp_vector_bind'
  LANGUAGE C STRICT VOLATILE PARALLEL UNSAFE;
CREATE FUNCTION mcp_proof.expression(text) RETURNS text
  AS '/opt/readonly-db-mcp/pgvector-proof.so', 'mcp_vector_expression'
  LANGUAGE C STRICT VOLATILE PARALLEL UNSAFE;
REVOKE ALL ON FUNCTION mcp_proof.bind(text), mcp_proof.expression(text) FROM PUBLIC;
GRANT USAGE ON SCHEMA vectors, mcp_proof TO vector_reader;
-- The dedicated vectors schema contains the reviewed pgvector installation.
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA vectors TO vector_reader;
GRANT EXECUTE ON FUNCTION mcp_proof.bind(text), mcp_proof.expression(text)
  TO vector_reader;
```

The runtime role must satisfy the existing PostgreSQL read-only role contract:
no privileged attributes, memberships, object ownership, schema/database CREATE,
TEMP, relation mutation or cross-database CONNECT. Grant SELECT only within the
configured reporting schemas. Effective PUBLIC function grants are checked too;
only exact reviewed pgvector routines and these two verified helpers receive
an exception. The schema-wide provisioning grant above is appropriate only for
the dedicated reviewed installation; unexpected executable routines cause
attestation to fail. Adding another function to either schema does not authorize it.

Record the hashes of the deployed `vector.so` and helper library through the
operator's deployment process, then add this to the PostgreSQL target:

```yaml
postgresql:
  pgvector:
    profile: pg16-vector0.8.2
    schema: vectors
    owner: trusted_deployment_owner
    helper_schema: mcp_proof
    helper_library: /opt/readonly-db-mcp/pgvector-proof.so
    deployment_id: reporting-vector-release-1
    vector_library_sha256: <64 hexadecimal characters>
    helper_library_sha256: <64 hexadecimal characters>
    max_scan_memory_bytes: 67108864
```

Library hashes are **operator assertions**, not hashes read from the remote
server by MCP. Catalogs verify definitions, signatures, ownership and membership;
they cannot authenticate loaded library bytes. The assertions are included in
the target policy revision, and capabilities report `library_identity` as
`operator_asserted`. Use authenticated transport and a trusted database build,
deployment process, administrators and installed server hooks. These local tests
do not certify a production deployment.

## Query and result contract

Example `query_select` input:

```json
{
  "target": "vectors",
  "sql": "SELECT id, embedding, embedding <-> $1::vectors.vector AS distance FROM reporting.items WHERE category=$2 ORDER BY embedding <-> $1::vectors.vector LIMIT 20",
  "parameters": ["[0.1,0.2,0.3]", "articles"],
  "postgresql_options": {
    "hnsw_ef_search": 100,
    "hnsw_iterative_scan": "strict_order"
  }
}
```

Supply a string such as `"[0.1,0.2,0.3]"`; a JSON array parameter is not a vector
binding in this contract. Dimensions and finite component values follow native
pgvector validation. Invalid input is rejected without truncating it. Results
of type `vector` or `halfvec` are JSON numeric arrays, with database type metadata
`VECTOR` or `HALFVEC`. An over-budget vector cell becomes
`{"type":"vector","truncated":true}` (or `halfvec`) and increments
`truncated_cells`; a partial vector is never presented as a complete value.

Distance values retain pgvector's units: `<->` is L2, `<#>` is negative inner
product, `<=>` is cosine distance and `<+>` is L1. An undefined native score, such
as cosine distance to a zero vector, is represented as
`{"type":"float64","value":"NaN"}`. Infinity uses `+Inf` or `-Inf` in the
same envelope. These tagged values are not ordinary sortable JSON numbers.

The transaction search path is `pg_catalog` followed by the verified extension
schema. User relations still require schema qualification. Explicit
`OPERATOR(vectors.<->)` works too. CTEs, joins, LATERAL, windows, aggregates,
filters, native casts and reranking remain native SQL; admission resolves their
actual executable dependencies. `query_explain` plans the same checked SELECT
with `ANALYZE FALSE`. The synchronous internal `ValidateQuery` method performs
syntax checks only and is not an execution authorization.

## Request-local controls

| `postgresql_options` field | pgvector 0.8.2 accepted values |
| --- | --- |
| `hnsw_ef_search` | 1–1000 |
| `hnsw_iterative_scan` | `off`, `strict_order`, `relaxed_order` |
| `hnsw_max_scan_tuples` | 1–2147483647 |
| `hnsw_scan_mem_multiplier` | finite number, 1–1000, subject to memory allowance |
| `ivfflat_probes` | 1–32768 |
| `ivfflat_max_probes` | 1–32768 |
| `ivfflat_iterative_scan` | `off`, `relaxed_order` |

Native index/list behavior still applies: IVFFlat does not provide an L1 index,
and probes interact with the index's list count. These options do not guarantee
ANN recall. See [pgvector's query controls](https://github.com/pgvector/pgvector/tree/v0.8.2#query-options).

For `query_batch`, put one options object on the batch; every query shares that
transaction scope and the original batch deadline. Options also apply to
`query_explain`. Arbitrary GUC names, raw SET and `set_config` in user SQL are
not admitted. Other engines and PostgreSQL targets without this profile reject
the options object.

The configured scan memory allowance defaults to 64 MiB and can be set between
1 MiB and 1 GiB. The effective server `work_mem` multiplied by the requested or
default HNSW scan multiplier must fit it. This bounds that iterative-scan
allowance, not total backend RSS, sorts, joins or aggregate memory. Size the
allowance together with target concurrency and database capacity. No vector
specific query timeout, pool or hidden connection is introduced.

## Freshness and rollback

Each execution rechecks the runtime role, effective grants, helper definitions,
extension catalog and dependencies on the executing read-only transaction.
Native binding runs before planning; unreviewed callbacks are rejected before
analysis. Prepared statement caching is disabled on the vector path. A change to
the dense type OIDs requires reopening the target so connection codecs cannot
silently use stale identities. Administrative concurrent DDL is a trusted
operator boundary, not a race eliminated by these checks.

The transaction is rolled back on success, failure and cancellation, clearing
local search settings before connection reuse. The request deadline includes
admission and proof work; each batch statement gets only the remaining time.
Removing the profile restores the original policy, which requires revoking the
extension/helper EXECUTE privileges that are no longer approved. Data and indexes
are not changed by disabling the profile.
