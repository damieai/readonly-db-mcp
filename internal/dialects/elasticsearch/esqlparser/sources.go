package esqlparser

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"

	"github.com/antlr4-go/antlr/v4"
)

type parameter struct {
	value any
	kind  string
}
type parameters struct {
	values    []parameter
	named     map[string]parameter
	anonymous int
	style     string
}

// Match RequestXContent and ParametrizedTokenSource. Native parameters never
// substitute command/source text, but can supply function and field identifiers.
func newParameters(raw []any) *parameters {
	p := &parameters{named: map[string]parameter{}}
	for _, value := range raw {
		name := ""
		if object, ok := value.(map[string]any); ok {
			if len(object) != 1 {
				stop("invalid_request")
			}
			for key, v := range object {
				name, value = key, v
			}
			if name == "" {
				stop("invalid_request")
			}
			for i, r := range name {
				if r != '_' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
					stop("invalid_request")
				}
			}
		}
		param := parameter{value: value, kind: "VALUE"}
		if object, ok := value.(map[string]any); ok {
			if len(object) != 1 {
				stop("invalid_request")
			}
			for key, v := range object {
				param.kind, param.value = strings.ToUpper(key), v
			}
		}
		switch param.kind {
		case "VALUE":
			switch param.value.(type) {
			case nil, string, bool, json.Number:
			default:
				stop("invalid_request")
			}
		case "IDENTIFIER", "PATTERN":
			s, ok := param.value.(string)
			if !ok || (param.kind == "PATTERN" && !strings.Contains(s, "*")) {
				stop("invalid_request")
			}
		default:
			stop("invalid_request")
		}
		p.values = append(p.values, param)
		if name != "" {
			if _, exists := p.named[name]; exists {
				stop("invalid_request")
			}
			p.named[name] = param
		}
	}
	if len(p.named) != 0 && len(p.named) != len(p.values) {
		stop("invalid_request")
	}
	return p
}
func (p *parameters) bind(text string) parameter {
	key := strings.TrimLeft(text, "?")
	style := "named"
	position := 0
	if key == "" {
		style = "anonymous"
		p.anonymous++
		position = p.anonymous
	} else if key[0] >= '0' && key[0] <= '9' {
		style = "positional"
		var err error
		position, err = strconv.Atoi(key)
		if err != nil {
			stop("invalid_request")
		}
	}
	if p.style != "" && p.style != style {
		stop("invalid_request")
	}
	p.style = style
	if style != "named" {
		if position < 1 || position > len(p.values) {
			stop("invalid_request")
		}
		return p.values[position-1]
	}
	param, ok := p.named[key]
	if !ok {
		stop("invalid_request")
	}
	return param
}

type sourceVisitor struct {
	b               *budget
	rules           []string
	parameters      map[int]parameter
	analysis        *Analysis
	depth, maxDepth int
	groups          map[antlr.Tree]int
	aliases         map[string][]string
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
	v.depth++
	defer func() { v.depth-- }()
	if v.depth > v.maxDepth {
		stop("resource_limit")
	}
	switch v.name(c) {
	case "indexPattern":
		// All FROM, JOIN and nested EXPLAIN/FORK relations use this node.
		// Quoted sources can contain comma-separated selectors too.
		s := c.GetText()
		if child := v.child(c, "indexString"); child != nil {
			s = unquote(child.GetText())
		}
		group, exists := v.groups[c.GetParent()]
		if !exists {
			group = len(v.analysis.SourceGroups)
			v.groups[c.GetParent()] = group
			v.analysis.SourceGroups = append(v.analysis.SourceGroups, nil)
		}
		for _, source := range strings.Split(s, ",") {
			source = strings.TrimSpace(source)
			if source == "" {
				stop("invalid_request")
			}
			v.analysis.Sources = append(v.analysis.Sources, source)
			v.analysis.SourceGroups[group] = append(v.analysis.SourceGroups[group], source)
		}
		return
	case "enrichPolicyName":
		// LogicalPlanBuilder.parsePolicyName strips exactly one quote at each
		// end; unlike indexString, it does not unescape or trim triple quotes.
		name := c.GetText()
		if strings.HasPrefix(name, `"`) {
			name = name[1 : len(name)-1]
		}
		v.analysis.EnrichPolicies = append(v.analysis.EnrichPolicies, name)
		return
	case "completionCommand", "rerankCommand":
		v.analysis.Inference = true
	case "matchBooleanExpression":
		v.analysis.FullText = true
		v.analysis.FullTextFields = append(v.analysis.FullTextFields, v.fieldName(v.child(c, "qualifiedName")))
	case "functionName":
		name := v.identifier(v.child(c, "identifierOrParameter"))
		switch strings.ToLower(name) {
		case "match", "match_phrase", "multi_match", "qstr", "kql":
			v.analysis.FullText = true
			field := "*"
			if name = strings.ToLower(name); name == "match" || name == "match_phrase" {
				if parent, ok := c.GetParent().(antlr.ParserRuleContext); ok {
					field = v.operandField(v.child(parent, "booleanExpression"))
				}
			}
			v.analysis.FullTextFields = append(v.analysis.FullTextFields, field)
		}
	case "field":
		if left := v.child(c, "qualifiedName"); left != nil {
			name := v.fieldName(left)
			if right := v.child(c, "booleanExpression"); right != nil {
				v.aliases[name] = append(v.aliases[name], v.referencedFields(right)...)
			}
		}
	case "renameClause":
		var names []string
		assignment := false
		for _, t := range c.GetChildren() {
			if r, ok := t.(antlr.ParserRuleContext); ok {
				names = append(names, v.patternName(r))
			} else if t.(antlr.TerminalNode).GetText() == "=" {
				assignment = true
			}
		}
		if len(names) != 2 {
			stop("invalid_request")
		}
		old, next := names[0], names[1]
		if assignment {
			old, next = next, old
		}
		if old == "*" || next == "*" {
			v.aliases["*"] = []string{"*"}
		} else {
			v.aliases[next] = append(v.aliases[next], old)
		}
	}
	for _, t := range c.GetChildren() {
		if r, ok := t.(antlr.ParserRuleContext); ok {
			v.walk(r)
		}
	}
}

// Track possible field lineage without changing expression semantics. Keep all
// alias definitions across branches; an alias can refer to an earlier field of
// the same name. Native analysis still decides which expressions are valid.
func (v *sourceVisitor) fieldName(c antlr.ParserRuleContext) string {
	if c == nil {
		return "*"
	}
	var parts []string
	for _, t := range c.GetChildren() {
		if r, ok := t.(antlr.ParserRuleContext); ok {
			parts = append(parts, v.identifier(r))
		}
	}
	if len(parts) == 0 {
		return "*"
	}
	return strings.Join(parts, ".")
}
func (v *sourceVisitor) patternName(c antlr.ParserRuleContext) string {
	var parts []string
	for _, t := range c.GetChildren() {
		if r, ok := t.(antlr.ParserRuleContext); ok {
			s := r.GetText()
			if strings.Contains(s, "*") {
				return "*"
			}
			if strings.HasPrefix(s, "?") {
				if param, ok := v.parameters[r.GetStart().GetTokenIndex()]; ok && param.kind == "PATTERN" {
					return "*"
				}
				parts = append(parts, v.identifier(r))
				continue
			}
			var part strings.Builder
			quoted := false
			for i := 0; i < len(s); i++ {
				if s[i] != '`' {
					part.WriteByte(s[i])
					continue
				}
				if quoted && i+1 < len(s) && s[i+1] == '`' {
					part.WriteByte('`')
					i++
					continue
				}
				quoted = !quoted
			}
			if quoted {
				stop("invalid_request")
			}
			parts = append(parts, part.String())
		}
	}
	return strings.Join(parts, ".")
}
func (v *sourceVisitor) operandField(c antlr.ParserRuleContext) string {
	for c != nil {
		v.b.step()
		if v.name(c) == "qualifiedName" {
			return v.fieldName(c)
		}
		if v.name(c) == "parameter" {
			if param, ok := v.parameters[c.GetStart().GetTokenIndex()]; ok && param.kind == "IDENTIFIER" {
				return v.identifier(c)
			}
			return "*"
		}
		var next antlr.ParserRuleContext
		for _, t := range c.GetChildren() {
			if r, ok := t.(antlr.ParserRuleContext); ok && v.name(r) != "dataType" {
				if next != nil {
					return "*"
				}
				next = r
			}
		}
		c = next
	}
	return "*"
}
func (v *sourceVisitor) referencedFields(c antlr.ParserRuleContext) []string {
	// Append into one bounded result; copying a growing child slice at each
	// ancestor would make long left-associative expressions quadratic.
	var fields []string
	var collect func(antlr.ParserRuleContext, int)
	collect = func(node antlr.ParserRuleContext, depth int) {
		v.b.step()
		if depth+v.depth > v.maxDepth {
			stop("resource_limit")
		}
		if v.name(node) == "qualifiedName" {
			fields = append(fields, v.fieldName(node))
			return
		}
		if v.name(node) == "parameter" {
			// A constant grammar alternative may bind to a native field identifier.
			param, ok := v.parameters[node.GetStart().GetTokenIndex()]
			if !ok {
				stop("invalid_request")
			}
			if param.kind == "IDENTIFIER" {
				fields = append(fields, v.identifier(node))
			}
			if param.kind == "PATTERN" {
				fields = append(fields, "*")
			}
			return
		}
		for _, t := range node.GetChildren() {
			if r, ok := t.(antlr.ParserRuleContext); ok {
				collect(r, depth+1)
			}
		}
	}
	collect(c, 0)
	return fields
}
func (v *sourceVisitor) expandFields() {
	if !v.analysis.FullText {
		return
	}
	if _, unknown := v.aliases["*"]; unknown {
		v.analysis.FullTextFields = []string{"*"}
		return
	}
	queue := v.analysis.FullTextFields
	seen := map[string]bool{}
	for i := 0; i < len(queue); i++ {
		v.b.step()
		field := queue[i]
		if seen[field] {
			continue
		}
		seen[field] = true
		if field == "*" {
			v.analysis.FullTextFields = []string{"*"}
			return
		}
		queue = append(queue, v.aliases[field]...)
	}
	v.analysis.FullTextFields = nil
	for field := range seen {
		v.analysis.FullTextFields = append(v.analysis.FullTextFields, field)
	}
}
func (v *sourceVisitor) identifier(c antlr.ParserRuleContext) string {
	if c == nil {
		stop("invalid_request")
	}
	s := c.GetText()
	if strings.HasPrefix(s, "?") {
		p, ok := v.parameters[c.GetStart().GetTokenIndex()]
		if !ok || p.kind == "PATTERN" || (!strings.HasPrefix(s, "??") && p.kind != "IDENTIFIER") {
			stop("invalid_request")
		}
		name, ok := p.value.(string)
		if !ok {
			stop("invalid_request")
		}
		return name
	}
	if strings.HasPrefix(s, "`") {
		return strings.ReplaceAll(s[1:len(s)-1], "``", "`")
	}
	return s
}
func unquote(s string) string {
	if strings.HasPrefix(s, `"""`) {
		return s[3 : len(s)-3]
	}
	if strings.HasPrefix(s, `"`) {
		var value string
		if json.Unmarshal([]byte(s), &value) != nil {
			stop("invalid_request")
		}
		return value
	}
	return s
}
