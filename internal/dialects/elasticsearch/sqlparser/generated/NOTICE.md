# Elasticsearch SQL parser notices

`v8/` and `v9/` are derived from the complete Elasticsearch `SqlBase.g4` files
at the immutable commits listed in
[`sql-manifest.json`](../../../../../tools/es-language-parser/sql-manifest.json).
Those upstream grammars carry Apache License 2.0 and note their Presto-parser
origin. Generated files preserve the complete upstream header; their license is
not replaced by the repository's MIT license. The complete license text is in
[`LICENSE-Apache-2.0.txt`](../../../../../tools/es-language-parser/upstream/LICENSE-Apache-2.0.txt).

Generation and deterministic Go-only cleanup are documented in the
[parser packaging guide](../../../../../tools/es-language-parser/README.md).
