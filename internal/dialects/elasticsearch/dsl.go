package elasticsearch

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func object(v any) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return nil, failure("invalid_request", "expected a native JSON object")
	}
	return m, nil
}

func (p *queryProof) step(depth int) error {
	p.work++
	if p.ctx.Err() != nil {
		return failure("deadline_exceeded", "query proof exceeded deadline")
	}
	if depth > p.t.cfg.Elasticsearch.MaxJSONDepth || p.work > p.t.cfg.Elasticsearch.MaxJSONNodes {
		return failure("resource_limit", "query proof exceeds traversal budget")
	}
	return nil
}

func (p *queryProof) search(m map[string]any, rows, depth int) error {
	if err := p.step(depth); err != nil {
		return err
	}
	if _, ok := m["pit"]; ok {
		return failure("capability_unavailable", "PIT requires a service-owned context")
	}
	if err := fieldsOnly(m, "query post_filter aggs aggregations runtime_mappings sort rescore knn retriever suggest highlight script_fields collapse size from timeout terminate_after track_total_hits track_scores _source fields docvalue_fields stored_fields explain profile search_after slice min_score indices_boost version seq_no_primary_term stats rank boost"); err != nil {
		return err
	}
	if err := pageSize(m, rows, 10); err != nil {
		return err
	}
	for _, key := range []string{"query", "post_filter"} {
		if v, ok := m[key]; ok {
			if err := p.query(v, rows, depth+1); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"aggs", "aggregations"} {
		if v, ok := m[key]; ok {
			if err := p.aggregations(v, rows, depth+1); err != nil {
				return err
			}
		}
	}
	if v, ok := m["runtime_mappings"]; ok {
		if err := p.runtime(v); err != nil {
			return err
		}
	}
	if v, ok := m["sort"]; ok {
		if err := p.sort(v); err != nil {
			return err
		}
	}
	if v, ok := m["knn"]; ok {
		for _, item := range list(v) {
			if err := p.knn(item, rows, depth+1); err != nil {
				return err
			}
		}
	}
	if v, ok := m["retriever"]; ok {
		if err := p.retriever(v, rows, depth+1); err != nil {
			return err
		}
	}
	if v, ok := m["highlight"]; ok {
		if err := p.highlight(v, rows, depth+1); err != nil {
			return err
		}
	}
	if v, ok := m["script_fields"]; ok {
		fields, err := object(v)
		if err != nil {
			return err
		}
		for _, field := range fields {
			f, err := object(field)
			if err != nil {
				return err
			}
			if err := p.scriptAt(f, "script"); err != nil {
				return err
			}
		}
	}
	if v, ok := m["rescore"]; ok {
		for _, item := range list(v) {
			r, err := object(item)
			if err != nil {
				return err
			}
			if err := fieldsOnly(r, "window_size query"); err != nil {
				return err
			}
			if v, ok := r["query"]; ok {
				q, err := object(v)
				if err != nil {
					return err
				}
				if err := p.query(q["rescore_query"], rows, depth+1); err != nil {
					return err
				}
			}
		}
	}
	if v, ok := m["collapse"]; ok {
		c, err := object(v)
		if err != nil {
			return err
		}
		if err := p.innerHits(c, rows, depth+1); err != nil {
			return err
		}
	}
	if v, ok := m["suggest"]; ok {
		if err := p.suggest(v, rows, depth+1); err != nil {
			return err
		}
	}
	if v, ok := m["rank"]; ok {
		r, err := object(v)
		if err != nil {
			return err
		}
		if err := fieldsOnly(r, "rrf"); err != nil {
			return failure("capability_unavailable", "ranker requires an inference/plugin profile")
		}
	}
	if v, ok := m["indices_boost"]; ok {
		for _, item := range list(v) {
			boosts, err := object(item)
			if err != nil {
				return err
			}
			for index := range boosts {
				if _, err := p.sources([]string{index}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func list(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return []any{v}
}

func pageSize(m map[string]any, limit, fallback int) error {
	size := fallback
	if v, ok := m["size"]; ok {
		n, ok := v.(json.Number)
		if !ok {
			return failure("invalid_request", "size must be an integer")
		}
		i, err := n.Int64()
		if err != nil || i < 0 || i > int64(limit) {
			return failure("resource_limit", "requested hit page exceeds max_rows; use search_after or a smaller explicit page")
		}
		size = int(i)
	}
	if size > limit {
		return failure("resource_limit", "native default hit page exceeds max_rows; set an explicit size")
	}
	return nil
}

// Only executable grammar positions are traversed. User field names, literals,
// artificial documents, script params, metadata and vector values stay opaque.
// Unknown query/plugin names are unavailable profiles, never called mutations.
func (p *queryProof) query(v any, rows, depth int) error {
	if err := p.step(depth); err != nil {
		return err
	}
	m, err := object(v)
	if err != nil {
		return err
	}
	if len(m) != 1 {
		return failure("invalid_request", "query container must name one native query")
	}
	for kind, value := range m {
		q, err := object(value)
		if err != nil {
			return err
		}
		children := func(keys ...string) error {
			for _, key := range keys {
				if v, ok := q[key]; ok {
					for _, child := range list(v) {
						if err := p.query(child, rows, depth+1); err != nil {
							return err
						}
					}
				}
			}
			return nil
		}
		switch kind {
		case "bool":
			return children("must", "filter", "should", "must_not")
		case "boosting":
			return children("positive", "negative")
		case "constant_score":
			return children("filter")
		case "dis_max":
			return children("queries")
		case "nested", "has_child", "has_parent":
			if err := children("query"); err != nil {
				return err
			}
			return p.innerHits(q, rows, depth+1)
		case "function_score":
			if err := children("query"); err != nil {
				return err
			}
			if err := p.scoreFunction(q, rows, depth+1); err != nil {
				return err
			}
			if functions, ok := q["functions"]; ok {
				for _, f := range list(functions) {
					fn, err := object(f)
					if err != nil {
						return err
					}
					if err := p.scoreFunction(fn, rows, depth+1); err != nil {
						return err
					}
				}
			}
		case "script_score":
			if err := children("query"); err != nil {
				return err
			}
			return p.scriptAt(q, "script")
		case "script":
			return p.scriptAt(q, "script")
		case "terms":
			for field, v := range q {
				if field == "boost" || field == "_name" {
					continue
				}
				if lookup, ok := v.(map[string]any); ok {
					if err := p.reference(lookup, "index", ""); err != nil {
						return err
					}
				}
			}
		case "geo_shape":
			for _, v := range q {
				if field, ok := v.(map[string]any); ok {
					if v, ok := field["indexed_shape"]; ok {
						shape, err := object(v)
						if err != nil {
							return err
						}
						if err := p.reference(shape, "index", "shapes"); err != nil {
							return err
						}
					}
				}
			}
		case "percolate":
			if _, ok := q["id"]; ok {
				if err := p.reference(q, "index", ""); err != nil {
					return err
				}
			}
		case "more_like_this":
			for _, key := range []string{"like", "unlike"} {
				if values, ok := q[key]; ok {
					for _, item := range list(values) {
						if doc, ok := item.(map[string]any); ok {
							if _, ok := doc["_index"]; ok {
								if err := p.reference(doc, "_index", ""); err != nil {
									return err
								}
							}
						}
					}
				}
			}
		case "pinned":
			if err := children("organic"); err != nil {
				return err
			}
			if docs, ok := q["docs"]; ok {
				for _, v := range list(docs) {
					doc, err := object(v)
					if err != nil {
						return err
					}
					if _, ok := doc["_index"]; ok {
						if err := p.reference(doc, "_index", ""); err != nil {
							return err
						}
					}
				}
			}
		case "wrapper":
			encoded, ok := q["query"].(string)
			if !ok {
				return failure("invalid_request", "wrapper query must be base64 JSON")
			}
			raw, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return failure("invalid_request", "invalid wrapper base64")
			}
			if err := strictJSONContext(p.ctx, raw, p.t.cfg.Elasticsearch.MaxJSONDepth-depth, p.t.cfg.Elasticsearch.MaxJSONNodes-p.work, 0); err != nil {
				return err
			}
			wrapped, err := decodeObject(raw)
			if err != nil {
				return err
			}
			if err := p.query(wrapped, rows, depth+1); err != nil {
				return err
			}
			// Stored scripts inside wrappers are frozen just as ordinary scripts.
			raw, _ = json.Marshal(wrapped)
			q["query"] = base64.StdEncoding.EncodeToString(raw)
		case "span_first", "field_masking_span":
			return children("match", "query")
		case "span_multi":
			return children("match")
		case "span_near", "span_or":
			return children("clauses")
		case "span_not":
			return children("include", "exclude")
		case "span_containing", "span_within":
			return children("big", "little")
		case "intervals":
			for field, v := range q {
				if field == "boost" || field == "_name" {
					continue
				}
				if err := p.intervals(v, depth+1); err != nil {
					return err
				}
			}
		case "knn":
			return p.knn(q, rows, depth+1)
		case "sparse_vector":
			if _, ok := q["query_vector"]; !ok {
				return failure("capability_unavailable", "text inference requires an explicit endpoint/model profile")
			}
			if _, ok := q["inference_id"]; ok {
				return failure("capability_unavailable", "inference profile is unavailable")
			}
			if _, ok := q["query"]; ok {
				return failure("capability_unavailable", "text inference requires an explicit endpoint/model profile")
			}
		case "semantic", "text_expansion", "rule", "rule_query":
			return failure("capability_unavailable", "inference or stored-rule source profile is unavailable")
		case "match", "match_phrase", "match_phrase_prefix", "match_bool_prefix", "multi_match", "combined_fields", "query_string", "simple_query_string":
			// Semantic-text fields can turn apparently ordinary matches into remote
			// inference. Check mappings before dispatch, including default fields.
			if err := p.localTextFields(kind, q); err != nil {
				return err
			}
		case "match_all", "match_none", "ids", "term", "range", "exists", "prefix", "wildcard", "regexp", "fuzzy", "span_term", "span_gap", "distance_feature", "rank_feature", "rank_features", "geo_bounding_box", "geo_distance", "geo_polygon", "shape", "parent_id", "type", "weighted_tokens":
			// Native source-local leaves: their strings/field values are data.
		default:
			return failure("capability_unavailable", "query type requires a reviewed source/effect profile")
		}
	}
	return nil
}

func (p *queryProof) reference(m map[string]any, key, fallback string) error {
	name := fallback
	if v, ok := m[key]; ok {
		var valid bool
		name, valid = v.(string)
		if !valid {
			return failure("invalid_request", "embedded index must be a string")
		}
	}
	if name == "" {
		return failure("invalid_request", "embedded lookup requires an explicit index")
	}
	physical, err := p.sources([]string{name})
	if err == nil {
		p.embedded[name] = physical
	}
	return err
}

func (p *queryProof) scoreFunction(m map[string]any, rows, depth int) error {
	if v, ok := m["filter"]; ok {
		if err := p.query(v, rows, depth+1); err != nil {
			return err
		}
	}
	if v, ok := m["script_score"]; ok {
		script, err := object(v)
		if err != nil {
			return err
		}
		if err := p.scriptAt(script, "script"); err != nil {
			return err
		}
	}
	// These are the native source-local scoring functions. A plugin function
	// needs its own effect profile even when attached to a built-in query.
	for key := range m {
		switch key {
		case "filter", "script_score", "random_score", "field_value_factor", "gauss", "linear", "exp", "weight", "query", "functions", "score_mode", "boost_mode", "max_boost", "min_score", "boost", "_name":
		default:
			return failure("capability_unavailable", "custom score function requires a plugin profile")
		}
	}
	return nil
}

func (p *queryProof) innerHits(m map[string]any, rows, depth int) error {
	if v, ok := m["inner_hits"]; ok {
		for _, item := range list(v) {
			h, err := object(item)
			if err != nil {
				return err
			}
			copy := map[string]any{}
			for k, v := range h {
				if k != "name" && k != "ignore_unmapped" {
					copy[k] = v
				}
			}
			if _, ok := copy["size"]; !ok {
				copy["size"] = json.Number("3")
			}
			if err := p.search(copy, rows, depth+1); err != nil {
				return err
			}
			for k, v := range copy {
				h[k] = v
			}
		}
	}
	return nil
}

func (p *queryProof) knn(v any, rows, depth int) error {
	m, err := object(v)
	if err != nil {
		return err
	}
	if _, ok := m["query_vector_builder"]; ok {
		return failure("capability_unavailable", "vector builders require an inference/plugin profile; supplied vectors are supported")
	}
	if filter, ok := m["filter"]; ok {
		for _, q := range list(filter) {
			if err := p.query(q, rows, depth+1); err != nil {
				return err
			}
		}
	}
	return p.innerHits(m, rows, depth+1)
}

func (p *queryProof) retriever(v any, rows, depth int) error {
	if err := p.step(depth); err != nil {
		return err
	}
	m, err := object(v)
	if err != nil {
		return err
	}
	if len(m) != 1 {
		return failure("invalid_request", "retriever must name one type")
	}
	for kind, value := range m {
		r, err := object(value)
		if err != nil {
			return err
		}
		if filter, ok := r["filter"]; ok {
			for _, q := range list(filter) {
				if err := p.query(q, rows, depth+1); err != nil {
					return err
				}
			}
		}
		switch kind {
		case "standard":
			if q, ok := r["query"]; ok {
				if err := p.query(q, rows, depth+1); err != nil {
					return err
				}
			}
			if sort, ok := r["sort"]; ok {
				if err := p.sort(sort); err != nil {
					return err
				}
			}
			if value, ok := r["collapse"]; ok {
				collapse, err := object(value)
				if err != nil {
					return err
				}
				if err := p.innerHits(collapse, rows, depth+1); err != nil {
					return err
				}
			}
		case "knn":
			if err := p.knn(r, rows, depth+1); err != nil {
				return err
			}
		case "rrf", "linear":
			children, ok := r["retrievers"].([]any)
			if !ok {
				return failure("invalid_request", "fusion requires retrievers")
			}
			for _, child := range children {
				if kind == "linear" {
					weighted, err := object(child)
					if err != nil {
						return err
					}
					child = weighted["retriever"]
				}
				if err := p.retriever(child, rows, depth+1); err != nil {
					return err
				}
			}
		case "pinned":
			if err := p.retriever(r["retriever"], rows, depth+1); err != nil {
				return err
			}
			if docs, ok := r["docs"]; ok {
				for _, v := range list(docs) {
					doc, err := object(v)
					if err != nil {
						return err
					}
					if _, ok := doc["_index"]; ok {
						if err := p.reference(doc, "_index", ""); err != nil {
							return err
						}
					}
				}
			}
		case "rescorer":
			if err := p.retriever(r["retriever"], rows, depth+1); err != nil {
				return err
			}
			if err := p.search(map[string]any{"size": json.Number("0"), "rescore": r["rescore"]}, rows, depth+1); err != nil {
				return err
			}
		default:
			return failure("capability_unavailable", "retriever requires an inference/plugin profile")
		}
	}
	return nil
}

func (p *queryProof) scriptAt(m map[string]any, key string) error {
	v, ok := m[key]
	if !ok {
		return nil
	}
	if source, ok := v.(string); ok {
		m[key] = map[string]any{"source": source, "lang": "painless"}
		return nil
	}
	s, err := object(v)
	if err != nil {
		return err
	}
	if err := fieldsOnly(s, "id source lang params options"); err != nil {
		return err
	}
	if id, exists := s["id"]; exists {
		name, ok := id.(string)
		if !ok || name == "" || len(name) > 1024 {
			return failure("invalid_request", "invalid stored script id")
		}
		if _, exists := s["source"]; exists {
			return failure("invalid_request", "script id and source are mutually exclusive")
		}
		stored, err := p.storedScript(name)
		if err != nil {
			return err
		}
		frozen := map[string]any{}
		for k, v := range stored {
			frozen[k] = v
		}
		if params, ok := s["params"]; ok {
			frozen["params"] = params
		}
		if lang, ok := s["lang"]; ok && lang != stored["lang"] {
			return failure("invalid_request", "stored script language differs")
		}
		if options, ok := s["options"]; ok {
			frozen["options"] = options
		}
		s = frozen
		m[key] = s
	}
	lang := "painless"
	if v, ok := s["lang"]; ok {
		lang, _ = v.(string)
	}
	if lang != "painless" && lang != "expression" {
		return failure("capability_unavailable", "script language requires a reviewed sandbox/effect profile")
	}
	if _, ok := s["source"].(string); !ok {
		return failure("invalid_request", "script requires source or a readable stored id")
	}
	return nil
}

func (p *queryProof) storedScript(id string) (map[string]any, error) {
	if s, ok := p.scripts[id]; ok {
		return s, nil
	}
	if len(p.scripts) >= p.t.limits.MaxParameters {
		return nil, failure("resource_limit", "stored script proof count exceeds limit")
	}
	if err := p.t.ready(); err != nil {
		return nil, err
	}
	raw, err := p.t.wire.request(p.ctx, p.endpoint, http.MethodGet, "/_scripts/"+url.PathEscape(id), nil, nil, "application/json", p.t.limits.MaxResultBytes, false)
	if err != nil {
		return nil, err
	}
	if err := p.retainProof(len(raw)); err != nil {
		return nil, err
	}
	root, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	s, err := object(root["script"])
	if err != nil {
		return nil, err
	}
	if err := fieldsOnly(s, "source lang options"); err != nil {
		return nil, err
	}
	p.scripts[id] = s
	return s, nil
}

func (p *queryProof) render(body map[string]any) (map[string]any, error) {
	if err := fieldsOnly(body, "source id params explain profile"); err != nil {
		return nil, err
	}
	for _, key := range []string{"explain", "profile"} {
		if value, exists := body[key]; exists {
			if _, ok := value.(bool); !ok {
				return nil, failure("invalid_request", "template explain/profile controls must be boolean")
			}
		}
	}
	if id, ok := body["id"]; ok {
		if _, ok := body["source"]; ok {
			return nil, failure("invalid_request", "template source and id are mutually exclusive")
		}
		name, ok := id.(string)
		if !ok || name == "" || len(name) > 1024 {
			return nil, failure("invalid_request", "invalid stored template id")
		}
		s, err := p.storedScript(name)
		if err != nil {
			return nil, err
		}
		if s["lang"] != "mustache" {
			return nil, failure("capability_unavailable", "stored template must use Mustache")
		}
		delete(body, "id")
		body["source"] = s["source"]
	}
	if _, ok := body["source"]; !ok {
		return nil, failure("invalid_request", "template requires source or id")
	}
	raw, _ := json.Marshal(body)
	if err := p.t.ready(); err != nil {
		return nil, err
	}
	raw, err := p.t.wire.request(p.ctx, p.endpoint, http.MethodPost, "/_render/template", nil, raw, "application/json", p.t.cfg.Elasticsearch.MaxRequestBytes, false)
	if err != nil {
		return nil, err
	}
	rendered, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	return object(rendered["template_output"])
}

func (p *queryProof) runtime(v any) error {
	fields, err := object(v)
	if err != nil {
		return err
	}
	for _, v := range fields {
		field, err := object(v)
		if err != nil {
			return err
		}
		if field["type"] == "lookup" {
			if err := p.reference(field, "target_index", ""); err != nil {
				return err
			}
		}
		if err := p.scriptAt(field, "script"); err != nil {
			return err
		}
	}
	return nil
}

func (p *queryProof) sort(v any) error {
	for _, item := range list(v) {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for name, value := range m {
			options, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if name == "_script" {
				if err := p.scriptAt(options, "script"); err != nil {
					return err
				}
			}
			if nested, ok := options["nested"]; ok {
				if err := p.nestedSort(nested, 0); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (p *queryProof) nestedSort(v any, depth int) error {
	if err := p.step(depth); err != nil {
		return err
	}
	m, err := object(v)
	if err != nil {
		return err
	}
	if filter, ok := m["filter"]; ok {
		if err := p.query(filter, p.t.limits.MaxRows, depth+1); err != nil {
			return err
		}
	}
	if nested, ok := m["nested"]; ok {
		return p.nestedSort(nested, depth+1)
	}
	return nil
}

func (p *queryProof) highlight(v any, rows, depth int) error {
	m, err := object(v)
	if err != nil {
		return err
	}
	if q, ok := m["highlight_query"]; ok {
		if err := p.query(q, rows, depth+1); err != nil {
			return err
		}
	}
	if v, ok := m["fields"]; ok {
		fields, err := object(v)
		if err != nil {
			return err
		}
		for _, v := range fields {
			f, err := object(v)
			if err != nil {
				return err
			}
			if q, ok := f["highlight_query"]; ok {
				if err := p.query(q, rows, depth+1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (p *queryProof) intervals(v any, depth int) error {
	if err := p.step(depth); err != nil {
		return err
	}
	m, err := object(v)
	if err != nil {
		return err
	}
	for kind, v := range m {
		r, err := object(v)
		if err != nil {
			return err
		}
		switch kind {
		case "match", "prefix", "wildcard", "fuzzy", "range", "regexp":
		case "all_of", "any_of":
			for _, child := range list(r["intervals"]) {
				if err := p.intervals(child, depth+1); err != nil {
					return err
				}
			}
		default:
			return failure("capability_unavailable", "interval rule requires a reviewed profile")
		}
		if v, ok := r["filter"]; ok {
			filter, err := object(v)
			if err != nil {
				return err
			}
			for kind, child := range filter {
				if kind == "script" {
					if err := p.scriptAt(filter, kind); err != nil {
						return err
					}
				} else {
					if err := p.intervals(child, depth+1); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (p *queryProof) suggest(v any, rows, depth int) error {
	m, err := object(v)
	if err != nil {
		return err
	}
	for name, value := range m {
		if name == "text" {
			continue
		}
		s, err := object(value)
		if err != nil {
			return err
		}
		if err := fieldsOnly(s, "text prefix regex term phrase completion"); err != nil {
			return err
		}
		if value, ok := s["phrase"]; ok {
			phrase, err := object(value)
			if err != nil {
				return err
			}
			if value, ok := phrase["collate"]; ok {
				if err := p.prepareCollate(phrase, value, rows, depth+1); err != nil {
					return err
				}
			}
		}
		// Ordinary term/phrase/completion suggestions have no external sources.
	}
	return nil
}

func (p *queryProof) documents(body map[string]any, indices []string, rows int, operation string) error {
	if err := fieldsOnly(body, "docs ids"); err != nil {
		return err
	}
	if _, ok := body["docs"]; ok {
		if _, also := body["ids"]; also {
			return failure("invalid_request", "docs and ids are mutually exclusive")
		}
	}
	if ids, ok := body["ids"]; ok {
		a, ok := ids.([]any)
		if !ok || len(a) > rows {
			return failure("resource_limit", "document count exceeds max_rows")
		}
		if len(indices) != 1 || !concreteName(indices[0]) {
			return failure("invalid_request", "ids require one concrete index or alias")
		}
		for _, id := range a {
			if _, ok := id.(string); !ok {
				return failure("invalid_request", "document ids must be strings")
			}
		}
	}
	if v, ok := body["docs"]; ok {
		docs, ok := v.([]any)
		if !ok || len(docs) > rows {
			return failure("resource_limit", "document count exceeds max_rows")
		}
		for _, v := range docs {
			doc, err := object(v)
			if err != nil {
				return err
			}
			allowed := "_index _id routing _source _source_includes _source_excludes stored_fields version version_type"
			if operation == "mtermvectors" {
				allowed += " doc fields offsets positions payloads term_statistics field_statistics per_field_analyzer filter termStatistics fieldStatistics perFieldAnalyzer"
			}
			if err := fieldsOnly(doc, allowed); err != nil {
				return err
			}
			if index, ok := doc["_index"]; ok {
				name, ok := index.(string)
				if !ok || !concreteName(name) {
					return failure("invalid_request", "document index must be concrete")
				}
				if err := p.reference(doc, "_index", ""); err != nil {
					return err
				}
			} else if len(indices) != 1 || !concreteName(indices[0]) {
				return failure("invalid_request", "each document needs an index or one concrete default")
			}
		}
	}
	if _, ok := body["docs"]; !ok {
		if _, ok := body["ids"]; !ok {
			return failure("invalid_request", "multi-document read requires docs or ids")
		}
	}
	return nil
}

// Mapping inspection is deliberately separate from DSL rewriting. Match on a
// semantic_text field can invoke inference even without an explicit semantic
// query. Other fields on the same index remain available.
func (p *queryProof) localTextFields(kind string, q map[string]any) error {
	var patterns []string
	switch kind {
	case "multi_match", "combined_fields", "query_string", "simple_query_string":
		if fields, ok := q["fields"].([]any); ok {
			for _, v := range fields {
				if s, ok := v.(string); ok {
					patterns = append(patterns, strings.Split(s, "^")[0])
				}
			}
		}
		if field, ok := q["default_field"].(string); ok {
			patterns = append(patterns, field)
		}
		// Query-string can name fields inside the expression; mapping-wide proof
		// is needed until a native query-string field visitor is supplied.
		if kind == "query_string" || kind == "simple_query_string" {
			patterns = nil
		}
	default:
		for name := range q {
			if name != "boost" && name != "_name" {
				patterns = append(patterns, name)
			}
		}
	}
	if len(patterns) == 0 {
		patterns = []string{"*"}
	}
	root, err := p.sourceMappings(p.activeIndices)
	if err != nil {
		return err
	}
	for _, index := range root {
		m, _ := index.(map[string]any)
		mapping, _ := m["mappings"].(map[string]any)
		if err := semanticMappingFields(p, mapping, patterns); err != nil {
			return err
		}
	}
	return nil
}
