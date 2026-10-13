# Neo4j read-only connector

The Neo4j target exposes `neo4j_read` for native Cypher and `neo4j_metadata`
for indexes, constraints, labels and relationship types. The disposable
Enterprise 2026.09.0 gate [passed locally on 2026-10-13](NEO4J-ACCEPTANCE-2026-10-13.md).
Production TLS and sustained-read qualification remain open. The pinned server
patch must be Neo4j Enterprise 2026.03 or newer; the example uses 2026.09.0.
See [RFC-0008](RFC-0008-neo4j-readonly-graphrag.md) for the security model.

## Provisioning contract

Use exactly one configured graph, a dedicated native query user, and a distinct
native attestor user. The query user's only positive grants may be `ACCESS` on
that database, graph `MATCH`/`READ`/`TRAVERSE` there, and `SHOW INDEX` or
`SHOW CONSTRAINT` there. It must not have write, database-management, `LOAD`,
procedure-execution, boosted-execution, wildcard-graph or other-database
grants. This includes grants inherited from `PUBLIC` and other roles. Neo4j's
default `PUBLIC` role grants home-database access, data loading and
procedure/function execution; on a disposable instance, revoke every inherited
grant that fails the connector's startup proof. The connector does not alter
privileges.

Give the attestor only `SHOW USER`, `SHOW PRIVILEGE`, `SHOW AUTH RULE`, and
`SHOW ROLE` DBMS inspection privileges, with access needed to run these fixed
commands against `system`. `SHOW ROLE` matters: without it, Neo4j may return
`null` for the `roles` column of `SHOW AUTH RULES`; the connector fails closed.
An enabled auth rule with assigned roles blocks this profile, even if its
current condition excludes the query user. This protects against dynamic
role changes between checks. The two accounts use separate secrets.

The query tool accepts a Cypher string and named parameters, but no host,
database, username or URI. The connector executes in a bounded transaction
on that graph. The server grants are the write boundary; Bolt read mode is
only a routing hint. No keyword allowlist is applied to Cypher, so traversal,
subqueries, aggregation, native vector `SEARCH` and native full-text `SEARCH`
remain available when the pinned server supports them. Vector `SEARCH` starts
in Neo4j 2026.01; full-text `SEARCH` requires 2026.09 and Cypher 25. Earlier
supported patches can still use graph reads and vector `SEARCH`. Only query text,
parameters, returned rows and time are bounded. Procedure-based full-text
access is outside the current authority profile.

MCP JSON decoders can lose precision for integer tokens above 2^53−1. Pass
such values as decimal strings and call Cypher `toInteger($id)` when needed;
the connector rejects integer-valued floating parameters above that boundary.

Start from [the configuration example](../configs/neo4j.example.yaml). Pin
`neo4j.version` to the exact `GetServerInfo` agent patch and set both password
environment variables (or POSIX-protected secret files). Use `bolt+s` with
verified TLS for production; local loopback can use plaintext for a disposable
fixture. The startup proof fails if the server version, user directory,
privilege rows or auth-rule catalog cannot be read in full. The query connection
also runs `SHOW CURRENT USER` and must report the same native user and roles
as the attestor sees, with an active account and no pending password change.
It rechecks before new calls when `privilege_recheck_interval` expires.

## Reproducible Enterprise gate

For a local disposable server, use the pinned official image after reviewing
[Neo4j's Enterprise license requirement](https://neo4j.com/docs/operations-manual/current/docker/introduction/).
Set `NEO4J_AUTH` to `neo4j/<admin-password>` and explicitly set
`NEO4J_ACCEPT_LICENSE_AGREEMENT=yes` if you accept that agreement. Then run:

```bash
docker run --rm --detach --name neo4j-mcp-disposable \
  --publish 127.0.0.1:7687:7687 \
  --env NEO4J_AUTH --env NEO4J_ACCEPT_LICENSE_AGREEMENT \
  neo4j:2026.09.0-enterprise
```

Provision the isolated Enterprise fixture. Create the native users
`agent_graph_ro` and `agent_graph_attestor` with separate passwords, then run
[the system provisioning script](../examples/neo4j/provision.cypher) as an
administrator against `system`. This script creates the `knowledge` and
`private` databases, grants only the fixture roles, and revokes all four
default `PUBLIC` grants for the **entire disposable instance**. Do not apply
it to a shared or production DBMS. Apply
[the data fixture](../examples/neo4j/fixture.cypher) to `knowledge`, then wait
until `neo4j_mcp_doc_vector` and `neo4j_mcp_doc_text` both report `ONLINE`. For a
local disposable server, run these commands; `cypher-shell` prompts for the
admin password:

```bash
cypher-shell -a bolt://127.0.0.1:7687 -u neo4j -d system -f examples/neo4j/provision.cypher
cypher-shell -a bolt://127.0.0.1:7687 -u neo4j -d knowledge -f examples/neo4j/fixture.cypher
```

Start from [the local test configuration](../examples/neo4j/neo4j-test.yaml).
The opt-in tests accept only the fixture's `local` environment,
`127.0.0.1` host, `knowledge` database and two dedicated user names. Set
`NEO4J_QUERY_PASSWORD`, `NEO4J_ATTESTOR_PASSWORD`,
`NEO4J_DISPOSABLE_ADMIN_USER` and `NEO4J_DISPOSABLE_ADMIN_PASSWORD` in the
test process environment. The admin credentials are used only by the opt-in
tests to verify the private database, temporarily change the dedicated
fixture role, and observe running transactions. Run:

```bash
NEO4J_DISPOSABLE_CONFIG=/absolute/path/to/neo4j-test.yaml \
  go test ./internal/dialects/neo4j ./internal/mcpserver -run TestNeo4jDisposable -count=1 -v
```

That opt-in test proves startup attestation, parameterized bounded reads,
index metadata, cited graph traversal, tenant-filtered vector `SEARCH`,
full-text `SEARCH` on 2026.09+, result byte limits, connector write rejection and direct
**native** denial of `CREATE`, `MERGE`, `SET`, `DELETE`, index creation,
local and remote `LOAD CSV`, cross-database `USE`, procedure execution and
`SHOW USERS` under the query account. The test requires
Neo4j's `Security.Forbidden` code and then checks that the fixture is unchanged
and another read succeeds. It may create
`Neo4jMCPWriteProbe` if the account is misconfigured, so use a disposable graph.
The privilege-drift test confirms the private graph exists under the admin
identity, then checks the query identity cannot read it. It revokes the read role and,
separately, grants write authority to that dedicated role; each change must
make a new call fail after the one-second proof interval. It restores the
fixture role after each check and verifies read recovery. A crashed process
can leave a temporary grant in place, so discard the disposable instance
after qualification. The MCP test exercises `neo4j_read` with a `CALL {}`
subquery, parameterized vector `SEARCH`, a low row cap, and index metadata.
It checks the cited `element_id` and truncation marker.
The cancellation test observes a running, computation-heavy read in
`SHOW TRANSACTIONS`, cancels its request, verifies that its server transaction
disappears, and checks that a new read succeeds. It repeats the recovery check
after a short query deadline and after an active query saturates the one-permit
test controller's queue.

Measure five-minute and multi-minute reads on the pinned server. Record
the server image digest, exact patch, driver version, roles, test logs, native
denials and cancellation timings before marking this connector live-qualified.

The local test suite cannot establish those server behaviors without a
provisioned Enterprise fixture. The [Neo4j user](https://neo4j.com/docs/operations-manual/current/authentication-authorization/manage-users/),
[privilege](https://neo4j.com/docs/operations-manual/current/authentication-authorization/manage-privileges/),
[auth-rule](https://neo4j.com/docs/operations-manual/current/authentication-authorization/attribute-based-access-control/)
and [vector-search](https://neo4j.com/docs/cypher-manual/current/indexes/semantic-indexes/vector-indexes/)
manuals, along with the [full-text index manual](https://neo4j.com/docs/cypher-manual/25/indexes/semantic-indexes/full-text-indexes/),
describe the server features this gate relies on.
