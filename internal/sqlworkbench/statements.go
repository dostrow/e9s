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
}

func SplitStatements(source string) []Statement {
	var statements []Statement
	start := 0
	for _, end := range statementBoundaries(source) {
		appendStatement(&statements, source, start, end)
		start = end + 1
	}
	appendStatement(&statements, source, start, len(source))
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
		switch first {
		case "select", "with", "show", "values", "table", "explain":
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
		case "explain":
			if containsWord(words[1:], "analyze") {
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

func statementBoundaries(source string) []int {
	var boundaries []int
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
				state = 's'
			case character == '"':
				state = 'd'
			case character == '-' && index+1 < len(source) && source[index+1] == '-':
				state = 'l'
				index++
			case character == '/' && index+1 < len(source) && source[index+1] == '*':
				state, blockDepth = 'b', 1
				index++
			case character == '$':
				if tag := readDollarTag(source[index:]); tag != "" {
					state, dollarTag = '$', tag
					index += len(tag) - 1
				}
			case character == ';':
				boundaries = append(boundaries, index)
			}
		}
	}
	return boundaries
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
				state = 's'
			case character == '"':
				state = 'd'
			case character == '-' && index+1 < len(source) && source[index+1] == '-':
				state = 'l'
				index++
			case character == '/' && index+1 < len(source) && source[index+1] == '*':
				state, blockDepth = 'b', 1
				index++
			case character == '$':
				if tag := readDollarTag(source[index:]); tag != "" {
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
	left := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
	right := len(raw) - len(strings.TrimRightFunc(raw, unicode.IsSpace))
	trimmedStart, trimmedEnd := start+left, end-right
	if trimmedStart >= trimmedEnd {
		return
	}
	*statements = append(*statements, Statement{SQL: source[trimmedStart:trimmedEnd], Start: trimmedStart, End: trimmedEnd})
}
