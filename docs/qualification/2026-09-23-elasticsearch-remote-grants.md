# Elasticsearch remote grant attestation — 2026-09-23

Status: implemented for local-query identities with pinned 8.19.21 and 9.1.10
TLS/API-key fixture evidence. No cross-cluster query route is enabled.

An Elasticsearch identity can hold both local and remote read-only roles. The
adapter now examines `remote_indices` and `remote_cluster` entries in effective
realm-user privileges and API-key role descriptors instead of rejecting any
remote grant. Remote index entries may grant `read`, `read_cross_cluster`,
`view_index_metadata` or `none`; remote cluster entries may grant
`monitor_enrich` or `monitor_stats`. Cluster alias patterns and index selectors
must be well formed, restricted-index access is rejected, and unrecognized
fields or other privileges fail the authority proof. DLS/FLS fields are retained
as native grant restrictions. API-key assigned and limiting descriptor sets
continue to use Elasticsearch's intersection: one fully proved read-only side
must bound the effective key.

This validation permits existing **local** metadata and queries for a user or
API key with additional read-only remote roles. It does not authorize a remote
query. The source resolver still rejects `alias:index` expressions, SQL remote
catalogs and ES|QL remote sources; `cross_cluster` remains unavailable in
target capabilities. In particular, wildcard remote grants are not a configured
remote scope. Cross-cluster execution still needs a configured alias map,
remote connection identity, remote API-key/scope proof, per-request source
resolution and complete-result handling before any route is opened.

Fixtures cover both pinned builds, API-key intersection, local metadata access,
pre-dispatch rejection of a remote search, scalar/array grant syntax, and
mutation, unknown-field, malformed-alias and restricted-index rejection. A live
Elasticsearch cluster with remote roles has not been used. The native
[self-privileges response](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-security-get-user-privileges)
defines these grant fields; Elasticsearch's
[privilege reference](https://www.elastic.co/docs/reference/elasticsearch/security-privileges)
defines their read-only actions.
