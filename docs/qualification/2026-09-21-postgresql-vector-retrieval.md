# PostgreSQL dense retrieval — RFC-0007 P2 local evidence

The opt-in runtime path is exercised through production configuration, registry,
MCP client/server, PostgreSQL role attestation and query execution. This extends
the [P1 proof evidence](2026-09-21-postgresql-vector-proof.md); it does not complete
RFC-0007 P3 release qualification.

## Fixture identity

PostgreSQL 16.2 / pgvector 0.8.2 on Linux x86-64, using the same source-built
extension and helper artifacts recorded in P1. The pgvector source commit is
`cab9da72c04353f143bb06b42ab70a403daac64a`. These historical fixture bytes are not
a production version recommendation or evidence for every PostgreSQL 16 patch.

Each test starts a new cluster with a private Unix socket and no TCP listener.
The fixture owner provisions the extension, helper, vectors and indexes. MCP has
only the separate reader identity. Tests use a one-connection target pool and
ephemeral configuration with operator-asserted artifact hashes. Native reader
calls and the fixture owner use separate connections.

## Covered assertions

`TestLocalPostgreSQLDenseVectorsThroughMCP` checks 22 combinations:

- `vector` and `halfvec`.
- L2, negative inner product, cosine distance and L1.
- Exact retrieval and HNSW for all four metrics; IVFFlat for L2/IP/cosine.

Each checks IDs, typed text parameters, JSON array decoding and distances against
the native reader and independent arithmetic with the fixture's quantized
components. ANN cases also call `query_explain` and verify the actual index name.
The test deployment sets `enable_seqscan=off` for its reader to make index usage
deterministic. Its four three-dimensional vectors, small HNSW graph and one-list IVFFlat index prove
functional integration, not realistic planner choices, recall or scale.

Additional assertions cover:

- Filtered iterative HNSW and IVFFlat retrieval (the latter with two lists),
  CTE/join/window, LATERAL, vector aggregation and explicit vector functions.
- Single-query, batch and explain admission through the same proof path.
- Batch-wide tuning and restoration on a reused connection; unknown tuning
  fields, native range violations and excessive scan memory allowance fail.
- Invalid dimensions/non-finite vector components; native NaN distance encoding.
- MCP mutation rejection and separate native reader denial for insert, update,
  delete, table drop and index creation. No owner credentials reach MCP.
- Helper security-definer drift and an added PUBLIC-executable routine rejected
  by the next request, without waiting for background attestation.
- Extended-statistics expressions with an unreviewed routine rejected even
  after its direct EXECUTE privilege has been revoked.
- Bounded request timeout and successful subsequent queries with restored
  defaults. This does not establish that every cancellation occurred after
  backend execution began, nor measure backend termination latency or RSS.

## Reproduction

Build the helper and its separate test fixture using the
[helper instructions](../../tools/pgvector-proof/README.md). Set absolute paths:

```sh
export READONLY_DB_MCP_PG_BIN=/absolute/path/to/postgresql16/bin
export READONLY_DB_MCP_PGVECTOR_PROOF="$PWD/bin/pgvector-proof.so"
make test-postgresql-vectors
```

pgvector 0.8.2 must already be installed in that PostgreSQL prefix. The Make
target requires both path variables; direct Go tests skip when they are absent.
The fixture hashes `<prefix>/lib/postgresql/vector.so` and the helper for its
deployment assertion. The Unix socket needs local socket permissions; no
external service or existing database is contacted.

Observed 2026-09-21: `go test -race ./... -count=1` passed with the PostgreSQL
fixture variables set (MCP package: 61.192 seconds; proof package: 16.683 seconds).
After extending dependency coverage to extended-statistics expressions, the
affected PostgreSQL/MCP race suites passed again (60.151 seconds for MCP and
16.169 seconds for the proof package). All 22 dense combinations and the two
filtered iterative cases passed. Final `go vet ./...`, `make build` and whitespace
checks passed. Other environment-gated native engine fixtures were not enabled
in this run; their skips are not new qualification evidence.

## Remaining release gates

The subsequent [P3 local record](2026-09-21-postgresql-vector-resources.md) adds
synthetic recall, filtered/reranked ANN, observed native interruption, backend
loss and sampled pool-ceiling acceptance. The limitations below describe the
original P2 run; production release gates remain open as detailed in P3.

Qualify current production server/library builds, TLS and deployment topology;
larger ANN corpora and recall criteria; iterative scan/reranking combinations at
scale; native mid-execution cancellation and cleanup timing; long-running reads,
fairness, physical connection limits, backend/process memory and failure recovery.
The scan allowance check is not a total memory bound. Operator assertions do not
cryptographically prove which library a remote server loaded. Runtime admission
still trusts administrators, catalog integrity, installed server hooks and the
reviewed native binaries across the planning/execution interval.
