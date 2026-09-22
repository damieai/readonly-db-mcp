The Go ES|QL lexer/parser code in v8 and v9 is generated from Elasticsearch's
complete version-pinned grammars, which are licensed under Elastic License 2.0.
The original copyright/license notices remain in those files. The license text
is at tools/es-language-parser/upstream/LICENSE-Elastic-2.0.txt. These derived
files are not covered by the repository's MIT license. The generated config.go
base classes are handwritten generator output under the repository license.

Immutable upstream URLs and source/output hashes are recorded in
tools/es-language-parser/esql-manifest.json. Regeneration instructions and the
Go target predicate spelling adaptation are in tools/es-language-parser/README.md.
