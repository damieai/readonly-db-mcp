# PostgreSQL native binding prototype

RFC-0007 P1 uses the PostgreSQL 16 analyzer and rewrite engine to resolve actual
function, operator, type and relation OIDs. This helper produces binding data;
it does not execute or plan the supplied query. The Go `vectorproof` package
compares the extension catalog against the source-pinned profile and checks
bound expression objects. Neither component is connected to public MCP query
admission yet.

## Build and run the isolated fixture

Use PostgreSQL 16 server headers and binaries from the same installation, with
pgvector 0.8.2 already installed in that installation's library/extension paths.
The pgvector source identity is commit
`cab9da72c04353f143bb06b42ab70a403daac64a`. Run from the repository root:

```sh
export PG_CONFIG=/absolute/path/to/pg_config
make build-pgvector-proof
export READONLY_DB_MCP_PG_BIN=/absolute/path/to/postgresql/bin
export READONLY_DB_MCP_PGVECTOR_PROOF="$PWD/bin/pgvector-proof.so"
make test-pgvector-proof
```

The fixture creates its own cluster and roles in a temporary directory. It uses
a private Unix socket with no TCP listener. Administrator-only catalog mutations
and helper grants belong to this fixture and are rolled back or removed with the
cluster. No existing database connection or administrator credential is needed.
The build command only creates the shared library; it does not install it.
Direct Go tests skip the native fixture when paths are unset; the Make target
requires them. Skips are not native qualification.

## Profile and binding contract

The embedded profile contains 327 descriptors for PostgreSQL 16 / pgvector
0.8.2, including extension membership, function bodies/attributes, aggregate
support, type I/O, operators, casts, access methods and index operator families.
Local OIDs are discovered each time. Namespace relocation is normalized and
tested; the prototype currently accepts simple lower-case installation names.
This prototype restriction is not a proposed limit on native query syntax.

Bindings carry the original SQL digest, database and effective role OIDs,
resolved parameter types and class-qualified object references. Snapshot checks
require the same Go transaction. An exported profile cannot mutate the snapshot,
and a failed catalog comparison clears its verified revision. These are internal
data structures from trusted code, not signed assertions accepted from clients.
Binding identity does not independently authenticate the helper binary or attest
that metadata has stayed unchanged since analysis.

Resource ceilings: 1 MiB SQL and binding JSON, 10,000 parameters, 4,096 catalog
objects / 4 MiB descriptors, 8,192 binding references, 100,000 visited raw/bound
nodes and traversal depth 256. PostgreSQL's stack and interrupt checks also
apply. Native analysis allocations precede some traversal checks; these limits
are not a measured bound on backend memory. The future caller must supply the
remaining request deadline and existing resource budget on the same lease.

## Admission gates still open

Native analysis can call type input and typmod routines before it returns a
tree. Do not grant this prototype to an untrusted query caller or use its output
as the sole safety decision. P1 still needs pre-analysis callback admission,
complete dependency closure (including domains, collations and relation/index
support), hostile callback tests, and proof refresh around execution. The
native binding corpus covers advanced SQL instead of replacing SQL with a KNN
template; more dependency coverage must preserve that contract.

Catalog metadata does not prove which shared-library bytes a remote server
loaded. A reviewed deployment/library identity assertion, helper identity,
role/schema ownership checks and concurrency boundaries are required before
P2 enables extension EXECUTE exceptions. The current production startup and
query policies retain their existing behavior. P2 codecs, retrieval execution
and transaction-local tuning, and P3 release qualification remain pending.

See the [native evidence record](../../docs/qualification/2026-09-21-postgresql-vector-proof.md).
The pgvector license accompanying the source-derived catalog descriptors is
[LICENSE.pgvector](LICENSE.pgvector).
