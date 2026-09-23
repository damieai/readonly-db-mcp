# Elasticsearch API-key authority — 2026-09-23

Status: implemented with pinned 8.19.21/9.1.10 TLS fixture evidence. A live
Elasticsearch key/attestor pair has not been available for certification.

An Elasticsearch target can use either its existing dedicated realm user or an
API key. API-key mode omits the top-level `username` and `password_*` fields:

```yaml
elasticsearch:
  api_key:
    id: provisioned-key-id
    owner_username: reporting_owner
    key_file: /run/secrets/es-reporting-api-key-encoded
    attestor:
      username: reporting_key_auditor
      password_file: /run/secrets/es-reporting-key-auditor-password
```

The key file holds the encoded `id:api_key` value returned by Elasticsearch;
the configured ID must match its decoded ID and the authenticated identity.
`key_env` and attestor `password_env` are alternatives to protected files. The
attestor is a separate realm user with only the read-only `read_security`
cluster privilege and no index, application, delegation or remote grants. It
is used only for fixed security-proof requests and shares the target's existing
TLS origins and total socket limit with query traffic.

At startup and periodic re-attestation, the adapter checks the key's owner and
ID from `/_security/_authenticate`, then uses the attestor to read
`/_security/api_key?id=...&with_limited_by=true`. It rejects invalidated,
expired, cross-cluster, missing or mismatched keys. The complete assigned and
owner-limiting descriptors enter the authority fingerprint. Either descriptor
side must independently satisfy the existing read-only cluster/index scope
proof; this is sufficient because Elasticsearch intersects the two sides and
the other side can only remove authority. Empty assigned descriptors inherit
the limiting side and are supported. DLS/FLS restrictions are preserved.
Neither key management nor a privileged query fallback is used. See the
[8.19 API-key information contract](https://www.elastic.co/guide/en/elasticsearch/reference/8.19/security-api-get-api-key.html)
and [effective-intersection behavior](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-security-query-api-keys).

Fixtures cover both pinned builds, both safe sides of the intersection, empty
assigned roles, DLS/FLS, shared connection budget, wrong key identity, missing
limiting descriptors, elevated attestor or effective authority, expiry,
invalidation and re-attestation drift. They do not establish that a particular
deployed Elasticsearch build permits a `read_security` realm attestor to
retrieve `limited_by`; that is a live acceptance gate.

To run the optional read-only test, configure `READONLY_DB_MCP_ES_CONFIG` and
`READONLY_DB_MCP_ES_TARGET` for an `environment: test` API-key target, then run
`go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveAPIKeyAttestation -v`.
The test authenticates, inspects key descriptors and resolves configured
index scope; it creates or changes nothing.
