# PostgreSQL native binding helper

RFC-0007 P1 uses the PostgreSQL 16 analyzer and rewrite engine to resolve actual
function, operator, type and relation OIDs. This helper produces binding data;
it does not execute or plan the supplied query. The Go `vectorproof` package
compares the extension catalog against the source-pinned profile, checks the
analysis surface before invoking the analyzer, and follows bound dependencies.
The opt-in runtime path uses these components before planning and execution;
see the [operator guide](../../docs/POSTGRESQL-VECTOR-RETRIEVAL.md).

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

The test target also builds `bin/pgvector-test-fixture.so`. Its deliberately
failing callbacks count invocations in the test backend, even across rollback.
This library is for disposable fixtures only. Direct Go runs with native paths
set require this library beside `pgvector-proof.so`; build it with
`make build-pgvector-test-fixture` first.

## Profile and binding contract

The embedded profile contains 327 descriptors for PostgreSQL 16 / pgvector
0.8.2, including extension membership, function bodies/attributes, aggregate
support, type I/O, operators, casts, access methods and index operator families.
Local OIDs are discovered each time. Namespace relocation is normalized and
tested; this profile currently accepts dedicated lower-case installation names.
Native query expressions retain PostgreSQL's syntax.

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

## Analysis dependency checks

`Snapshot.Analyze` refreshes the extension comparison, inventories visible types
and callable argument/result types, then checks readable target relations and
their dependencies before native analysis. It includes types even when USAGE or
function EXECUTE was revoked: those privileges alone do not block every coercion
callback. The check follows type I/O, typmod/subscript handlers, arrays,
composites, domain constraints, range callbacks, TABLESAMPLE handlers, view/RLS
expressions, extended statistics, indexes and operator families, partition children/keys and
collations. After binding, it checks the actual expression references too.

The `expression(text)` helper inspects serialized `pg_node_tree` values obtained
directly from catalogs, without analyzing or evaluating them. Never accept these
values from clients. This catches references to pinned builtins which are absent
from [pg_depend](https://www.postgresql.org/docs/16/catalog-pg-depend.html), such
as `set_config` hidden in a policy. Materialized views are checked as stored
relations; their defining query is not executed by a read.

Preflight covers the target role's analysis surface, not just explicit type
names in one query. An unreviewed callback in an accessible type or readable
target relation prevents analysis until that capability is reviewed or removed
from the role's scope. It does not replace SQL with a restricted KNN grammar.
Dependencies allow 8,192 objects and 32,768 edges; stored expressions allow 1,024
trees, 1 MiB each and 4 MiB total. Traversal is iterative with cycle detection.
The inspector and binder share the caller's transaction and request context;
successful analysis restores `search_path=pg_catalog`. Roll back after a database
error or cancellation. These checks do not cache proof across requests.

## Runtime trust and remaining qualification

Native analysis can call type input and typmod routines before it returns a
tree. Do not grant this helper to an untrusted query caller or use its output
as the sole safety decision. The Go preflight now checks those callbacks and
has native evidence of rejection before callback entry. Runtime integration
checks helper definitions, effective role/schema authority and dependencies on
the execution transaction; it requires operator-asserted deployment identity.
Foreign/native extensions and
unreviewed routines need their own capabilities; a volatility label is not one.
The dependency corpus is not a claim of complete execution-path qualification.

Catalog metadata does not prove which shared-library bytes a remote server
loaded. Deployment identity and concurrent administrative DDL remain trusted
operator boundaries. The default-off P2 profile now enables exact extension and
helper EXECUTE exceptions, codecs, retrieval and transaction-local tuning.
[Local MCP qualification](../../docs/qualification/2026-09-21-postgresql-vector-retrieval.md)
covers the initial dense matrix; P3 release/scale/resource gates remain open.

See the [native evidence record](../../docs/qualification/2026-09-21-postgresql-vector-proof.md).
The pgvector license accompanying the source-derived catalog descriptors is
[LICENSE.pgvector](LICENSE.pgvector).
