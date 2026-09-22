# Pinned Elasticsearch language parser generation

The EQL profile uses the complete upstream `EqlBase.g4` from Elasticsearch
8.19.21 and 9.1.10. Both immutable commits contain the same grammar bytes.
`manifest.json` records source URLs, the grammar hash, ANTLR generator hash and
every generated Go file. No local grammar simplification or dialect substitution
is applied. The adapter obtains a structured query kind/source contract from
the parse tree and sends the original EQL text to the native endpoint.

Runtime packaging is a compiled Go parser with `github.com/antlr4-go/antlr/v4`
4.13.1. Operators do **not** install Java or a separate parser process. Java is
only a maintainer prerequisite for regeneration.

## Reproduce or verify

Offline integrity check (does not need Java or network):

```sh
python3 tools/es-language-parser/generate.py --check
```

For full regeneration, obtain the official
[ANTLR 4.13.1 complete JAR](https://www.antlr.org/download/antlr-4.13.1-complete.jar)
and use Java 11 or newer. The script verifies the JAR's exact SHA-256 before
executing it; downloading or running Java is never part of service startup.

```sh
python3 tools/es-language-parser/generate.py --antlr-jar /path/to/antlr-4.13.1-complete.jar
python3 tools/es-language-parser/generate.py --antlr-jar /path/to/antlr-4.13.1-complete.jar --check
python3 tools/es-language-parser/generate.py --check --verify-upstream
```

`--java /path/to/java` selects an explicit generation runtime. The last command
additionally compares the official immutable upstream grammar bytes over HTTPS.
Generated source headers preserve the upstream license notice. Do not edit the
generated files manually or enable another ES version by changing only its name.
Review grammar, native request parsing, execution effects and source positions
together before changing a profile.

The generation script deterministically removes ANTLR's unreachable trailing
`goto errorExit` markers and labels that become unused, then runs `gofmt`.
This Go-only cleanup allows `go vet` to pass; it changes neither the grammar nor
executable parser transitions. Generated hashes include this cleanup.
Git attributes keep the pinned grammar, license, manifest and generated Go files
in LF form so checkout newline conversion cannot invalidate their byte hashes.

## Analysis and resource contract

`eqlparser.Analyze` consumes `singleStatement` through EOF. It reports event,
sequence, sample or join syntax and the `request_indices` source contract.
Event categories, quoted identifiers, function names and string literals cannot
add index sources in this grammar. The enclosing adapter resolves REST indices,
native filter references and runtime lookup fields. EQL functions, types, pipe
availability and other semantic checks remain native-server decisions.

The lexer and parser use fresh per-call DFA/prediction caches; user input cannot
grow process-global caches across calls. Character/token access and parse-rule
entry check cancellation and work limits. Tokenized delimiter/prefix depth is
checked before parsing, with a second parse-rule depth bound. Hidden tokens and
EOF count toward the token budget. Error listeners fail without printing query
excerpts or sensitive literals. No background parser goroutine outlives a call.

The separate process parser pool is 128 MiB. Each parse reserves 2 MiB plus 64
times the UTF-8 input length, then 4 KiB before fetching each token. Success,
syntax failures, budget errors and cancellation all release the exact charge.
This conservative adapter accounting is not a guarantee about process RSS;
allocation benchmarks, adversarial tests and deployment measurements remain
part of qualification. Test and benchmark entry points:

```sh
go test -race ./internal/dialects/elasticsearch/eqlparser
go test ./internal/dialects/elasticsearch/eqlparser -run '^$' -fuzz FuzzEQLParser -fuzztime=8s -parallel=2
go test ./internal/dialects/elasticsearch/eqlparser -run '^$' -bench BenchmarkEQLParser -benchmem
```

## Licensing

`upstream/EqlBase.g4` and the derived generated Go files retain Elasticsearch's
Elastic License 2.0, whose complete text is in `upstream/LICENSE-Elastic-2.0.txt`.
They are not covered by this repository's MIT license. The ANTLR Go runtime has
its own BSD license. The handwritten adapter and parser wrapper remain under
the repository license.

## Native SQL profiles

SQL has distinct unmodified `SqlBase.g4` files under `upstream/sql8/` and
`upstream/sql9/`: the 8.19.21 grammar declares LP/RP lexer tokens, while 9.1.10
uses implicit literal tokens. Each is compiled into its own Go package. Do not
substitute one generated parser for the other or use a MySQL/PostgreSQL dialect.
`sql-manifest.json` records both source URLs, exact grammar hashes, the pinned
ANTLR artifact, the Apache 2.0 license file and all generated Go hashes.

```sh
python3 tools/es-language-parser/generate_sql.py --check
python3 tools/es-language-parser/generate_sql.py --antlr-jar /path/to/antlr-4.13.1-complete.jar
python3 tools/es-language-parser/generate_sql.py --antlr-jar /path/to/antlr-4.13.1-complete.jar --check
python3 tools/es-language-parser/generate_sql.py --check --verify-upstream
```

`--java` works as for EQL. SQL generation uses the same unreachable-marker
cleanup and LF checkout attributes. SQL grammar/generated files retain their
upstream Apache License 2.0 notices, including the Presto-parser origin; the
complete license is `upstream/LICENSE-Apache-2.0.txt`. These SQL notices differ
from the EQL grammar's Elastic License 2.0.

The SQL wrapper uppercases character lookahead while retaining original text,
consumes a complete statement through EOF and visits grammar nodes for every
relation, nested query, CTE definition/reference, metadata pattern and catalog.
Parameter tokens bind by token order without interpolating strings into SQL.
LIKE source expansion follows the native mapping resolver (`_` and `%` become
`*`, respecting native escapes). Dedicated full-text nodes provide inference
field selectors. Native postprocessor restrictions on backquoted/digit-starting
identifiers are retained; SQL function/type/plan availability stays with ES.

EQL and SQL share `languagebudget.Memory`: the 128 MiB process parser pool is
reserved once across both languages, using the same byte/token/depth/work and
cancellation guards. Both use fresh per-call DFA/prediction caches. SQL test,
fuzz and allocation entry points:

```sh
go test -race ./internal/dialects/elasticsearch/sqlparser
go test ./internal/dialects/elasticsearch/sqlparser -run '^$' -fuzz FuzzSQLParser -fuzztime=8s -parallel=2
go test ./internal/dialects/elasticsearch/sqlparser -run '^$' -bench BenchmarkSQLParser -benchmem
```

## Native ES|QL profiles

`generate_esql.py` preserves each pinned release's full lexer/parser and imports
under `upstream/esql8/` and `upstream/esql9/`. The 9.1 grammar imports separate
lexer and parser files with overlapping names; generation uses distinct ANTLR
library directories. `esql-manifest.json` records all twenty grammar hashes,
immutable URLs, generator/license hashes and generated Go outputs.

```sh
python3 tools/es-language-parser/generate_esql.py --check
python3 tools/es-language-parser/generate_esql.py --antlr-jar /path/to/antlr-4.13.1-complete.jar
python3 tools/es-language-parser/generate_esql.py --antlr-jar /path/to/antlr-4.13.1-complete.jar --check
python3 tools/es-language-parser/generate_esql.py --check --verify-upstream
```

`--java` selects a maintainer runtime as above. Vendored grammars are unchanged.
Besides the shared unreachable-marker cleanup, generated Go changes the spelling
of the Java `this.isDevVersion()` predicate to `p.IsDevVersion()`. Small generated
Go `LexerConfig`/`ParserConfig` bases return false, matching `EsqlConfig` on the
attested non-snapshot release builds. No development predicate is removed or
forced true. The grammar itself implements case-insensitivity; the character
stream preserves source case. Grammar and generated code retain Elastic License
2.0, independently of the handwritten adapter and SQL's Apache notices.

`esqlparser.Analyze` consumes a full statement, follows every `indexPattern` node
and groups source selectors by their native relation. Quoted index strings may
contain multiple selectors. ENRICH policy references and inference commands are
classified separately from index sources. Native anonymous, positional and named
parameters bind by token identity, including typed identifier/pattern parameters
and double-marker identifiers; they are never interpolated into source text.
Function/field bindings and assignment/rename lineage drive full-text inference
checks. Native release predicates distinguish 8.19 EXPLAIN from 9.1 FORK; parsing
is not a claim that every native function or plan is available in every license.

ES|QL uses the same cancellation, token, depth, work and per-call DFA guards and
shares the 128 MiB `languagebudget.Memory` pool. Its more expensive mode-based
lexer/grammar reserves **8 MiB** plus 64 times UTF-8 input bytes, then 4 KiB per
token. The SQL/EQL baselines remain 2 MiB. This is bounded admission accounting,
not a measured RSS guarantee. Resource forecasting still reserves the pool once.

```sh
go test -race ./internal/dialects/elasticsearch/esqlparser
go test ./internal/dialects/elasticsearch/esqlparser -run '^$' -fuzz FuzzESQLParser -fuzztime=10s -parallel=2
go test ./internal/dialects/elasticsearch/esqlparser -run '^$' -bench BenchmarkESQLParser -benchmem
```
