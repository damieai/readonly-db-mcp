# Elasticsearch ENRICH qualification — 2026-09-22

Status: code and TLS/MCP fixture evidence for the pinned 8.19.21 and 9.1.10
profiles. **No real-server certification is claimed.** This increment implements
the explicit enrichment authority profile left open by the preceding ES|QL
increment. Finer snapshot isolation and cross-cluster authority remain separate.

## Authorization and provisioning

ENRICH is disabled by default. The operator can grant a separate data namespace:

```yaml
elasticsearch:
  # Other required endpoint/build/cluster/index settings are omitted here.
  enrich:
    scope: all_cluster_snapshots
```

This grants every historical, current and future native enrichment snapshot in
the configured cluster UUID. It is independent of ordinary `allowed_indices`,
`denied_indices`, source-index grants and DLS/FLS. It is an actual broad data
authorization, not a boolean assertion that those other controls secure ENRICH.
Do not enable it on a shared cluster containing enrichment data this target must
not expose. Provision a separate cluster when policy/document/field isolation is
required. Current policy sources, current alias UUIDs and master-state comparisons
cannot establish confidentiality of historical or stale distributed snapshots.

Keep the dedicated runtime user's normal index `read`/`view_index_metadata` and
cluster `monitor` grants. Add explicit `monitor_enrich`; monitor alone does not
provide the complete privilege checked by native ENRICH lookup. `manage_enrich`,
policy create/execute/delete and every index write remain forbidden. Operators
build policies/snapshots separately. Complete provisioning before startup or
retry: a partially built snapshot without its native write block fails inventory
validation, including when it has not yet become the active alias.

Normal FROM/JOIN sources retain their configured/request scope checks. Policy
`indices` and `query` describe a previous snapshot build and are not queried or
executed by ENRICH preflight. Consequently they can name source indices outside
the ordinary scope, because the separate grant authorizes their snapshot data.
Snapshot contents still require the operator's approval for that cluster-wide
scope. The service never provisions or executes an enrichment policy itself.

## Native behavior and proof

Original ES|QL text and parameters go to synchronous `POST /_query` unchanged.
Coverage includes match/range/geo_match policies, quoted policy names, ON/WITH
renaming, chaining, 9.1 FORK and ENRICH inside 8.19 EXPLAIN pipelines. Quoted policy
names follow `LogicalPlanBuilder.parsePolicyName`'s single-quote-pair removal,
without applying the distinct index-string unescaping rules. All native modes
(`_any`, `_coordinator`, `_remote`, case-insensitive) resolve locally when every
source is local. This does not enable remote FROM/JOIN or remote credentials.
Native field/type/function/plan availability remains the server's decision.

Fixed internal GET routes read filtered cluster state and selected policies:

- `/_cluster/state/metadata,version/.enrich-*`, with flat settings, all index
  expansion and a remaining-deadline master timeout;
- `/_enrich/policy/{exact-escaped-name}`.

The current inventory includes active and retired snapshots. Validation checks
cluster identity, state UUID/version, unique snapshot UUIDs, open one-shard native
snapshots, write blocks, policy aliases, native `_meta`, stored/enabled source,
dynamic=false mappings and match-field type. Runtime/lookup/alias/semantic-text
and scripted mapping extensions are outside the native snapshot profile. Native
replicas and multi-node local topologies are retained. Selected policy type/name/
match field must agree with its active snapshot. No raw policy/state data is
exposed through a public metadata or query operation.

Inventory bytes and combined physical-index/snapshot counts use existing proof
limits. Startup and periodic checks include inventory digests in authority
revisions, invalidating owned contexts on change. Query/batch preflight records
selected policy digests and inventory state; final verification rejects observed
policy, snapshot or cluster-state drift before returning results. Unrelated
cluster-state changes can therefore require an explicit caller retry. These
checks are **not** atomic distributed locks, provenance attestations or a way to
retrofit policy/DLS/FLS isolation. Stock native snapshot provisioning and the
all-snapshot data grant remain the deployment contract. Execution is never
automatically retried and no new query cursor or asynchronous state is created.

## Source review

The operation manifest records immutable REST sources and SHA-256 hashes for
both pins. Effect classification marks cluster-state/policy reads internal-only
and policy put/execute/delete as writes. Reviewed implementation sources include
the following at each pinned commit (links below select 8.19.21; the corresponding
9.1.10 commit is `f2e019bd8110088070638ca779ec1543188c0f43`):

- [EnrichPolicyRunner](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/enrich/src/main/java/org/elasticsearch/xpack/enrich/EnrichPolicyRunner.java): snapshot mappings, write block and alias promotion.
- [IndexMetadata](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/server/src/main/java/org/elasticsearch/cluster/metadata/IndexMetadata.java): API flat settings, typed mappings and alias-name arrays.
- [LogicalPlanBuilder](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/parser/LogicalPlanBuilder.java): native policy-name handling.
- [ENRICH prerequisites](https://www.elastic.co/guide/en/elasticsearch/reference/8.19/esql-enrich-data.html): native authorization and policy/DLS/FLS limitations.

## Automated evidence and remaining live gates

Tests exercise both parser/build profiles, native text/parameter precision, all
three policy types, local native modes, chains/branches, MCP query/batch calls,
explicit scope and privilege requirements, write denial, private/remote source
rejection, internal-route denial, malformed/unsafe/duplicate/retired snapshot
inventories, policy and state drift, whole-batch preflight, combined source and
byte budgets, periodic authority invalidation and metadata-read cancellation.
The fixtures test the adapter contract; they do not execute native ENRICH plans.

Local validation completed with `go test ./...`, race tests for the Elasticsearch,
MCP and configuration packages, `go vet ./...`, a CLI build and the pinned REST
catalog regeneration check. A two-worker, ten-second ES|QL parser fuzz run
completed 14,261 executions without a failure. The live ENRICH test reported
SKIP because its dedicated configuration/policy/key/label settings were absent.

The live test is opt-in and read-only. Provision an existing `match` policy named
`mcp-es-acceptance-*` with keyword match field `id` and enrich field `label`, build
its snapshot externally, and use an `environment: test` target explicitly granting
the snapshot scope and `monitor_enrich`. Set:

```text
READONLY_DB_MCP_ES_CONFIG=<qualification YAML>
READONLY_DB_MCP_ES_TARGET=<target alias>
READONLY_DB_MCP_ES_ENRICH_POLICY=mcp-es-acceptance-<policy>
READONLY_DB_MCP_ES_ENRICH_KEY=<existing string id>
READONLY_DB_MCP_ES_ENRICH_LABEL=<expected string label>
```

Run `go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveEnrich -v`.
It asserts the actual label for all local mode qualifiers without modifying the
configuration, policy or data. Missing settings produce a skip, not a pass.

Before production qualification, run each pinned release with real privileges
and serialized cluster metadata; exercise range/geo_match, nested/chained plans,
licensed features, replicas and multiple nodes, empty/retired/building snapshots,
policy promotion/deletion, ordinary source DLS/FLS, cancellation/task termination,
resource saturation and authority/context revision invalidation. Confirm that
all historical and future snapshot data belongs to the operator's granted scope.
No such deployment or credentials were available for this implementation run.
