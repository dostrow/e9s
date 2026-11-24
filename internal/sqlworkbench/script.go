package sqlworkbench

import (
	"fmt"
	"strings"
	"unicode"
)

// Script is source parsed for running: its SQL statements, with psql
// \echo text attached as headings, and the psql settings e9s ignored.
type Script struct {
	Statements []Statement
	// Ignored lists psql commands that do not apply to e9s, as written.
	Ignored []string
	// TrailingHeading is \echo text after the last statement.
	TrailingHeading string
}

// supportedPsqlCommands is shown when a script uses any other command.
const supportedPsqlCommands = `\echo, \qecho, \set ON_ERROR_STOP on, \pset, \timing and \x`

// ParseScript splits source into statements and interprets the psql
// meta-commands that scripts written for psql commonly use. \echo and
// \qecho name the next result; \set ON_ERROR_STOP on, \pset, \timing and \x
// only affect psql's own behaviour or output and are ignored. Any other
// meta-command is refused before anything runs, because running the SQL
// without it would not do what the script intends.
func ParseScript(source string) (Script, error) {
	var script Script
	var heading []string
	for _, segment := range scanSegments(source) {
		if !segment.meta {
			before := len(script.Statements)
			appendStatement(&script.Statements, source, segment.start, segment.end)
			if len(script.Statements) > before && len(heading) > 0 {
				script.Statements[before].Heading = strings.Join(heading, "\n")
				heading = nil
			}
			continue
		}
		line := StatementLine(source, segment.start)
		text := strings.TrimSpace(source[segment.start:segment.end])
		name, arguments := splitMetaCommand(text)
		if segment.inStatement {
			return Script{}, fmt.Errorf("line %d: the psql command %s follows a statement that has no semicolon; e9s supports psql commands only between statements", line, name)
		}
		switch name {
		case `\echo`, `\qecho`:
			echo, err := psqlEchoText(arguments)
			if err != nil {
				return Script{}, fmt.Errorf("line %d: %s: %w", line, name, err)
			}
			heading = append(heading, echo)
		case `\set`:
			if err := checkPsqlSet(arguments); err != nil {
				return Script{}, fmt.Errorf("line %d: %w", line, err)
			}
			script.Ignored = append(script.Ignored, text)
		case `\pset`, `\timing`, `\x`:
			script.Ignored = append(script.Ignored, text)
		default:
			return Script{}, fmt.Errorf("line %d: e9s does not support the psql command %s; it supports %s. Run the script with psql, or remove the command",
				line, name, supportedPsqlCommands)
		}
	}
	script.TrailingHeading = strings.Join(heading, "\n")
	return script, nil
}

// IgnoredEvent describes the psql settings a run ignored, for its events.
func (s Script) IgnoredEvent() string {
	if len(s.Ignored) == 0 {
		return ""
	}
	return "Ignored psql settings that do not apply to e9s, which always stops at the first error: " + strings.Join(s.Ignored, "; ") + "."
}

func splitMetaCommand(text string) (string, string) {
	end := strings.IndexFunc(text, unicode.IsSpace)
	if end < 0 {
		return text, ""
	}
	return text[:end], strings.TrimSpace(text[end:])
}

// checkPsqlSet accepts \set ON_ERROR_STOP with a true value. e9s always
// stops at the first error, so turning that off cannot be honoured, and
// psql variables are not interpolated.
func checkPsqlSet(arguments string) error {
	fields := strings.Fields(arguments)
	if len(fields) == 0 || fields[0] != "ON_ERROR_STOP" {
		return fmt.Errorf(`psql variables are not supported; only \set ON_ERROR_STOP on is accepted`)
	}
	if len(fields) > 2 {
		return fmt.Errorf(`put each psql command on its own line`)
	}
	if len(fields) == 1 {
		return nil
	}
	switch strings.ToLower(fields[1]) {
	case "on", "1", "true", "yes":
		return nil
	case "off", "0", "false", "no":
		return fmt.Errorf("e9s always stops at the first error, so ON_ERROR_STOP cannot be turned off")
	}
	return fmt.Errorf("unrecognised ON_ERROR_STOP value %q", fields[1])
}

// psqlEchoText returns what psql's \echo would print: its arguments
// separated by spaces, with single-quoted arguments unquoted.
func psqlEchoText(arguments string) (string, error) {
	var words []string
	for index := 0; index < len(arguments); {
		switch character := arguments[index]; {
		case character == ' ' || character == '\t':
			index++
		case character == '`':
			return "", fmt.Errorf("backquoted shell commands are not supported")
		case character == '\\':
			return "", fmt.Errorf("put each psql command on its own line")
		default:
			word, next, err := psqlEchoWord(arguments, index)
			if err != nil {
				return "", err
			}
			words = append(words, word)
			index = next
		}
	}
	if len(words) > 0 && words[0] == "-n" {
		words = words[1:]
	}
	return strings.Join(words, " "), nil
}

// psqlEchoWord reads one argument starting at index, which may mix quoted
// and unquoted parts as psql allows.
func psqlEchoWord(arguments string, index int) (string, int, error) {
	var word strings.Builder
	for index < len(arguments) && arguments[index] != ' ' && arguments[index] != '\t' {
		if arguments[index] != '\'' {
			word.WriteByte(arguments[index])
			index++
			continue
		}
		index++
		for {
			if index >= len(arguments) {
				return "", 0, fmt.Errorf("unterminated quoted string")
			}
			character := arguments[index]
			switch {
			case character == '\'' && index+1 < len(arguments) && arguments[index+1] == '\'':
				word.WriteByte('\'')
				index += 2
				continue
			case character == '\'':
				index++
			case character == '\\' && index+1 < len(arguments):
				switch escaped := arguments[index+1]; escaped {
				case 'n':
					word.WriteByte('\n')
				case 't':
					word.WriteByte('\t')
				default:
					word.WriteByte(escaped)
				}
				index += 2
				continue
			default:
				word.WriteByte(character)
				index++
				continue
			}
			break
		}
	}
	return word.String(), index, nil
}
