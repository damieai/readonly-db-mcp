# Redis dense vector qualification — RFC-0007 P0

This record concerns real local standalone processes owned by the test suite.
It is not a production module signature or qualification of PostgreSQL pgvector.

## Artifacts

Redis Stack 7.4.0-v8, Linux x86-64 Jammy archive, containing Redis 7.4.7,
Search 2.10.20 and RedisJSON 2.8.9. Download source:
[the pinned Redis package](https://packages.redis.io/redis-stack/redis-stack-server-7.4.0-v8.jammy.x86_64.tar.gz).

| Artifact | Observed SHA-256 |
| --- | --- |
| Archive | `b333009ea673c8055e56496a95485ff1f4ba504dd2e1e1ee8687a831aef33715` |
| redis-server | `cb55ff7ea8f994364668d388d7cbd39a98e78fd70450f6da021fffe47e46ed84` |
| redisearch.so | `c55c19e785a6f98e3176643d0b01b47dd8b4456f0986beef9ae4a5acea67b064` |
| rejson.so | `642c14444c93088e3fa4fc76f03e64d8ad2325c409a6ecec7d54055ee2f67dfc` |

These hashes identify tested bytes; they do not certify a remotely loaded binary
or recommend this historical build for a new production deployment.

## Covered assertions

`TestLocalRedisDenseVectorsThroughMCP` starts isolated loopback instances with
random credentials, no persistence and ephemeral signed module profiles. It
loads production configuration, opens the registry and uses an in-memory MCP
client/server session. The runtime reader has no administrator credentials.

The matrix has 48 combinations: two protocols (RESP2/RESP3), two storage kinds
(HASH/JSON), two index algorithms (FLAT/HNSW), three metrics (L2/IP/COSINE), and
two dtypes (FLOAT32/FLOAT64). Each combination checks:

- Live `FT.INFO` vector metadata and scoped canonical index identity.
- KNN, tag-filtered KNN and vector range results, with IDs and distances computed
  independently from a deterministic four-vector fixture. HNSW uses EF_RUNTIME.
- The same native reader command against the MCP result, including scores.
- HASH binary vector round trip with zero and non-UTF-8 bytes through MCP.
- Dimension mismatch rejection and subsequent connection recovery.
- MCP rejection and native ACL `NOPERM` for document mutation and index deletion.

Representative cases also check metadata/search batching, invalid or ambiguous
binary encoding, already-cancelled requests and admission recovery. Unit tests
cover hostile vector metadata and non-finite RESP3 JSON encoding. Existing local
Search prefix drift, standalone ACL, Sentinel promotion and Cluster tests are
regression checks, not evidence of distributed vector search.

The four-document graph demonstrates functional HNSW integration, not recall or
latency at scale. No server-side mid-query cancellation claim follows from an
already-cancelled client request.

## Reproduction

Supply absolute paths to operator-downloaded matching artifacts:

```sh
export READONLY_DB_MCP_REDIS_SERVER=/absolute/path/to/redis-server
export READONLY_DB_MCP_REDIS_SEARCH_MODULE=/absolute/path/to/redisearch.so
export READONLY_DB_MCP_REDIS_JSON_MODULE=/absolute/path/to/rejson.so
make test-redis-vectors
```

The make target requires all three artifacts so its full-matrix acceptance cannot
silently skip JSON. Direct Go runs skip native tests without the server/Search
variables and skip JSON subcases without the JSON module; skips are not passes.

Repository validation uses `go test -race ./...`, `go vet ./...`, `make build`
and `git diff --check`. The native suite requires permission to bind loopback
ports. Temporary processes and files are cleaned up by the fixture.

Observed 2026-09-21: all 48 native combinations passed with all three artifact
variables set. The final repository race run passed, including the MCP package
in 3.637 seconds and Redis package in 12.203 seconds; vet, binary build and diff
whitespace checks also passed. Other environment-gated engine tests may skip;
these results do not promote their pending acceptance gates.

## Changes found by native testing

- Configuration previously required RESP3 despite existing RESP2 decoding. Both
  protocols are now explicitly configurable; RESP3 remains the default.
- RESP3 `FT.INFO` can contain NaN ratios. Encoding those directly as JSON failed
  the entire call. Non-finite doubles now have explicit tagged values.
- `FT.INFO` now exposes a bounded vector summary and rechecks returned scope.

## Remaining RFC-0007 work

P1/P2: PostgreSQL extension identity, exact expression binding, approved function
privileges, codecs and transaction-local tuning remain unimplemented. P3: qualify
production builds/TLS/topologies, larger ANN corpora and recall thresholds,
mid-execution native timeout/cancellation, fairness and backend/RSS ceilings.
Redis 8 integrated modules and newer vector algorithms/types require their own
build evidence. Advanced native reads are not banned just because those release
matrix entries remain unqualified.
