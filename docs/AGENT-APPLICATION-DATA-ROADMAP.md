# Agent application data connector roadmap

- Updated: 2026-09-24
- Scope: read-only MCP access to application and retrieval data
- Priority basis: coverage of distinct agent workloads, current repository gaps,
  and the cost of proving a useful read-only security boundary. This is a project
  priority, not a ranking of employer demand.

## Architectural boundary

This server supplies bounded, auditable reads from operator-configured targets.
Conversation state, workflow checkpoints, memory writes, embedding ingestion and
index creation belong to a separate application-owned write path. In particular,
exposing a read-only PostgreSQL target does not turn it into a LangGraph
checkpointer: a checkpointer must persist state. LangGraph documents PostgreSQL
checkpointers for production and SQLite checkpointers for local development
([persistence guide](https://github.com/langchain-ai/docs/blob/main/src/oss/langgraph/persistence.mdx)).

## Default sequence

| Priority | Capability | Why it comes here | Completion gate |
| --- | --- | --- | --- |
| P0 | Qualify existing PostgreSQL + pgvector, Redis Search, and Elasticsearch in one agent retrieval example; retain MySQL and SQL Server as business-data sources | The repository already implements these paths. A real question that joins business facts with cited retrieval results demonstrates more than another adapter. Elasticsearch still has live-server qualification gates. | One reproducible local scenario with schema discovery, bounded query, vector/full-text retrieval, source IDs, cancellation, and evidence that write attempts fail; publish exact tested versions and limits. |
| P1 | Qdrant specialist vector database connector | Adds a dedicated vector database alongside existing general-purpose engines. One native vector service makes the agent-retrieval portfolio credible across both architectural choices. Qdrant supports dense, sparse/hybrid and metadata-filtered retrieval. | Scoped collection metadata and native read/query operations, including filters and hybrid prefetch/fusion where the pinned build supports them; use a collection-scoped read-only key, bounded top-K/payload and cancellation; prove mutation denial on a live server. |
| P2 | MongoDB document connector | Adds document-shaped operational data, which the current SQL/Redis/Elasticsearch adapters do not represent as a primary database. MongoDB also has a native vector-search path, so one connector can serve document reads and later retrieval. | Start with scoped `find`, count, collection metadata and an explicitly reviewed read-only aggregation subset; prove effective read permissions, collection scope, no `$out`/`$merge`/server-side code or network effects, result bounds and cancellation on a live build. Add `$vectorSearch` only after a separately versioned capability and live index proof. |
| P3 | SQLite connector | Useful for local/offline demos, evaluation datasets and desktop agents. It adds less production coverage than MongoDB or Qdrant to this repository. | Open existing files with `mode=ro`, confine file paths to configured roots, verify schema and read-only SQL policy, bound results and cancellation, and reject ATTACH/external access and mutation paths. Do not use `immutable=1` for files that may change. |
| Conditional | Neo4j, Snowflake/BigQuery, ClickHouse, Milvus/Weaviate, OpenSearch | Add one only when a target role or real workload needs graph traversal, warehouse analytics, another vector engine, or an OpenSearch-specific estate. | A named workload, pinned deployment and a read-only authority model precede an RFC. Protocol similarity to an existing adapter does not establish safety or compatibility. |

Qdrant is the default first specialist vector database because it offers a
locally deployable service, a native hybrid Query API and collection-scoped
read-only keys. Milvus is a reasonable alternative for a role centered on
distributed vector infrastructure or multiple index types; Weaviate is an
alternative for object-oriented vector/keyword search; Pinecone is relevant
when the target stack is managed vector search. Choose **one** specialist for
the first implementation and use the same retrieval evaluation to compare it
with pgvector/Redis/Elasticsearch. For enterprise data assistants, MongoDB may
move ahead of Qdrant, and existing SQL Server/MySQL live qualification matters
more. Do not expand Elasticsearch into every optional endpoint to justify its
place in this plan; its useful core is read-only search, metadata, retrieval
and safe resource accounting.

## Implementation rules for each new connector

Follow the [engine completion contract](ARCHITECTURE.md#adding-a-database-engine):
engine-specific policy, effective authority proof, native read-only enforcement,
safe metadata, cancellation/resource tests and a hostile mutation corpus. Expose
native advanced **read** capabilities when their effects and scope can be proved;
do not reduce an engine to one simplified query shape merely to ease policy work.
Keep host, credentials, files and allowed collections/indexes in operator config,
never in model-selected tool arguments.

The sequence should be reviewed after P0 evidence. It can change with a real
target job description, deployment, or user workload; adding more engines alone
does not improve agent design skills such as retrieval evaluation, grounding,
authorization boundaries and durable state management.

## Primary references

- [MongoDB vector search and pre-filtering](https://www.mongodb.com/docs/vector-search/)
- [Qdrant hybrid search](https://qdrant.tech/documentation/search/text-search/hybrid-search/)
- [Qdrant read-only and collection-scoped keys](https://qdrant.tech/documentation/security/)
- [Milvus search and deployment overview](https://milvus.io/docs/overview.md)
- [Weaviate hybrid search](https://docs.weaviate.io/weaviate/concepts/search/hybrid-search)
- [Pinecone vector search concepts](https://sdk.pinecone.io/python/guides/concepts.html)
- [SQLite read-only URI mode](https://www.sqlite.org/uri.html)
