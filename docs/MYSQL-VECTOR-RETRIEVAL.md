# MySQL exact vector retrieval

`mysql_vector_search` ranks a complete, bounded MySQL `SELECT` candidate set by
cosine distance, Euclidean (L2) distance or dot-product similarity. It works
with the existing MySQL 8 connector and numeric embeddings stored as JSON
arrays. A query can use the connector's normal advanced read-only SQL for
joins, CTEs and indexed business filters; the projected embedding must have a
plain column alias. The tool does not install a plugin or generate embeddings.

MySQL 8 does not provide a general built-in vector-distance function. MySQL's
newer `DISTANCE()` function is [limited to HeatWave on OCI and MySQL AI](https://dev.mysql.com/doc/refman/9.7/en/vector-functions.html),
so this profile computes scores in the MCP process. It is **exact only over the
rows returned by the supplied candidate SELECT**, not an ANN index or a full
table nearest-neighbor guarantee. Use ordinary SQL predicates to narrow the
candidate set. A `LIMIT` in that SQL intentionally limits the ranking scope.

```json
{
  "target": "catalog",
  "sql": "SELECT id, source_id, title, embedding FROM catalog.documents WHERE tenant_id = ? AND category = ?",
  "parameters": ["team-a", "manual"],
  "embedding_column": "embedding",
  "vector": [0.1, 0.2, 0.3],
  "metric": "cosine",
  "top_k": 10,
  "max_candidates": 1000,
  "timeout_ms": 10000
}
```

`max_candidates` is required and cannot exceed the target's configured
`limits.max_rows`; `top_k` cannot exceed it. If the SELECT yields one more row
than `max_candidates`, or any candidate cell/result is clipped by configured
budgets, the entire request fails instead of reporting a misleading ranking.
Every candidate embedding must be a numeric JSON array with the same length as
the query vector. Malformed, zero-norm cosine and non-finite vectors fail the
whole call. The embedding column is omitted from ranked hits to keep responses
small. Ties retain SQL candidate order. `score_order` is `ascending` for cosine
and L2 distance, and `descending` for dot product. The response includes
`candidate_count` to make the ranking scope visible.

This tool reuses the MySQL SELECT policy, grant proof, read-only transaction,
admission and query limits. The vector calculation shares the whole-call
deadline. A failed or incomplete candidate read returns no ranked hits. An
operator should size `max_candidates`, `max_result_bytes`, `max_cell_bytes`
and timeout for the expected vector dimension and row payload. The candidate
result must fit in the configured result-byte budget before ranking.

The local tests cover ranking, malformed embeddings, incomplete candidate
sets, mutation rejection and MCP registration. A disposable MySQL fixture with
representative JSON embeddings and its provisioned read-only account is still
needed for live throughput, cancellation and native-denial acceptance. This
profile does not claim HeatWave `DISTANCE()` or index-accelerated ANN support.
