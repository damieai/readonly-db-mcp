# Elasticsearch phrase collate templates — 2026-09-23

Status: implemented with pinned 8.19.21 and 9.1.10 TLS fixture evidence. A live
Elasticsearch server has not yet been used for this qualification.

`es_query` search and search-template bodies now accept native phrase suggester
`collate` queries. Inline and readable stored Mustache sources, caller parameters
and `prune` are supported. Before dispatch, the adapter renders every fixed
parameter with the pinned server's `/_render/template`, inspects the complete
resulting Query DSL for external sources, and freezes it into an inline collate
template. The only remaining variable is the per-candidate `suggestion`, placed
in a whole JSON scalar with Mustache's `toJson` helper. The stored template ID
and caller parameters are removed from the executable search body, so template
changes or parameter substitution cannot change the proved source graph.

For example, a collate source `{"match":{"{{field_name}}":"{{suggestion}}"}}`
with `field_name: title` becomes a frozen query source containing
`{"match":{"title":{{#toJson}}suggestion{{/toJson}}}}`. Native `prune` semantics
are retained. Candidate-dependent field names, index names, script code,
sections and partial-string interpolation remain unavailable: their possible
query shape cannot be proved before the candidate is generated. This restriction
applies to dynamic executable/source positions, not to ordinary static complex
Query DSL, Mustache logic driven by fixed caller parameters, or suggestion text
used as a complete scalar value.

Fixtures cover both pinned builds, inline and stored templates, fixed parameter
rendering, frozen dispatch and candidate-controlled source/code rejection. They
do not establish actual Mustache rendering or per-shard collate behavior on a
real server. Live acceptance should compare a read-only phrase suggestion with
and without `prune`, including punctuation and backslashes in suggestion text,
on both builds. [Elastic's phrase suggester reference](https://www.elastic.co/docs/reference/elasticsearch/rest-apis/search-suggesters)
defines `collate`, and its [search-template reference](https://www.elastic.co/docs/reference/elasticsearch/rest-apis/search-template)
defines JSON-safe `toJson` substitution.
