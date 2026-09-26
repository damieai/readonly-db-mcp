# Qdrant read-only connector, P1

The `qdrant_read` MCP tool accepts an exact configured target and collection.
Its operations are `metadata`, `query`, `query_batch`, `query_groups`, `scroll`,
`retrieve`, `count`, and `facet`. Query bodies are native Qdrant JSON: dense and
sparse vectors, filters, recursive prefetch, fusion, recommend/discover queries,
grouping, payload selection and search parameters remain available. The adapter
does not generate embeddings or call Qdrant's query-time inference service.

Copy [the example configuration](../configs/qdrant.example.yaml) and pin the
actual service to Qdrant `1.19.1`. The target takes one HTTPS origin and a CA
file. Plain HTTP is accepted only on a loopback, non-production target with
`tls.mode: disabled`. The endpoint, collection list and JWT are operator
configuration; no tool argument can select a host or supply credentials.

Enable Qdrant `service.api_key` and `service.jwt_rbac: true`, then mint an HS256
JWT with `exp` and exactly one `{"collection":"name","access":"r"}` entry
for each configured collection. Keep the signing/admin key outside this server.
The connector rejects global read-only tokens, admin keys, extra collections,
write grants, and tokens within one minute of expiry. Startup checks the
pinned version, verifies the scoped collection catalog exactly matches the
configured names, reads each collection with the JWT, and verifies
that an unauthenticated collection read is denied. Successful authentication
validates the token signature server-side. This is a credential and access
check; a native mutation-denial test remains the live release gate.

For example, the following native hybrid request uses dense and sparse
prefetch with reciprocal-rank fusion and a payload filter:

```json
{
  "target": "vector-docs",
  "collection": "agent_docs",
  "operation": "query",
  "body": {
    "prefetch": [
      {"query": [0.1, 0.2], "using": "dense", "limit": 100},
      {"query": {"indices": [1, 5], "values": [0.2, 0.3]}, "using": "sparse", "limit": 100}
    ],
    "query": {"fusion": "rrf"},
    "filter": {"must": [{"key": "tenant", "match": {"value": "team-a"}}]},
    "limit": 10,
    "with_payload": ["source_id", "title"]
  }
}
```

The adapter checks exact collection scope at the route and at every
`lookup_from`/`with_lookup` reference. It rejects duplicate JSON keys, invalid
JSON, oversized bodies, excessive prefetch trees, result limits beyond
`limits.max_rows`, and aggregate query/prefetch work beyond eight times that
limit. The response is either complete or an error when it exceeds
`limits.max_result_bytes`; it is never silently clipped into invalid JSON.
Admission, a whole-call deadline, cancellation, metrics and audit records use
the common server controls. `count` can scan a large collection, so the
configured deadline remains important even when its response is small.

## Reproducible live acceptance

On a machine with Docker, this fixture uses a disposable Qdrant 1.19.1
instance bound to loopback. Generate an admin key in your operator environment,
then start the server with JWT RBAC. Record the pulled image digest as part of
the acceptance evidence:

```sh
export QDRANT_ADMIN_KEY="$(openssl rand -hex 32)"
export QDRANT__SERVICE__API_KEY="$QDRANT_ADMIN_KEY"
docker run -d --rm --name readonly-qdrant-p1 \
  -p 127.0.0.1:6333:6333 \
  -e QDRANT__SERVICE__API_KEY \
  -e QDRANT__SERVICE__JWT_RBAC=true \
  qdrant/qdrant:v1.19.1
docker image inspect qdrant/qdrant:v1.19.1 --format '{{index .RepoDigests 0}}'
```

Create a two-dimensional dense vector, a sparse vector and two points with
source IDs and tenant payloads. Fixture writes use the admin key outside the
MCP process:

```sh
curl -fsS -X PUT http://127.0.0.1:6333/collections/agent_docs \
  -H "api-key: $QDRANT_ADMIN_KEY" -H 'Content-Type: application/json' \
  -d '{"vectors":{"dense":{"size":2,"distance":"Cosine"}},"sparse_vectors":{"sparse":{}}}'
curl -fsS -X PUT 'http://127.0.0.1:6333/collections/agent_docs/points?wait=true' \
  -H "api-key: $QDRANT_ADMIN_KEY" -H 'Content-Type: application/json' \
  -d '{"points":[{"id":1,"vector":{"dense":[0.1,0.2],"sparse":{"indices":[1,5],"values":[0.2,0.3]}},"payload":{"tenant":"team-a","source_id":"doc-1","title":"first"}},{"id":2,"vector":{"dense":[0.2,0.1],"sparse":{"indices":[1,5],"values":[0.3,0.2]}},"payload":{"tenant":"team-b","source_id":"doc-2","title":"second"}}]}'
```

Mint the read JWT offline with the same admin signing key. The Python snippet
uses only its standard library; it does not contact Qdrant:

```sh
export QDRANT_LIVE_JWT="$(python3 - <<'PY'
import base64, hashlib, hmac, json, os, time
def encode(value):
    raw = json.dumps(value, separators=(',', ':')).encode()
    return base64.urlsafe_b64encode(raw).rstrip(b'=').decode()
claims = {'exp': int(time.time()) + 3600,
          'access': [{'collection': 'agent_docs', 'access': 'r'}]}
message = encode({'alg': 'HS256', 'typ': 'JWT'}) + '.' + encode(claims)
signature = hmac.new(os.environ['QDRANT_ADMIN_KEY'].encode(),
                     message.encode(), hashlib.sha256).digest()
print(message + '.' + base64.urlsafe_b64encode(signature).rstrip(b'=').decode())
PY
)"
```

Run the optional live test against this fixture. For a remote HTTPS fixture,
set `QDRANT_LIVE_CA_FILE` to its CA PEM file and use an HTTPS URL instead:

```sh
export QDRANT_LIVE_URL=http://127.0.0.1:6333
export QDRANT_LIVE_COLLECTION=agent_docs
export QDRANT_LIVE_QUERY_BODY='{"prefetch":[{"query":[0.1,0.2],"using":"dense","limit":10},{"query":{"indices":[1],"values":[0.2]},"using":"sparse","limit":10}],"query":{"fusion":"rrf"},"limit":5}'
go test ./internal/dialects/qdrant -run '^TestQdrantLiveReadOnlyGate$' -v
```

The live test verifies metadata, native hybrid query, scroll, cancellation,
and Qdrant's denial of a native point-delete request for a freshly generated
absent ID. Record the image digest, service version, JWT grant, test output,
latency and resource measurements for release qualification. Repository tests
use a TLS mock and do not establish this live-server result.

Qdrant's [security documentation](https://qdrant.tech/documentation/security/)
defines collection-scoped `r` access and mutation denial. Its
[Query API](https://api.qdrant.tech/api-reference/search/query-points) and
[hybrid search guide](https://qdrant.tech/documentation/search/hybrid-queries/)
define the native read shapes used here.
