// Package highlight compiles and applies portable log highlighting rules.
package highlight

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dostrow/e9s/internal/model"
)

// CorrelationRules builds temporary rules that keep the originating search
// visible in an unfiltered correlation window. When a CloudWatch expression
// has no usable literal, the selected event's first non-empty line is used.
func CorrelationRules(pattern, message string) []model.LogHighlightRule {
	pattern = strings.TrimSpace(pattern)
	candidates := correlationCandidates(pattern)
	seen := make(map[string]struct{}, len(candidates))
	rules := make([]model.LogHighlightRule, 0, len(candidates))
	messageLower := strings.ToLower(message)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		key := strings.ToLower(candidate)
		if candidate == "" || !strings.Contains(messageLower, key) {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		rules = append(rules, model.LogHighlightRule{
			Pattern: candidate,
			Match:   model.LogHighlightLiteralCI,
			Style:   model.LogHighlightError,
		})
	}
	if len(rules) > 0 {
		return rules
	}
	for _, line := range strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return []model.LogHighlightRule{{
				Pattern: line,
				Match:   model.LogHighlightLiteral,
				Style:   model.LogHighlightDefault,
			}}
		}
	}
	return nil
}

func correlationCandidates(pattern string) []string {
	if pattern == "" {
		return nil
	}
	if pattern[0] != '{' && pattern[0] != '[' {
		if len(pattern) >= 2 && pattern[0] == '"' && pattern[len(pattern)-1] == '"' {
			if value, err := strconv.Unquote(pattern); err == nil {
				return []string{value}
			}
		}
		return []string{strings.ReplaceAll(pattern, `"`, "")}
	}

	var candidates []string
	for start := strings.IndexByte(pattern, '"'); start >= 0; {
		end := start + 1
		escaped := false
		for end < len(pattern) {
			char := pattern[end]
			if char == '"' && !escaped {
				break
			}
			if char == '\\' {
				escaped = !escaped
			} else {
				escaped = false
			}
			end++
		}
		if end >= len(pattern) {
			break
		}
		raw := pattern[start : end+1]
		if value, err := strconv.Unquote(raw); err == nil {
			candidates = append(candidates, value)
		}
		next := end + 1
		if offset := strings.IndexByte(pattern[next:], '"'); offset >= 0 {
			start = next + offset
		} else {
			break
		}
	}
	return candidates
}

type compiledRule struct {
	style model.LogHighlightStyle
	re    *regexp.Regexp
}

// Matcher is an immutable, reusable set of compiled highlight rules.
type Matcher struct {
	rules []compiledRule
}

// Compile validates and compiles highlight rules. Empty patterns and unknown
// match or style values are rejected before anything is rendered or saved.
func Compile(rules []model.LogHighlightRule) (*Matcher, error) {
	compiled := make([]compiledRule, 0, len(rules))
	for i, rule := range rules {
		if rule.Pattern == "" {
			return nil, fmt.Errorf("highlight rule %d: pattern cannot be empty", i+1)
		}
		if !validStyle(rule.Style) {
			return nil, fmt.Errorf("highlight rule %d: unknown style %q", i+1, rule.Style)
		}

		pattern := rule.Pattern
		switch rule.Match {
		case model.LogHighlightLiteral:
			pattern = regexp.QuoteMeta(pattern)
		case model.LogHighlightLiteralCI:
			pattern = "(?i:" + regexp.QuoteMeta(pattern) + ")"
		case model.LogHighlightRegex:
		default:
			return nil, fmt.Errorf("highlight rule %d: unknown match type %q", i+1, rule.Match)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("highlight rule %d: invalid regular expression: %w", i+1, err)
		}
		compiled = append(compiled, compiledRule{style: rule.Style, re: re})
	}
	return &Matcher{rules: compiled}, nil
}

func validStyle(style model.LogHighlightStyle) bool {
	switch style {
	case model.LogHighlightDefault, model.LogHighlightInfo, model.LogHighlightSuccess,
		model.LogHighlightWarning, model.LogHighlightError:
		return true
	default:
		return false
	}
}

// Spans returns non-overlapping rune ranges ordered by their position. Rules
// are evaluated in configuration order; a match touching an earlier match is
// ignored so the earlier rule wins predictably.
func (m *Matcher) Spans(text string) []model.LogHighlightSpan {
	if m == nil || len(m.rules) == 0 || text == "" {
		return nil
	}
	claimed := make([]bool, utf8.RuneCountInString(text))
	spans := make([]model.LogHighlightSpan, 0)
	for _, rule := range m.rules {
		for _, match := range rule.re.FindAllStringIndex(text, -1) {
			if match[0] == match[1] {
				continue
			}
			start := utf8.RuneCountInString(text[:match[0]])
			end := start + utf8.RuneCountInString(text[match[0]:match[1]])
			if overlaps(claimed, start, end) {
				continue
			}
			for i := start; i < end; i++ {
				claimed[i] = true
			}
			spans = append(spans, model.LogHighlightSpan{Start: start, End: end, Style: rule.style})
		}
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
	return spans
}

func overlaps(claimed []bool, start, end int) bool {
	for i := start; i < end; i++ {
		if claimed[i] {
			return true
		}
	}
	return false
}
