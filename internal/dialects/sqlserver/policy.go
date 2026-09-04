package sqlserver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Policy is the fast, database-independent policy layer. Production execution
// also requires a successful SHOWPLAN_XML proof on SQL Server before the
// original query is run. Keeping those layers separate lets this scanner reject
// obvious mutations without pretending to be a complete T-SQL compiler.
type Policy struct {
	database       string
	defaultSchema  string
	allowedSchemas map[string]struct{}
	deniedTables   map[string]struct{}
	maxSQLBytes    int
}

func NewPolicy(database, defaultSchema string, allowedSchemas, deniedTables []string, maxSQLBytes int) *Policy {
	return &Policy{
		database:       strings.ToLower(database),
		defaultSchema:  strings.ToLower(defaultSchema),
		allowedSchemas: lowerSet(allowedSchemas),
		deniedTables:   lowerSet(deniedTables),
		maxSQLBytes:    maxSQLBytes,
	}
}

var forbiddenWords = stringSet(
	"alter", "backup", "bulk", "commit", "create", "dbcc", "delete",
	"deny", "disable", "drop", "enable", "execute", "exec", "grant",
	"insert", "kill", "merge", "reconfigure", "restore", "revert",
	"revoke", "rollback", "save", "send", "set", "shutdown", "truncate",
	"update", "use", "waitfor",
)

var externalRowsets = stringSet(
	"openrowset", "opendatasource", "openquery", "openxml",
	"sp_execute_external_script", "sp_invoke_external_rest_endpoint",
)

var volatileWords = stringSet(
	"current_timestamp", "getdate", "getutcdate", "newid", "newsequentialid",
	"rand", "sysdatetime", "sysdatetimeoffset", "sysutcdatetime",
)

func (p *Policy) Validate(query string, parameterCount int) (*core.Validation, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if len(trimmed) > p.maxSQLBytes {
		return nil, fmt.Errorf("query exceeds the configured SQL size limit")
	}
	tokens, err := lexTSQL(trimmed)
	if err != nil {
		return nil, err
	}
	for len(tokens) > 0 && tokens[len(tokens)-1].kind == tokenSemicolon {
		tokens = tokens[:len(tokens)-1]
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("query is empty")
	}
	for _, tok := range tokens {
		if tok.kind == tokenSemicolon {
			return nil, fmt.Errorf("exactly one SELECT statement is required")
		}
	}
	first := tokens[0]
	if first.kind != tokenWord || (first.lower != "select" && first.lower != "with") {
		return nil, fmt.Errorf("only SELECT statements are allowed")
	}

	params := make(map[int]struct{})
	cacheable := true
	for i, tok := range tokens {
		if tok.kind == tokenParameter {
			position, ok := parameterPosition(tok.lower)
			if !ok {
				return nil, fmt.Errorf("SQL Server parameters must use @p1, @p2, ...")
			}
			params[position] = struct{}{}
			continue
		}
		if tok.kind != tokenWord {
			continue
		}
		if _, forbidden := forbiddenWords[tok.lower]; forbidden {
			return nil, fmt.Errorf("T-SQL operation %q is not available to read-only queries", tok.lower)
		}
		if tok.lower == "into" {
			return nil, fmt.Errorf("SELECT INTO is not available to query_select")
		}
		if tok.lower == "next" && nextWords(tokens, i, "value", "for") {
			return nil, fmt.Errorf("NEXT VALUE FOR mutates persistent sequence state")
		}
		if _, external := externalRowsets[tok.lower]; external || strings.HasPrefix(tok.lower, "ai_generate_") {
			return nil, fmt.Errorf("external T-SQL capability %q is outside the selected target", tok.lower)
		}
		if _, volatile := volatileWords[tok.lower]; volatile || strings.HasPrefix(tok.lower, "@@") {
			cacheable = false
		}
	}
	if first.lower == "with" && !containsWord(tokens, "select") {
		return nil, fmt.Errorf("a CTE must terminate in a SELECT statement")
	}
	if err := validateParameterSet(params, parameterCount); err != nil {
		return nil, err
	}

	tables, err := p.tableReferences(tokens)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(trimmed))
	return &core.Validation{
		Fingerprint: hex.EncodeToString(sum[:12]),
		Tables:      tables,
		Cacheable:   cacheable,
	}, nil
}

func (p *Policy) tableReferences(tokens []token) ([]string, error) {
	ctes := cteNames(tokens)
	tables := make(map[string]struct{})
	for i := 0; i < len(tokens); i++ {
		if tokens[i].kind != tokenWord || (tokens[i].lower != "from" && tokens[i].lower != "join" && tokens[i].lower != "apply") {
			continue
		}
		j := i + 1
		for j < len(tokens) && tokens[j].kind == tokenPunctuation && tokens[j].text == "(" {
			// A parenthesized source is a subquery or built-in rowset. Its own
			// FROM/JOIN tokens are inspected by the outer scan.
			j++
		}
		if j >= len(tokens) || (tokens[j].kind != tokenWord && tokens[j].kind != tokenIdentifier) {
			continue
		}
		if tokens[j].kind == tokenWord && (tokens[j].lower == "select" || tokens[j].lower == "with" || tokens[j].lower == "values") {
			continue
		}
		parts := []string{tokens[j].lower}
		k := j + 1
		for k+1 < len(tokens) && tokens[k].kind == tokenPunctuation && tokens[k].text == "." && (tokens[k+1].kind == tokenWord || tokens[k+1].kind == tokenIdentifier) {
			parts = append(parts, tokens[k+1].lower)
			k += 2
		}
		if k < len(tokens) && tokens[k].kind == tokenPunctuation && tokens[k].text == "(" {
			// Table-valued functions are executable capabilities. SHOWPLAN and
			// effective permissions provide the mandatory semantic proof.
			if len(parts) == 2 {
				if err := p.allowSchema(parts[0]); err != nil {
					return nil, err
				}
			} else if len(parts) == 3 {
				if parts[0] != p.database {
					return nil, fmt.Errorf("database %q is outside the selected target", parts[0])
				}
				if err := p.allowSchema(parts[1]); err != nil {
					return nil, err
				}
			} else if len(parts) > 3 {
				return nil, fmt.Errorf("cross-database table-valued functions are outside the selected target")
			}
			continue
		}
		var schema, name string
		switch len(parts) {
		case 1:
			if _, cte := ctes[parts[0]]; cte {
				continue
			}
			schema, name = p.defaultSchema, parts[0]
		case 2:
			schema, name = parts[0], parts[1]
		case 3:
			if parts[0] != p.database {
				return nil, fmt.Errorf("database %q is outside the selected target", parts[0])
			}
			schema, name = parts[1], parts[2]
		default:
			return nil, fmt.Errorf("linked-server and four-part names are outside the selected target")
		}
		if err := p.allowSchema(schema); err != nil {
			return nil, err
		}
		qualified := schema + "." + name
		if _, denied := p.deniedTables[name]; denied {
			return nil, fmt.Errorf("table %q is denied by target policy", name)
		}
		if _, denied := p.deniedTables[qualified]; denied {
			return nil, fmt.Errorf("table %q is denied by target policy", qualified)
		}
		tables[qualified] = struct{}{}
	}
	result := make([]string, 0, len(tables))
	for table := range tables {
		result = append(result, table)
	}
	sort.Strings(result)
	return result, nil
}

func (p *Policy) allowSchema(schema string) error {
	if schema == "sys" || schema == "information_schema" {
		return fmt.Errorf("system schema %q is not available to free-form queries", schema)
	}
	if _, allowed := p.allowedSchemas[schema]; !allowed {
		return fmt.Errorf("schema %q is outside the selected target", schema)
	}
	return nil
}

func cteNames(tokens []token) map[string]struct{} {
	result := make(map[string]struct{})
	if len(tokens) < 4 || tokens[0].kind != tokenWord || tokens[0].lower != "with" {
		return result
	}
	depth := 0
	for i := 1; i+1 < len(tokens); i++ {
		tok := tokens[i]
		if tok.kind == tokenPunctuation {
			switch tok.text {
			case "(":
				depth++
			case ")":
				if depth > 0 {
					depth--
				}
			}
		}
		if depth != 0 || (tok.kind != tokenWord && tok.kind != tokenIdentifier) {
			continue
		}
		j := i + 1
		if j < len(tokens) && tokens[j].kind == tokenPunctuation && tokens[j].text == "(" {
			for j < len(tokens) && !(tokens[j].kind == tokenWord && tokens[j].lower == "as") {
				j++
			}
		}
		if j < len(tokens) && tokens[j].kind == tokenWord && tokens[j].lower == "as" {
			result[tok.lower] = struct{}{}
		}
	}
	return result
}

func validateParameterSet(params map[int]struct{}, count int) error {
	if count < 0 {
		return nil
	}
	if len(params) != count {
		return fmt.Errorf("SQL Server parameter count does not match placeholders")
	}
	for i := 1; i <= count; i++ {
		if _, ok := params[i]; !ok {
			return fmt.Errorf("SQL Server parameter placeholders must be contiguous")
		}
	}
	return nil
}

func parameterPosition(value string) (int, bool) {
	if len(value) < 3 || !strings.HasPrefix(value, "@p") {
		return 0, false
	}
	n := 0
	for _, r := range value[2:] {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
		if n > 1_000_000 {
			return 0, false
		}
	}
	return n, n > 0
}

func nextWords(tokens []token, at int, words ...string) bool {
	if at+len(words) >= len(tokens) {
		return false
	}
	for i, word := range words {
		tok := tokens[at+i+1]
		if tok.kind != tokenWord || tok.lower != word {
			return false
		}
	}
	return true
}

func containsWord(tokens []token, word string) bool {
	for _, tok := range tokens {
		if tok.kind == tokenWord && tok.lower == word {
			return true
		}
	}
	return false
}

type tokenKind uint8

const (
	tokenWord tokenKind = iota
	tokenIdentifier
	tokenParameter
	tokenLiteral
	tokenPunctuation
	tokenSemicolon
)

type token struct {
	kind  tokenKind
	text  string
	lower string
}

func lexTSQL(input string) ([]token, error) {
	result := make([]token, 0, len(input)/4)
	for i := 0; i < len(input); {
		r, size := utf8.DecodeRuneInString(input[i:])
		if r == utf8.RuneError && size == 1 {
			return nil, fmt.Errorf("query is not valid UTF-8")
		}
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		if i+1 < len(input) && input[i:i+2] == "--" {
			i += 2
			for i < len(input) && input[i] != '\n' && input[i] != '\r' {
				i++
			}
			continue
		}
		if i+1 < len(input) && input[i:i+2] == "/*" {
			start, depth := i, 1
			i += 2
			for i < len(input) && depth > 0 {
				switch {
				case i+1 < len(input) && input[i:i+2] == "/*":
					depth++
					i += 2
				case i+1 < len(input) && input[i:i+2] == "*/":
					depth--
					i += 2
				default:
					_, n := utf8.DecodeRuneInString(input[i:])
					i += n
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("unterminated block comment at byte %d", start)
			}
			continue
		}
		if input[i] == '\'' || ((input[i] == 'N' || input[i] == 'n') && i+1 < len(input) && input[i+1] == '\'') {
			start := i
			if input[i] != '\'' {
				i++
			}
			i++
			closed := false
			for i < len(input) {
				if input[i] != '\'' {
					_, n := utf8.DecodeRuneInString(input[i:])
					i += n
					continue
				}
				if i+1 < len(input) && input[i+1] == '\'' {
					i += 2
					continue
				}
				i++
				closed = true
				break
			}
			if !closed {
				return nil, fmt.Errorf("unterminated string literal at byte %d", start)
			}
			result = append(result, token{kind: tokenLiteral, text: input[start:i]})
			continue
		}
		if input[i] == '[' || input[i] == '"' {
			start, delimiter := i, input[i]
			closing := delimiter
			if delimiter == '[' {
				closing = ']'
			}
			i++
			var value strings.Builder
			closed := false
			for i < len(input) {
				if input[i] != closing {
					r, n := utf8.DecodeRuneInString(input[i:])
					value.WriteRune(r)
					i += n
					continue
				}
				if i+1 < len(input) && input[i+1] == closing {
					value.WriteByte(closing)
					i += 2
					continue
				}
				i++
				closed = true
				break
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quoted identifier at byte %d", start)
			}
			text := value.String()
			result = append(result, token{kind: tokenIdentifier, text: text, lower: strings.ToLower(text)})
			continue
		}
		if input[i] == '@' {
			start := i
			i++
			if i < len(input) && input[i] == '@' {
				i++
			}
			for i < len(input) {
				r, n := utf8.DecodeRuneInString(input[i:])
				if !isWordRune(r) {
					break
				}
				i += n
			}
			text := input[start:i]
			kind := tokenParameter
			if strings.HasPrefix(text, "@@") {
				kind = tokenWord
			}
			result = append(result, token{kind: kind, text: text, lower: strings.ToLower(text)})
			continue
		}
		if isWordRune(r) && !unicode.IsDigit(r) {
			start := i
			for i < len(input) {
				r, n := utf8.DecodeRuneInString(input[i:])
				if !isWordRune(r) {
					break
				}
				i += n
			}
			text := input[start:i]
			result = append(result, token{kind: tokenWord, text: text, lower: strings.ToLower(text)})
			continue
		}
		if unicode.IsDigit(r) {
			start := i
			for i < len(input) {
				r, n := utf8.DecodeRuneInString(input[i:])
				if !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == '.' || r == '_') {
					break
				}
				i += n
			}
			result = append(result, token{kind: tokenLiteral, text: input[start:i]})
			continue
		}
		if input[i] == ';' {
			result = append(result, token{kind: tokenSemicolon, text: ";"})
			i++
			continue
		}
		result = append(result, token{kind: tokenPunctuation, text: input[i : i+size]})
		i += size
	}
	return result, nil
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$' || r == '#' || r == '@'
}

func lowerSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[strings.ToLower(value)] = struct{}{}
	}
	return result
}

func stringSet(values ...string) map[string]struct{} { return lowerSet(values) }
