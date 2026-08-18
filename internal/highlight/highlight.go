// Package highlight compiles and applies portable log highlighting rules.
package highlight

import (
	"fmt"
	"regexp"
	"sort"
	"unicode/utf8"

	"github.com/dostrow/e9s/internal/model"
)

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
