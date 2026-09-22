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
