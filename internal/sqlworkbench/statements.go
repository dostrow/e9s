package sqlworkbench

import (
	"fmt"
	"strings"
	"unicode"
)

type Statement struct {
	SQL   string
	Start int
	End   int
	// Heading is the text of the psql \echo commands just before the
	// statement, which name the result that follows them.
	Heading string
}

// SplitStatements returns the SQL statements in source. psql meta-command
// lines separate statements but are not returned; ParseScript interprets
// them.
func SplitStatements(source string) []Statement {
	var statements []Statement
	for _, segment := range scanSegments(source) {
		if !segment.meta {
			appendStatement(&statements, source, segment.start, segment.end)
		}
	}
	return statements
}

func CurrentStatement(source string, cursor int) (Statement, bool) {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(source) {
		cursor = len(source)
	}
	statements := SplitStatements(source)
	for _, statement := range statements {
		if cursor >= statement.Start && cursor <= statement.End {
			return statement, true
		}
	}
	if len(statements) > 0 {
		return statements[len(statements)-1], true
	}
	return Statement{}, false
}

func ValidateReadOnly(source string) error {
	statements := SplitStatements(source)
	if len(statements) == 0 {
		return fmt.Errorf("query is empty")
	}
	for _, statement := range statements {
		words := SQLWords(statement.SQL)
		if len(words) == 0 {
			return fmt.Errorf("query contains an empty statement")
		}
		first := words[0]
		if containsWord(words, "set_config") {
			// set_config can turn off the session's server-side read-only
			// default that enforces a locked tab.
			return fmt.Errorf("set_config changes session settings and requires the per-tab write break-glass control")
		}
		switch first {
		case "select", "with", "show", "values", "table", "explain":
		case "rollback", "abort":
			// Ending a transaction never changes data, and a locked tab must
			// still be able to discard one. Prepared transactions belong to
			// the whole server and stay behind the write guard.
			if containsWord(words[1:], "prepared") {
				return fmt.Errorf("ROLLBACK PREPARED statements require the per-tab write break-glass control")
			}
		default:
			return fmt.Errorf("%s statements require the per-tab write break-glass control", strings.ToUpper(first))
		}
		switch first {
		case "select":
			if containsWord(words[1:], "into") {
				return fmt.Errorf("SELECT INTO is not allowed in read-only mode")
			}
		case "with":
			for _, word := range words[1:] {
				if _, forbidden := cteWriteKeywords[word]; forbidden {
					return fmt.Errorf("%s is not allowed in a read-only CTE", strings.ToUpper(word))
				}
			}
			if containsWord(words[1:], "into") {
				return fmt.Errorf("SELECT INTO is not allowed in read-only mode")
			}
		case "explain":
			if containsWord(words[1:], "analyze") || containsWord(words[1:], "analyse") {
				return fmt.Errorf("EXPLAIN ANALYZE executes the statement and requires the per-tab write break-glass control")
			}
			for _, word := range words[1:] {
				if _, forbidden := cteWriteKeywords[word]; forbidden {
					return fmt.Errorf("EXPLAIN of %s is not allowed in read-only mode", strings.ToUpper(word))
				}
			}
		}
	}
	return nil
}

var cteWriteKeywords = map[string]struct{}{
	"insert": {}, "update": {}, "delete": {}, "merge": {},
}

func containsWord(words []string, match string) bool {
	for _, word := range words {
		if word == match {
			return true
		}
	}
	return false
}

func SQLWords(source string) []string {
	clean := sqlCodeOnly(source)
	return strings.FieldsFunc(strings.ToLower(clean), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' })
}

// segment is a SQL statement (without its semicolon) or a psql
// meta-command line.
type segment struct {
	start, end int
	meta       bool
	// inStatement marks a meta-command that follows unterminated SQL, as
	// in "SELECT 1 \gset".
	inStatement bool
}

func scanSegments(source string) []segment {
	var segments []segment
	start := 0
	content := false // SQL other than whitespace and comments since start
	state, blockDepth := byte(0), 0
	dollarTag := ""
	for index := 0; index < len(source); index++ {
		character := source[index]
		switch state {
		case 's':
			if character == '\'' {
				if index+1 < len(source) && source[index+1] == '\'' {
					index++
				} else {
					state = 0
				}
			}
		case 'e':
			index, state = escapeStringStep(source, index, state)
		case 'd':
			if character == '"' {
				if index+1 < len(source) && source[index+1] == '"' {
					index++
				} else {
					state = 0
				}
			}
		case 'l':
			if character == '\n' {
				state = 0
			}
		case 'b':
			if character == '/' && index+1 < len(source) && source[index+1] == '*' {
				blockDepth++
				index++
			} else if character == '*' && index+1 < len(source) && source[index+1] == '/' {
				blockDepth--
				index++
				if blockDepth == 0 {
					state = 0
				}
			}
		case '$':
			if strings.HasPrefix(source[index:], dollarTag) {
				index += len(dollarTag) - 1
				state = 0
			}
		default:
			switch {
			case character == '\'':
				state, content = quoteState(source, index), true
			case character == '"':
				state, content = 'd', true
			case character == '-' && index+1 < len(source) && source[index+1] == '-':
				state = 'l'
				index++
			case character == '/' && index+1 < len(source) && source[index+1] == '*':
				state, blockDepth = 'b', 1
				index++
			case character == '$':
				content = true
				if tag := dollarQuoteTag(source, index); tag != "" {
					state, dollarTag = '$', tag
					index += len(tag) - 1
				}
			case character == ';':
				segments = append(segments, segment{start: start, end: index})
				start, content = index+1, false
			case character == '\\':
				// A psql meta-command runs to the end of its line.
				end := strings.IndexByte(source[index:], '\n')
				if end < 0 {
					end = len(source)
				} else {
					end += index
				}
				segments = append(segments, segment{start: start, end: index},
					segment{start: index, end: end, meta: true, inStatement: content})
				start, content = end, false
				index = end - 1
			case !unicode.IsSpace(rune(character)):
				content = true
			}
		}
	}
	return append(segments, segment{start: start, end: len(source)})
}

func sqlCodeOnly(source string) string {
	result := []byte(source)
	for index := range result {
		if result[index] != '\n' {
			result[index] = ' '
		}
	}
	state, blockDepth, dollarTag := byte(0), 0, ""
	for index := 0; index < len(source); index++ {
		character := source[index]
		if state == 0 {
			switch {
			case character == '\'':
				state = quoteState(source, index)
			case character == '"':
				state = 'd'
			case character == '-' && index+1 < len(source) && source[index+1] == '-':
				state = 'l'
				index++
			case character == '/' && index+1 < len(source) && source[index+1] == '*':
				state, blockDepth = 'b', 1
				index++
			case character == '$':
				if tag := dollarQuoteTag(source, index); tag != "" {
					state, dollarTag = '$', tag
					index += len(tag) - 1
				} else {
					result[index] = character
				}
			default:
				result[index] = character
			}
			continue
		}
		switch state {
		case 's':
			if character == '\'' {
				if index+1 < len(source) && source[index+1] == '\'' {
					index++
				} else {
					state = 0
				}
			}
		case 'e':
			index, state = escapeStringStep(source, index, state)
		case 'd':
			if character == '"' {
				if index+1 < len(source) && source[index+1] == '"' {
					index++
				} else {
					state = 0
				}
			}
		case 'l':
			if character == '\n' {
				state = 0
				result[index] = '\n'
			}
		case 'b':
			if character == '/' && index+1 < len(source) && source[index+1] == '*' {
				blockDepth++
				index++
			} else if character == '*' && index+1 < len(source) && source[index+1] == '/' {
				blockDepth--
				index++
				if blockDepth == 0 {
					state = 0
				}
			}
		case '$':
			if strings.HasPrefix(source[index:], dollarTag) {
				index += len(dollarTag) - 1
				state = 0
			}
		}
	}
	return string(result)
}

// isIdentifierByte reports a byte that can continue an identifier. Bytes of
// multi-byte UTF-8 characters count, as PostgreSQL allows letters beyond
// ASCII.
func isIdentifierByte(character byte) bool {
	return character == '_' || character == '$' || character >= 0x80 ||
		character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

// quoteState is the scanner state a quote at index opens: 'e' for an
// escape string (E'...'), where backslash escapes the next character, or
// 's' for a standard string.
func quoteState(source string, index int) byte {
	if index > 0 && (source[index-1] == 'E' || source[index-1] == 'e') && (index < 2 || !isIdentifierByte(source[index-2])) {
		return 'e'
	}
	return 's'
}

// escapeStringStep advances through an escape string at index.
func escapeStringStep(source string, index int, state byte) (int, byte) {
	switch source[index] {
	case '\\':
		return index + 1, state
	case '\'':
		if index+1 < len(source) && source[index+1] == '\'' {
			return index + 1, state
		}
		return index, 0
	}
	return index, state
}

// dollarQuoteTag returns the tag of a dollar quote opening at index. A $
// that continues an identifier, as in a$b, does not open one.
func dollarQuoteTag(source string, index int) string {
	if index > 0 && isIdentifierByte(source[index-1]) {
		return ""
	}
	return readDollarTag(source[index:])
}

func readDollarTag(source string) string {
	if len(source) == 0 || source[0] != '$' {
		return ""
	}
	for index := 1; index < len(source); index++ {
		character := source[index]
		if character == '$' {
			return source[:index+1]
		}
		if !(character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || index > 1 && character >= '0' && character <= '9') {
			return ""
		}
	}
	return ""
}

func appendStatement(statements *[]Statement, source string, start, end int) {
	raw := source[start:end]
	if strings.TrimSpace(sqlCodeOnly(raw)) == "" {
		// Only whitespace and comments: there is nothing to run.
		return
	}
	left := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
	right := len(raw) - len(strings.TrimRightFunc(raw, unicode.IsSpace))
	trimmedStart, trimmedEnd := start+left, end-right
	if trimmedStart >= trimmedEnd {
		return
	}
	*statements = append(*statements, Statement{SQL: source[trimmedStart:trimmedEnd], Start: trimmedStart, End: trimmedEnd})
}

// IsTransactionControl reports whether a statement starts, ends or
// subdivides a transaction.
func IsTransactionControl(sql string) bool {
	words := SQLWords(sql)
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "begin", "commit", "end", "rollback", "abort", "savepoint", "release":
		return true
	case "start":
		return len(words) > 1 && words[1] == "transaction"
	case "prepare":
		return len(words) > 1 && words[1] == "transaction"
	}
	return false
}

// StatementLine returns the 1-based line on which byte offset falls.
func StatementLine(source string, offset int) int {
	offset = max(0, min(offset, len(source)))
	return strings.Count(source[:offset], "\n") + 1
}
