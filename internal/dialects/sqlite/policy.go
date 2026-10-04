package sqlite

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-sqlite3"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

// The authorizer is the execution-time boundary. The lexical check only
// enforces one statement and a read-shaped entry point before preparing it.
func validateSQL(query string, maxBytes int) (*core.Validation, error) {
	if len(query) == 0 || len(query) > maxBytes || !utf8.ValidString(query) || strings.IndexByte(query, 0) >= 0 {
		return nil, errors.New("invalid_request: SQLite SQL is empty, invalid or too large")
	}
	var code strings.Builder
	code.Grow(len(query))
	state := byte(0)
	semi := -1
	for i := 0; i < len(query); i++ {
		c := query[i]
		next := byte(0)
		if i+1 < len(query) {
			next = query[i+1]
		}
		switch state {
		case 0:
			switch {
			case c == '-' && next == '-':
				state = 'l'
				code.WriteByte(' ')
				i++
			case c == '/' && next == '*':
				state = 'b'
				code.WriteByte(' ')
				i++
			case c == '\'', c == '"', c == '`':
				state = c
				code.WriteByte(' ')
			case c == '[':
				state = ']'
				code.WriteByte(' ')
			case c == ';':
				if semi >= 0 {
					return nil, errors.New("invalid_request: SQLite requires one statement")
				}
				semi = code.Len()
				code.WriteByte(';')
			default:
				code.WriteByte(c)
			}
		case 'l':
			if c == '\n' {
				state = 0
				code.WriteByte('\n')
			}
		case 'b':
			if c == '*' && next == '/' {
				state = 0
				i++
				code.WriteByte(' ')
			}
		case ']':
			if c == ']' {
				state = 0
			}
		default:
			if c == state {
				if next == state {
					i++
				} else {
					state = 0
				}
			}
		}
	}
	if state != 0 && state != 'l' {
		return nil, errors.New("invalid_request: unterminated SQLite quote or comment")
	}
	clean := code.String()
	if semi >= 0 {
		before, after := clean[:semi], clean[semi+1:]
		if strings.TrimSpace(after) != "" {
			return nil, errors.New("invalid_request: SQLite requires one statement")
		}
		clean = before
	}
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return nil, errors.New("invalid_request: SQLite SQL is empty")
	}
	upper := strings.ToUpper(clean)
	readStart := false
	for _, keyword := range []string{"SELECT", "WITH", "VALUES"} {
		if strings.HasPrefix(upper, keyword) && len(upper) > len(keyword) {
			c := upper[len(keyword)]
			if !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '_' {
				readStart = true
			}
		}
	}
	if !readStart {
		return nil, errors.New("permission_denied: SQLite query must start with SELECT, WITH or VALUES")
	}
	h := sha256.Sum256([]byte(query))
	return &core.Validation{Fingerprint: hex.EncodeToString(h[:12])}, nil
}

func authorize(action int, arg1, arg2, database string) int {
	switch action {
	case sqlite3.SQLITE_SELECT:
		return sqlite3.SQLITE_OK
	case 33: // SQLITE_RECURSIVE; omitted from go-sqlite3's exported constants.
		return sqlite3.SQLITE_OK
	case sqlite3.SQLITE_TRANSACTION:
		if arg1 == "BEGIN" || arg1 == "ROLLBACK" {
			return sqlite3.SQLITE_OK
		}
	case sqlite3.SQLITE_READ:
		// COUNT(*) reports an empty database and column name. ATTACH and
		// creation of TEMP tables are denied below, so this still reads main.
		if database == "main" || database == "" {
			return sqlite3.SQLITE_OK
		}
	case sqlite3.SQLITE_FUNCTION:
		name := strings.ToLower(arg2)
		if name == "load_extension" || name == "readfile" || name == "writefile" || name == "fts3_tokenizer" || strings.HasPrefix(name, "pragma_") {
			return sqlite3.SQLITE_DENY
		}
		return sqlite3.SQLITE_OK
	case sqlite3.SQLITE_PRAGMA:
		name := strings.ToLower(arg1)
		if name == "table_xinfo" || name == "index_list" || name == "index_xinfo" {
			return sqlite3.SQLITE_OK
		}
	}
	return sqlite3.SQLITE_DENY
}
