# P0 agent data-access acceptance

This example asks which checkout timeout incident is relevant and assembles
grounded evidence from one business row and three retrieval paths. It uses one
stdio MCP session and only public tools: `list_targets`, SQL schema discovery,
parameterized `query_select`, `redis_command`, and `es_query`. The report names
the target, source ID and distance for each vector hit, captures exact server
versions, and fails if the selected incident is absent from any retrieval path.
It also checks that write attempts are rejected through all three MCP tools.
The program does not generate an LLM answer or ingest embeddings.

## Required disposable services and targets

Use a pinned PostgreSQL 16 build with pgvector 0.8.2 and the repository's
[pgvector proof profile](../../docs/POSTGRESQL-VECTOR-RETRIEVAL.md), Redis 7.4
with a signed Redis Search module profile as described in the
[Redis vector guide](../../docs/REDIS-VECTOR-RETRIEVAL.md), and a pinned
Elasticsearch 8.19.21 or 9.1.10 build with the core scoped read-only search
profile in [RFC-0006](../../docs/RFC-0006-elasticsearch-readonly-support.md).
Other versions require their own qualification; version strings from the
example's JSON output are evidence of what was connected, not certification.

Configure three exact target aliases in one MCP YAML file:

| Alias | Required scope | Fixture identity |
| --- | --- | --- |
| `agent-pg` | PostgreSQL `reporting` plus the trusted `vectors` extension; `SELECT` on both fixture tables; `connection.max_open: 1` for the cancellation/recovery proof | Dedicated non-owning reader |
| `agent-redis` | RESP2, `tenant:agent:*` keys, `search-agent-incidents` Search index, signed Search module profile | Dedicated read-only ACL user |
| `agent-es` | Concrete `agent-incidents` index, search and metadata read privileges | Dedicated read-only ES user or attested API key |

Use the existing engine guides for the rest of each target's TLS, authority,
resource and proof settings. Never give the MCP process fixture-administrator
credentials. The provisioning identity must be separate and used only for the
three scripts below. Run against disposable test data:

```sh
psql -X -v ON_ERROR_STOP=1 -v reader=agent_pg_reader -f examples/agent-p0/postgres-fixture.sql
REDISCLI_AUTH="$(cat /private/path/redis-admin.password)" sh examples/agent-p0/redis-fixture.sh
ES_URL=https://localhost:9200 ES_CA=/private/path/es-ca.pem \
  ES_CURL_CONFIG=/private/path/admin.curlrc sh examples/agent-p0/elasticsearch-fixture.sh
```

The PostgreSQL script expects the `vectors` extension schema already installed.
The Redis script creates a FLOAT32/L2/FLAT index and three HASH documents;
`redis-cli` and Python 3 are used only by provisioning. The ES script creates
one index and three documents. The private curl config may hold an admin `user`
entry; keep it out of the repository and use mode 0600. Provision the readers'
grants after fixture creation and verify the whole server configuration with:

```sh
go build -o bin/readonly-db-mcp ./cmd/readonly-db-mcp
bin/readonly-db-mcp -config /private/path/agent-p0.yaml -check
go run ./examples/agent-p0 -config /private/path/agent-p0.yaml
```

The JSON output must contain incident `101`, at least one source ID from each
retrieval engine, `cancel_recovery: true`, all three `write_denial` values set
to `true`, and the three server versions. The example validates a slow
read-only query with `query_explain`, cancels it after 100 ms, then checks a
follow-up query on the one-connection PostgreSQL target. It caps each SQL/ES
result at three rows and each ordinary request at 10 seconds; Redis Search
requests three hits with a bounded reply.
It fails on partial or truncated results rather than treating them as evidence.
The vector is an explicitly supplied little-endian FLOAT32 `[1,0,0]`, matching
the PostgreSQL vector text parameter. Change fixture data and query vector
together if adapting the scenario.

## Acceptance beyond the example

The cross-engine JSON report proves the application path and MCP-level write
rejection. Native write denial, mid-query cancellation/recovery, and release
qualification must also be run on the provisioned builds. The repository's
existing tests provide those gates:

```sh
make test-redis-vectors
make test-postgresql-vectors
make test-postgresql-vector-qualification
make test-postgresql-vector-transport
make test-postgresql-vector-long-read
READONLY_DB_MCP_ES_CONFIG=/private/path/es-test.yaml \
  READONLY_DB_MCP_ES_TARGET=agent-es \
  READONLY_DB_MCP_ES_WRITE_PROBE_INDEX=mcp-es-acceptance-disposable \
  go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveReadOnlyMetadata -count=1 -v
```

The ES native-denial test needs its own operator-provisioned
`mcp-es-acceptance-*` index included in the ES reader's allowed scope; it never
uses administrator credentials. PG/Redis test environment variables and exact
artifact requirements are in the linked vector guides. Save the command
outputs, build hashes and configuration scope in a qualification record. A
skipped test or an HTTP fixture result is not live-server certification.
