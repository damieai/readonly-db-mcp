# Pinned Elasticsearch operation catalog

`generate.py` reads REST JSON from immutable official Elasticsearch commits for
8.19.21 and 9.1.10. It records routes, methods, parameter names and each source
file's SHA-256 in `internal/dialects/elasticsearch/operations.generated.json`.
The effect/implementation decisions are separately reviewed entries in the
generator, not inferred from HTTP verbs or upstream descriptions.

```sh
python3 tools/es-rest-catalog/generate.py --cache /tmp/readonly-es-rest-specs
python3 tools/es-rest-catalog/generate.py --cache /tmp/readonly-es-rest-specs --check
```

The cache makes repeat runs offline. `--check` detects any divergence, including
modified cache files, against the checked-in catalog. Inspect generated diffs
when intentionally updating profiles. Never enable a route merely because its
REST specification exists: source, effect, privilege and lifecycle handlers must
be implemented before changing its availability.

Native query handlers now enable search/count, document/term-vector reads,
diagnostics and render/execute templates, alongside `resolve` and `mappings`.
`msearch` is implemented through `es_batch`, and `get_script` is an internal
definition-proof route, not a public query operation. PIT/scroll remain planned
owned-context operations. SQL/ES|QL/EQL require their own upstream grammar and REST-profile
work. Their pending status is not a mutation classification.

The selected build hashes are compatibility pins, **not** live-server release
certification. See the phase qualification record before deployment.
