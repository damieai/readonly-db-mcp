// Package sqlparser parses the complete version-pinned Elasticsearch SQL grammars.
package sqlparser

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/antlr4-go/antlr/v4"
	"github.com/your-org/readonly-db-mcp/internal/dialects/elasticsearch/languagebudget"
	v8 "github.com/your-org/readonly-db-mcp/internal/dialects/elasticsearch/sqlparser/generated/v8"
	v9 "github.com/your-org/readonly-db-mcp/internal/dialects/elasticsearch/sqlparser/generated/v9"
)

// MemoryLimit is a process-wide parser reservation, separately forecast from
// request JSON and retained cursors. DFA caches are per invocation, never global.
const MemoryLimit int64 = languagebudget.MemoryLimit

var memory = languagebudget.Memory

type Limits struct{ Bytes, Tokens, Depth int }
type Source struct{ Index, Catalog string }
type Catalog struct {
	Value   string
	Pattern bool
}
type Analysis struct {
	Kind           string
	Sources        []Source
	Catalogs       []Catalog
	Parameters     int
	FullTextFields []string
}
type Error struct{ Code string }

func (e *Error) Error() string {
	switch e.Code {
	case "deadline_exceeded":
		return "deadline_exceeded: SQL analysis exceeded request deadline"
	case "resource_limit":
		return "resource_limit: SQL analysis exceeds its byte, token, depth, work or memory budget"
	case "capability_unavailable":
		return "capability_unavailable: SQL grammar is not pinned for this version"
	default:
		return "invalid_request: SQL does not match the pinned single-statement grammar"
	}
}

type abort struct{ code string }

func stop(code string) { panic(abort{code}) }

type budget struct {
	ctx           context.Context
	work, maxWork int
	charge        *int64
}

func (b *budget) step() {
	if b.ctx.Err() != nil {
		stop("deadline_exceeded")
	}
	b.work++
	if b.work > b.maxWork {
		stop("resource_limit")
	}
}

type characters struct {
	antlr.CharStream
	b *budget
}

func (c *characters) LA(i int) int {
	c.b.step()
	r := c.CharStream.LA(i)
	if r > 0 {
		return int(unicode.ToUpper(rune(r)))
	}
	return r
}
func (c *characters) Consume() { c.b.step(); c.CharStream.Consume() }

type lexer struct {
	antlr.Lexer
	count, limit int
	b            *budget
}

func (l *lexer) NextToken() antlr.Token {
	l.b.step()
	if !memory.TryAcquire(4096) {
		stop("resource_limit")
	}
	*l.b.charge += 4096
	token := l.Lexer.NextToken()
	l.count++
	if l.count > l.limit {
		stop("resource_limit")
	}
	return token
}

type tokens struct {
	*antlr.CommonTokenStream
	b *budget
}

func (t *tokens) LA(i int) int         { t.b.step(); return t.CommonTokenStream.LA(i) }
func (t *tokens) LT(i int) antlr.Token { t.b.step(); return t.CommonTokenStream.LT(i) }
func (t *tokens) Consume()             { t.b.step(); t.CommonTokenStream.Consume() }

type errors struct{ *antlr.DefaultErrorListener }

func (*errors) SyntaxError(antlr.Recognizer, interface{}, int, int, string, antlr.RecognitionException) {
	stop("invalid_request")
}

type listener struct {
	*antlr.BaseParseTreeListener
	b               *budget
	depth, maxDepth int
}

func (l *listener) EnterEveryRule(antlr.ParserRuleContext) {
	l.b.step()
	l.depth++
	if l.depth > l.maxDepth {
		stop("resource_limit")
	}
}
func (l *listener) ExitEveryRule(antlr.ParserRuleContext) { l.depth-- }

func freshDFA(atn *antlr.ATN) []*antlr.DFA {
	result := make([]*antlr.DFA, len(atn.DecisionToState))
	for i, state := range atn.DecisionToState {
		result[i] = antlr.NewDFA(state, i)
	}
	return result
}

// Analyze neither rewrites the query nor substitutes parameters. Semantic
// function/type validation stays with the pinned native engine. Errors omit
// lexer excerpts, query text, literals and parser diagnostics.
func Analyze(ctx context.Context, version, query string, params []any, limits Limits) (result *Analysis, err error) {
	defer func() {
		if r := recover(); r != nil {
			if a, ok := r.(abort); ok {
				result = nil
				err = &Error{a.code}
			} else {
				panic(r)
			}
		}
	}()
	if version != "8.19.21" && version != "9.1.10" {
		return nil, &Error{"capability_unavailable"}
	}
	if query == "" || !utf8.ValidString(query) {
		return nil, &Error{"invalid_request"}
	}
	if limits.Bytes < 1 || limits.Bytes > 1<<20 || limits.Tokens < 1 || limits.Tokens > 100000 || limits.Depth < 1 || limits.Depth > 512 || len(query) > limits.Bytes {
		return nil, &Error{"resource_limit"}
	}
	if ctx.Err() != nil {
		return nil, &Error{"deadline_exceeded"}
	}
	charge := int64(64*len(query) + (2 << 20))
	if !memory.TryAcquire(charge) {
		return nil, &Error{"resource_limit"}
	}
	defer func() { memory.Release(charge) }()
	b := &budget{ctx: ctx, maxWork: 256*len(query) + 64*limits.Tokens + 4096, charge: &charge}
	input := &characters{CharStream: antlr.NewInputStream(query), b: b}

	var native antlr.Lexer
	if version == "8.19.21" {
		l := v8.NewSqlBaseLexer(input)
		l.Interpreter = antlr.NewLexerATNSimulator(l, l.GetATN(), freshDFA(l.GetATN()), antlr.NewPredictionContextCache())
		native = l
	} else {
		l := v9.NewSqlBaseLexer(input)
		l.Interpreter = antlr.NewLexerATNSimulator(l, l.GetATN(), freshDFA(l.GetATN()), antlr.NewPredictionContextCache())
		native = l
	}
	native.RemoveErrorListeners()
	native.AddErrorListener(&errors{})
	lex := &lexer{Lexer: native, limit: limits.Tokens, b: b}
	stream := &tokens{CommonTokenStream: antlr.NewCommonTokenStream(lex, antlr.TokenDefaultChannel), b: b}
	stream.Fill()
	depth, prefix := 0, 0
	parameters := map[int]any{}
	var previous antlr.Token
	for _, token := range stream.GetAllTokens() {
		b.step()
		// An unterminated block comment can otherwise fall back to adjacent
		// division/star tokens in this upstream catch-all lexer.
		if previous != nil && previous.GetText() == "/" && token.GetText() == "*" && previous.GetStop()+1 == token.GetStart() {
			stop("invalid_request")
		}
		previous = token
		if token.GetChannel() != antlr.TokenDefaultChannel {
			continue
		}
		text := strings.ToUpper(token.GetText())
		switch text {
		case "(", "{":
			depth++
			prefix = 0
		case ")", "}":
			depth--
			prefix = 0
		case "NOT", "-", "+":
			prefix++
		default:
			prefix = 0
		}
		if depth+prefix > limits.Depth {
			stop("resource_limit")
		}
		if text == "?" {
			if len(parameters) >= len(params) {
				stop("invalid_request")
			}
			parameters[token.GetTokenIndex()] = params[len(parameters)]
		}
	}
	if len(parameters) != len(params) {
		stop("invalid_request")
	}
	var parser interface {
		antlr.Parser
		AddParseListener(antlr.ParseTreeListener)
	}
	var parse func() antlr.ParserRuleContext
	if version == "8.19.21" {
		p := v8.NewSqlBaseParser(stream)
		p.Interpreter = antlr.NewParserATNSimulator(p, p.GetATN(), freshDFA(p.GetATN()), antlr.NewPredictionContextCache())
		parser, parse = p, func() antlr.ParserRuleContext { return p.SingleStatement() }
	} else {
		p := v9.NewSqlBaseParser(stream)
		p.Interpreter = antlr.NewParserATNSimulator(p, p.GetATN(), freshDFA(p.GetATN()), antlr.NewPredictionContextCache())
		parser, parse = p, func() antlr.ParserRuleContext { return p.SingleStatement() }
	}
	parser.RemoveErrorListeners()
	parser.AddErrorListener(&errors{})
	parser.AddParseListener(&listener{BaseParseTreeListener: &antlr.BaseParseTreeListener{}, b: b, maxDepth: 16*limits.Depth + 64})
	root := parse()
	if parser.HasError() {
		return nil, &Error{"invalid_request"}
	}
	result = &Analysis{Kind: "query", Parameters: len(parameters)}
	ast := &sourceVisitor{b: b, rules: parser.GetRuleNames(), parameters: parameters, analysis: result}
	ast.walk(root)
	b.step()
	return result, nil
}
