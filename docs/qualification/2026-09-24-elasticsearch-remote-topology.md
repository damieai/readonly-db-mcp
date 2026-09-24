# Elasticsearch remote topology proof — 2026-09-24

Status: implemented against pinned 8.19.21 and 9.1.10 TLS fixture responses.
No live cross-cluster deployment was available for this record, and remote
query execution remains unavailable.

The optional `elasticsearch.remote_clusters` list defines exact operator-owned
aliases, proxy or sniff connection addresses, and future index scopes. It does
not create or update Elasticsearch cluster settings. When configured, every
local endpoint reads `GET /_remote/info` during startup and periodic authority
refresh. The proof requires each alias to exist with the configured connection
mode and exact proxy address or seed set, plus the native redacted
`cluster_credentials` marker for API-key authentication. It checks the native
`connected` and `skip_unavailable` fields and binds their values into the
authority revision. A disconnected alias does not block local reads; topology
or API-key-model drift invalidates the target proof.
Other remote aliases on the local cluster do not become allowed aliases.

Example operator configuration:

```yaml
remote_clusters:
  - alias: archive
    mode: proxy
    proxy_address: archive-proxy.example.invalid:9443
    allowed_indices: [logs-*]
    denied_indices: [logs-private-*]
```

This check establishes local connection topology and use of the API-key
security model only. Elasticsearch deliberately redacts the credential value;
`/_remote/info` does **not** disclose the key ID or remote cluster UUID. The
configured index patterns are reserved for the later request-scope proof and
do not authorize `archive:logs-*` today. A remote API-key identity/scope proof,
binding to the actual local-cluster credential, and complete-result validation
must precede dispatch. The native [8.19 remote-info reference](https://www.elastic.co/guide/en/elasticsearch/reference/8.19/cluster-remote-info.html)
defines the redacted marker and connection fields.

Fixtures cover proxy and sniff modes on both pinned builds, local reads with a
configured remote, missing aliases, certificate authentication, wrong address
or mode, malformed availability metadata and periodic drift invalidation.
They do not test a real remote connection or key rotation.
