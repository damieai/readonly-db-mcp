# Elasticsearch ES|QL pragma resource profile — 2026-09-23

Status: implementation with pinned 8.19.21 and 9.1.10 source review and
TLS/MCP fixture evidence. The optional live read-only test has not run against
an Elasticsearch deployment in this environment.

Both pinned release builds parse `pragma` in synchronous ES|QL requests but
require `accept_pragma_risks` for a non-empty pragma object. The adapter owns
that flag: callers cannot set it independently. The per-target `esql_pragmas`
configuration is an explicit operator opt-in. For example:

```yaml
elasticsearch:
  esql_pragmas:
    max_task_concurrency: 8
    max_page_size: 2048
    max_fold_bytes: 67108864
    max_fold_percent: 5
    min_status_interval: 100ms
```

```json
{"target":"search-reporting","operation":"esql.query","body":{"query":"FROM reports-* | LIMIT 10","pragma":{"task_concurrency":4,"page_size":1024,"fold_limit":"32mb"}}}
```

Each resource control has its own optional ceiling: exchange buffer size,
concurrent exchange clients, ENRICH workers, task concurrency, page size,
concurrent nodes per cluster, concurrent shards per node, shard resolution
attempts, fold memory in bytes or percent, and minimum task status interval.
An unset ceiling does not authorize an increase through that control. Native
default `max_concurrent_nodes_per_cluster=-1` and adaptive `page_size=0` remain
available; unlimited shard retries require a separate
`allow_unlimited_shard_retries: true` grant. `node_level_reduction` and
`data_partitioning` retain native types and semantics;
`field_extract_preference` is available only in the pinned 9.1.10 build.
Unknown settings fail instead of being silently ignored by the native settings
wrapper. Query parsing, index/ENRICH source proof, native deadline, output bounds
and whole-result validation are unchanged.

The integer ceiling names in `esql_pragmas` correspond to native settings as
follows:

| Operator ceiling | Native pragma |
| --- | --- |
| `max_exchange_buffer_size` | `exchange_buffer_size` |
| `max_exchange_concurrent_clients` | `exchange_concurrent_clients` |
| `max_enrich_workers` | `enrich_max_workers` |
| `max_task_concurrency` | `task_concurrency` |
| `max_page_size` | `page_size` |
| `max_concurrent_nodes_per_cluster` | `max_concurrent_nodes_per_cluster` |
| `max_concurrent_shards_per_node` | `max_concurrent_shards_per_node` |
| `max_shard_resolution_attempts` | `unavailable_shard_resolution_attempts` |

`max_fold_bytes` and `max_fold_percent` bound native `fold_limit`; the separate
`min_status_interval` bounds native `status_interval` from below.

The reviewed sources are the pinned
[8.19 QueryPragmas](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/plugin/QueryPragmas.java),
[9.1 QueryPragmas](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/plugin/QueryPragmas.java),
and [9.1 release-request validation](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/action/EsqlQueryRequest.java).
The fixture suite covers both builds, preservation of native settings, 9.1-only
fields, ceilings, invalid types and unknown keys, operator opt-in, risk flag
ownership and admission release. It does not certify real-server planner or
cluster load behavior.

For optional live acceptance, configure `READONLY_DB_MCP_ES_CONFIG` and
`READONLY_DB_MCP_ES_TARGET` for an `environment: test` target with an explicit
`esql_pragmas` block, then run
`go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveESQLPragmas -v`.
The test executes `ROW x=1` without reading an index or changing cluster state.
