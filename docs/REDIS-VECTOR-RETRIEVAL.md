# Redis dense vector retrieval

Use the existing `redis_command` and `redis_batch` tools with an operator-installed
Redis Search index. The MCP service does not install modules, create indexes or
load documents. Native Search query expressions and options are preserved.

## Provisioning

Use a dedicated reader with `%R~tenant:*`, no write commands, and the introspection
grants required by [Redis setup](../README.md). Search index names must also be
scoped. For legacy Search builds that require an index RW key flag, a separate
`~search-*` grant is accepted only when it is disjoint from document keys and
backed by signed `index-prefix-attested` command rules.

Configure the target using the existing Redis shape, for example:

```yaml
redis:
  protocol: 3 # 2 is also supported; default remains 3
  key_patterns: ["tenant:*"]
  module_profiles:
    - /etc/readonly-db-mcp/search-profile.json
    - /etc/readonly-db-mcp/json-profile.json # only when JSON is installed
  trusted_profile_keys:
    reviewed-build: "BASE64_ED25519_PUBLIC_KEY"
  module_object_patterns:
    FT.SEARCH: ["search-*"]
    FT.AGGREGATE: ["search-*"]
    FT.INFO: ["search-*"]
```

Profiles must cover the complete installed module/command inventory and pin
module version, artifact path/hash, build identity, Redis compatibility and
validity period. Mark reviewed Search reads `readonly: true` with
`key_model: index-prefix-attested`; leave writes unadmitted. Sign the exact JSON
payload using the envelope defined in
[the module profile implementation](../internal/dialects/redis/modules/profile.go).
A vector field does not require a separate command grant. JSON indexes still
require an installed JSON module profile even if clients only call `FT.SEARCH`.
Test-fixture signing keys/profiles are never production authorization.

## Discover the live vector field

Call `redis_command` with:

```json
{"target":"vectors","command":"FT.INFO","arguments":[{"string":"search-documents"}]}
```

The result preserves the native `value` and includes an optional `search_index`
summary with index name, HASH/JSON storage, and vector field identifier, attribute,
algorithm, data type, dimensions and distance metric. The returned document's
canonical name and prefixes are scope-checked again, after execution. The summary
describes live fields; it does not claim that every build/algorithm is qualified.

## Supply a dense vector

For a FLOAT32 field of dimension 3, `[1, -0.5, 0.25]` in the tested little-endian
format is base64 `AACAPwAAAL8AAIA+`. For FLOAT64 use eight bytes per component.
Match the field's type, dimension and the server platform's vector byte order.
JSON document storage still uses a binary query-vector parameter.

```json
{
  "target": "vectors",
  "command": "FT.SEARCH",
  "arguments": [
    {"string":"search-documents"},
    {"string":"(@category:{keep})=>[KNN 3 @embedding $v AS distance]"},
    {"string":"PARAMS"}, {"string":"2"},
    {"string":"v"}, {"base64":"AACAPwAAAL8AAIA+"},
    {"string":"SORTBY"}, {"string":"distance"}, {"string":"ASC"},
    {"string":"RETURN"}, {"string":"1"}, {"string":"distance"},
    {"string":"LIMIT"}, {"string":"0"}, {"string":"3"},
    {"string":"DIALECT"}, {"string":"2"}
  ]
}
```

Each argument contains exactly one of `string` or `base64`. Base64 is decoded
before dispatch; do not send a textual JSON array as a HASH vector blob. Native
dimension errors fail the request without changing the input or the index.

Native vector range expressions are also preserved, for example
`@embedding:[VECTOR_RANGE 0.5 $v]=>{$yield_distance_as: distance}`.
HNSW query options such as `EF_RUNTIME` remain available in their native grammar.
The service does not reduce K, change a filter or replace a distance metric.

Results retain native distance semantics and RESP2/RESP3 shapes. In the qualified
build, L2 scores are squared Euclidean distances, IP is `1 - dot(a,b)`, and
COSINE is cosine distance. Lower is closer. Binary results that are not valid
UTF-8 use a `base64` object. RESP3 non-finite doubles use `{"float":"nan"}`,
`{"float":"+inf"}` or `{"float":"-inf"}` because JSON has no such numbers;
this also lets native `FT.INFO` ratio fields survive transport.

Check `truncated` on every result. Row/element/cell/byte limits can make output
incomplete; approximate recall and output truncation are different concerns.
Batch responses have per-command results and one shared request deadline.

## Qualification boundary

See [the native qualification record](qualification/2026-09-21-redis-dense-vectors.md).
Standalone HASH/JSON, FLAT/HNSW, FLOAT32/FLOAT64, L2/IP/COSINE and RESP2/RESP3
have local end-to-end evidence. Production TLS/topology, large-corpus recall,
server work after cancellation and other build/type combinations remain separate
release gates. PostgreSQL pgvector still requires RFC-0007 P1/P2 implementation.
