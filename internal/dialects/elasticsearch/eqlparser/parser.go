// Package eqlparser parses the complete pinned upstream EQL grammar. The
// generated grammar is separately licensed; see generated/NOTICE.md.
package eqlparser

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/antlr4-go/antlr/v4"
	"github.com/your-org/readonly-db-mcp/internal/dialects/elasticsearch/eqlparser/generated"
	"golang.org/x/sync/semaphore"
)

// MemoryLimit is a process-wide parser reservation, separately forecast from
// request JSON and retained cursors. DFA caches are per invocation, never global.
const MemoryLimit int64 = 128 << 20

var memory = semaphore.NewWeighted(MemoryLimit)

type Limits struct{ Bytes, Tokens, Depth int }
type Analysis struct {
	Kind string
	// EQL event categories/field names/functions are not index selectors. The
	// pinned grammar reads the enclosing REST indices; JSON filter/runtime
	// lookups are proved separately by the native DSL visitor.
	SourceMode   string
	EventFilters int
	MissingTerms int
}
type Error struct{ Code string }

func (e *Error) Error() string {
	switch e.Code {
	case "deadline_exceeded":
		return "deadline_exceeded: EQL analysis exceeded request deadline"
	case "resource_limit":
		return "resource_limit: EQL analysis exceeds its byte, token, depth, work or memory budget"
	case "capability_unavailable":
		return "capability_unavailable: EQL grammar is not pinned for this version"
	default:
		return "invalid_request: EQL does not match the pinned single-statement grammar"
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

func (c *characters) LA(i int) int { c.b.step(); return c.CharStream.LA(i) }
func (c *characters) Consume()     { c.b.step(); c.CharStream.Consume() }

type lexer struct {
	*generated.EqlBaseLexer
	count, limit int
	b            *budget
}

func (l *lexer) NextToken() antlr.Token {
	l.b.step()
	if !memory.TryAcquire(4096) {
		stop("resource_limit")
	}
	*l.b.charge += 4096
	token := l.EqlBaseLexer.NextToken()
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
	*generated.BaseEqlBaseListener
	b               *budget
	depth, maxDepth int
	analysis        *Analysis
}

func (l *listener) EnterEveryRule(antlr.ParserRuleContext) {
	l.b.step()
	l.depth++
	if l.depth > l.maxDepth {
		stop("resource_limit")
	}
}
func (l *listener) ExitEveryRule(antlr.ParserRuleContext)          { l.depth-- }
func (l *listener) EnterEventFilter(*generated.EventFilterContext) { l.analysis.EventFilters++ }
func (l *listener) ExitSubquery(c *generated.SubqueryContext) {
	if c.MISSING_EVENT_OPEN() != nil {
		l.analysis.MissingTerms++
	}
}

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
func Analyze(ctx context.Context, version, query string, limits Limits) (result *Analysis, err error) {
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
	native := generated.NewEqlBaseLexer(input)
	native.Interpreter = antlr.NewLexerATNSimulator(native, native.GetATN(), freshDFA(native.GetATN()), antlr.NewPredictionContextCache())
	native.RemoveErrorListeners()
	native.AddErrorListener(&errors{})
	lex := &lexer{EqlBaseLexer: native, limit: limits.Tokens, b: b}
	stream := &tokens{CommonTokenStream: antlr.NewCommonTokenStream(lex, antlr.TokenDefaultChannel), b: b}
	stream.Fill()
	// Mirror the upstream recursion guard, using tokens so comments and string
	// contents cannot count as operators/delimiters. Brackets also cover nested
	// process relationships; the listener provides a second rule-depth bound.
	depth, prefix := 0, 0
	for _, token := range stream.GetAllTokens() {
		b.step()
		if token.GetChannel() != antlr.TokenDefaultChannel {
			continue
		}
		switch token.GetTokenType() {
		case generated.EqlBaseLexerLP, generated.EqlBaseLexerLB, generated.EqlBaseLexerMISSING_EVENT_OPEN:
			depth++
			prefix = 0
		case generated.EqlBaseLexerRP, generated.EqlBaseLexerRB:
			depth--
			prefix = 0
		case generated.EqlBaseLexerNOT, generated.EqlBaseLexerMINUS, generated.EqlBaseLexerPLUS:
			prefix++
		default:
			prefix = 0
		}
		if depth+prefix > limits.Depth {
			stop("resource_limit")
		}
	}
	parser := generated.NewEqlBaseParser(stream)
	parser.Interpreter = antlr.NewParserATNSimulator(parser, parser.GetATN(), freshDFA(parser.GetATN()), antlr.NewPredictionContextCache())
	parser.RemoveErrorListeners()
	parser.AddErrorListener(&errors{})
	result = &Analysis{SourceMode: "request_indices"}
	parser.AddParseListener(&listener{BaseEqlBaseListener: &generated.BaseEqlBaseListener{}, b: b, maxDepth: 16*limits.Depth + 64, analysis: result})
	root := parser.SingleStatement()
	if parser.HasError() {
		return nil, &Error{"invalid_request"}
	}
	q := root.Statement().Query()
	switch {
	case q.Sequence() != nil:
		result.Kind = "sequence"
	case q.Sample() != nil:
		result.Kind = "sample"
	case q.Join() != nil:
		result.Kind = "join"
	case q.EventQuery() != nil:
		result.Kind = "event"
	default:
		return nil, fmt.Errorf("invalid_request: unrecognized EQL statement root")
	}
	b.step()
	return result, nil
}
