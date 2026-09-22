package sqlparser

import (
	"strings"
	"unicode/utf8"

	"github.com/antlr4-go/antlr/v4"
)

// Inspect grammar nodes, never substrings of the original SQL. The native
// pre-analyzer inspects unresolved relations before CTE substitution, so retain
// both relation names and every CTE definition/subquery/join source. This also
// covers EXPLAIN/DEBUG, whose planning stages can resolve mappings.
type sourceVisitor struct {
	b          *budget
	rules      []string
	parameters map[int]any
	analysis   *Analysis
}

func (v *sourceVisitor) name(c antlr.ParserRuleContext) string { return v.rules[c.GetRuleIndex()] }
func (v *sourceVisitor) child(c antlr.ParserRuleContext, name string) antlr.ParserRuleContext {
	for _, t := range c.GetChildren() {
		if r, ok := t.(antlr.ParserRuleContext); ok && v.name(r) == name {
			return r
		}
	}
	return nil
}
func (v *sourceVisitor) walk(c antlr.ParserRuleContext) {
	v.b.step()
	switch v.name(c) {
	case "tableIdentifier":
		v.analysis.Sources = append(v.analysis.Sources, v.table(c))
		return
	case "statement":
		v.statement(c)
	case "booleanExpression":
		v.fullText(c)
	case "backQuotedIdentifier", "digitIdentifier":
		stop("invalid_request") // native postprocessor rejects these
	}
	for _, t := range c.GetChildren() {
		if r, ok := t.(antlr.ParserRuleContext); ok {
			v.walk(r)
		}
	}
}

func (v *sourceVisitor) fullText(c antlr.ParserRuleContext) {
	// Only the grammar's dedicated QUERY/MATCH alternatives have a direct
	// string child; parent expressions must not re-interpret their descendants.
	if v.child(c, "string") == nil {
		return
	}
	switch strings.ToUpper(c.GetStart().GetText()) {
	case "QUERY":
		v.analysis.FullTextFields = append(v.analysis.FullTextFields, "*")
	case "MATCH":
		if name := v.child(c, "qualifiedName"); name != nil {
			var parts []string
			for _, t := range name.GetChildren() {
				if r, ok := t.(antlr.ParserRuleContext); ok {
					parts = append(parts, identifier(r.GetText()))
				}
			}
			// SQL table aliases can prefix the real field. Check both the full
			// path and possible alias-stripped paths against semantic mappings.
			for i := range parts {
				v.analysis.FullTextFields = append(v.analysis.FullTextFields, strings.Join(parts[i:], "."))
			}
		} else {
			for _, field := range strings.FieldsFunc(v.text(v.child(c, "string")), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
				v.analysis.FullTextFields = append(v.analysis.FullTextFields, strings.Split(field, "^")[0])
			}
			if len(v.analysis.FullTextFields) == 0 {
				v.analysis.FullTextFields = append(v.analysis.FullTextFields, "*")
			}
		}
	}
}
func identifier(text string) string {
	if strings.HasPrefix(text, "\"") && strings.HasSuffix(text, "\"") {
		return strings.ReplaceAll(text[1:len(text)-1], "\"\"", "\"")
	}
	if strings.HasPrefix(text, "`") {
		stop("invalid_request")
	}
	if len(text) > 0 && text[0] >= '0' && text[0] <= '9' {
		stop("invalid_request")
	}
	return text
}
func (v *sourceVisitor) table(c antlr.ParserRuleContext) Source {
	var values []string
	catalog, selector := false, false
	for _, t := range c.GetChildren() {
		switch x := t.(type) {
		case antlr.ParserRuleContext:
			values = append(values, identifier(x.GetText()))
		case antlr.TerminalNode:
			switch x.GetText() {
			case ":":
				catalog = true
			case "::":
				selector = true
			default:
				values = append(values, identifier(x.GetText()))
			}
		}
	}
	i := 0
	result := Source{}
	if catalog {
		result.Catalog = values[i]
		i++
	}
	if i >= len(values) {
		stop("invalid_request")
	}
	result.Index = values[i]
	i++
	if selector {
		if i >= len(values) {
			stop("invalid_request")
		}
		result.Index += "::" + values[i]
	}
	return result
}
func (v *sourceVisitor) text(c antlr.ParserRuleContext) string {
	if c == nil {
		stop("invalid_request")
	}
	s := c.GetText()
	if s == "?" {
		value, ok := v.parameters[c.GetStart().GetTokenIndex()].(string)
		if !ok {
			stop("invalid_request")
		}
		return value
	}
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		stop("invalid_request")
	}
	return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
}
func (v *sourceVisitor) pattern(c antlr.ParserRuleContext) (string, string) {
	p := v.child(c, "pattern")
	raw := v.text(v.child(p, "string"))
	var escape rune
	if e := v.child(p, "patternEscape"); e != nil {
		s := v.text(v.child(e, "string"))
		if utf8.RuneCountInString(s) != 1 {
			stop("invalid_request")
		}
		escape, _ = utf8.DecodeRuneInString(s)
	}
	var result strings.Builder
	escaped := false
	for _, r := range raw {
		v.b.step()
		if !escaped && escape != 0 && r == escape {
			escaped = true
			continue
		}
		switch r {
		case '%':
			if escaped {
				result.WriteRune(r)
			} else {
				result.WriteByte('*')
			}
		case '_':
			if escaped {
				result.WriteRune(r)
			} else {
				result.WriteByte('*')
			} // native likeToIndexWildcard uses *, not ?
		default:
			if escaped {
				stop("invalid_request")
			}
			result.WriteRune(r)
		}
		escaped = false
	}
	if escaped {
		stop("invalid_request")
	}
	return result.String(), raw
}
func (v *sourceVisitor) statement(c antlr.ParserRuleContext) {
	catalogStart := len(v.analysis.Catalogs)
	var keywords []string
	for _, t := range c.GetChildren() {
		if x, ok := t.(antlr.TerminalNode); ok {
			keywords = append(keywords, strings.ToUpper(x.GetText()))
		}
	}
	if len(keywords) == 0 {
		return
	}
	first := keywords[0]
	if first == "EXPLAIN" || first == "DEBUG" {
		v.analysis.Kind = "plan"
		return
	}
	if first != "SHOW" && first != "SYS" && first != "DESC" && first != "DESCRIBE" {
		return
	}
	v.analysis.Kind = "metadata"
	second := ""
	if len(keywords) > 1 {
		second = keywords[1]
	}
	if first == "SHOW" && (second == "FUNCTIONS" || second == "SCHEMAS" || second == "CATALOGS") || first == "SYS" && second == "TYPES" {
		return
	}
	columns := first == "DESC" || first == "DESCRIBE" || second == "COLUMNS"
	catalogPending, tablePending := false, false
	hasTable := v.child(c, "tableIdentifier") != nil
	tablePattern, tableRaw := "*", "*"
	clusterRaw := ""
	hasCluster := false
	typesNil := true
	for _, t := range c.GetChildren() {
		if x, ok := t.(antlr.TerminalNode); ok {
			switch strings.ToUpper(x.GetText()) {
			case "CATALOG":
				catalogPending = true
			case "TABLE", "FROM", "IN":
				tablePending = true
			}
			continue
		}
		r, ok := t.(antlr.ParserRuleContext)
		if !ok {
			continue
		}
		name := v.name(r)
		if catalogPending && (name == "string" || name == "likePattern") {
			value := ""
			if name == "string" {
				value = v.text(r)
				clusterRaw = value
			} else {
				value, clusterRaw = v.pattern(r)
			}
			v.analysis.Catalogs = append(v.analysis.Catalogs, Catalog{Value: value, Pattern: name == "likePattern"})
			hasCluster = true
			catalogPending = false
			continue
		}
		if name == "likePattern" && (!columns || first != "SYS" || tablePending) {
			tablePattern, tableRaw = v.pattern(r)
			hasTable = true
			tablePending = false
		} else if name == "string" && first == "SYS" && second == "TABLES" {
			s := v.text(r)
			if s != "" && s != "%" {
				typesNil = false
			}
		}
	}
	// The native SYS TABLES empty-pattern forms enumerate catalogs/types without
	// touching any index. All other omitted selectors really mean *, not the
	// configured index scope, and must pass the ordinary scope proof unchanged.
	if first == "SYS" && second == "TABLES" && tableRaw == "" && typesNil && (!hasCluster || clusterRaw == "" || clusterRaw == "%") {
		v.analysis.Catalogs = v.analysis.Catalogs[:catalogStart]
		return
	}
	if v.child(c, "tableIdentifier") == nil {
		v.analysis.Sources = append(v.analysis.Sources, Source{Index: tablePattern})
	} else if !hasTable {
		stop("invalid_request")
	}
}
