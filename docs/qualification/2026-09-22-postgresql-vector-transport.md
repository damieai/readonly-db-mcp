# PostgreSQL vectors — TLS, network interruption and long reads

This P3 record extends the [local recall/recovery acceptance](2026-09-21-postgresql-vector-resources.md).
It qualifies a local source build and loopback transport. It does not qualify a
production deployment, proxy fleet, replica failover or sustained capacity.

## Build identity

PostgreSQL 16.15 was built from the [official source archive](https://ftp.postgresql.org/pub/source/v16.15/)
with its published SHA-256 checked before extraction. Configuration was
`--with-ssl=openssl --without-readline --without-icu --without-zlib`, with an
isolated temporary installation prefix. SSL build flags follow the
[PostgreSQL 16 build documentation](https://www.postgresql.org/docs/16/install-make.html).
The linked library is OpenSSL 3.5.5. pgvector 0.8.2 uses the previously reviewed
source commit `cab9da72c04353f143bb06b42ab70a403daac64a`, rebuilt against this prefix.
The native helper and its separate hostile test fixture were rebuilt too.

| Artifact | SHA-256 |
| --- | --- |
| PostgreSQL 16.15 source tar.bz2 | `c1575341fa7bd40f5274ea465b34390f4dc64cdd0770af327005caaeb9f6b7ed` |
| PostgreSQL server executable | `c9f4acf25287253b389d083a0a4b442bf21f619f8e95247a3cb320c855a344a6` |
| pgvector shared library | `09796ad236e97b6fced1b42e388f053d798f8835205702a5a15ecf34a20e126c` |
| Native proof helper | `1126c701edfb2d53920a487eeb82bda4c7f4055265cf58bd2220b20b8e3b973d` |
| Separate test-only callback fixture | `03059784bd1088a1e5e57156b52dc4961f8886c98c5d3459831a41c062f274a7` |

These are local artifact identities, not remotely authenticated library hashes.
Production still requires the operator assertions and catalog/privilege proofs
in the [setup guide](../POSTGRESQL-VECTOR-RETRIEVAL.md).

## TLS and interrupted transport

`TestLocalPostgreSQLVectorTLS` starts a fresh cluster with a loopback TCP listener.
Its private Unix socket is reserved for fixture provisioning and observation.
Disposable ECDSA CA/server keys are generated in the private test directory;
neither keys nor CA trust are installed on the host. The reader uses SCRAM over
`hostssl`; plaintext TCP is rejected. MCP uses `verify-full`, the fixture CA and
an explicit server name. The native `pg_stat_ssl` view confirms TLS 1.3 on both
the initial backend and replacement backends.

Acceptance includes wrong CA, wrong server name, wrong password and plaintext
denial; typed vector retrieval; and two failures during an observed expensive
vector query. An opaque TCP relay preserves end-to-end TLS while it either
closes established connections or discards traffic in both directions, including
new cancellation sockets. Each failed request must return within 6.5 seconds of
submission under its 5-second tool deadline, without transparently retrying the
failed query. A subsequent request must reconnect, negotiate TLS and return
correct vector data with default tuning on a different backend PID.

In the bidirectional drop scenario, the relay deliberately keeps the upstream
socket open even after the client gives up. The independent server statement
timeout must stop native computation. The test observes the isolated backend
in `idle in transaction (aborted)` before restoring connectivity. Restoration
closes those stranded sockets, after which the old backend must disappear and
the next request must succeed. This explicitly distinguishes stopping work from
cleaning up an unreachable session: client cancellation cannot prove immediate
remote transaction cleanup across a network partition. The relay is not a
kernel-level packet-loss, TCP retransmission or network-device simulation.

## Multi-minute read and concurrent maintenance

`TestLocalPostgreSQLVectorLongRead` uses native TLS and a two-connection target.
A recursive SQL query repeatedly calculates vector distances and small sums
until a 125-second wall-clock bound. Native execution must be observed for at
least 120 seconds, then return a complete scalar result checked against an
independent float32-aware distance calculation. It uses the normal 5-minute
request default, without increasing query deadlines or weakening SQL policy.

During that read, fresh metadata requests and short vector queries each have
five-second client deadlines. Background identity/privilege checks run every
ten seconds using the same admission and connection budgets. At least ten
metadata and ten interactive requests must succeed; sampled physical reader
connections must not exceed two. Successful requests beyond twenty seconds
also require refreshed privilege evidence rather than stale startup evidence.

This test exposed an existing PostgreSQL scheduling defect: reads held a target
read lock throughout execution, while a scheduled privilege check waited for
the write lock. The waiting writer then blocked subsequent readers independently
of their request deadlines. The pre-fix native test failed at approximately
15 seconds with a metadata deadline exceeded.

The fix removes that target-wide execution lock. Rechecks use the existing pool
and admission controller, publish immutable policies through the existing atomic
pointer, and update atomic health/freshness. Failed or expired checks still deny
new requests. Existing reads are allowed to finish, and every vector request
still performs its transaction-specific catalog/privilege/dependency checks
before native binding/execution. No extra pool, permission exception, SQL
restriction or shorter query deadline was introduced. A separate unit regression
proves a failed recheck is published while a read is still running and denies
new queries immediately.

This is one bounded mixed workload, not sustained multi-target fairness or a
production memory benchmark. It does not establish success at the 10/15-minute
configuration ceilings or cancellation inside every ANN phase.

## Reproduction

Use a PostgreSQL 16 build with OpenSSL support, pgvector 0.8.2 and the native
helper built against its headers. The fixture locates `vector.so` through that
prefix's `pg_config --pkglibdir`, rather than assuming a library-directory layout.

```sh
export READONLY_DB_MCP_PG_BIN=/absolute/path/to/postgresql16/bin
export READONLY_DB_MCP_PGVECTOR_PROOF=/absolute/path/to/pgvector-proof.so
make test-postgresql-vector-transport
make test-postgresql-vector-long-read
```

The long-read target takes over two minutes and uses one backend for repeated
vector arithmetic. Both targets require the artifact variables. Direct Go tests
also require `READONLY_DB_MCP_PG_TLS=1` or `READONLY_DB_MCP_PG_LONG_READ=1` respectively;
otherwise these optional acceptance suites skip. The original functional/recall
suites still run against installations without TLS support.

For the binding/dependency suite, also build the separate test-only library next
to the selected helper (it is never provisioned in the MCP retrieval fixture):

```sh
PG_CONFIG="$READONLY_DB_MCP_PG_BIN/pg_config" sh tools/pgvector-proof/build.sh \
  "$(dirname "$READONLY_DB_MCP_PGVECTOR_PROOF")/pgvector-test-fixture.so" \
  tools/pgvector-proof/test-fixture.c
```

Run the CPU-bound native acceptance suites without overlapping separate copies.
To include all optional PostgreSQL tests in a repository race run, set both
`READONLY_DB_MCP_PG_TLS=1` and `READONLY_DB_MCP_PG_LONG_READ=1` alongside the two
artifact paths before `go test -race ./... -count=1 -v`.

## Observed validation

On 2026-09-22 the final repository race run passed with all four fixture variables
set: MCP package 317.077 seconds; PostgreSQL binding/dependency package 17.186
seconds. The original 22 functional retrieval combinations, 36 recall cases,
native recovery and hostile dependency/callback tests passed on this build.
All 36 synthetic recall cases measured recall@10 of 1.0 for the declared corpus
and tuning; this is not a production recall guarantee.

| Measurement | Observed result |
| --- | --- |
| TLS authentication/transport suite | 17.90 s, all six subcases passed |
| Broken-connection request | Error returned in 2.948 s, subsequent verified TLS query succeeded |
| Bidirectional-drop request | Error returned in 5.003 s; server timeout stopped work; restoration removed the old backend |
| Long-read request | Complete result in 125.006 s; native execution observed for 122.832 s |
| Vector aggregation iterations | 85,784, with the returned aggregate matching independent arithmetic |
| Concurrent successful metadata / vector requests | 31 / 31 |
| Sampled reader backend peak | 2, matching the configured ceiling |

`go vet ./...`, `make build`, document-link and whitespace checks passed. Other
engines' environment-gated native fixtures were not enabled; skipped suites are
not new qualification evidence. Times and connection samples describe this local
run, not a production latency or capacity guarantee.

## Remaining P3 gates

Production build and proxy/replica topology; certificate rotation and mutual TLS;
realistic high-dimensional data and additional metric recall; sustained mixed
workloads and larger pools; backend/MCP peak RSS and temporary-file ceilings;
kernel-level network faults and failover; and ANN-phase-specific cancellation
remain open. The trusted-administrator, catalog-integrity and operator-asserted
binary identity boundaries remain unchanged.
